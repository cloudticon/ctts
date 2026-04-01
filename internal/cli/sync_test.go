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

// stdlibCtTs uses embedded stdlib imports so CLI tests don't trigger git cloning.
const stdlibCtTs = `import { deployment } from "ctts/k8s/apps/v1";
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

func initWithStdlibImports(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, scaffold.InitWith(dir, scaffold.NopPackageSyncer()))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ct.ts"), []byte(stdlibCtTs), 0o644))
}

func TestSyncCmd_SyncsExistingProject(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	initWithStdlibImports(t, dir)

	require.NoError(t, os.Remove(filepath.Join(dir, ".ctts", "types", "values.d.ts")))

	cmd := newSyncCmd()
	cmd.SetArgs([]string{dir})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "Synced types")
	assert.FileExists(t, filepath.Join(dir, ".ctts", "types", "values.d.ts"))
}

func TestSyncCmd_DefaultDir(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { os.Chdir(origDir) })

	initWithStdlibImports(t, "ct")

	cmd := newSyncCmd()
	cmd.SetArgs([]string{})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "Synced types in ct/")
}

func TestSyncCmd_ErrorOnMissingProject(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")

	cmd := newSyncCmd()
	cmd.SetArgs([]string{dir})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ct.ts not found")
}
