package gitops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/eddywiyatno/tmctl/pkg/termutil"
)

// ResolveDefaultGitOpsDir returns the default directory for GitOps operations.
func ResolveDefaultGitOpsDir() string {
	if runtime.GOOS == "windows" {
		progFiles := os.Getenv("ProgramFiles")
		if progFiles == "" {
			progFiles = "C:/Program Files"
		}
		return filepath.ToSlash(filepath.Join(progFiles, "tmctl", "gitops"))
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/etc/tmctl/gitops"
	}
	return filepath.Join(homeDir, ".config", "tmctl", "gitops")
}

// InitOptions provides configuration parameters for tmctl gitops init.
type InitOptions struct {
	RepoURL     string
	Branch      string
	TargetDir   string
	SpecFile    string
	Interval    string
	EnableTimer bool
}

// InitGitOps sets up the local GitOps environment, clones the manifest repo,
// writes a default starter spec if missing, and registers the systemd user timer.
func InitGitOps(opts InitOptions) error {
	termutil.Header("Initializing Autonomous GitOps Environment (tmctl gitops init)")

	targetDir := opts.TargetDir
	if targetDir == "" {
		targetDir = ResolveDefaultGitOpsDir()
	}
	targetDir = filepath.ToSlash(targetDir)

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory '%s': %w", targetDir, err)
	}
	termutil.Info("GitOps Working Directory: %s", targetDir)

	branch := opts.Branch
	if branch == "" {
		branch = "main"
	}

	interval := opts.Interval
	if interval == "" {
		interval = "5min"
	}

	specName := opts.SpecFile
	if specName == "" {
		specName = "monitoring-spec.yaml"
	}

	specPath := filepath.ToSlash(filepath.Join(targetDir, specName))

	// 1. If repo URL provided, try git clone or REST fetch
	if opts.RepoURL != "" {
		termutil.Info("Configuring GitOps Repository: %s (Branch: %s)", opts.RepoURL, branch)

		// Check if git CLI is available
		if gitPath, err := exec.LookPath("git"); err == nil && gitPath != "" {
			termutil.Info("Git CLI detected. Checking local repository...")
			gitDir := filepath.Join(targetDir, ".git")
			if _, err := os.Stat(gitDir); os.IsNotExist(err) {
				termutil.Info("Cloning repository '%s' into '%s'...", opts.RepoURL, targetDir)
				cmd := exec.Command("git", "clone", "--branch", branch, "--depth", "1", opts.RepoURL, targetDir)
				if out, err := cmd.CombinedOutput(); err != nil {
					termutil.Warn("git clone warning: %v (falling back to REST API fetch)", err)
					_ = out
				} else {
					termutil.Success("Repository successfully cloned.")
				}
			}
		}

		// If spec still not present, download directly via REST API
		if _, err := os.Stat(specPath); os.IsNotExist(err) {
			termutil.Info("Fetching '%s' from remote Git server via HTTP API...", specName)
			data, err := FetchSpecFileHTTP(opts.RepoURL, branch, specName)
			if err != nil {
				termutil.Warn("Could not download spec via REST API: %v (Creating default starter spec)", err)
			} else {
				if err := os.WriteFile(specPath, data, 0644); err != nil {
					return fmt.Errorf("failed to write downloaded spec file: %w", err)
				}
				termutil.Success("Downloaded and saved: %s", specPath)
			}
		}
	}

	// 2. If spec still missing, generate default starter manifest
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		termutil.Info("No existing spec found. Generating starter '%s'...", specName)
		starterContent := GenerateStarterYAML()
		if err := os.WriteFile(specPath, []byte(starterContent), 0644); err != nil {
			return fmt.Errorf("failed to write starter spec: %w", err)
		}
		termutil.Success("Starter manifest generated: %s", specPath)
	}

	// 3. Save gitops-config.json
	cfg := &GitOpsConfig{
		RepoURL:   opts.RepoURL,
		Branch:    branch,
		SpecFile:  specName,
		TargetDir: targetDir,
		Interval:  interval,
	}
	if err := SaveConfig(targetDir, cfg); err != nil {
		return fmt.Errorf("failed to save gitops config: %w", err)
	}
	termutil.Success("Saved GitOps configuration: %s/gitops-config.json", targetDir)

	// 4. Register autonomous scheduler if requested
	if opts.EnableTimer {
		termutil.Info("Enabling autonomous platform scheduler (Interval: %s)...", interval)
		if err := SetupScheduler(specPath, targetDir, interval); err != nil {
			termutil.Warn("Scheduler registration warning: %v", err)
		}
	} else {
		termutil.Info("Scheduler registration skipped. Run 'tmctl gitops init --timer' to enable autonomous timer.")
	}

	termutil.Success("Autonomous GitOps Environment initialized successfully.")
	termutil.Info("Run 'tmctl gitops sync --spec %s' to execute initial reconciliation.", specPath)
	return nil
}
