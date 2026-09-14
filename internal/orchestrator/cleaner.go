package orchestrator

import (
	"context"

	"github.com/eddywiyatno/tmctl/internal/config"
	"github.com/eddywiyatno/tmctl/internal/engine"
	"github.com/eddywiyatno/tmctl/pkg/termutil"
)

// CleanStack stops and removes platform containers, and optionally cleans volumes and networks.
func CleanStack(ctx context.Context, client engine.EngineClient, cfg *config.StackConfig, cleanAll bool) error {
	containers := []string{
		"diagnostic-service",
		"alertmanager",
		"prometheus",
		"tomcat-jmx-exporter",
		"postfix-relay",
		"mailpit",
		"diagnostic-service-rollback-snapshot",
		"alertmanager-rollback-snapshot",
		"prometheus-rollback-snapshot",
		"tomcat-jmx-exporter-rollback-snapshot",
	}

	termutil.Info("Stopping and removing platform containers...")
	for _, c := range containers {
		inspect, _ := client.InspectContainer(ctx, c)
		if inspect != nil {
			termutil.Info("Removing container %s...", c)
			_ = client.StopContainer(ctx, c, 5)
			_ = client.RemoveContainer(ctx, c, true)
		}
	}

	if cleanAll {
		termutil.Warn("Flag --all specified. Cleaning volumes and network...")
		volumes := []string{
			cfg.TomcatLogVolume,
			cfg.DiagnosticDataVolume,
			cfg.PrometheusConfigVolume,
			cfg.PrometheusTruststoreVolume,
			cfg.PrometheusDataVolume,
			cfg.AlertmanagerConfigVolume,
			cfg.AlertmanagerTruststoreVolume,
			cfg.AlertmanagerDataVolume,
		}
		for _, vol := range volumes {
			if vol != "" {
				termutil.Info("Removing volume %s...", vol)
				_ = client.RemoveVolume(ctx, vol, true)
			}
		}

		termutil.Info("Removing network %s...", cfg.NetworkName)
		_ = client.RemoveNetwork(ctx, cfg.NetworkName)
	}

	termutil.Success("Stack cleanup completed.")
	return nil
}
