package scaffold_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudticon/ctts/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit_CreatesDirectoryStructure(t *testing.T) {
	ctDir := filepath.Join(t.TempDir(), "ct")

	require.NoError(t, scaffold.Init(ctDir))

	for _, sub := range []string{
		"",
		".ctts/types/k8s",
		".ctts/types/k8s/apps",
		".ctts/types/k8s/core",
		".ctts/types/k8s/networking",
	} {
		path := filepath.Join(ctDir, sub)
		info, err := os.Stat(path)
		require.NoError(t, err, "directory should exist: %s", path)
		assert.True(t, info.IsDir(), "should be a directory: %s", path)
	}
}

func TestInit_CreatesStarterFiles(t *testing.T) {
	ctDir := filepath.Join(t.TempDir(), "ct")

	require.NoError(t, scaffold.Init(ctDir))

	for _, name := range []string{"ct.ts", "values.ts", "tsconfig.json"} {
		path := filepath.Join(ctDir, name)
		info, err := os.Stat(path)
		require.NoError(t, err, "file should exist: %s", name)
		assert.False(t, info.IsDir())
	}
}

func TestInit_CtTsContainsImports(t *testing.T) {
	ctDir := filepath.Join(t.TempDir(), "ct")

	require.NoError(t, scaffold.Init(ctDir))

	content, err := os.ReadFile(filepath.Join(ctDir, "ct.ts"))
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, `from "ctts/k8s/apps/v1"`)
	assert.Contains(t, s, `from "ctts/k8s/core/v1"`)
	assert.Contains(t, s, "deployment(")
	assert.Contains(t, s, "service(")
}

func TestInit_ValuesTsHasDefaultExport(t *testing.T) {
	ctDir := filepath.Join(t.TempDir(), "ct")

	require.NoError(t, scaffold.Init(ctDir))

	content, err := os.ReadFile(filepath.Join(ctDir, "values.ts"))
	require.NoError(t, err)

	assert.Contains(t, string(content), "export default")
}

func TestInit_TsconfigContainsPathMapping(t *testing.T) {
	ctDir := filepath.Join(t.TempDir(), "ct")

	require.NoError(t, scaffold.Init(ctDir))

	content, err := os.ReadFile(filepath.Join(ctDir, "tsconfig.json"))
	require.NoError(t, err)

	var cfg map[string]interface{}
	require.NoError(t, json.Unmarshal(content, &cfg))

	compilerOpts, ok := cfg["compilerOptions"].(map[string]interface{})
	require.True(t, ok)

	paths, ok := compilerOpts["paths"].(map[string]interface{})
	require.True(t, ok)

	cttsPaths, ok := paths["ctts/*"].([]interface{})
	require.True(t, ok)
	require.Len(t, cttsPaths, 1)
	assert.Equal(t, ".ctts/types/*", cttsPaths[0])
}

func TestInit_CopiesStdlibTypes(t *testing.T) {
	ctDir := filepath.Join(t.TempDir(), "ct")

	require.NoError(t, scaffold.Init(ctDir))

	stdlibFiles := []string{
		".ctts/types/k8s/resource.ts",
		".ctts/types/k8s/apps/v1.ts",
		".ctts/types/k8s/core/v1.ts",
		".ctts/types/k8s/networking/v1.ts",
	}
	for _, rel := range stdlibFiles {
		path := filepath.Join(ctDir, rel)
		info, err := os.Stat(path)
		require.NoError(t, err, "stdlib file should exist: %s", rel)
		assert.False(t, info.IsDir())
		assert.Greater(t, info.Size(), int64(0), "stdlib file should not be empty: %s", rel)
	}
}

func TestInit_StdlibResourceContainsExports(t *testing.T) {
	ctDir := filepath.Join(t.TempDir(), "ct")

	require.NoError(t, scaffold.Init(ctDir))

	content, err := os.ReadFile(filepath.Join(ctDir, ".ctts/types/k8s/resource.ts"))
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, "export function resource(")
	assert.Contains(t, s, "export function resourceClusterScope(")
}

func TestInit_GeneratesValuesDts(t *testing.T) {
	ctDir := filepath.Join(t.TempDir(), "ct")

	require.NoError(t, scaffold.Init(ctDir))

	dtsPath := filepath.Join(ctDir, ".ctts/types/values.d.ts")
	content, err := os.ReadFile(dtsPath)
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, "declare const Values")
	assert.Contains(t, s, "image: string")
	assert.Contains(t, s, "replicas: number")
}
