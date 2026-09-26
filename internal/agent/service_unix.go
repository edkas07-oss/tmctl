//go:build !windows

package agent

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// RunService runs the collector daemon on Unix systems with signal handling.
func RunService(c *Collector) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	return c.Run(ctx)
}
