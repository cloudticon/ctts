package k8s

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// waitLog writes directly to stderr so messages are visible even when the
// global log output is redirected (e.g. terminal mode in ct dev).
var waitLog = log.New(os.Stderr, "", log.LstdFlags)

var (
	fetchPreviousLogsFn = fetchPreviousLogs
	retrySleep          = defaultRetrySleep
)

var problemWaitingReasons = map[string]bool{
	"CrashLoopBackOff":           true,
	"ImagePullBackOff":           true,
	"ErrImagePull":               true,
	"CreateContainerConfigError": true,
}

func containerProblem(pod *corev1.Pod) string {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil && problemWaitingReasons[cs.State.Waiting.Reason] {
			return fmt.Sprintf("container %q is in %s", cs.Name, cs.State.Waiting.Reason)
		}
		if cs.State.Terminated != nil {
			return fmt.Sprintf("container %q has terminated (%s, exit code %d)",
				cs.Name, cs.State.Terminated.Reason, cs.State.Terminated.ExitCode)
		}
	}
	return ""
}

// WaitForPod waits until a healthy running pod matching selector is available.
func WaitForPod(ctx context.Context, c *Client, selector map[string]string) (string, error) {
	return waitForPod(ctx, c, selector)
}

func waitForPod(ctx context.Context, c *Client, selector map[string]string) (string, error) {
	if c.CoreV1 == nil {
		return "", errors.New("kubernetes core/v1 client is required")
	}

	labelSelector := labels.Set(selector).String()
	podsClient := c.CoreV1.Pods(c.Namespace)

	for {
		list, err := podsClient.List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return "", fmt.Errorf("listing pods for selector %q: %w", labelSelector, err)
		}

		name, problem := firstRunningPodName(list.Items)
		switch {
		case name != "" && problem == "":
			return name, nil
		case name != "":
			logContainerProblem(ctx, c, name, problem)
			if err := retrySleep(ctx); err != nil {
				return "", err
			}
			continue
		}

		// No running pods — fall back to watch (original behavior).
		watcher, err := podsClient.Watch(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return "", fmt.Errorf("watching pods for selector %q: %w", labelSelector, err)
		}

		for {
			select {
			case <-ctx.Done():
				watcher.Stop()
				return "", ctx.Err()
			case event, ok := <-watcher.ResultChan():
				if !ok {
					watcher.Stop()
					return "", errors.New("pod watch channel closed before a running pod appeared")
				}
				pod, ok := event.Object.(*corev1.Pod)
				if !ok || pod == nil || pod.DeletionTimestamp != nil {
					continue
				}
				if pod.Status.Phase == corev1.PodRunning {
					watcher.Stop()
					return pod.Name, nil
				}
			}
		}
	}
}

// firstRunningPodName returns the first healthy running pod. If all running
// pods have container problems, it returns the first problematic pod's name
// and the problem description. If no running pods exist, both values are empty.
func firstRunningPodName(pods []corev1.Pod) (name, problem string) {
	for i := range pods {
		pod := &pods[i]
		if pod.DeletionTimestamp != nil {
			continue
		}
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		if p := containerProblem(pod); p != "" {
			if name == "" {
				name = pod.Name
				problem = p
			}
			continue
		}
		return pod.Name, ""
	}
	return name, problem
}

func logContainerProblem(ctx context.Context, c *Client, podName, problem string) {
	waitLog.Printf("[wait] pod %q: %s, fetching crash logs...", podName, problem)
	if logs := fetchPreviousLogsFn(ctx, c, podName); logs != "" {
		waitLog.Printf("[wait] previous logs for %q:\n%s", podName, logs)
	}
	waitLog.Printf("[wait] retrying in 5s...")
}

func fetchPreviousLogs(ctx context.Context, c *Client, podName string) string {
	tailLines := int64(20)
	req := c.CoreV1.Pods(c.Namespace).GetLogs(podName, &corev1.PodLogOptions{
		Previous:  true,
		TailLines: &tailLines,
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return ""
	}
	defer stream.Close()
	data, err := io.ReadAll(stream)
	if err != nil {
		return ""
	}
	return string(data)
}

func defaultRetrySleep(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return nil
	}
}
