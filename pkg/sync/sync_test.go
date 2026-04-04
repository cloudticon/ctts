package sync

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudticon/ctts/pkg/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectFiles_RespectsExclude(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "node_modules", "pkg"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "src", "main.ts"), []byte("ok"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "debug.log"), []byte("skip"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "node_modules", "pkg", "x.js"), []byte("skip"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "important.log"), []byte("keep"), 0o644))

	files, err := collectFiles(root, []string{"/node_modules", "*.log", "!important.log"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"important.log", "src/main.ts"}, files)
}

func TestWriteTarFromFiles_WritesExpectedEntries(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "src", "a.txt"), []byte("A"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "src", "b.txt"), []byte("BB"), 0o644))

	var buf bytes.Buffer
	size, err := writeTarFromFiles(&buf, root, []string{"src/a.txt", "src/b.txt"})
	require.NoError(t, err)
	assert.Equal(t, int64(3), size)

	tr := tar.NewReader(bytes.NewReader(buf.Bytes()))
	names := make([]string, 0, 2)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		names = append(names, h.Name)
	}
	assert.ElementsMatch(t, []string{"src/a.txt", "src/b.txt"}, names)
}

func TestSyncerIncrementalSync_TarsChangedAndDeletesRemoved(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "src", "app.js"), []byte("console.log(1)"), 0o644))

	origExecStreamFn := execStreamFn
	origExecSimpleFn := execSimpleFn
	t.Cleanup(func() {
		execStreamFn = origExecStreamFn
		execSimpleFn = origExecSimpleFn
	})

	streamCalls := 0
	simpleCalls := 0
	var streamCmd []string
	var deleteCmd []string

	execStreamFn = func(_ context.Context, _ *k8s.Client, _ string, cmd []string, stdin io.Reader) error {
		streamCalls++
		streamCmd = append([]string(nil), cmd...)
		data, err := io.ReadAll(stdin)
		require.NoError(t, err)
		assert.NotEmpty(t, data)
		return nil
	}
	execSimpleFn = func(_ context.Context, _ *k8s.Client, _ string, cmd []string) error {
		simpleCalls++
		deleteCmd = append([]string(nil), cmd...)
		return nil
	}

	s := &Syncer{
		client:  &k8s.Client{},
		rule:    SyncRule{From: root, To: "/app"},
		podName: "pod-1",
	}

	err := s.incrementalSync(context.Background(), []FileChange{
		{Path: "src/app.js", Type: ChangeModify},
		{Path: "src/old.js", Type: ChangeDelete},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, streamCalls)
	assert.Equal(t, []string{"tar", "xf", "-", "-C", "/app"}, streamCmd)
	assert.Equal(t, 1, simpleCalls)
	assert.Equal(t, []string{"rm", "-rf", "/app/src/old.js"}, deleteCmd)
}

func TestSyncerRun_ValidatesInput(t *testing.T) {
	s := NewSyncer(nil, nil, SyncRule{From: ".", To: "/app"})
	err := s.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "k8s client is required")
}

func TestSyncerRun_CallsInitialAndExitsOnCancel(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o644))

	origWaitForPodFn := waitForPodFn
	origExecStreamFn := execStreamFn
	t.Cleanup(func() {
		waitForPodFn = origWaitForPodFn
		execStreamFn = origExecStreamFn
	})

	waitForPodFn = func(_ context.Context, _ *k8s.Client, _ map[string]string) (string, error) {
		return "pod-1", nil
	}
	execStreamFn = func(_ context.Context, _ *k8s.Client, _ string, _ []string, _ io.Reader) error {
		return nil
	}

	s := NewSyncer(&k8s.Client{}, map[string]string{"app": "x"}, SyncRule{
		From:    root,
		To:      "/app",
		Polling: true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- s.Run(ctx)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("syncer run did not exit after context cancel")
	}
}
