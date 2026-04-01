package scaffold_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudticon/ctts/internal/packages"
	"github.com/cloudticon/ctts/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSync_RegeneratesStdlibAndValues(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	require.NoError(t, scaffold.InitWith(dir, nop()))

	require.NoError(t, os.Remove(filepath.Join(dir, ".ctts", "types", "k8s", "resource.ts")))
	require.NoError(t, os.Remove(filepath.Join(dir, ".ctts", "types", "values.d.ts")))

	require.NoError(t, scaffold.SyncWith(dir, nop()))

	assert.FileExists(t, filepath.Join(dir, ".ctts", "types", "k8s", "resource.ts"))
	assert.FileExists(t, filepath.Join(dir, ".ctts", "types", "values.d.ts"))
}

func TestSync_UpdatesValuesDtsAfterValuesChange(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	require.NoError(t, scaffold.InitWith(dir, nop()))

	newValues := `export default {
  image: "nginx:1.25",
  replicas: 3,
  domain: "example.com",
};
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "values.ts"), []byte(newValues), 0o644))

	require.NoError(t, scaffold.SyncWith(dir, nop()))

	content, err := os.ReadFile(filepath.Join(dir, ".ctts", "types", "values.d.ts"))
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, "domain: string")
	assert.Contains(t, s, "image: string")
	assert.Contains(t, s, "replicas: number")
}

func TestSync_ErrorWhenNoCTts(t *testing.T) {
	dir := t.TempDir()

	err := scaffold.SyncWith(dir, nop())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ct.ts not found")
}

func TestSync_PreservesUserFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	require.NoError(t, scaffold.InitWith(dir, nop()))

	original, err := os.ReadFile(filepath.Join(dir, "ct.ts"))
	require.NoError(t, err)

	require.NoError(t, scaffold.SyncWith(dir, nop()))

	after, err := os.ReadFile(filepath.Join(dir, "ct.ts"))
	require.NoError(t, err)
	assert.Equal(t, original, after)
}

func TestSync_CreatesTypesDirsIfMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	require.NoError(t, scaffold.InitWith(dir, nop()))

	require.NoError(t, os.RemoveAll(filepath.Join(dir, ".ctts", "types", "k8s")))

	require.NoError(t, scaffold.SyncWith(dir, nop()))

	for _, sub := range []string{"apps", "core", "networking"} {
		assert.DirExists(t, filepath.Join(dir, ".ctts", "types", "k8s", sub))
	}
}

func TestSync_GeneratesTsconfigWithPackagePaths(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	require.NoError(t, scaffold.InitWith(dir, nop()))

	lf := packages.NewLockFile()
	lf.Packages["github.com/someone/ctts-webapp"] = packages.LockEntry{Ref: "v1.0.0", SHA: "abc123"}
	require.NoError(t, packages.WriteLock(filepath.Join(dir, "ct.lock"), lf))

	pkgDir := filepath.Join(dir, ".ctts", "packages", "github.com", "someone", "ctts-webapp")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "index.ts"), []byte(`export const x = 1;`), 0o644))

	require.NoError(t, scaffold.SyncWith(dir, nop()))

	content, err := os.ReadFile(filepath.Join(dir, "tsconfig.json"))
	require.NoError(t, err)

	var cfg map[string]interface{}
	require.NoError(t, json.Unmarshal(content, &cfg))

	compilerOpts := cfg["compilerOptions"].(map[string]interface{})
	paths := compilerOpts["paths"].(map[string]interface{})

	assert.Contains(t, paths, "ctts/*")
	assert.Contains(t, paths, "github.com/someone/ctts-webapp")
	assert.Contains(t, paths, "github.com/someone/ctts-webapp/*")

	include := cfg["include"].([]interface{})
	assert.Contains(t, include, ".ctts/packages/**/*.ts")
}

func TestSync_TsconfigWithoutPackages(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	require.NoError(t, scaffold.InitWith(dir, nop()))

	require.NoError(t, scaffold.SyncWith(dir, nop()))

	content, err := os.ReadFile(filepath.Join(dir, "tsconfig.json"))
	require.NoError(t, err)

	var cfg map[string]interface{}
	require.NoError(t, json.Unmarshal(content, &cfg))

	compilerOpts := cfg["compilerOptions"].(map[string]interface{})
	paths := compilerOpts["paths"].(map[string]interface{})

	assert.Contains(t, paths, "ctts/*")
	assert.Len(t, paths, 1)

	include := cfg["include"].([]interface{})
	assert.NotContains(t, include, ".ctts/packages/**/*.ts")
}
