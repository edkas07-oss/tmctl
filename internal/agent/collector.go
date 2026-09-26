package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/eddywiyatno/tmctl/internal/engine"
	"github.com/eddywiyatno/tmctl/pkg/termutil"
)

// Relevant lifecycle actions to trigger snapshot.
var relevantActions = map[string]bool{
	"died":    true,
	"die":     true,
	"stop":    true,
	"start":   true,
	"unpause": true,
	"oom":     true,
	"kill":    true,
	"restart": true,
}

// Config represents runtime parameters for the telemetry collector.
type Config struct {
	TargetContainer    string
	TargetID           string
	SpoolDir           string
	Generation         int
	MaxSpoolAgeHours   int
	MaxSpoolFiles      int
	StaleTmpAgeMinutes int
	MaxRecordBytes     int64
	RunOnce            bool
	Engine             string
	SocketPath         string
}

// DefaultConfig returns baseline configuration for agent collector.
func DefaultConfig() *Config {
	return &Config{
		TargetContainer:    "tomcat-jmx-exporter",
		TargetID:           "default/tomcat",
		SpoolDir:           "/opt/tm-home/spool",
		Generation:         1,
		MaxSpoolAgeHours:   24,
		MaxSpoolFiles:      1000,
		StaleTmpAgeMinutes: 60,
		MaxRecordBytes:     16384,
		RunOnce:            false,
		Engine:             "auto",
		SocketPath:         "",
	}
}

// Collector orchestrates container event streaming, snapshot generation, and spool retention.
type Collector struct {
	cfg          *Config
	engineClient engine.EngineClient
}

// NewCollector initializes a new Collector instance.
func NewCollector(cfg *Config, engineClient engine.EngineClient) *Collector {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Collector{
		cfg:          cfg,
		engineClient: engineClient,
	}
}

// RecordSnapshot inspects the target container and writes canonical evidence records to spool.
func (c *Collector) RecordSnapshot(ctx context.Context) ([]string, error) {
	now := time.Now().UTC()
	var writtenFiles []string

	inspect, err := c.engineClient.InspectContainer(ctx, c.cfg.TargetContainer)
	if err != nil {
		// Engine communication failure
		rec := NewEventRecord(
			"collector_status",
			c.cfg.TargetID,
			c.cfg.Generation,
			StatusUnavailable,
			StrengthContextual,
			CollectorStatusValue{Error: fmt.Sprintf("container_engine_error: %v", err)},
			now,
		)
		path, wErr := WriteRecord(c.cfg.SpoolDir, rec, c.cfg.MaxRecordBytes)
		if wErr != nil {
			return nil, fmt.Errorf("failed to write collector_status record: %w", wErr)
		}
		writtenFiles = append(writtenFiles, path)
	} else if inspect == nil {
		// Container not found
		rec := NewEventRecord(
			"container_state",
			c.cfg.TargetID,
			c.cfg.Generation,
			StatusNotFound,
			StrengthContextual,
			ContainerStateValue{
				State:     "not_found",
				Container: c.cfg.TargetContainer,
			},
			now,
		)
		path, wErr := WriteRecord(c.cfg.SpoolDir, rec, c.cfg.MaxRecordBytes)
		if wErr != nil {
			return nil, fmt.Errorf("failed to write not_found record: %w", wErr)
		}
		writtenFiles = append(writtenFiles, path)
	} else {
		// Container found - write container_state
		stateStr := "exited"
		if inspect.State.Running {
			stateStr = "running"
		}

		stateRec := NewEventRecord(
			"container_state",
			c.cfg.TargetID,
			c.cfg.Generation,
			StatusCollected,
			StrengthDirect,
			ContainerStateValue{State: stateStr},
			now,
		)
		path1, wErr := WriteRecord(c.cfg.SpoolDir, stateRec, c.cfg.MaxRecordBytes)
		if wErr != nil {
			return nil, fmt.Errorf("failed to write container_state record: %w", wErr)
		}
		writtenFiles = append(writtenFiles, path1)

		// Write runtime_oom
		oomRec := NewEventRecord(
			"runtime_oom",
			c.cfg.TargetID,
			c.cfg.Generation,
			StatusCollected,
			StrengthDirect,
			RuntimeOOMValue{
				OOMKilled: inspect.State.OOMKilled,
				ExitCode:  inspect.State.ExitCode,
			},
			now,
		)
		path2, wErr := WriteRecord(c.cfg.SpoolDir, oomRec, c.cfg.MaxRecordBytes)
		if wErr != nil {
			return nil, fmt.Errorf("failed to write runtime_oom record: %w", wErr)
		}
		writtenFiles = append(writtenFiles, path2)
	}

	// Run retention pruning cycle
	_, _ = PruneSpool(c.cfg.SpoolDir, c.cfg.MaxSpoolAgeHours, c.cfg.MaxSpoolFiles, c.cfg.StaleTmpAgeMinutes)

	return writtenFiles, nil
}

// Run executes the collector lifecycle.
func (c *Collector) Run(ctx context.Context) error {
	termutil.Info("Starting tmctl Host Telemetry & Event Collector Daemon")
	termutil.Info("Target Workload : %s (%s)", c.cfg.TargetID, c.cfg.TargetContainer)
	termutil.Info("Spool Directory : %s", c.cfg.SpoolDir)
	if c.engineClient != nil && c.engineClient.GetInfo() != nil {
		termutil.Info("Container Engine: %s (%s)", c.engineClient.GetInfo().EngineType, c.engineClient.GetInfo().SocketPath)
	}

	// 1. Ensure spool directory exists
	if err := EnsureSpoolDir(c.cfg.SpoolDir); err != nil {
		return fmt.Errorf("failed to initialize spool directory: %w", err)
	}

	// 2. Initial retention pruning
	report, err := PruneSpool(c.cfg.SpoolDir, c.cfg.MaxSpoolAgeHours, c.cfg.MaxSpoolFiles, c.cfg.StaleTmpAgeMinutes)
	if err == nil && (report.StaleTmpPruned > 0 || report.StaleJsonPruned > 0 || report.QuotaPruned > 0) {
		termutil.Info("Startup Spool Pruning: %d stale tmp, %d expired json, %d quota pruned (active: %d)",
			report.StaleTmpPruned, report.StaleJsonPruned, report.QuotaPruned, report.TotalRemaining)
	}

	// 3. Record initial snapshot
	files, err := c.RecordSnapshot(ctx)
	if err != nil {
		termutil.Warn("Initial snapshot warning: %v", err)
	} else {
		termutil.Success("Initial snapshot recorded (%d evidence files written)", len(files))
	}

	if c.cfg.RunOnce {
		termutil.Info("One-shot snapshot execution completed successfully")
		return nil
	}

	termutil.Info("Subscribing to real-time Container Engine socket event stream...")

	// Periodic housekeeping ticker (every 10 minutes)
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	// Event streaming retry loop
	for {
		select {
		case <-ctx.Done():
			termutil.Info("tmctl agent shutting down gracefully")
			return nil
		default:
		}

		eventCh, errCh := c.engineClient.StreamEvents(ctx, c.cfg.TargetContainer)

	streamLoop:
		for {
			select {
			case <-ctx.Done():
				termutil.Info("tmctl agent received stop signal")
				return nil

			case <-ticker.C:
				// Periodic housekeeping
				r, _ := PruneSpool(c.cfg.SpoolDir, c.cfg.MaxSpoolAgeHours, c.cfg.MaxSpoolFiles, c.cfg.StaleTmpAgeMinutes)
				if r != nil && (r.StaleTmpPruned > 0 || r.StaleJsonPruned > 0 || r.QuotaPruned > 0) {
					termutil.Info("Housekeeping: pruned %d tmp, %d expired json, %d quota (active: %d)",
						r.StaleTmpPruned, r.StaleJsonPruned, r.QuotaPruned, r.TotalRemaining)
				}

			case ev, ok := <-eventCh:
				if !ok {
					break streamLoop
				}

				action := strings.ToLower(ev.GetAction())
				containerName := ev.GetContainerName()

				if (containerName == "" || containerName == c.cfg.TargetContainer) && relevantActions[action] {
					termutil.Info("Event received: container=%s action=%s -> recording snapshot", c.cfg.TargetContainer, action)
					wFiles, sErr := c.RecordSnapshot(ctx)
					if sErr != nil {
						termutil.Warn("Failed to record snapshot for event %s: %v", action, sErr)
					} else {
						termutil.Success("Evidence snapshot written (%d files)", len(wFiles))
					}
				}

			case sErr, ok := <-errCh:
				if ok && sErr != nil {
					termutil.Warn("Event stream disconnected: %v", sErr)
				}
				break streamLoop
			}
		}

		// Reconnection backoff
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(3 * time.Second):
			termutil.Info("Attempting to reconnect to Container Engine event stream...")
		}
	}
}
