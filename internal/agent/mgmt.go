package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/eddywiyatno/tmctl/pkg/termutil"
)

// InstallAgent configures and registers the agent as a persistent background service.
func InstallAgent(cfg *Config) error {
	termutil.Header("Installing tmctl Host Telemetry Agent Daemon")

	tmctlBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine tmctl binary path: %w", err)
	}
	tmctlBin = filepath.ToSlash(tmctlBin)

	if runtime.GOOS == "windows" {
		return installWindowsService(tmctlBin, cfg)
	}
	return installSystemdUserService(tmctlBin, cfg)
}

func installSystemdUserService(binPath string, cfg *Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine user home directory: %w", err)
	}

	systemdDir := filepath.Join(homeDir, ".config", "systemd", "user")
	if err := os.MkdirAll(systemdDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd user directory: %w", err)
	}

	servicePath := filepath.Join(systemdDir, "tmctl-agent.service")
	serviceContent := fmt.Sprintf(`[Unit]
Description=tmctl Host Telemetry & Event Collector Daemon
After=network.target

[Service]
Type=simple
ExecStart=%s agent run --target %s --spool-dir %s
Restart=always
RestartSec=5s

[Install]
WantedBy=default.target
`, binPath, cfg.TargetContainer, cfg.SpoolDir)

	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write systemd service file: %w", err)
	}
	termutil.Success("Service file written: %s", servicePath)

	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	cmd := exec.Command("systemctl", "--user", "enable", "--now", "tmctl-agent.service")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to enable systemd service: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	termutil.Success("tmctl-agent.service successfully registered and started via systemd --user")
	return nil
}

func installWindowsService(binPath string, cfg *Config) error {
	psScript := fmt.Sprintf(`
$bin = '%s'
$arg = 'agent run --target %s --spool-dir "%s"'
New-Service -Name 'TomcatMonitoringAgent' -BinaryPathName "$bin $arg" -DisplayName 'Tomcat Monitoring Telemetry Agent' -StartupType Automatic
Start-Service -Name 'TomcatMonitoringAgent'
`, binPath, cfg.TargetContainer, cfg.SpoolDir)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to register Windows Service: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	termutil.Success("TomcatMonitoringAgent Windows Service successfully registered and started")
	return nil
}

// ShowAgentStatus displays the telemetry agent status and spool directory health.
func ShowAgentStatus(cfg *Config) error {
	termutil.Header("tmctl Telemetry Agent Status")

	termutil.Info("Target Container : %s", cfg.TargetContainer)
	termutil.Info("Target ID        : %s", cfg.TargetID)
	termutil.Info("Spool Directory  : %s", cfg.SpoolDir)

	// Check OS service status
	serviceStatus := "unknown"
	if runtime.GOOS != "windows" {
		cmd := exec.Command("systemctl", "--user", "is-active", "tmctl-agent.service")
		out, err := cmd.CombinedOutput()
		if err == nil {
			serviceStatus = strings.TrimSpace(string(out))
		} else {
			serviceStatus = "inactive / not installed"
		}
	} else {
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", "(Get-Service -Name 'TomcatMonitoringAgent' -ErrorAction SilentlyContinue).Status")
		out, err := cmd.CombinedOutput()
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			serviceStatus = strings.TrimSpace(string(out))
		} else {
			serviceStatus = "not installed"
		}
	}
	termutil.Info("Daemon Status    : %s", serviceStatus)

	// Inspect Spool directory
	if _, err := os.Stat(cfg.SpoolDir); os.IsNotExist(err) {
		termutil.Warn("Spool Directory does not exist yet (%s)", cfg.SpoolDir)
		return nil
	}

	jsonFiles, _ := filepath.Glob(filepath.Join(cfg.SpoolDir, "*.json"))
	tmpFiles, _ := filepath.Glob(filepath.Join(cfg.SpoolDir, "*.tmp"))

	var totalBytes int64
	var oldestTime, newestTime time.Time

	for i, f := range jsonFiles {
		fi, err := os.Stat(f)
		if err == nil {
			totalBytes += fi.Size()
			mt := fi.ModTime()
			if i == 0 || mt.Before(oldestTime) {
				oldestTime = mt
			}
			if i == 0 || mt.After(newestTime) {
				newestTime = mt
			}
		}
	}

	termutil.Success("Spool Metrics:")
	termutil.Bullet("Active Records (.json) : %d files", len(jsonFiles))
	termutil.Bullet("Pending Writes (.tmp)  : %d files", len(tmpFiles))
	termutil.Bullet("Total Storage Size     : %.2f KiB", float64(totalBytes)/1024.0)
	if len(jsonFiles) > 0 {
		termutil.Bullet("Oldest Record Age      : %s (%s ago)", oldestTime.UTC().Format(time.RFC3339), time.Since(oldestTime).Round(time.Second))
		termutil.Bullet("Newest Record Age      : %s (%s ago)", newestTime.UTC().Format(time.RFC3339), time.Since(newestTime).Round(time.Second))
	}

	return nil
}
