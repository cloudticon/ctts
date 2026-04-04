package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWatcher_ValidatesRoot(t *testing.T) {
	_, err := NewWatcher("", nil, false)
	require.Error(t, err)

	filePath := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("x"), 0o644))
	_, err = NewWatcher(filePath, nil, false)
	require.Error(t, err)
}

func TestPathExcluder_GitignoreLikeRules(t *testing.T) {
	excluder, err := newPathExcluder([]string{
		"/node_modules",
		"*.log",
		"**/*.tmp",
		"!important.log",
	})
	require.NoError(t, err)

	assert.True(t, excluder.IsExcluded("node_modules/pkg/index.js", false))
	assert.True(t, excluder.IsExcluded("logs/app.log", false))
	assert.True(t, excluder.IsExcluded("a/b/file.tmp", false))
	assert.False(t, excluder.IsExcluded("important.log", false))
	assert.False(t, excluder.IsExcluded("src/main.ts", false))
}

func TestWatcherPolling_EmitsCreateAndDelete(t *testing.T) {
	root := t.TempDir()

	w, err := NewWatcher(root, []string{"*.tmp"}, true)
	require.NoError(t, err)
	w.pollEvery = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ch := w.Watch(ctx)
	time.Sleep(120 * time.Millisecond)

	target := filepath.Join(root, "notes.txt")
	require.NoError(t, os.WriteFile(target, []byte("hello"), 0o644))

	batch := waitBatch(t, ch)
	assert.Contains(t, batch, FileChange{Path: "notes.txt", Type: ChangeCreate})

	require.NoError(t, os.Remove(target))
	batch = waitBatch(t, ch)
	assert.Contains(t, batch, FileChange{Path: "notes.txt", Type: ChangeDelete})
}

func TestWatcherFsnotify_DebouncesRapidChanges(t *testing.T) {
	root := t.TempDir()
	w, err := NewWatcher(root, nil, false)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := w.Watch(ctx)
	target := filepath.Join(root, "rapid.txt")

	require.NoError(t, os.WriteFile(target, []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(target, []byte("b"), 0o644))
	require.NoError(t, os.WriteFile(target, []byte("c"), 0o644))

	batch := waitBatch(t, ch)

	count := 0
	for _, c := range batch {
		if c.Path == "rapid.txt" {
			count++
		}
	}
	assert.Equal(t, 1, count, "expected debounced single change for rapid writes")
}

func waitBatch(t *testing.T, ch <-chan []FileChange) []FileChange {
	t.Helper()
	select {
	case batch := <-ch:
		return batch
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for watcher batch")
		return nil
	}
}
