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

func TestSyncCmd_SyncsExistingProject(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	require.NoError(t, scaffold.Init(dir))

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

	require.NoError(t, scaffold.Init("ct"))

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
