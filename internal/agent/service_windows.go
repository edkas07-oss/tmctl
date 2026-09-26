//go:build windows

package agent

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/eddywiyatno/tmctl/pkg/termutil"
	"golang.org/x/sys/windows/svc"
)

type windowsService struct {
	collector *Collector
}

func (m *windowsService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Notify SCM that service is Running immediately
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	errCh := make(chan error, 1)
	go func() {
		errCh <- m.collector.Run(ctx)
	}()

loop:
	for {
		select {
		case err := <-errCh:
			if err != nil {
				termutil.Error("Service execution failed: %v", err)
			}
			break loop
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				break loop
			default:
				termutil.Warn("Unexpected service control request #%d", c.Cmd)
			}
		}
	}

	changes <- svc.Status{State: svc.Stopped}
	return
}

// RunService runs the collector daemon on Windows either as a console app or as a Windows Service.
func RunService(c *Collector) error {
	isService, err := svc.IsWindowsService()
	if err == nil && isService {
		return svc.Run("TomcatMonitoringAgent", &windowsService{collector: c})
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	return c.Run(ctx)
}
