package packages_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudticon/ctts/internal/packages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeGit struct {
	repos map[string]map[string]string // url -> filename -> content
}

func (f *fakeGit) Clone(url, ref, destDir string) (string, error) {
	files, ok := f.repos[url]
	if !ok {
		return "", os.ErrNotExist
	}
	for name, content := range files {
		p := filepath.Join(destDir, name)
		require2(os.MkdirAll(filepath.Dir(p), 0o755))
		require2(nil, os.WriteFile(p, []byte(content), 0o644))
	}
	return "fake-sha-" + ref, nil
}

func (f *fakeGit) FetchSHA(url, ref string) (string, error) {
	if _, ok := f.repos[url]; !ok {
		return "", os.ErrNotExist
	}
	return "fake-sha-" + ref, nil
}

func require2(_ any, err ...error) {
	for _, e := range err {
		if e != nil {
			panic(e)
		}
	}
}

func setupProject(t *testing.T, ctTsContent string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "proj")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".ctts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ct.ts"), []byte(ctTsContent), 0o644))
	return dir
}

func TestSyncPackages_NoGitImports(t *testing.T) {
	dir := setupProject(t, `import { deployment } from "ctts/k8s/apps/v1";`)
	git := &fakeGit{repos: map[string]map[string]string{}}
	syncer := packages.NewSyncerWithGit(git)

	err := syncer.Sync(dir)
	require.NoError(t, err)

	_, statErr := os.Stat(filepath.Join(dir, "ct.lock"))
	assert.True(t, os.IsNotExist(statErr), "ct.lock should not be created when there are no git packages")
}

func TestSyncPackages_SinglePackage(t *testing.T) {
	dir := setupProject(t, `
import { deployment } from "ctts/k8s/apps/v1";
import { webApp } from "github.com/someone/ctts-webapp";
`)
	git := &fakeGit{repos: map[string]map[string]string{
		"https://github.com/someone/ctts-webapp.git": {
			"index.ts": `export function webApp() { return {}; }`,
		},
	}}
	syncer := packages.NewSyncerWithGit(git)

	err := syncer.Sync(dir)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(dir, ".ctts", "packages", "github.com", "someone", "ctts-webapp", "index.ts"))
	assert.FileExists(t, filepath.Join(dir, "ct.lock"))

	lf, err := packages.ReadLock(filepath.Join(dir, "ct.lock"))
	require.NoError(t, err)
	assert.Contains(t, lf.Packages, "github.com/someone/ctts-webapp")
	assert.Equal(t, "fake-sha-", lf.Packages["github.com/someone/ctts-webapp"].SHA)
}

func TestSyncPackages_SubPathImport(t *testing.T) {
	dir := setupProject(t, `
import { redis } from "github.com/someone/ctts-redis/presets/ha";
`)
	git := &fakeGit{repos: map[string]map[string]string{
		"https://github.com/someone/ctts-redis.git": {
			"index.ts":      `export const redis = {};`,
			"presets/ha.ts":  `export function redis() {}`,
		},
	}}
	syncer := packages.NewSyncerWithGit(git)

	err := syncer.Sync(dir)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(dir, ".ctts", "packages", "github.com", "someone", "ctts-redis", "presets", "ha.ts"))
}

func TestSyncPackages_TransitiveDeps(t *testing.T) {
	dir := setupProject(t, `import { webApp } from "github.com/someone/ctts-webapp";`)
	git := &fakeGit{repos: map[string]map[string]string{
		"https://github.com/someone/ctts-webapp.git": {
			"index.ts": `
import { helper } from "github.com/someone/ctts-utils";
export function webApp() { return helper(); }
`,
		},
		"https://github.com/someone/ctts-utils.git": {
			"index.ts": `export function helper() { return 42; }`,
		},
	}}

	syncer := packages.NewSyncerWithGit(git)
	err := syncer.Sync(dir)
	require.NoError(t, err)

	assert.DirExists(t, filepath.Join(dir, ".ctts", "packages", "github.com", "someone", "ctts-webapp"))
	assert.DirExists(t, filepath.Join(dir, ".ctts", "packages", "github.com", "someone", "ctts-utils"))

	lf, err := packages.ReadLock(filepath.Join(dir, "ct.lock"))
	require.NoError(t, err)
	assert.Contains(t, lf.Packages, "github.com/someone/ctts-webapp")
	assert.Contains(t, lf.Packages, "github.com/someone/ctts-utils")
}

func TestSyncPackages_SkipsAlreadyInstalled(t *testing.T) {
	dir := setupProject(t, `import { webApp } from "github.com/someone/ctts-webapp";`)

	pkgDir := filepath.Join(dir, ".ctts", "packages", "github.com", "someone", "ctts-webapp")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "index.ts"), []byte(`export const x = 1;`), 0o644))

	lockPath := filepath.Join(dir, "ct.lock")
	lf := packages.NewLockFile()
	lf.Packages["github.com/someone/ctts-webapp"] = packages.LockEntry{Ref: "v1.0.0", SHA: "existingsha"}
	require.NoError(t, packages.WriteLock(lockPath, lf))

	cloneCalled := false
	git := &countingGit{
		fakeGit: &fakeGit{repos: map[string]map[string]string{
			"https://github.com/someone/ctts-webapp.git": {
				"index.ts": `export const x = 1;`,
			},
		}},
		onClone: func() { cloneCalled = true },
	}
	syncer := packages.NewSyncerWithGit(git)

	err := syncer.Sync(dir)
	require.NoError(t, err)
	assert.False(t, cloneCalled, "Clone should not be called for already installed packages")

	after, err := packages.ReadLock(lockPath)
	require.NoError(t, err)
	assert.Equal(t, "existingsha", after.Packages["github.com/someone/ctts-webapp"].SHA)
}

func TestSyncPackages_RestoresMissingPackage(t *testing.T) {
	dir := setupProject(t, `import { webApp } from "github.com/someone/ctts-webapp";`)

	lockPath := filepath.Join(dir, "ct.lock")
	lf := packages.NewLockFile()
	lf.Packages["github.com/someone/ctts-webapp"] = packages.LockEntry{Ref: "v1.0.0", SHA: "oldsha"}
	require.NoError(t, packages.WriteLock(lockPath, lf))

	git := &fakeGit{repos: map[string]map[string]string{
		"https://github.com/someone/ctts-webapp.git": {
			"index.ts": `export const restored = true;`,
		},
	}}
	syncer := packages.NewSyncerWithGit(git)

	err := syncer.Sync(dir)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(dir, ".ctts", "packages", "github.com", "someone", "ctts-webapp", "index.ts"))
}

func TestSyncPackages_MultiplePackages(t *testing.T) {
	dir := setupProject(t, `
import { webApp } from "github.com/someone/ctts-webapp";
import { redis } from "github.com/someone/ctts-redis";
`)
	git := &fakeGit{repos: map[string]map[string]string{
		"https://github.com/someone/ctts-webapp.git": {"index.ts": `export function webApp() {}`},
		"https://github.com/someone/ctts-redis.git":  {"index.ts": `export function redis() {}`},
	}}
	syncer := packages.NewSyncerWithGit(git)

	err := syncer.Sync(dir)
	require.NoError(t, err)

	lf, err := packages.ReadLock(filepath.Join(dir, "ct.lock"))
	require.NoError(t, err)
	assert.Len(t, lf.Packages, 2)
}

func TestUpdate_ReinstallsWhenSHAChanged(t *testing.T) {
	dir := setupProject(t, `import { webApp } from "github.com/someone/ctts-webapp";`)

	pkgDir := filepath.Join(dir, ".ctts", "packages", "github.com", "someone", "ctts-webapp")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "index.ts"), []byte(`export const old = true;`), 0o644))

	lockPath := filepath.Join(dir, "ct.lock")
	lf := packages.NewLockFile()
	lf.Packages["github.com/someone/ctts-webapp"] = packages.LockEntry{Ref: "v1.0.0", SHA: "old-sha"}
	require.NoError(t, packages.WriteLock(lockPath, lf))

	git := &fakeGit{repos: map[string]map[string]string{
		"https://github.com/someone/ctts-webapp.git": {
			"index.ts": `export const updated = true;`,
		},
	}}
	syncer := packages.NewSyncerWithGit(git)

	err := syncer.Update(dir, nil)
	require.NoError(t, err)

	after, err := packages.ReadLock(lockPath)
	require.NoError(t, err)
	assert.Equal(t, "fake-sha-v1.0.0", after.Packages["github.com/someone/ctts-webapp"].SHA)

	content, err := os.ReadFile(filepath.Join(pkgDir, "index.ts"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "updated")
}

func TestUpdate_SkipsWhenSHAUnchanged(t *testing.T) {
	dir := setupProject(t, `import { webApp } from "github.com/someone/ctts-webapp";`)

	pkgDir := filepath.Join(dir, ".ctts", "packages", "github.com", "someone", "ctts-webapp")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "index.ts"), []byte(`export const x = 1;`), 0o644))

	lockPath := filepath.Join(dir, "ct.lock")
	lf := packages.NewLockFile()
	lf.Packages["github.com/someone/ctts-webapp"] = packages.LockEntry{Ref: "v1.0.0", SHA: "fake-sha-v1.0.0"}
	require.NoError(t, packages.WriteLock(lockPath, lf))

	cloneCalled := false
	git := &countingGit{
		fakeGit: &fakeGit{repos: map[string]map[string]string{
			"https://github.com/someone/ctts-webapp.git": {"index.ts": `export const x = 1;`},
		}},
		onClone: func() { cloneCalled = true },
	}
	syncer := packages.NewSyncerWithGit(git)

	err := syncer.Update(dir, nil)
	require.NoError(t, err)
	assert.False(t, cloneCalled, "Clone should not be called when SHA is unchanged")
}

func TestUpdate_SpecificPackage(t *testing.T) {
	dir := setupProject(t, `
import { webApp } from "github.com/someone/ctts-webapp";
import { redis } from "github.com/someone/ctts-redis";
`)

	lockPath := filepath.Join(dir, "ct.lock")
	lf := packages.NewLockFile()
	lf.Packages["github.com/someone/ctts-webapp"] = packages.LockEntry{Ref: "main", SHA: "old-sha-1"}
	lf.Packages["github.com/someone/ctts-redis"] = packages.LockEntry{Ref: "main", SHA: "old-sha-2"}
	require.NoError(t, packages.WriteLock(lockPath, lf))

	git := &fakeGit{repos: map[string]map[string]string{
		"https://github.com/someone/ctts-webapp.git": {"index.ts": `export function webApp() {}`},
		"https://github.com/someone/ctts-redis.git":  {"index.ts": `export function redis() {}`},
	}}
	syncer := packages.NewSyncerWithGit(git)

	err := syncer.Update(dir, []string{"github.com/someone/ctts-webapp"})
	require.NoError(t, err)

	after, err := packages.ReadLock(lockPath)
	require.NoError(t, err)
	assert.Equal(t, "fake-sha-main", after.Packages["github.com/someone/ctts-webapp"].SHA)
	assert.Equal(t, "old-sha-2", after.Packages["github.com/someone/ctts-redis"].SHA)
}

func TestUpdate_ErrorOnUnknownPackage(t *testing.T) {
	dir := setupProject(t, `import { x } from "ctts/k8s/apps/v1";`)

	lockPath := filepath.Join(dir, "ct.lock")
	lf := packages.NewLockFile()
	require.NoError(t, packages.WriteLock(lockPath, lf))

	git := &fakeGit{repos: map[string]map[string]string{}}
	syncer := packages.NewSyncerWithGit(git)

	err := syncer.Update(dir, []string{"github.com/nonexistent/pkg"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in ct.lock")
}

func TestSyncPackages_CycleDetection(t *testing.T) {
	dir := setupProject(t, `import { a } from "github.com/someone/pkg-a";`)
	git := &fakeGit{repos: map[string]map[string]string{
		"https://github.com/someone/pkg-a.git": {
			"index.ts": `
import { b } from "github.com/someone/pkg-b";
export function a() { return b(); }
`,
		},
		"https://github.com/someone/pkg-b.git": {
			"index.ts": `
import { a } from "github.com/someone/pkg-a";
export function b() { return a(); }
`,
		},
	}}

	syncer := packages.NewSyncerWithGit(git)
	err := syncer.Sync(dir)
	require.NoError(t, err, "cycle should not cause infinite loop or error")

	assert.DirExists(t, filepath.Join(dir, ".ctts", "packages", "github.com", "someone", "pkg-a"))
	assert.DirExists(t, filepath.Join(dir, ".ctts", "packages", "github.com", "someone", "pkg-b"))

	lf, err := packages.ReadLock(filepath.Join(dir, "ct.lock"))
	require.NoError(t, err)
	assert.Len(t, lf.Packages, 2)
}

func TestSyncPackages_DiamondDependency(t *testing.T) {
	dir := setupProject(t, `
import { a } from "github.com/someone/pkg-a";
import { b } from "github.com/someone/pkg-b";
`)
	git := &fakeGit{repos: map[string]map[string]string{
		"https://github.com/someone/pkg-a.git": {
			"index.ts": `
import { shared } from "github.com/someone/pkg-shared";
export function a() { return shared(); }
`,
		},
		"https://github.com/someone/pkg-b.git": {
			"index.ts": `
import { shared } from "github.com/someone/pkg-shared";
export function b() { return shared(); }
`,
		},
		"https://github.com/someone/pkg-shared.git": {
			"index.ts": `export function shared() { return 42; }`,
		},
	}}

	cloneCount := 0
	counting := &countingGit{
		fakeGit: git,
		onClone: func() { cloneCount++ },
	}
	syncer := packages.NewSyncerWithGit(counting)

	err := syncer.Sync(dir)
	require.NoError(t, err)

	lf, err := packages.ReadLock(filepath.Join(dir, "ct.lock"))
	require.NoError(t, err)
	assert.Len(t, lf.Packages, 3)
	assert.Contains(t, lf.Packages, "github.com/someone/pkg-a")
	assert.Contains(t, lf.Packages, "github.com/someone/pkg-b")
	assert.Contains(t, lf.Packages, "github.com/someone/pkg-shared")

	assert.Equal(t, 3, cloneCount, "shared package should be cloned only once")
}

func TestSyncPackages_NestedSubdirectories(t *testing.T) {
	dir := setupProject(t, `
import { deep } from "github.com/someone/ctts-deep/a/b/c";
`)
	git := &fakeGit{repos: map[string]map[string]string{
		"https://github.com/someone/ctts-deep.git": {
			"index.ts":   `export const root = 1;`,
			"a/b/c.ts":   `export function deep() { return 42; }`,
			"a/b/d.ts":   `export function other() { return 0; }`,
		},
	}}
	syncer := packages.NewSyncerWithGit(git)

	err := syncer.Sync(dir)
	require.NoError(t, err)

	pkgDir := filepath.Join(dir, ".ctts", "packages", "github.com", "someone", "ctts-deep")
	assert.FileExists(t, filepath.Join(pkgDir, "index.ts"))
	assert.FileExists(t, filepath.Join(pkgDir, "a", "b", "c.ts"))
	assert.FileExists(t, filepath.Join(pkgDir, "a", "b", "d.ts"))
}

func TestSyncPackages_IdempotentDoubleSync(t *testing.T) {
	dir := setupProject(t, `import { x } from "github.com/someone/ctts-lib";`)

	cloneCount := 0
	git := &countingGit{
		fakeGit: &fakeGit{repos: map[string]map[string]string{
			"https://github.com/someone/ctts-lib.git": {
				"index.ts": `export const x = 1;`,
			},
		}},
		onClone: func() { cloneCount++ },
	}
	syncer := packages.NewSyncerWithGit(git)

	err := syncer.Sync(dir)
	require.NoError(t, err)
	assert.Equal(t, 1, cloneCount)

	err = syncer.Sync(dir)
	require.NoError(t, err)
	assert.Equal(t, 1, cloneCount, "second sync should not re-clone installed packages")
}

func TestSyncPackages_OnlyRelativeImports(t *testing.T) {
	dir := setupProject(t, `
import { helper } from "./lib/helpers";
import defaults from "./config";
const x = 1;
`)
	git := &fakeGit{repos: map[string]map[string]string{}}
	syncer := packages.NewSyncerWithGit(git)

	err := syncer.Sync(dir)
	require.NoError(t, err)

	_, statErr := os.Stat(filepath.Join(dir, "ct.lock"))
	assert.True(t, os.IsNotExist(statErr), "ct.lock should not be created for relative-only imports")
}

func TestSyncPackages_TransitiveChain(t *testing.T) {
	dir := setupProject(t, `import { a } from "github.com/someone/pkg-a";`)
	git := &fakeGit{repos: map[string]map[string]string{
		"https://github.com/someone/pkg-a.git": {
			"index.ts": `
import { b } from "github.com/someone/pkg-b";
export function a() { return b(); }
`,
		},
		"https://github.com/someone/pkg-b.git": {
			"index.ts": `
import { c } from "github.com/someone/pkg-c";
export function b() { return c(); }
`,
		},
		"https://github.com/someone/pkg-c.git": {
			"index.ts": `export function c() { return 99; }`,
		},
	}}

	syncer := packages.NewSyncerWithGit(git)
	err := syncer.Sync(dir)
	require.NoError(t, err)

	lf, err := packages.ReadLock(filepath.Join(dir, "ct.lock"))
	require.NoError(t, err)
	assert.Len(t, lf.Packages, 3, "should resolve full transitive chain A->B->C")
	assert.Contains(t, lf.Packages, "github.com/someone/pkg-a")
	assert.Contains(t, lf.Packages, "github.com/someone/pkg-b")
	assert.Contains(t, lf.Packages, "github.com/someone/pkg-c")
}

type countingGit struct {
	fakeGit *fakeGit
	onClone func()
}

func (c *countingGit) Clone(url, ref, dest string) (string, error) {
	if c.onClone != nil {
		c.onClone()
	}
	return c.fakeGit.Clone(url, ref, dest)
}

func (c *countingGit) FetchSHA(url, ref string) (string, error) {
	return c.fakeGit.FetchSHA(url, ref)
}
