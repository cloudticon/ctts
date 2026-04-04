package k8s

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type readCloserWithCloseFn struct {
	io.Reader
	closeFn func() error
}

func (r *readCloserWithCloseFn) Close() error {
	if r.closeFn != nil {
		return r.closeFn()
	}
	return nil
}

func TestStreamLogs_ReconnectsAndWritesPrefixedLines(t *testing.T) {
	origWait := waitForPodForLogsFn
	origStream := streamPodLogsForLogsFn
	origSleep := sleepForLogReconnectsFn
	t.Cleanup(func() {
		waitForPodForLogsFn = origWait
		streamPodLogsForLogsFn = origStream
		sleepForLogReconnectsFn = origSleep
	})

	sleepForLogReconnectsFn = func(time.Duration) {}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	waitForPodForLogsFn = func(_ context.Context, _ *Client, _ map[string]string) (string, error) {
		return "pod-1", nil
	}

	streamCalls := 0
	streamPodLogsForLogsFn = func(_ context.Context, _ *Client, _ string) (io.ReadCloser, error) {
		streamCalls++
		switch streamCalls {
		case 1:
			return nil, errors.New("temporary stream error")
		case 2:
			return &readCloserWithCloseFn{
				Reader: strings.NewReader("line-a\nline-b\n"),
				closeFn: func() error {
					cancel()
					return nil
				},
			}, nil
		default:
			return nil, context.Canceled
		}
	}

	var out bytes.Buffer
	err := StreamLogs(ctx, &Client{}, "remix", map[string]string{"app": "remix"}, &out)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, streamCalls, 2)
	assert.Contains(t, out.String(), "[remix]")
	assert.Contains(t, out.String(), "line-a")
	assert.Contains(t, out.String(), "line-b")
}

func TestStreamLogs_ReturnsWaitError(t *testing.T) {
	origWait := waitForPodForLogsFn
	t.Cleanup(func() {
		waitForPodForLogsFn = origWait
	})

	waitForPodForLogsFn = func(_ context.Context, _ *Client, _ map[string]string) (string, error) {
		return "", errors.New("no pod found")
	}

	var out bytes.Buffer
	err := StreamLogs(context.Background(), &Client{}, "remix", map[string]string{"app": "remix"}, &out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pod found")
}

func TestStreamLogs_ValidatesInput(t *testing.T) {
	err := StreamLogs(context.Background(), nil, "remix", nil, io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client is required")

	err = StreamLogs(context.Background(), &Client{}, "remix", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "writer is required")
}

func TestLogPrefix_IsStableForTarget(t *testing.T) {
	prefixA := logPrefix("service-a")
	prefixB := logPrefix("service-a")
	assert.Equal(t, prefixA, prefixB)
	assert.Contains(t, prefixA, "[service-a]")
}
