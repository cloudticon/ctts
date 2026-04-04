package k8s

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

var (
	waitForPodForExecFn   = waitForPod
	execStreamRunnerFn    = ExecStream
	buildExecURLFn        = buildExecURL
	newExecExecutorForURL = remotecommand.NewSPDYExecutor
	parameterCodec        runtime.ParameterCodec = scheme.ParameterCodec

	makeRawFn     = func(fd int) (*term.State, error) { return term.MakeRaw(fd) }
	restoreTermFn = func(fd int, oldState *term.State) error { return term.Restore(fd, oldState) }
	getTermSizeFn = func(fd int) (int, int, error) { return term.GetSize(fd) }
)

// ExecStreamOpts configures Kubernetes exec stream behavior.
type ExecStreamOpts struct {
	Container string

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	TTY               bool
	TerminalSizeQueue remotecommand.TerminalSizeQueue
}

// termSizeQueue delivers terminal resize events to the remote shell.
type termSizeQueue struct {
	resizeCh chan remotecommand.TerminalSize
	sigCh    chan os.Signal
	done     chan struct{}
}

func newTermSizeQueue(fd int) *termSizeQueue {
	q := &termSizeQueue{
		resizeCh: make(chan remotecommand.TerminalSize, 1),
		sigCh:    make(chan os.Signal, 1),
		done:     make(chan struct{}),
	}

	if w, h, err := getTermSizeFn(fd); err == nil {
		q.resizeCh <- remotecommand.TerminalSize{Width: uint16(w), Height: uint16(h)}
	}

	signal.Notify(q.sigCh, syscall.SIGWINCH)
	go q.monitor(fd)
	return q
}

func (q *termSizeQueue) monitor(fd int) {
	defer signal.Stop(q.sigCh)
	for {
		select {
		case <-q.sigCh:
			if w, h, err := getTermSizeFn(fd); err == nil {
				select {
				case q.resizeCh <- remotecommand.TerminalSize{Width: uint16(w), Height: uint16(h)}:
				default:
				}
			}
		case <-q.done:
			return
		}
	}
}

func (q *termSizeQueue) Next() *remotecommand.TerminalSize {
	select {
	case size, ok := <-q.resizeCh:
		if !ok {
			return nil
		}
		return &size
	case <-q.done:
		return nil
	}
}

func (q *termSizeQueue) stop() {
	select {
	case <-q.done:
	default:
		close(q.done)
	}
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

	fd := int(os.Stdin.Fd())
	oldState, err := makeRawFn(fd)
	if err != nil {
		return fmt.Errorf("setting terminal raw mode: %w", err)
	}
	defer restoreTermFn(fd, oldState)

	sizeQueue := newTermSizeQueue(fd)
	defer sizeQueue.stop()

	return execStreamRunnerFn(ctx, c, pod, []string{"/bin/sh", "-c", command}, ExecStreamOpts{
		Stdin:             os.Stdin,
		Stdout:            os.Stdout,
		Stderr:            os.Stderr,
		TTY:               true,
		TerminalSizeQueue: sizeQueue,
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

	streamOpts := remotecommand.StreamOptions{
		Stdin:  opts.Stdin,
		Stdout: opts.Stdout,
		Stderr: opts.Stderr,
		Tty:    opts.TTY,
	}
	if opts.TerminalSizeQueue != nil {
		streamOpts.TerminalSizeQueue = opts.TerminalSizeQueue
	}

	if err := executor.StreamWithContext(ctx, streamOpts); err != nil {
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
