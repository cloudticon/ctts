package k8s

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// PortRule describes a single local->remote forwarding rule.
type PortRule struct {
	Local  int
	Remote int
}

var (
	waitForPodFn   = waitForPod
	forwardPortsFn = forwardPorts
)

// WaitForPod waits until a running pod matching selector is available.
func WaitForPod(ctx context.Context, c *Client, selector map[string]string) (string, error) {
	return waitForPod(ctx, c, selector)
}

// PortForward starts port forwarding for the selected workload and reconnects on connection loss.
func PortForward(ctx context.Context, c *Client, selector map[string]string, ports []PortRule) error {
	if c == nil {
		return errors.New("client is required")
	}
	if len(ports) == 0 {
		return errors.New("at least one port rule is required")
	}

	for {
		pod, err := waitForPodFn(ctx, c, selector)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		if err := forwardPortsFn(ctx, c, pod, ports); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("[port-forward] connection lost for pod %s, reconnecting: %v", pod, err)
			continue
		}
		return nil
	}
}

func waitForPod(ctx context.Context, c *Client, selector map[string]string) (string, error) {
	if c.Clientset == nil {
		return "", errors.New("kubernetes clientset is required")
	}

	labelSelector := labels.Set(selector).String()
	podsClient := c.Clientset.CoreV1().Pods(c.Namespace)

	list, err := podsClient.List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return "", fmt.Errorf("listing pods for selector %q: %w", labelSelector, err)
	}
	if podName, ok := firstRunningPodName(list.Items); ok {
		return podName, nil
	}

	watcher, err := podsClient.Watch(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return "", fmt.Errorf("watching pods for selector %q: %w", labelSelector, err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return "", errors.New("pod watch channel closed before a running pod appeared")
			}
			pod, ok := event.Object.(*corev1.Pod)
			if !ok || pod == nil {
				continue
			}
			if pod.Status.Phase == corev1.PodRunning {
				return pod.Name, nil
			}
		}
	}
}

func forwardPorts(ctx context.Context, c *Client, pod string, ports []PortRule) error {
	if c.Config == nil {
		return errors.New("rest config is required for port-forwarding")
	}
	if c.Clientset == nil {
		return errors.New("kubernetes clientset is required for port-forwarding")
	}

	reqURL := c.Clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Namespace(c.Namespace).
		Name(pod).
		SubResource("portforward").
		URL()

	transport, upgrader, err := spdy.RoundTripperFor(c.Config)
	if err != nil {
		return fmt.Errorf("creating spdy round tripper: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, reqURL)
	stopCh := make(chan struct{})
	readyCh := make(chan struct{})

	go func() {
		select {
		case <-ctx.Done():
			close(stopCh)
		case <-stopCh:
		}
	}()

	forwarder, err := portforward.New(dialer, toPFPorts(ports), stopCh, readyCh, io.Discard, io.Discard)
	if err != nil {
		return fmt.Errorf("creating port forwarder: %w", err)
	}

	go func() {
		<-readyCh
		for _, p := range ports {
			log.Printf("[port-forward] localhost:%d -> %s:%d", p.Local, pod, p.Remote)
		}
	}()

	if err := forwarder.ForwardPorts(); err != nil {
		return fmt.Errorf("forwarding ports for pod %s: %w", pod, err)
	}

	return nil
}

func toPFPorts(ports []PortRule) []string {
	result := make([]string, 0, len(ports))
	for _, p := range ports {
		result = append(result, fmt.Sprintf("%d:%d", p.Local, p.Remote))
	}
	return result
}

func firstRunningPodName(pods []corev1.Pod) (string, bool) {
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodRunning {
			return pod.Name, true
		}
	}
	return "", false
}
