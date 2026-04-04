//go:build windows

package k8s

import "os"

func notifyResize(_ chan<- os.Signal) {}

func stopResize(_ chan<- os.Signal) {}
