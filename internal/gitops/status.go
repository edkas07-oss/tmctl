package gitops

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/eddywiyatno/tmctl/pkg/termutil"
	"gopkg.in/yaml.v3"
)

// GitOpsStatusExport represents the structured output of GitOps status.
type GitOpsStatusExport struct {
	Timestamp      string          `json:"timestamp"`
	Directory      string          `json:"directory"`
	SpecFile       string          `json:"spec_file"`
	Git            GitInfo         `json:"git"`
	Spec           *MonitoringSpec `json:"spec,omitempty"`
	State          *GitOpsState    `json:"state,omitempty"`
	TimerActive    bool            `json:"timer_active"`
	TimerStatus    string          `json:"timer_status"`
	ActiveServices []string        `json:"active_services"`
}

// GitInfo details current Git worktree state.
type GitInfo struct {
	RemoteURL string `json:"remote_url"`
	Branch    string `json:"branch"`
	Commit    string `json:"commit"`
}

// StatusGitOps displays status to console.
func StatusGitOps(dir, specFile, jsonOut string) error {
	export, err := StatusGitOpsWithExport(dir, specFile)
	if err != nil {
		return err
	}

	if jsonOut != "" {
		data, mErr := json.MarshalIndent(export, "", "  ")
		if mErr != nil {
			return fmt.Errorf("failed to format json status: %w", mErr)
		}
		if err := os.WriteFile(jsonOut, data, 0644); err != nil {
			return fmt.Errorf("failed to write json output to %s: %w", jsonOut, err)
		}
		termutil.Success("Exported GitOps status JSON to %s", jsonOut)
	}

	return nil
}

// StatusGitOpsWithExport displays status to console and returns structured export.
func StatusGitOpsWithExport(dir, specFile string) (*GitOpsStatusExport, error) {
	termutil.Header("Tomcat Monitoring — GitOps Reconciler Status")

	if dir == "" {
		dir = ResolveDefaultGitOpsDir()
	}
	dir = filepath.ToSlash(dir)

	if specFile == "" {
		specFile = filepath.ToSlash(filepath.Join(dir, "monitoring-spec.yaml"))
	} else {
		specFile = filepath.ToSlash(specFile)
	}

	termutil.Info("GitOps Directory : %s", dir)
	termutil.Info("Spec File        : %s", specFile)

	export := &GitOpsStatusExport{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Directory: dir,
		SpecFile:  specFile,
	}

	// 1. Read Git configuration
	cfg, _ := LoadConfig(dir)
	if cfg != nil {
		export.Git.RemoteURL = cfg.RepoURL
		export.Git.Branch = cfg.Branch
	}

	// 2. Read state.json
	statePath := filepath.Join(dir, "state.json")
	state, err := LoadState(statePath)
	if err == nil && state != nil {
		export.State = state
		export.Git.Commit = state.LastCommit
		export.ActiveServices = state.ActiveServices
	}

	// 3. Read monitoring-spec.yaml
	if data, err := os.ReadFile(specFile); err == nil {
		var spec MonitoringSpec
		if err := yaml.Unmarshal(data, &spec); err == nil {
			export.Spec = &spec
		}
	}

	// 4. Check Timer / Scheduler status
	if runtime.GOOS == "windows" {
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", `(Get-ScheduledTask -TaskName 'tmctl-gitops-reconciler' -ErrorAction SilentlyContinue).State`)
		out, err := cmd.CombinedOutput()
		stateStr := strings.TrimSpace(string(out))
		if err == nil && stateStr != "" {
			export.TimerActive = stateStr == "Ready" || stateStr == "Running"
			export.TimerStatus = fmt.Sprintf("Windows Task: %s", stateStr)
		} else {
			export.TimerStatus = "Not Registered"
		}
	} else {
		cmd := exec.Command("systemctl", "--user", "is-active", "tmctl-gitops.timer")
		out, err := cmd.CombinedOutput()
		stateStr := strings.TrimSpace(string(out))
		if err == nil && stateStr == "active" {
			export.TimerActive = true
			export.TimerStatus = "systemd --user timer active"
		} else {
			export.TimerStatus = "inactive / not enabled"
		}
	}

	// Console Display
	termutil.Info("Git Remote       : %s (Branch: %s)", export.Git.RemoteURL, export.Git.Branch)
	termutil.Info("Last Synced SHA  : %s", export.Git.Commit)
	if state != nil {
		termutil.Info("Sync Status      : %s", state.Status)
		termutil.Info("Last Sync Time   : %s", state.LastSyncTime)
		termutil.Info("Commit Message   : %s", state.CommitMessage)
		if len(state.ActiveServices) > 0 {
			termutil.Info("Active Services  : %s", strings.Join(state.ActiveServices, ", "))
		}
		if state.Error != "" {
			termutil.Warn("Reconciler Error : %s", state.Error)
		}
	}
	termutil.Info("Autonomous Timer : %s", export.TimerStatus)

	return export, nil
}
