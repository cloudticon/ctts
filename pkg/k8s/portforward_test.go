package k8s

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortForward_ReconnectsAfterForwardError(t *testing.T) {
	origWaitForPodFn := waitForPodFn
	origForwardPortsFn := forwardPortsFn
	t.Cleanup(func() {
		waitForPodFn = origWaitForPodFn
		forwardPortsFn = origForwardPortsFn
	})

	waitCalls := 0
	waitForPodFn = func(_ context.Context, _ *Client, _ map[string]string) (string, error) {
		waitCalls++
		return "pod-1", nil
	}

	forwardCalls := 0
	forwardPortsFn = func(_ context.Context, _ *Client, _ string, _ []PortRule) error {
		forwardCalls++
		if forwardCalls == 1 {
			return errors.New("connection dropped")
		}
		return nil
	}

	err := PortForward(context.Background(), &Client{}, map[string]string{"app": "web"}, []PortRule{{Local: 3000, Remote: 3000}})
	require.NoError(t, err)
	assert.Equal(t, 2, waitCalls)
	assert.Equal(t, 2, forwardCalls)
}

func TestPortForward_GracefulOnContextCancel(t *testing.T) {
	origWaitForPodFn := waitForPodFn
	origForwardPortsFn := forwardPortsFn
	t.Cleanup(func() {
		waitForPodFn = origWaitForPodFn
		forwardPortsFn = origForwardPortsFn
	})

	ctx, cancel := context.WithCancel(context.Background())
	waitForPodFn = func(_ context.Context, _ *Client, _ map[string]string) (string, error) {
		return "pod-1", nil
	}
	forwardPortsFn = func(_ context.Context, _ *Client, _ string, _ []PortRule) error {
		cancel()
		return context.Canceled
	}

	err := PortForward(ctx, &Client{}, map[string]string{"app": "web"}, []PortRule{{Local: 3000, Remote: 3000}})
	require.NoError(t, err)
}

func TestPortForward_ReturnsWaitError(t *testing.T) {
	origWaitForPodFn := waitForPodFn
	origForwardPortsFn := forwardPortsFn
	t.Cleanup(func() {
		waitForPodFn = origWaitForPodFn
		forwardPortsFn = origForwardPortsFn
	})

	waitForPodFn = func(_ context.Context, _ *Client, _ map[string]string) (string, error) {
		return "", errors.New("no pods")
	}
	forwardPortsFn = func(_ context.Context, _ *Client, _ string, _ []PortRule) error {
		return nil
	}

	err := PortForward(context.Background(), &Client{}, map[string]string{"app": "web"}, []PortRule{{Local: 3000, Remote: 3000}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pods")
}

func TestPortForward_ValidatesInput(t *testing.T) {
	err := PortForward(context.Background(), nil, map[string]string{"app": "web"}, []PortRule{{Local: 3000, Remote: 3000}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client is required")

	err = PortForward(context.Background(), &Client{}, map[string]string{"app": "web"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one port rule is required")
}

func TestToPFPorts(t *testing.T) {
	ports := toPFPorts([]PortRule{
		{Local: 3000, Remote: 3000},
		{Local: 15432, Remote: 5432},
	})

	assert.Equal(t, []string{"3000:3000", "15432:5432"}, ports)
}
