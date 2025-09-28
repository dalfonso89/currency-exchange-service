//go:build !windows
// +build !windows

package platform

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// NewShutdownContext creates a context that is canceled on SIGINT or SIGTERM (Unix).
func NewShutdownContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}
