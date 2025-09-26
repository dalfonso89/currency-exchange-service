package platform

import (
	"context"
	"os"
	"os/signal"
)

// NewShutdownContext creates a context that is canceled on Windows Ctrl+C (os.Interrupt).
// Windows does not reliably deliver SIGTERM to console apps, so we only listen for os.Interrupt.
func NewShutdownContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt)
}


