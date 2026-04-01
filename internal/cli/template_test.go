package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudticon/ctts/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupProject(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ct")
	require.NoError(t, scaffold.InitWith(dir, scaffold.NopPackageSyncer()))
	ctTs := `import { deployment } from "ctts/k8s/apps/v1";
import { service } from "ctts/k8s/core/v1";

const app = deployment({
  name: "web-app",
  image: Values.image,
  replicas: Values.replicas,
  ports: [{ containerPort: 8080 }],
});

service({
  name: "web-app-svc",
  selector: { app: app.metadata.name },
  ports: [{ port: 80, targetPort: 8080 }],
});
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ct.ts"), []byte(ctTs), 0o644))
	return dir
}

func TestTemplateCmd_BasicYAML(t *testing.T) {
	dir := setupProject(t)

	cmd := newTemplateCmd()
	cmd.SetArgs([]string{dir, "--namespace", "production"})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "apiVersion: apps/v1")
	assert.Contains(t, out, "kind: Deployment")
	assert.Contains(t, out, "namespace: production")
	assert.Contains(t, out, "kind: Service")
}

func TestTemplateCmd_JSONOutput(t *testing.T) {
	dir := setupProject(t)

	cmd := newTemplateCmd()
	cmd.SetArgs([]string{dir, "--namespace", "default", "--output", "json"})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, `"apiVersion": "apps/v1"`)
	assert.Contains(t, out, `"kind": "Deployment"`)
}

func TestTemplateCmd_SetOverride(t *testing.T) {
	dir := setupProject(t)

	cmd := newTemplateCmd()
	cmd.SetArgs([]string{dir, "--namespace", "test", "--set", "replicas=10"})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "replicas: 10")
}

func TestTemplateCmd_ExplicitValuesFile(t *testing.T) {
	dir := setupProject(t)

	customValues := filepath.Join(t.TempDir(), "custom-values.ts")
	require.NoError(t, os.WriteFile(customValues, []byte(`export default { image: "custom:1.0", replicas: 7 };`), 0o644))

	cmd := newTemplateCmd()
	cmd.SetArgs([]string{dir, "--namespace", "staging", "--values", customValues})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "image: custom:1.0")
	assert.Contains(t, out, "replicas: 7")
}

func TestTemplateCmd_MissingCtTs(t *testing.T) {
	dir := t.TempDir()

	cmd := newTemplateCmd()
	cmd.SetArgs([]string{dir})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entry point not found")
}

func TestTemplateCmd_MissingCttsDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ct.ts"), []byte(`// empty`), 0o644))

	cmd := newTemplateCmd()
	cmd.SetArgs([]string{dir})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ct init")
}

func TestTemplateCmd_NoNamespace(t *testing.T) {
	dir := setupProject(t)

	cmd := newTemplateCmd()
	cmd.SetArgs([]string{dir})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "kind: Deployment")
	assert.NotContains(t, out, "namespace:")
}

func TestTemplateCmd_CustomCtTs(t *testing.T) {
	dir := setupProject(t)

	customCt := `import { deployment } from "ctts/k8s/apps/v1";
deployment({ name: "custom-app", image: Values.image, replicas: Values.replicas });
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ct.ts"), []byte(customCt), 0o644))

	cmd := newTemplateCmd()
	cmd.SetArgs([]string{dir, "--namespace", "ns1"})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "name: custom-app")
	assert.Contains(t, out, "namespace: ns1")
}
