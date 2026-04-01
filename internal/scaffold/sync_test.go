package scaffold_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudticon/ctts/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSync_RegeneratesStdlibAndValues(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	require.NoError(t, scaffold.Init(dir))

	require.NoError(t, os.Remove(filepath.Join(dir, ".ctts", "types", "k8s", "resource.ts")))
	require.NoError(t, os.Remove(filepath.Join(dir, ".ctts", "types", "values.d.ts")))

	require.NoError(t, scaffold.Sync(dir))

	assert.FileExists(t, filepath.Join(dir, ".ctts", "types", "k8s", "resource.ts"))
	assert.FileExists(t, filepath.Join(dir, ".ctts", "types", "values.d.ts"))
}

func TestSync_UpdatesValuesDtsAfterValuesChange(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	require.NoError(t, scaffold.Init(dir))

	newValues := `export default {
  image: "nginx:1.25",
  replicas: 3,
  domain: "example.com",
};
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "values.ts"), []byte(newValues), 0o644))

	require.NoError(t, scaffold.Sync(dir))

	content, err := os.ReadFile(filepath.Join(dir, ".ctts", "types", "values.d.ts"))
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, "domain: string")
	assert.Contains(t, s, "image: string")
	assert.Contains(t, s, "replicas: number")
}

func TestSync_ErrorWhenNoCTts(t *testing.T) {
	dir := t.TempDir()

	err := scaffold.Sync(dir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ct.ts not found")
}

func TestSync_PreservesUserFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	require.NoError(t, scaffold.Init(dir))

	original, err := os.ReadFile(filepath.Join(dir, "ct.ts"))
	require.NoError(t, err)

	require.NoError(t, scaffold.Sync(dir))

	after, err := os.ReadFile(filepath.Join(dir, "ct.ts"))
	require.NoError(t, err)
	assert.Equal(t, original, after)
}

func TestSync_CreatesTypesDirsIfMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	require.NoError(t, scaffold.Init(dir))

	require.NoError(t, os.RemoveAll(filepath.Join(dir, ".ctts", "types", "k8s")))

	require.NoError(t, scaffold.Sync(dir))

	for _, sub := range []string{"apps", "core", "networking"} {
		assert.DirExists(t, filepath.Join(dir, ".ctts", "types", "k8s", sub))
	}
}
