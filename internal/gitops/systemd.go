package gitops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/eddywiyatno/tmctl/pkg/termutil"
)

// SetupScheduler configures the platform-native autonomous timer (systemd timer on Linux, Scheduled Task on Windows).
func SetupScheduler(specPath, workDir, interval string) error {
	if runtime.GOOS == "windows" {
		return SetupWindowsScheduledTask(specPath, workDir, interval)
	}
	return SetupSystemdUserTimer(specPath, workDir, interval)
}

// SetupWindowsScheduledTask registers a scheduled task on Windows for GitOps reconciliation.
func SetupWindowsScheduledTask(specPath, workDir, interval string) error {
	tmctlBin, err := os.Executable()
	if err != nil {
		tmctlBin = "C:/Program Files/tmctl/tmctl.exe"
	}
	tmctlBin = filepath.ToSlash(tmctlBin)
	workDir = filepath.ToSlash(workDir)
	specPath = filepath.ToSlash(specPath)

	minutes := 5
	if strings.HasSuffix(interval, "min") {
		var m int
		if _, errScan := fmt.Sscanf(interval, "%dmin", &m); errScan == nil && m > 0 {
			minutes = m
		}
	} else if strings.HasSuffix(interval, "m") {
		var m int
		if _, errScan := fmt.Sscanf(interval, "%dm", &m); errScan == nil && m > 0 {
			minutes = m
		}
	}

	argStr := "gitops sync"
	if filepath.Base(specPath) != "monitoring-spec.yaml" || filepath.ToSlash(filepath.Dir(specPath)) != workDir {
		argStr = fmt.Sprintf("gitops sync --spec \\\"%s\\\"", specPath)
	}

	psScript := fmt.Sprintf(
		`$a = New-ScheduledTaskAction -Execute '%s' -Argument '%s' -WorkingDirectory '%s'; `+
			`$t = New-ScheduledTaskTrigger -Once -At (Get-Date) -RepetitionInterval (New-TimeSpan -Minutes %d); `+
			`Register-ScheduledTask -TaskName 'tmctl-gitops-reconciler' -Action $a -Trigger $t -User 'SYSTEM' -Force`,
		tmctlBin, argStr, workDir, minutes,
	)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to register Windows scheduled task: %w (Output: %s)", err, strings.TrimSpace(string(out)))
	}

	termutil.Success("Windows Scheduled Task 'tmctl-gitops-reconciler' successfully registered (Interval: %d min)", minutes)
	return nil
}

// SetupSystemdUserTimer generates and enables a systemd user service and timer on Linux.
func SetupSystemdUserTimer(specPath, workDir, interval string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to determine user home directory: %w", err)
	}

	systemdDir := filepath.Join(homeDir, ".config", "systemd", "user")
	if err := os.MkdirAll(systemdDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd user directory '%s': %w", systemdDir, err)
	}

	tmctlBin, err := os.Executable()
	if err != nil {
		tmctlBin = "/usr/local/bin/tmctl"
	}
	tmctlBin = filepath.ToSlash(tmctlBin)
	workDir = filepath.ToSlash(workDir)
	specPath = filepath.ToSlash(specPath)

	onCalendarInterval := "*:0/5" // Default 5 min
	if strings.HasSuffix(interval, "min") {
		var m int
		if _, errScan := fmt.Sscanf(interval, "%dmin", &m); errScan == nil && m > 0 {
			onCalendarInterval = fmt.Sprintf("*:0/%d", m)
		}
	} else if strings.HasSuffix(interval, "m") {
		var m int
		if _, errScan := fmt.Sscanf(interval, "%dm", &m); errScan == nil && m > 0 {
			onCalendarInterval = fmt.Sprintf("*:0/%d", m)
		}
	} else if strings.HasSuffix(interval, "s") || strings.HasSuffix(interval, "sec") {
		onCalendarInterval = "*:*:0/30"
	}

	argStr := "gitops sync"
	if filepath.Base(specPath) != "monitoring-spec.yaml" || filepath.ToSlash(filepath.Dir(specPath)) != workDir {
		argStr = fmt.Sprintf("gitops sync --spec %s", specPath)
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=tmctl Autonomous GitOps Reconciler Service
After=network.target

[Service]
Type=oneshot
WorkingDirectory=%s
ExecStart=%s %s
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=default.target
`, workDir, tmctlBin, argStr)

	timerContent := fmt.Sprintf(`[Unit]
Description=tmctl Autonomous GitOps Reconciler Timer
After=network.target

[Timer]
OnCalendar=%s
Persistent=true

[Install]
WantedBy=timers.target
`, onCalendarInterval)

	servicePath := filepath.Join(systemdDir, "tmctl-gitops.service")
	timerPath := filepath.Join(systemdDir, "tmctl-gitops.timer")

	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file '%s': %w", servicePath, err)
	}
	if err := os.WriteFile(timerPath, []byte(timerContent), 0644); err != nil {
		return fmt.Errorf("failed to write timer file '%s': %w", timerPath, err)
	}

	// Reload systemd daemon
	reloadCmd := exec.Command("systemctl", "--user", "daemon-reload")
	if out, err := reloadCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reload systemd user daemon: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	// Enable and start timer
	enableCmd := exec.Command("systemctl", "--user", "enable", "--now", "tmctl-gitops.timer")
	if out, err := enableCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to enable systemd user timer: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	termutil.Success("Systemd user timer 'tmctl-gitops.timer' enabled and started (Interval: %s)", onCalendarInterval)
	return nil
}
