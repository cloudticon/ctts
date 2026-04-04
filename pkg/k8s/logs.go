package k8s

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
)

var logColors = []string{"\033[36m", "\033[33m", "\033[32m", "\033[35m", "\033[34m"}

var (
	waitForPodForLogsFn      = waitForPod
	streamPodLogsForLogsFn   = streamPodLogs
	sleepForLogReconnectsFn  = time.Sleep
)

// StreamLogs streams pod logs for a selected target and reconnects on pod/log stream churn.
func StreamLogs(ctx context.Context, c *Client, targetName string, selector map[string]string, w io.Writer) error {
	if c == nil {
		return errors.New("client is required")
	}
	if w == nil {
		return errors.New("writer is required")
	}

	prefix := logPrefix(targetName)
	for {
		pod, err := waitForPodForLogsFn(ctx, c, selector)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		stream, err := streamPodLogsForLogsFn(ctx, c, pod)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("[logs] failed to open stream for %s (%s), reconnecting: %v", targetName, pod, err)
			sleepForLogReconnectsFn(2 * time.Second)
			continue
		}

		scanner := bufio.NewScanner(stream)
		for scanner.Scan() {
			fmt.Fprintf(w, "%s%s\n", prefix, scanner.Text())
		}
		_ = stream.Close()

		if scanErr := scanner.Err(); scanErr != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("[logs] stream interrupted for %s (%s), reconnecting: %v", targetName, pod, scanErr)
			sleepForLogReconnectsFn(2 * time.Second)
			continue
		}

		if ctx.Err() != nil {
			return nil
		}
	}
}

func streamPodLogs(ctx context.Context, c *Client, pod string) (io.ReadCloser, error) {
	if c.CoreV1 == nil {
		return nil, errors.New("kubernetes core/v1 client is required")
	}

	req := c.CoreV1.Pods(c.Namespace).GetLogs(pod, &corev1.PodLogOptions{
		Follow: true,
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("opening logs stream for pod %s: %w", pod, err)
	}
	return stream, nil
}

func logPrefix(targetName string) string {
	color := logColors[int(hashString(targetName))%len(logColors)]
	reset := "\033[0m"
	return fmt.Sprintf("%s[%s]%s ", color, targetName, reset)
}

func hashString(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}
