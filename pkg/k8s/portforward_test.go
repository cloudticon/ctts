package k8s

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
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

func TestWaitForPod_ReturnsRunningPodFromList(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-pending", Namespace: "dev-ns", Labels: map[string]string{"app": "web"}},
			Status:     corev1.PodStatus{Phase: corev1.PodPending},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-running", Namespace: "dev-ns", Labels: map[string]string{"app": "web"}},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
	)
	client := NewClientFromInterfaces(clientset, nil, "dev-ns")

	pod, err := waitForPod(context.Background(), client, map[string]string{"app": "web"})
	require.NoError(t, err)
	assert.Equal(t, "pod-running", pod)
}

func TestWaitForPod_WaitsForRunningPodOnWatch(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	w := watch.NewFake()

	clientset.Fake.PrependWatchReactor("pods", func(action k8stesting.Action) (bool, watch.Interface, error) {
		return true, w, nil
	})

	client := NewClientFromInterfaces(clientset, nil, "dev-ns")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resultCh := make(chan struct {
		pod string
		err error
	}, 1)
	go func() {
		pod, err := waitForPod(ctx, client, map[string]string{"app": "web"})
		resultCh <- struct {
			pod string
			err error
		}{pod: pod, err: err}
	}()

	w.Add(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-running", Namespace: "dev-ns", Labels: map[string]string{"app": "web"}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	})

	select {
	case result := <-resultCh:
		require.NoError(t, result.err)
		assert.Equal(t, "pod-running", result.pod)
	case <-time.After(2 * time.Second):
		t.Fatal("waitForPod did not return after running pod event")
	}
}

func TestToPFPorts(t *testing.T) {
	ports := toPFPorts([]PortRule{
		{Local: 3000, Remote: 3000},
		{Local: 15432, Remote: 5432},
	})

	assert.Equal(t, []string{"3000:3000", "15432:5432"}, ports)
}
