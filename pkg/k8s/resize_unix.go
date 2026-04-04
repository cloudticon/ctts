//go:build !windows

package k8s

import (
	"os"
	"os/signal"
	"syscall"
)

func notifyResize(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGWINCH)
}

func stopResize(ch chan<- os.Signal) {
	signal.Stop(ch)
}
