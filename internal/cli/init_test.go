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
	projectDir := filepath.Join(dir, "myproject")

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--dir", projectDir})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "Initialized ct project")
	assert.FileExists(t, filepath.Join(projectDir, "main.ct"))
	assert.FileExists(t, filepath.Join(projectDir, "values.json"))
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

	assert.FileExists(t, filepath.Join(tmpDir, "main.ct"))
	assert.FileExists(t, filepath.Join(tmpDir, "values.json"))
}
