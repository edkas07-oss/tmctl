package gitops

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/eddywiyatno/tmctl/internal/agent"
	"github.com/eddywiyatno/tmctl/internal/config"
	"github.com/eddywiyatno/tmctl/internal/engine"
	"github.com/eddywiyatno/tmctl/internal/orchestrator"
	"github.com/eddywiyatno/tmctl/internal/rules"
	"github.com/eddywiyatno/tmctl/pkg/termutil"
	"gopkg.in/yaml.v3"
)

// SyncOptions provides configuration parameters for tmctl gitops sync.
type SyncOptions struct {
	SpecFile string
	WorkDir  string
	Force    bool
	DryRun   bool
}

// SyncGitOps pulls the latest Git commits, evaluates monitoring-spec.yaml,
// detects state drift against live runtime, and reconciles differences.
func SyncGitOps(opts SyncOptions) error {
	termutil.Header("Executing GitOps Autonomous Reconciliation (tmctl gitops sync)")

	// 1. Resolve SpecFile and WorkDir
	specFile := opts.SpecFile
	if specFile == "" {
		defaultDir := ResolveDefaultGitOpsDir()
		defaultPath := filepath.ToSlash(filepath.Join(defaultDir, "monitoring-spec.yaml"))
		if _, err := os.Stat("monitoring-spec.yaml"); err == nil {
			specFile = "monitoring-spec.yaml"
		} else if _, err := os.Stat(defaultPath); err == nil {
			specFile = defaultPath
		} else {
			return fmt.Errorf("monitoring-spec.yaml not found. Specify path using --spec <path>")
		}
	}
	specFile = filepath.ToSlash(specFile)

	workDir := opts.WorkDir
	if workDir == "" {
		workDir = filepath.ToSlash(filepath.Dir(specFile))
	} else {
		workDir = filepath.ToSlash(workDir)
	}

	termutil.Info("Specification File : %s", specFile)
	termutil.Info("Working Directory  : %s", workDir)

	// 2. Load Reconciler State
	stateFile := filepath.ToSlash(filepath.Join(workDir, "state.json"))
	state, err := LoadState(stateFile)
	if err != nil {
		termutil.Warn("Could not load state.json: %v (Creating fresh state)", err)
		state = &GitOpsState{}
	}

	// 3. Git Fetch & Pull (via Git CLI or Gitea REST API)
	currentCommit := state.LastCommit
	if currentCommit == "" {
		currentCommit = "unknown"
	}
	commitMsg := state.CommitMessage
	if commitMsg == "" {
		commitMsg = "local"
	}

	gitConfig, _ := LoadConfig(workDir)
	var latestCommit, latestMsg string

	if gitConfig != nil && gitConfig.RepoURL != "" {
		termutil.Info("Checking for remote updates from '%s' (branch: %s)...", gitConfig.RepoURL, gitConfig.Branch)

		// A. Try git pull if .git exists and git CLI available
		gitDir := filepath.Join(workDir, ".git")
		if _, statErr := os.Stat(gitDir); statErr == nil {
			if gitPath, lookErr := exec.LookPath("git"); lookErr == nil && gitPath != "" {
				pullCmd := exec.Command("git", "pull", "--ff-only")
				pullCmd.Dir = workDir
				if out, pErr := pullCmd.CombinedOutput(); pErr != nil {
					termutil.Warn("git pull warning: %v (Output: %s)", pErr, strings.TrimSpace(string(out)))
				} else {
					termutil.Info("git pull output: %s", strings.TrimSpace(string(out)))
				}

				// Read local HEAD commit
				revCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
				revCmd.Dir = workDir
				if out, rErr := revCmd.CombinedOutput(); rErr == nil {
					latestCommit = strings.TrimSpace(string(out))
				}
				logCmd := exec.Command("git", "log", "-1", "--pretty=%B")
				logCmd.Dir = workDir
				if out, lErr := logCmd.CombinedOutput(); lErr == nil {
					latestMsg = strings.TrimSpace(string(out))
				}
			}
		}

		// B. If not resolved via Git CLI, use pure HTTP REST API
		if latestCommit == "" {
			termutil.Info("Querying Git REST API for latest branch commit...")
			sha, msg, fetchErr := FetchCommitInfoHTTP(gitConfig.RepoURL, gitConfig.Branch)
			if fetchErr != nil {
				termutil.Warn("Failed to fetch commit info via REST API: %v", fetchErr)
			} else {
				latestCommit = sha
				latestMsg = msg

				// Download updated spec if commit has progressed or forced
				if latestCommit != state.LastCommit || opts.Force {
					termutil.Info("Remote commit changed (%s -> %s). Downloading latest spec...", state.LastCommit, latestCommit)
					data, err := FetchSpecFileHTTP(gitConfig.RepoURL, gitConfig.Branch, gitConfig.SpecFile)
					if err == nil && len(data) > 0 {
						_ = os.WriteFile(specFile, data, 0644)
						termutil.Success("Saved updated specification file from remote Git repository")
					}
				}
			}
		}
	}

	if latestCommit != "" {
		currentCommit = latestCommit
		commitMsg = latestMsg
	}

	termutil.Info("Active Revision    : %s (%s)", currentCommit, commitMsg)

	// 4. Parse declarative monitoring-spec.yaml
	data, err := os.ReadFile(specFile)
	if err != nil {
		state.Status = "Error"
		state.Error = fmt.Sprintf("failed to read spec file: %v", err)
		_ = SaveState(stateFile, state)
		return fmt.Errorf("failed to read spec file '%s': %w", specFile, err)
	}

	var spec MonitoringSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		state.Status = "Error"
		state.Error = fmt.Sprintf("invalid yaml syntax in spec: %v", err)
		_ = SaveState(stateFile, state)
		return fmt.Errorf("invalid YAML syntax in '%s': %w", specFile, err)
	}

	if err := spec.Validate(); err != nil {
		state.Status = "Error"
		state.Error = fmt.Sprintf("spec validation failed: %v", err)
		_ = SaveState(stateFile, state)
		return fmt.Errorf("spec validation failed: %w", err)
	}

	termutil.Success("Specification parsed: Topology=%s, Environment=%s", spec.Metadata.Topology, spec.Metadata.Environment)

	// 5. Connect to container engine
	ctx := context.Background()
	engineClient, err := engine.NewEngineClient(spec.Spec.Engine, "")
	if err != nil {
		state.Status = "Error"
		state.Error = fmt.Sprintf("failed to connect to container engine: %v", err)
		_ = SaveState(stateFile, state)
		return fmt.Errorf("container engine connection failed: %w", err)
	}

	info := engineClient.GetInfo()
	termutil.Info("Container Engine   : %s (%s)", info.EngineType, info.SocketPath)

	// 6. Detect Drift & Reconcile Container Stack
	driftDetected := false
	var activeServices []string

	containers, err := engineClient.ListContainers(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to list live containers: %w", err)
	}

	containerStatusMap := make(map[string]bool)
	for _, c := range containers {
		for _, name := range c.Names {
			cleanName := strings.TrimPrefix(name, "/")
			if c.State == "running" {
				containerStatusMap[cleanName] = true
			}
		}
	}

	// Helper to check service drift
	checkService := func(name string, enabled bool) {
		running := containerStatusMap[name]
		if enabled {
			if !running {
				termutil.Warn("Drift Detected: Service '%s' is enabled in spec but NOT running", name)
				driftDetected = true
			} else {
				activeServices = append(activeServices, name)
			}
		}
	}

	checkService("diagnostic-service", spec.Spec.Stack.DiagnosticService.Enabled)
	checkService("prometheus", spec.Spec.Stack.Prometheus.Enabled)
	checkService("alertmanager", spec.Spec.Stack.Alertmanager.Enabled)
	checkService("postfix-relay", spec.Spec.Stack.PostfixRelay.Enabled)

	if opts.Force {
		driftDetected = true
		termutil.Info("Forced reconciliation requested (--force)")
	}

	if opts.DryRun {
		termutil.Info("Dry-run mode active. Drift detected: %t. Skipping changes.", driftDetected)
		return nil
	}

	// Reconcile Stack if drift detected or new commit
	if driftDetected || latestCommit != state.LastCommit {
		termutil.Info("Reconciling container workloads via Stack Deployer...")
		stackCfg := config.DefaultConfig()
		deployer := orchestrator.NewDeployer(engineClient, stackCfg)
		if err := deployer.DeployTarget(ctx, "all"); err != nil {
			state.Status = "Error"
			state.Error = fmt.Sprintf("stack deployment failed: %v", err)
			_ = SaveState(stateFile, state)
			return fmt.Errorf("stack reconciliation failed: %w", err)
		}
		termutil.Success("Monitoring stack reconciliation completed successfully")
	} else {
		termutil.Success("Live container state is already in-sync with specification")
	}

	// 7. Reconcile Host Telemetry Agent
	if spec.Spec.Agent.Enabled {
		agentCfg := agent.DefaultConfig()
		if spec.Spec.Agent.TargetContainer != "" {
			agentCfg.TargetContainer = spec.Spec.Agent.TargetContainer
		}
		if spec.Spec.Agent.SpoolDir != "" {
			agentCfg.SpoolDir = spec.Spec.Agent.SpoolDir
		}
		if spec.Spec.Agent.RetentionHours > 0 {
			agentCfg.MaxSpoolAgeHours = spec.Spec.Agent.RetentionHours
		}
		if spec.Spec.Agent.MaxFiles > 0 {
			agentCfg.MaxSpoolFiles = spec.Spec.Agent.MaxFiles
		}

		// Check spool directory
		if err := agent.EnsureSpoolDir(agentCfg.SpoolDir); err != nil {
			termutil.Warn("Spool directory creation warning: %v", err)
		} else {
			activeServices = append(activeServices, "tmctl-agent")
		}
	}

	// 8. Reconcile Rules Ingestion
	if spec.Spec.Rules.AutoIngest && spec.Spec.Rules.RulepackPath != "" {
		rulePath := filepath.Join(workDir, spec.Spec.Rules.RulepackPath)
		if _, err := os.Stat(rulePath); err == nil {
			termutil.Info("Auto-ingesting rules from '%s'...", rulePath)
			stackCfg := config.DefaultConfig()
			endpoint := spec.Spec.Rules.Endpoint
			if endpoint == "" {
				endpoint = stackCfg.DiagnosticURL
			}
			if err := rules.IngestRules(ctx, stackCfg, rulePath, "", endpoint); err != nil {
				termutil.Warn("Rule ingestion warning: %v", err)
			} else {
				termutil.Success("Diagnostic rules ingested successfully into Diagnostic Service")
			}
		}
	}

	// 9. Save Updated Reconciler State
	state.LastCommit = currentCommit
	state.CommitMessage = commitMsg
	state.LastSyncTime = time.Now().UTC().Format(time.RFC3339)
	state.Status = "In-Sync"
	state.ActiveServices = activeServices
	state.Error = ""

	if err := SaveState(stateFile, state); err != nil {
		termutil.Warn("Failed to persist state.json: %v", err)
	}

	termutil.Success("Autonomous GitOps reconciliation finished. Status: In-Sync (Commit: %s)", currentCommit)
	return nil
}
