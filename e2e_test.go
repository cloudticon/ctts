package ctts_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ctBinary string

func TestMain(m *testing.M) {
	flag.Parse()

	tmpDir, err := os.MkdirTemp("", "ct-e2e-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	ctBinary = filepath.Join(tmpDir, "ct"+ext)

	cmd := exec.Command("go", "build", "-o", ctBinary, "./cmd/ct")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %s\n%v\n", string(out), err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func runCT(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := exec.Command(ctBinary, args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func TestE2E_Version(t *testing.T) {
	stdout, _, err := runCT(t, "--version")
	require.NoError(t, err)
	assert.Contains(t, stdout, "ct version")
}

func TestE2E_InitCreatesProject(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "myproject")

	_, _, err := runCT(t, "init", "--dir", dir)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(dir, "ct.ts"))
	assert.FileExists(t, filepath.Join(dir, "values.ts"))
	assert.FileExists(t, filepath.Join(dir, "tsconfig.json"))
	assert.DirExists(t, filepath.Join(dir, ".ctts", "types"))
	assert.FileExists(t, filepath.Join(dir, ".ctts", "types", "values.d.ts"))
	assert.FileExists(t, filepath.Join(dir, ".ctts", "types", "k8s", "resource.ts"))
	assert.FileExists(t, filepath.Join(dir, ".ctts", "types", "k8s", "apps", "v1.ts"))
	assert.FileExists(t, filepath.Join(dir, ".ctts", "types", "k8s", "core", "v1.ts"))
	assert.FileExists(t, filepath.Join(dir, ".ctts", "types", "k8s", "networking", "v1.ts"))
}

func TestE2E_InitThenTemplate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ct")

	_, _, err := runCT(t, "init", "--dir", dir)
	require.NoError(t, err)

	stdout, _, err := runCT(t, "template", dir, "--namespace", "production")
	require.NoError(t, err)

	assert.Contains(t, stdout, "apiVersion: apps/v1")
	assert.Contains(t, stdout, "kind: Deployment")
	assert.Contains(t, stdout, "namespace: production")
	assert.Contains(t, stdout, "image: nginx:1.25")
	assert.Contains(t, stdout, "kind: Service")
}

func TestE2E_TemplateWithTestdata(t *testing.T) {
	cases := []struct {
		name      string
		namespace string
	}{
		{name: "simple", namespace: "default"},
		{name: "no_values", namespace: "default"},
		{name: "multi_resource", namespace: "production"},
		{name: "low_level_resource", namespace: "redis-ns"},
		{name: "conditional", namespace: "production"},
		{name: "loop", namespace: "default"},
		{name: "cross_ref", namespace: "default"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srcDir := filepath.Join("testdata", tc.name)
			expectedPath := filepath.Join(srcDir, "expected.yaml")

			expected, err := os.ReadFile(expectedPath)
			require.NoError(t, err)

			dir := prepareTestdataProject(t, srcDir)

			stdout, stderr, err := runCT(t, "template", dir, "--namespace", tc.namespace)
			require.NoError(t, err, "stderr: %s", stderr)

			assert.Equal(t, string(expected), stdout)
		})
	}
}

func prepareTestdataProject(t *testing.T, srcDir string) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "project")

	_, _, err := runCT(t, "init", "--dir", dir)
	require.NoError(t, err)

	ctTs, err := os.ReadFile(filepath.Join(srcDir, "ct.ts"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ct.ts"), ctTs, 0o644))

	valuesPath := filepath.Join(srcDir, "values.ts")
	if _, statErr := os.Stat(valuesPath); statErr == nil {
		valuesTs, readErr := os.ReadFile(valuesPath)
		require.NoError(t, readErr)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "values.ts"), valuesTs, 0o644))
	} else {
		os.Remove(filepath.Join(dir, "values.ts"))
	}

	return dir
}

func TestE2E_TemplateJSONOutput(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ct")

	_, _, err := runCT(t, "init", "--dir", dir)
	require.NoError(t, err)

	stdout, _, err := runCT(t, "template", dir, "--namespace", "default", "--output", "json")
	require.NoError(t, err)

	var resources []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &resources))
	require.GreaterOrEqual(t, len(resources), 1)
	assert.Equal(t, "apps/v1", resources[0]["apiVersion"])
}

func TestE2E_TemplateSetOverride(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ct")

	_, _, err := runCT(t, "init", "--dir", dir)
	require.NoError(t, err)

	stdout, _, err := runCT(t, "template", dir, "--namespace", "test", "--set", "replicas=10")
	require.NoError(t, err)

	assert.Contains(t, stdout, "replicas: 10")
}

func TestE2E_TemplateCustomValues(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ct")

	_, _, err := runCT(t, "init", "--dir", dir)
	require.NoError(t, err)

	customValues := filepath.Join(t.TempDir(), "custom-values.ts")
	require.NoError(t, os.WriteFile(customValues, []byte(`export default { image: "custom:2.0", replicas: 7 };`), 0o644))

	stdout, _, err := runCT(t, "template", dir, "--namespace", "staging", "--values", customValues)
	require.NoError(t, err)

	assert.Contains(t, stdout, "image: custom:2.0")
	assert.Contains(t, stdout, "replicas: 7")
}

func TestE2E_TemplateMissingEntryPoint(t *testing.T) {
	dir := t.TempDir()

	_, stderr, err := runCT(t, "template", dir)
	require.Error(t, err)
	assert.Contains(t, stderr, "entry point not found")
}

func TestE2E_TemplateMissingCttsDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ct.ts"), []byte(`// empty`), 0o644))

	_, stderr, err := runCT(t, "template", dir)
	require.Error(t, err)
	assert.Contains(t, stderr, "ct init")
}

func TestE2E_TemplateNoNamespace(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ct")

	_, _, err := runCT(t, "init", "--dir", dir)
	require.NoError(t, err)

	stdout, _, err := runCT(t, "template", dir)
	require.NoError(t, err)

	assert.Contains(t, stdout, "kind: Deployment")
	assert.NotContains(t, stdout, "namespace:")
}

func TestE2E_HelpOutput(t *testing.T) {
	stdout, _, err := runCT(t, "--help")
	require.NoError(t, err)
	assert.Contains(t, stdout, "ct")
	assert.Contains(t, stdout, "init")
	assert.Contains(t, stdout, "template")
}

func TestE2E_InitHelp(t *testing.T) {
	stdout, _, err := runCT(t, "init", "--help")
	require.NoError(t, err)
	assert.Contains(t, stdout, "ct init")
	assert.Contains(t, stdout, "--dir")
}

func TestE2E_TemplateHelp(t *testing.T) {
	stdout, _, err := runCT(t, "template", "--help")
	require.NoError(t, err)
	assert.Contains(t, stdout, "ct template")
	assert.Contains(t, stdout, "--namespace")
	assert.Contains(t, stdout, "--values")
	assert.Contains(t, stdout, "--output")
	assert.Contains(t, stdout, "--set")
}
