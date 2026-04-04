package k8s

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

var (
	waitForPodForExecFn    = waitForPod
	execStreamRunnerFn     = ExecStream
	buildExecURLFn         = buildExecURL
	newExecExecutorForURL  = remotecommand.NewSPDYExecutor
	parameterCodec runtime.ParameterCodec = scheme.ParameterCodec
)

// ExecStreamOpts configures Kubernetes exec stream behavior.
type ExecStreamOpts struct {
	Container string

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	TTY bool
}

// Exec runs a shell command in the first running pod matching selector and attaches local stdio.
func Exec(ctx context.Context, c *Client, selector map[string]string, command string) error {
	if c == nil {
		return errors.New("client is required")
	}

	pod, err := waitForPodForExecFn(ctx, c, selector)
	if err != nil {
		return err
	}

	return execStreamRunnerFn(ctx, c, pod, []string{"/bin/sh", "-c", command}, ExecStreamOpts{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		TTY:    true,
	})
}

// ExecSimple runs a command without TTY and discards output.
func ExecSimple(ctx context.Context, c *Client, pod string, cmd []string) error {
	return execStreamRunnerFn(ctx, c, pod, cmd, ExecStreamOpts{
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
}

// ExecStream executes a command in a pod and streams I/O according to options.
func ExecStream(ctx context.Context, c *Client, pod string, cmd []string, opts ExecStreamOpts) error {
	if c == nil {
		return errors.New("client is required")
	}
	if c.Config == nil {
		return errors.New("rest config is required")
	}
	if pod == "" {
		return errors.New("pod name is required")
	}
	if len(cmd) == 0 {
		return errors.New("command is required")
	}

	reqURL, err := buildExecURLFn(c, pod, cmd, opts)
	if err != nil {
		return err
	}

	executor, err := newExecExecutorForURL(c.Config, http.MethodPost, reqURL)
	if err != nil {
		return fmt.Errorf("creating exec executor: %w", err)
	}

	if err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  opts.Stdin,
		Stdout: opts.Stdout,
		Stderr: opts.Stderr,
		Tty:    opts.TTY,
	}); err != nil {
		return fmt.Errorf("executing command in pod %s: %w", pod, err)
	}
	return nil
}

func buildExecURL(c *Client, pod string, cmd []string, opts ExecStreamOpts) (*url.URL, error) {
	if c.Clientset == nil {
		return nil, errors.New("kubernetes clientset is required")
	}

	req := c.Clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Namespace(c.Namespace).
		Name(pod).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: opts.Container,
		Command:   cmd,
		Stdin:     opts.Stdin != nil,
		Stdout:    opts.Stdout != nil,
		Stderr:    opts.Stderr != nil,
		TTY:       opts.TTY,
	}, parameterCodec)

	return req.URL(), nil
}
