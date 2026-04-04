package k8s

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

type fakeStreamExecutor struct {
	called bool
	opts   remotecommand.StreamOptions
	err    error
}

func (f *fakeStreamExecutor) Stream(opts remotecommand.StreamOptions) error {
	return f.StreamWithContext(context.Background(), opts)
}

func (f *fakeStreamExecutor) StreamWithContext(_ context.Context, opts remotecommand.StreamOptions) error {
	f.called = true
	f.opts = opts
	return f.err
}

func TestExec_ResolvesPodAndRunsShellCommand(t *testing.T) {
	origWait := waitForPodForExecFn
	origRunner := execStreamRunnerFn
	t.Cleanup(func() {
		waitForPodForExecFn = origWait
		execStreamRunnerFn = origRunner
	})

	waitForPodForExecFn = func(_ context.Context, _ *Client, _ map[string]string) (string, error) {
		return "pod-1", nil
	}

	called := false
	var gotPod string
	var gotCmd []string
	var gotOpts ExecStreamOpts
	execStreamRunnerFn = func(_ context.Context, _ *Client, pod string, cmd []string, opts ExecStreamOpts) error {
		called = true
		gotPod = pod
		gotCmd = append([]string(nil), cmd...)
		gotOpts = opts
		return nil
	}

	err := Exec(context.Background(), &Client{}, map[string]string{"app": "web"}, "npm run dev")
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "pod-1", gotPod)
	assert.Equal(t, []string{"/bin/sh", "-c", "npm run dev"}, gotCmd)
	assert.True(t, gotOpts.TTY)
}

func TestExec_ReturnsWaitForPodError(t *testing.T) {
	origWait := waitForPodForExecFn
	t.Cleanup(func() {
		waitForPodForExecFn = origWait
	})

	waitForPodForExecFn = func(_ context.Context, _ *Client, _ map[string]string) (string, error) {
		return "", errors.New("cannot find pod")
	}

	err := Exec(context.Background(), &Client{}, map[string]string{"app": "web"}, "bash")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot find pod")
}

func TestExecStream_BuildsExecutorAndStreams(t *testing.T) {
	origBuildURL := buildExecURLFn
	origNewExecutor := newExecExecutorForURL
	t.Cleanup(func() {
		buildExecURLFn = origBuildURL
		newExecExecutorForURL = origNewExecutor
	})

	buildExecURLFn = func(_ *Client, _ string, _ []string, _ ExecStreamOpts) (*url.URL, error) {
		return url.Parse("https://example.invalid/api/v1/namespaces/dev/pods/pod-1/exec")
	}

	fakeExec := &fakeStreamExecutor{}
	newExecExecutorForURL = func(_ *rest.Config, method string, reqURL *url.URL) (remotecommand.Executor, error) {
		assert.Equal(t, http.MethodPost, method)
		assert.Equal(t, "https://example.invalid/api/v1/namespaces/dev/pods/pod-1/exec", reqURL.String())
		return fakeExec, nil
	}

	in := bytes.NewBufferString("input")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	err := ExecStream(context.Background(), &Client{Config: &rest.Config{}}, "pod-1", []string{"echo", "ok"}, ExecStreamOpts{
		Stdin:  in,
		Stdout: out,
		Stderr: errOut,
		TTY:    true,
	})
	require.NoError(t, err)
	assert.True(t, fakeExec.called)
	assert.Equal(t, in, fakeExec.opts.Stdin)
	assert.Equal(t, out, fakeExec.opts.Stdout)
	assert.Equal(t, errOut, fakeExec.opts.Stderr)
	assert.True(t, fakeExec.opts.Tty)
}

func TestExecStream_ValidatesInput(t *testing.T) {
	err := ExecStream(context.Background(), nil, "pod-1", []string{"echo"}, ExecStreamOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client is required")

	err = ExecStream(context.Background(), &Client{}, "pod-1", []string{"echo"}, ExecStreamOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rest config is required")

	err = ExecStream(context.Background(), &Client{Config: &rest.Config{}}, "", []string{"echo"}, ExecStreamOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pod name is required")

	err = ExecStream(context.Background(), &Client{Config: &rest.Config{}}, "pod-1", nil, ExecStreamOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "command is required")
}

func TestExecSimple_UsesNonTTYDefaults(t *testing.T) {
	origRunner := execStreamRunnerFn
	t.Cleanup(func() {
		execStreamRunnerFn = origRunner
	})

	got := ExecStreamOpts{}
	execStreamRunnerFn = func(_ context.Context, _ *Client, _ string, _ []string, opts ExecStreamOpts) error {
		got = opts
		return nil
	}

	err := ExecSimple(context.Background(), &Client{}, "pod-1", []string{"ls"})
	require.NoError(t, err)
	assert.False(t, got.TTY)
	assert.NotNil(t, got.Stdout)
	assert.NotNil(t, got.Stderr)
}
