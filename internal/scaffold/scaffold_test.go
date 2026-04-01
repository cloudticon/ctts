package scaffold_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudticon/ctts/internal/packages"
	"github.com/cloudticon/ctts/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func nop() scaffold.PackageSyncer { return scaffold.NopPackageSyncer() }

func TestInit_CreatesDirectoryStructure(t *testing.T) {
	ctDir := filepath.Join(t.TempDir(), "ct")

	require.NoError(t, scaffold.InitWith(ctDir, nop()))

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

	require.NoError(t, scaffold.InitWith(ctDir, nop()))

	for _, name := range []string{"ct.ts", "values.ts", "tsconfig.json", ".gitignore"} {
		path := filepath.Join(ctDir, name)
		info, err := os.Stat(path)
		require.NoError(t, err, "file should exist: %s", name)
		assert.False(t, info.IsDir())
	}
}

func TestInit_GitignoreContainsPackagesDir(t *testing.T) {
	ctDir := filepath.Join(t.TempDir(), "ct")

	require.NoError(t, scaffold.InitWith(ctDir, nop()))

	content, err := os.ReadFile(filepath.Join(ctDir, ".gitignore"))
	require.NoError(t, err)
	assert.Contains(t, string(content), ".ctts/packages/")
}

func TestInit_CtTsContainsImports(t *testing.T) {
	ctDir := filepath.Join(t.TempDir(), "ct")

	require.NoError(t, scaffold.InitWith(ctDir, nop()))

	content, err := os.ReadFile(filepath.Join(ctDir, "ct.ts"))
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, `from "github.com/cloudticon/k8s/apps/v1"`)
	assert.Contains(t, s, `from "github.com/cloudticon/k8s/core/v1"`)
	assert.Contains(t, s, "deployment(")
	assert.Contains(t, s, "service(")
}

func TestInit_ValuesTsHasDefaultExport(t *testing.T) {
	ctDir := filepath.Join(t.TempDir(), "ct")

	require.NoError(t, scaffold.InitWith(ctDir, nop()))

	content, err := os.ReadFile(filepath.Join(ctDir, "values.ts"))
	require.NoError(t, err)

	assert.Contains(t, string(content), "export default")
}

func TestInit_TsconfigContainsPathMapping(t *testing.T) {
	ctDir := filepath.Join(t.TempDir(), "ct")

	require.NoError(t, scaffold.InitWith(ctDir, nop()))

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

	require.NoError(t, scaffold.InitWith(ctDir, nop()))

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

	require.NoError(t, scaffold.InitWith(ctDir, nop()))

	content, err := os.ReadFile(filepath.Join(ctDir, ".ctts/types/k8s/resource.ts"))
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, "export function resource(")
	assert.Contains(t, s, "export function resourceClusterScope(")
}

func TestInit_GeneratesValuesDts(t *testing.T) {
	ctDir := filepath.Join(t.TempDir(), "ct")

	require.NoError(t, scaffold.InitWith(ctDir, nop()))

	dtsPath := filepath.Join(ctDir, ".ctts/types/values.d.ts")
	content, err := os.ReadFile(dtsPath)
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, "declare const Values")
	assert.Contains(t, s, "image: string")
	assert.Contains(t, s, "replicas: number")
}

// --- Integration: Init → Sync → SyncPackages chain --------------------------

type fakeGitForInit struct {
	clonedURLs []string
}

func (f *fakeGitForInit) Clone(url, ref, destDir string) (string, error) {
	f.clonedURLs = append(f.clonedURLs, url)
	if url != "https://github.com/cloudticon/k8s.git" {
		return "", fmt.Errorf("unexpected clone URL: %s", url)
	}
	files := map[string]string{
		"index.ts":        `export { deployment } from "./apps/v1";`,
		"resource.ts":     `export function resource() {}`,
		"apps/v1.ts":      `export function deployment(opts: any) { return opts; }`,
		"core/v1.ts":      `export function service(opts: any) { return opts; }`,
		"networking/v1.ts": `export function ingress(opts: any) { return opts; }`,
	}
	for name, content := range files {
		p := filepath.Join(destDir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			return "", err
		}
	}
	return "abc123deadbeef", nil
}

func (f *fakeGitForInit) FetchSHA(string, string) (string, error) {
	return "abc123deadbeef", nil
}

type pkgSyncerAdapter struct{ s *packages.Syncer }

func (a pkgSyncerAdapter) SyncPackages(dir string) error { return a.s.Sync(dir) }

func TestInit_SyncPackagesInstallsK8sFromGit(t *testing.T) {
	git := &fakeGitForInit{}
	syncer := pkgSyncerAdapter{s: packages.NewSyncerWithGit(git)}

	ctDir := filepath.Join(t.TempDir(), "ct")
	require.NoError(t, scaffold.InitWith(ctDir, syncer))

	ctTs, err := os.ReadFile(filepath.Join(ctDir, "ct.ts"))
	require.NoError(t, err)
	assert.Contains(t, string(ctTs), `from "github.com/cloudticon/k8s/apps/v1"`)
	assert.Contains(t, string(ctTs), `from "github.com/cloudticon/k8s/core/v1"`)

	require.Len(t, git.clonedURLs, 1)
	assert.Equal(t, "https://github.com/cloudticon/k8s.git", git.clonedURLs[0])

	pkgBase := filepath.Join(ctDir, ".ctts", "packages", "github.com", "cloudticon", "k8s")
	assert.FileExists(t, filepath.Join(pkgBase, "index.ts"))
	assert.FileExists(t, filepath.Join(pkgBase, "apps", "v1.ts"))
	assert.FileExists(t, filepath.Join(pkgBase, "core", "v1.ts"))

	lf, err := packages.ReadLock(filepath.Join(ctDir, "ct.lock"))
	require.NoError(t, err)
	require.Contains(t, lf.Packages, "github.com/cloudticon/k8s")
	assert.Equal(t, "main", lf.Packages["github.com/cloudticon/k8s"].Ref)
	assert.Equal(t, "abc123deadbeef", lf.Packages["github.com/cloudticon/k8s"].SHA)
}
