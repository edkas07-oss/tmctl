package orchestrator

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/eddywiyatno/tomcat-monitoring/tmctl/internal/config"
	"github.com/eddywiyatno/tomcat-monitoring/tmctl/internal/engine"
	"github.com/eddywiyatno/tomcat-monitoring/tmctl/pkg/termutil"
)

// Deployer coordinates container stack provisioning and deployments.
type Deployer struct {
	client  engine.EngineClient
	cfg     *config.StackConfig
	builder *WorkloadSpecBuilder
	probe   *ReadinessProbe
}

// NewDeployer creates a new stack deployer.
func NewDeployer(client engine.EngineClient, cfg *config.StackConfig) *Deployer {
	return &Deployer{
		client:  client,
		cfg:     cfg,
		builder: NewWorkloadSpecBuilder(cfg),
		probe:   NewReadinessProbe(20*time.Second, 1*time.Second),
	}
}

// PrepareInfrastructure ensures bridge network, persistent volumes, and host directories exist.
func (d *Deployer) PrepareInfrastructure(ctx context.Context) error {
	termutil.Step(1, 3, "Reconciling Platform Network (%s)...", d.cfg.NetworkName)
	net, err := d.client.InspectNetwork(ctx, d.cfg.NetworkName)
	if err != nil {
		return fmt.Errorf("network inspection error: %w", err)
	}
	if net == nil {
		termutil.Info("Creating network %s...", d.cfg.NetworkName)
		if _, err := d.client.CreateNetwork(ctx, d.cfg.NetworkName, map[string]string{
			"platform": d.cfg.PlatformName,
		}); err != nil {
			return fmt.Errorf("failed to create network %s: %w", d.cfg.NetworkName, err)
		}
	}

	termutil.Step(2, 3, "Reconciling Named Volumes...")
	volumes := []string{
		d.cfg.TomcatLogVolume,
		d.cfg.DiagnosticDataVolume,
		d.cfg.PrometheusConfigVolume,
		d.cfg.PrometheusTruststoreVolume,
		d.cfg.PrometheusDataVolume,
		d.cfg.AlertmanagerConfigVolume,
		d.cfg.AlertmanagerTruststoreVolume,
		d.cfg.AlertmanagerDataVolume,
	}

	for _, volName := range volumes {
		if volName == "" {
			continue
		}
		vol, err := d.client.InspectVolume(ctx, volName)
		if err != nil {
			return fmt.Errorf("volume inspection error for %s: %w", volName, err)
		}
		if vol == nil {
			termutil.Info("Creating named volume %s...", volName)
			if _, err := d.client.CreateVolume(ctx, volName, map[string]string{
				"platform": d.cfg.PlatformName,
			}); err != nil {
				return fmt.Errorf("failed to create volume %s: %w", volName, err)
			}
		}
	}

	termutil.Step(3, 3, "Ensuring Persistent Host Directories...")
	dirs := []string{
		d.cfg.DefaultSpoolDir,
		d.cfg.DefaultSecretsDir,
		d.cfg.DefaultTLSDir,
		d.cfg.DefaultJMXTLSDir,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			termutil.Warn("Could not create directory %s: %v", dir, err)
		}
	}

	termutil.Success("Infrastructure prerequisites verified.")
	return nil
}

// DeployTarget reconciles desired state for a specific target or the entire stack.
func (d *Deployer) DeployTarget(ctx context.Context, target string) error {
	if err := d.PrepareInfrastructure(ctx); err != nil {
		return err
	}

	targets := d.resolveTargets(target)
	if len(targets) == 0 {
		return fmt.Errorf("unknown target: %s (available: tomcat, prometheus, alertmanager, diagnostic, postfix, mailpit, all)", target)
	}

	for idx, t := range targets {
		termutil.Step(idx+1, len(targets), "Deploying workload: %s...", t.Name)
		if err := d.deploySingleContainer(ctx, t); err != nil {
			return fmt.Errorf("deployment failed for %s: %w", t.Name, err)
		}
	}

	termutil.Success("Stack deployment completed successfully.")
	return nil
}

type targetTask struct {
	Name      string
	Spec      engine.ContainerSpec
	ProbePort int
}

func (d *Deployer) resolveTargets(target string) []targetTask {
	switch target {
	case "tomcat", "tomcat-jmx-exporter":
		return []targetTask{{Name: "tomcat-jmx-exporter", Spec: d.builder.BuildTomcatSpec(), ProbePort: d.cfg.TomcatJMXPort}}
	case "prometheus":
		return []targetTask{{Name: "prometheus", Spec: d.builder.BuildPrometheusSpec(), ProbePort: d.cfg.PrometheusPort}}
	case "alertmanager":
		return []targetTask{{Name: "alertmanager", Spec: d.builder.BuildAlertmanagerSpec(), ProbePort: d.cfg.AlertmanagerPort}}
	case "diagnostic", "diagnostic-service":
		return []targetTask{{Name: "diagnostic-service", Spec: d.builder.BuildDiagnosticSpec(), ProbePort: d.cfg.DiagnosticPort}}
	case "mailpit":
		return []targetTask{{Name: "mailpit", Spec: d.builder.BuildMailpitSpec(), ProbePort: d.cfg.MailpitHTTPPort}}
	case "postfix", "postfix-relay":
		return []targetTask{{Name: "postfix-relay", Spec: d.builder.BuildPostfixSpec(), ProbePort: d.cfg.PostfixPort}}
	case "all", "":
		return []targetTask{
			{Name: "mailpit", Spec: d.builder.BuildMailpitSpec(), ProbePort: d.cfg.MailpitHTTPPort},
			{Name: "postfix-relay", Spec: d.builder.BuildPostfixSpec(), ProbePort: d.cfg.PostfixPort},
			{Name: "tomcat-jmx-exporter", Spec: d.builder.BuildTomcatSpec(), ProbePort: d.cfg.TomcatJMXPort},
			{Name: "prometheus", Spec: d.builder.BuildPrometheusSpec(), ProbePort: d.cfg.PrometheusPort},
			{Name: "alertmanager", Spec: d.builder.BuildAlertmanagerSpec(), ProbePort: d.cfg.AlertmanagerPort},
			{Name: "diagnostic-service", Spec: d.builder.BuildDiagnosticSpec(), ProbePort: d.cfg.DiagnosticPort},
		}
	default:
		return nil
	}
}

func (d *Deployer) deploySingleContainer(ctx context.Context, t targetTask) error {
	rollbackName := fmt.Sprintf("%s-rollback-snapshot", t.Name)

	// Step 1: Check existing container
	existing, err := d.client.InspectContainer(ctx, t.Name)
	if err != nil {
		return err
	}

	if existing != nil {
		termutil.Info("Existing container %s found. Stopping and preparing rollback snapshot...", t.Name)
		_ = d.client.StopContainer(ctx, t.Name, 5)
		_ = d.client.RemoveContainer(ctx, rollbackName, true)
		if err := d.client.RenameContainer(ctx, t.Name, rollbackName); err != nil {
			termutil.Warn("Failed to rename snapshot (%v), falling back to force remove", err)
			_ = d.client.RemoveContainer(ctx, t.Name, true)
		}
	}

	// Step 2: Create new container
	termutil.Info("Creating container %s (%s)...", t.Name, t.Spec.Image)
	containerID, err := d.client.CreateContainer(ctx, t.Spec)
	if err != nil {
		d.rollbackIfAvailable(ctx, t.Name, rollbackName)
		return fmt.Errorf("failed to create container %s: %w", t.Name, err)
	}

	// Step 3: Start container
	termutil.Info("Starting container %s [%s]...", t.Name, containerID[:min(12, len(containerID))])
	if err := d.client.StartContainer(ctx, t.Name); err != nil {
		_ = d.client.RemoveContainer(ctx, t.Name, true)
		d.rollbackIfAvailable(ctx, t.Name, rollbackName)
		return fmt.Errorf("failed to start container %s: %w", t.Name, err)
	}

	// Step 4: Readiness check
	if t.ProbePort > 0 {
		termutil.Info("Verifying readiness for %s on port %d...", t.Name, t.ProbePort)
		if err := d.probe.ProbeService(ctx, t.Name, t.ProbePort); err != nil {
			termutil.Error("Readiness check failed: %v", err)
			termutil.Warn("Initiating automated rollback to previous healthy snapshot...")
			_ = d.client.RemoveContainer(ctx, t.Name, true)
			d.rollbackIfAvailable(ctx, t.Name, rollbackName)
			return fmt.Errorf("readiness probe failed for %s: %w", t.Name, err)
		}
	}

	// Step 5: Cleanup rollback snapshot upon successful deployment
	_ = d.client.RemoveContainer(ctx, rollbackName, true)
	termutil.Success("Workload %s is running and healthy.", t.Name)
	return nil
}

func (d *Deployer) rollbackIfAvailable(ctx context.Context, originalName, rollbackName string) {
	snap, err := d.client.InspectContainer(ctx, rollbackName)
	if err == nil && snap != nil {
		termutil.Info("Restoring snapshot %s -> %s...", rollbackName, originalName)
		_ = d.client.RenameContainer(ctx, rollbackName, originalName)
		_ = d.client.StartContainer(ctx, originalName)
		termutil.Success("Rollback completed. Container %s restored to previous state.", originalName)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
