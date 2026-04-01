package packages_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudticon/ctts/internal/packages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadLock_NonExistentReturnsNew(t *testing.T) {
	lf, err := packages.ReadLock("/nonexistent/ct.lock")
	require.NoError(t, err)
	assert.Equal(t, 1, lf.Version)
	assert.Empty(t, lf.Packages)
}

func TestReadLock_ParsesYAML(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ct.lock")
	content := `version: 1
packages:
  github.com/someone/ctts-webapp:
    ref: v1.2.0
    sha: abc123def456789
  github.com/someone/ctts-redis:
    ref: main
    sha: def789abc123456
`
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))

	lf, err := packages.ReadLock(p)
	require.NoError(t, err)
	assert.Equal(t, 1, lf.Version)
	require.Len(t, lf.Packages, 2)

	webapp := lf.Packages["github.com/someone/ctts-webapp"]
	assert.Equal(t, "v1.2.0", webapp.Ref)
	assert.Equal(t, "abc123def456789", webapp.SHA)

	redis := lf.Packages["github.com/someone/ctts-redis"]
	assert.Equal(t, "main", redis.Ref)
	assert.Equal(t, "def789abc123456", redis.SHA)
}

func TestReadLock_EmptyPackages(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ct.lock")
	require.NoError(t, os.WriteFile(p, []byte("version: 1\n"), 0o644))

	lf, err := packages.ReadLock(p)
	require.NoError(t, err)
	assert.NotNil(t, lf.Packages)
	assert.Empty(t, lf.Packages)
}

func TestReadLock_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ct.lock")
	require.NoError(t, os.WriteFile(p, []byte("{{invalid yaml"), 0o644))

	_, err := packages.ReadLock(p)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing lock file")
}

func TestWriteLock_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ct.lock")

	lf := packages.NewLockFile()
	lf.Packages["github.com/someone/ctts-webapp"] = packages.LockEntry{
		Ref: "v1.0.0",
		SHA: "abc123",
	}

	require.NoError(t, packages.WriteLock(p, lf))

	data, err := os.ReadFile(p)
	require.NoError(t, err)
	s := string(data)
	assert.Contains(t, s, "version: 1")
	assert.Contains(t, s, "github.com/someone/ctts-webapp")
	assert.Contains(t, s, "ref: v1.0.0")
	assert.Contains(t, s, "sha: abc123")
}

func TestWriteLock_SortedKeys(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ct.lock")

	lf := packages.NewLockFile()
	lf.Packages["github.com/z/z"] = packages.LockEntry{Ref: "main", SHA: "zzz"}
	lf.Packages["github.com/a/a"] = packages.LockEntry{Ref: "main", SHA: "aaa"}

	require.NoError(t, packages.WriteLock(p, lf))

	data, err := os.ReadFile(p)
	require.NoError(t, err)
	s := string(data)

	posA := indexOf(s, "github.com/a/a")
	posZ := indexOf(s, "github.com/z/z")
	assert.Less(t, posA, posZ, "packages should be sorted alphabetically")
}

func TestRoundTrip_WriteThenRead(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ct.lock")

	original := packages.NewLockFile()
	original.Packages["github.com/someone/ctts-webapp"] = packages.LockEntry{Ref: "v2.0.0", SHA: "deadbeef"}
	original.Packages["github.com/someone/ctts-redis"] = packages.LockEntry{Ref: "main", SHA: "cafebabe"}

	require.NoError(t, packages.WriteLock(p, original))

	loaded, err := packages.ReadLock(p)
	require.NoError(t, err)
	assert.Equal(t, original.Version, loaded.Version)
	assert.Equal(t, original.Packages, loaded.Packages)
}

func TestWriteLock_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ct.lock")

	lf1 := packages.NewLockFile()
	lf1.Packages["github.com/old/pkg"] = packages.LockEntry{Ref: "v1.0.0", SHA: "old-sha"}
	require.NoError(t, packages.WriteLock(p, lf1))

	lf2 := packages.NewLockFile()
	lf2.Packages["github.com/new/pkg"] = packages.LockEntry{Ref: "v2.0.0", SHA: "new-sha"}
	require.NoError(t, packages.WriteLock(p, lf2))

	loaded, err := packages.ReadLock(p)
	require.NoError(t, err)
	assert.Len(t, loaded.Packages, 1)
	assert.Contains(t, loaded.Packages, "github.com/new/pkg")
	assert.NotContains(t, loaded.Packages, "github.com/old/pkg")
}

func TestWriteLock_EmptyPackages(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ct.lock")

	lf := packages.NewLockFile()
	require.NoError(t, packages.WriteLock(p, lf))

	data, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Contains(t, string(data), "version: 1")

	loaded, err := packages.ReadLock(p)
	require.NoError(t, err)
	assert.Empty(t, loaded.Packages)
}

func TestRoundTrip_ModifyThenWrite(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ct.lock")

	lf := packages.NewLockFile()
	lf.Packages["github.com/a/pkg"] = packages.LockEntry{Ref: "v1.0.0", SHA: "aaa"}
	require.NoError(t, packages.WriteLock(p, lf))

	loaded, err := packages.ReadLock(p)
	require.NoError(t, err)
	loaded.Packages["github.com/b/pkg"] = packages.LockEntry{Ref: "main", SHA: "bbb"}
	delete(loaded.Packages, "github.com/a/pkg")
	require.NoError(t, packages.WriteLock(p, loaded))

	final, err := packages.ReadLock(p)
	require.NoError(t, err)
	assert.Len(t, final.Packages, 1)
	assert.Contains(t, final.Packages, "github.com/b/pkg")
	assert.Equal(t, "bbb", final.Packages["github.com/b/pkg"].SHA)
}

func TestWriteLock_ManyPackagesSorted(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ct.lock")

	lf := packages.NewLockFile()
	names := []string{
		"github.com/z/z",
		"github.com/m/m",
		"github.com/a/a",
		"bitbucket.org/team/repo",
		"gitlab.com/org/lib",
	}
	for _, n := range names {
		lf.Packages[n] = packages.LockEntry{Ref: "main", SHA: "sha-" + n}
	}
	require.NoError(t, packages.WriteLock(p, lf))

	data, err := os.ReadFile(p)
	require.NoError(t, err)
	s := string(data)

	positions := make([]int, len(names))
	for i, n := range []string{
		"bitbucket.org/team/repo",
		"github.com/a/a",
		"github.com/m/m",
		"github.com/z/z",
		"gitlab.com/org/lib",
	} {
		positions[i] = indexOf(s, n)
		require.NotEqual(t, -1, positions[i], "package %s not found in output", n)
	}
	for i := 1; i < len(positions); i++ {
		assert.Less(t, positions[i-1], positions[i], "packages should be sorted alphabetically")
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
