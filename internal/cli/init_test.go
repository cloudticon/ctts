package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCmd_CreatesProjectStructure(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "myct")

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--dir", projectDir})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "Initialized ct project")

	assert.FileExists(t, filepath.Join(projectDir, "ct.ts"))
	assert.FileExists(t, filepath.Join(projectDir, "values.ts"))
	assert.FileExists(t, filepath.Join(projectDir, "tsconfig.json"))
	assert.DirExists(t, filepath.Join(projectDir, ".ctts", "types"))
	assert.FileExists(t, filepath.Join(projectDir, ".ctts", "types", "values.d.ts"))
	assert.FileExists(t, filepath.Join(projectDir, ".ctts", "types", "k8s", "resource.ts"))
}

func TestInitCmd_DefaultDir(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { os.Chdir(origDir) })

	cmd := newInitCmd()
	cmd.SetArgs([]string{})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tmpDir, "ct", "ct.ts"))
}
