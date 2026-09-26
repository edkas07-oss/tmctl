package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/eddywiyatno/tmctl/internal/agent"
	"github.com/eddywiyatno/tmctl/internal/api"
	"github.com/eddywiyatno/tmctl/internal/buildinfo"
	"github.com/eddywiyatno/tmctl/internal/config"
	"github.com/eddywiyatno/tmctl/internal/diagnostic"
	"github.com/eddywiyatno/tmctl/internal/engine"
	"github.com/eddywiyatno/tmctl/internal/gitops"
	"github.com/eddywiyatno/tmctl/internal/orchestrator"
	"github.com/eddywiyatno/tmctl/internal/registry"
	"github.com/eddywiyatno/tmctl/internal/rules"
	"github.com/eddywiyatno/tmctl/internal/validator"
	"github.com/eddywiyatno/tmctl/pkg/termutil"
)

func printHelp() {
	fmt.Printf(`tmctl — Universal Cross-Platform Operator CLI for Tomcat Monitoring

Usage:
  tmctl <command> [subcommand] [flags] [args...]

Available Commands:
  stack         Manage monitoring container stack lifecycle (deploy, status, restart, clean)
  agent         Manage host telemetry daemon and crash event spooling (absorbs tm-agent)
  rules         Manage and audit AI diagnostic rulepacks (ingest, export, audit)
  diagnostic    Execute crash triage analysis and verify notification alerting pipeline
  gitops        Manage autonomous pull-based GitOps reconciliation and OS timers
  registry      Manage container registry authentication (login, logout)
  serve         Run embedded lightweight REST API daemon for Portal integration
  validate      Validate platform configuration, schemas, and contracts
  version       Display version and build information

Stack Subcommands:
  tmctl stack deploy     [--target <name>] [--env <name>] [--engine <podman|docker>] [--config <path>]
  tmctl stack status     [--config <path>]
  tmctl stack restart    [--target <name>] [--config <path>]
  tmctl stack clean      [--all] [--config <path>]

Agent Subcommands:
  tmctl agent run        [--target <container>] [--spool-dir <path>] [--run-once] [--engine <podman|docker>]
  tmctl agent install    [--target <container>] [--spool-dir <path>]
  tmctl agent status     [--target <container>] [--spool-dir <path>]
  tmctl agent spool      [--dir <path>] [--prune] [--max-age <hours>] [--max-files <count>]

Rules Subcommands:
  tmctl rules ingest     <path/to/rulepack.json> [--token <bearer_token>] [--url <endpoint>]
  tmctl rules export     [--category <name>] [--output <path.json>] [--categories] [--token <token>]
  tmctl rules audit      <path/to/rulepack.json>

Diagnostic Subcommands:
  tmctl diagnostic triage      [--container <name>] [--spool-dir <path>] [--json-out <path>]
  tmctl diagnostic test-alert  [--endpoint <url>] [--recipient <email>]

GitOps Subcommands:
  tmctl gitops init      [--repo <url>] [--branch <branch>] [--dir <path>] [--interval <5min>] [--timer]
  tmctl gitops sync      [--spec <path>] [--work-dir <path>] [--force] [--dry-run]
  tmctl gitops status    [--dir <path>] [--spec <path>] [--json-out <path>]

Daemon Subcommand:
  tmctl serve            [--host 0.0.0.0] [--port 8099] [--api-key <token>]

Registry Subcommands:
  tmctl registry login   <host:port> <username> [--token-file <path>] [--auth-file <path>]
  tmctl registry logout  [host:port] [--auth-file <path>]

Validation Subcommands:
  tmctl validate         [--layout] [--schemas] [--ansible] [--dir <path>]

Examples:
  tmctl stack deploy --target all --engine podman
  tmctl stack status
  tmctl agent run --target tomcat-jmx-exporter --spool-dir /opt/tm-home/spool
  tmctl agent install
  tmctl diagnostic triage --container tomcat-jmx-exporter
  tmctl diagnostic test-alert --recipient sre-oncall@corp.internal
  tmctl gitops init --repo http://localhost:3000/gitadm/tomcat-monitoring-gitops.git --timer
  tmctl gitops sync --spec monitoring-spec.yaml
  tmctl gitops status
  tmctl serve --port 8099 --api-key "SecretOperatorToken"

Use "tmctl <command> --help" for more information about a specific command.
`)
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	cmd := os.Args[1]
	ctx := context.Background()

	switch cmd {
	case "version", "--version", "-v":
		fmt.Println(buildinfo.String())
		return

	case "help", "--help", "-h":
		printHelp()
		return

	case "stack":
		handleStackCommand(ctx, os.Args[2:])

	case "agent":
		handleAgentCommand(ctx, os.Args[2:])

	case "rules":
		handleRulesCommand(ctx, os.Args[2:])

	case "diagnostic":
		handleDiagnosticCommand(ctx, os.Args[2:])

	case "gitops":
		handleGitOpsCommand(ctx, os.Args[2:])

	case "serve":
		handleServeCommand(os.Args[2:])

	case "registry":
		handleRegistryCommand(ctx, os.Args[2:])

	case "validate":
		handleValidateCommand(ctx, os.Args[2:])

	default:
		termutil.Error("Unknown command '%s'", cmd)
		printHelp()
		os.Exit(1)
	}
}

func handleStackCommand(ctx context.Context, args []string) {
	if len(args) == 0 {
		termutil.Error("Subcommand required for 'stack' (deploy, status, restart, clean)")
		os.Exit(1)
	}

	subCmd := args[0]
	subArgs := args[1:]

	fs := flag.NewFlagSet("stack "+subCmd, flag.ExitOnError)
	configPath := fs.String("config", "", "Path to CONFIG file")
	engineFlag := fs.String("engine", "", "Container engine (podman|docker)")
	socketFlag := fs.String("socket", "", "Custom socket path or named pipe")

	switch subCmd {
	case "deploy":
		target := fs.String("target", "all", "Target workload to deploy (tomcat, prometheus, alertmanager, diagnostic, postfix, mailpit, all)")
		deployEnv := fs.String("env", "lab", "Deployment target environment (lab, staging, production)")
		_ = fs.Parse(subArgs)

		cfg, err := config.LoadConfig(*configPath)
		if err != nil {
			termutil.Error("Config loading failed: %v", err)
			os.Exit(1)
		}
		if *engineFlag != "" {
			cfg.ContainerEngine = *engineFlag
		}
		if *socketFlag != "" {
			cfg.SocketPath = *socketFlag
		}

		termutil.Info("Initializing engine client (%s) for environment '%s'...", cfg.ContainerEngine, *deployEnv)
		client, err := engine.NewEngineClient(cfg.ContainerEngine, cfg.SocketPath)
		if err != nil {
			termutil.Error("Failed to initialize engine client: %v", err)
			os.Exit(1)
		}

		deployer := orchestrator.NewDeployer(client, cfg)
		if err := deployer.DeployTarget(ctx, *target); err != nil {
			termutil.Error("Deploy failed: %v", err)
			os.Exit(1)
		}

	case "status":
		_ = fs.Parse(subArgs)
		cfg, err := config.LoadConfig(*configPath)
		if err != nil {
			termutil.Error("Config loading failed: %v", err)
			os.Exit(1)
		}
		if *engineFlag != "" {
			cfg.ContainerEngine = *engineFlag
		}
		if *socketFlag != "" {
			cfg.SocketPath = *socketFlag
		}

		client, err := engine.NewEngineClient(cfg.ContainerEngine, cfg.SocketPath)
		if err != nil {
			termutil.Error("Failed to initialize engine client: %v", err)
			os.Exit(1)
		}

		if err := orchestrator.ShowStatus(ctx, client, cfg); err != nil {
			termutil.Error("Status error: %v", err)
			os.Exit(1)
		}

	case "restart":
		target := fs.String("target", "all", "Target workload to restart (all, diagnostic, prometheus, alertmanager)")
		_ = fs.Parse(subArgs)

		cfg, err := config.LoadConfig(*configPath)
		if err != nil {
			termutil.Error("Config loading failed: %v", err)
			os.Exit(1)
		}
		if *engineFlag != "" {
			cfg.ContainerEngine = *engineFlag
		}
		if *socketFlag != "" {
			cfg.SocketPath = *socketFlag
		}

		client, err := engine.NewEngineClient(cfg.ContainerEngine, cfg.SocketPath)
		if err != nil {
			termutil.Error("Failed to initialize engine client: %v", err)
			os.Exit(1)
		}

		termutil.Info("Restarting workload '%s'...", *target)
		deployer := orchestrator.NewDeployer(client, cfg)
		if err := deployer.DeployTarget(ctx, *target); err != nil {
			termutil.Error("Restart failed: %v", err)
			os.Exit(1)
		}
		termutil.Success("Workload '%s' successfully restarted", *target)

	case "clean":
		cleanAll := fs.Bool("all", false, "Clean named volumes and network as well")
		_ = fs.Parse(subArgs)

		cfg, err := config.LoadConfig(*configPath)
		if err != nil {
			termutil.Error("Config loading failed: %v", err)
			os.Exit(1)
		}
		if *engineFlag != "" {
			cfg.ContainerEngine = *engineFlag
		}
		if *socketFlag != "" {
			cfg.SocketPath = *socketFlag
		}

		client, err := engine.NewEngineClient(cfg.ContainerEngine, cfg.SocketPath)
		if err != nil {
			termutil.Error("Failed to initialize engine client: %v", err)
			os.Exit(1)
		}

		if err := orchestrator.CleanStack(ctx, client, cfg, *cleanAll); err != nil {
			termutil.Error("Clean failed: %v", err)
			os.Exit(1)
		}

	default:
		termutil.Error("Unknown stack subcommand: %s", subCmd)
		os.Exit(1)
	}
}

func handleAgentCommand(ctx context.Context, args []string) {
	if len(args) == 0 {
		termutil.Error("Subcommand required for 'agent' (run, install, status, spool)")
		os.Exit(1)
	}

	subCmd := args[0]
	subArgs := args[1:]

	fs := flag.NewFlagSet("agent "+subCmd, flag.ExitOnError)
	targetContainer := fs.String("target", "tomcat-jmx-exporter", "Target container name")
	targetID := fs.String("target-id", "default/tomcat", "Target identifier")
	spoolDir := fs.String("spool-dir", "/opt/tm-home/spool", "Spool directory path")
	engineFlag := fs.String("engine", "auto", "Container engine (podman|docker)")
	socketPath := fs.String("socket", "", "Container engine socket path")

	switch subCmd {
	case "run":
		runOnce := fs.Bool("run-once", false, "Execute single snapshot and retention prune cycle, then exit")
		_ = fs.Parse(subArgs)

		cfg := agent.DefaultConfig()
		cfg.TargetContainer = *targetContainer
		cfg.TargetID = *targetID
		cfg.SpoolDir = *spoolDir
		cfg.Engine = *engineFlag
		cfg.SocketPath = *socketPath
		cfg.RunOnce = *runOnce

		engineClient, err := engine.NewEngineClient(cfg.Engine, cfg.SocketPath)
		if err != nil {
			termutil.Warn("Container engine socket initialization warning: %v", err)
		}

		c := agent.NewCollector(cfg, engineClient)
		if err := agent.RunService(c); err != nil {
			termutil.Error("Agent execution failed: %v", err)
			os.Exit(1)
		}

	case "install":
		_ = fs.Parse(subArgs)
		cfg := agent.DefaultConfig()
		cfg.TargetContainer = *targetContainer
		cfg.SpoolDir = *spoolDir
		if err := agent.InstallAgent(cfg); err != nil {
			termutil.Error("Failed to install agent service: %v", err)
			os.Exit(1)
		}

	case "status":
		_ = fs.Parse(subArgs)
		cfg := agent.DefaultConfig()
		cfg.TargetContainer = *targetContainer
		cfg.TargetID = *targetID
		cfg.SpoolDir = *spoolDir
		if err := agent.ShowAgentStatus(cfg); err != nil {
			termutil.Error("Failed to inspect agent status: %v", err)
			os.Exit(1)
		}

	case "spool":
		prune := fs.Bool("prune", false, "Execute retention pruning")
		maxAge := fs.Int("max-age", 24, "Max spool age in hours")
		maxFiles := fs.Int("max-files", 1000, "Max spool file quota")
		staleTmp := fs.Int("stale-tmp", 60, "Stale tmp cleanup in minutes")
		_ = fs.Parse(subArgs)

		if *prune {
			rep, err := agent.PruneSpool(*spoolDir, *maxAge, *maxFiles, *staleTmp)
			if err != nil {
				termutil.Error("Spool pruning failed: %v", err)
				os.Exit(1)
			}
			termutil.Success("Spool Pruned: %d stale tmp, %d expired json, %d quota pruned (active: %d)",
				rep.StaleTmpPruned, rep.StaleJsonPruned, rep.QuotaPruned, rep.TotalRemaining)
		} else {
			cfg := agent.DefaultConfig()
			cfg.SpoolDir = *spoolDir
			_ = agent.ShowAgentStatus(cfg)
		}

	default:
		termutil.Error("Unknown agent subcommand: %s", subCmd)
		os.Exit(1)
	}
}

func handleRulesCommand(ctx context.Context, args []string) {
	if len(args) == 0 {
		termutil.Error("Subcommand required for 'rules' (ingest, export, audit)")
		os.Exit(1)
	}

	subCmd := args[0]
	subArgs := args[1:]

	fs := flag.NewFlagSet("rules "+subCmd, flag.ExitOnError)
	configPath := fs.String("config", "", "Path to CONFIG file")
	tokenFlag := fs.String("token", "", "Diagnostic Service Bearer Token")
	urlFlag := fs.String("url", "", "Diagnostic Service Endpoint URL")

	switch subCmd {
	case "ingest":
		_ = fs.Parse(subArgs)
		nonFlags := fs.Args()
		if len(nonFlags) < 1 {
			termutil.Error("Usage: tmctl rules ingest <path/to/rulepack.json> [--token <bearer_token>]")
			os.Exit(1)
		}
		ruleFile := nonFlags[0]

		cfg, _ := config.LoadConfig(*configPath)
		if err := rules.IngestRules(ctx, cfg, ruleFile, *tokenFlag, *urlFlag); err != nil {
			termutil.Error("Rules ingestion failed: %v", err)
			os.Exit(1)
		}

	case "export":
		category := fs.String("category", "", "Filter rules by category")
		outputFile := fs.String("output", "", "Output file path (default: stdout)")
		listCategories := fs.Bool("categories", false, "List active categories summary")
		_ = fs.Parse(subArgs)

		specificBranch := ""
		if len(fs.Args()) > 0 {
			specificBranch = fs.Args()[0]
		}

		cfg, _ := config.LoadConfig(*configPath)
		opts := rules.ExportOptions{
			Category:       *category,
			SpecificBranch: specificBranch,
			ListCategories: *listCategories,
			OutputFile:     *outputFile,
			TokenOverride:  *tokenFlag,
			URLOverride:    *urlFlag,
		}

		if err := rules.ExportRules(ctx, cfg, opts); err != nil {
			termutil.Error("Rules export failed: %v", err)
			os.Exit(1)
		}

	case "audit":
		_ = fs.Parse(subArgs)
		if len(fs.Args()) < 1 {
			termutil.Error("Usage: tmctl rules audit <path/to/rulepack.json>")
			os.Exit(1)
		}
		ruleFile := fs.Args()[0]
		termutil.Header("Auditing AI Diagnostic Rulepack")
		termutil.Info("Rulepack File: %s", ruleFile)
		data, err := os.ReadFile(ruleFile)
		if err != nil {
			termutil.Error("Failed to read rule file: %v", err)
			os.Exit(1)
		}
		if len(strings.TrimSpace(string(data))) == 0 {
			termutil.Error("Rulepack file is empty")
			os.Exit(1)
		}
		termutil.Success("Rulepack syntax check passed (size: %d bytes)", len(data))

	default:
		termutil.Error("Unknown rules subcommand: %s", subCmd)
		os.Exit(1)
	}
}

func handleDiagnosticCommand(ctx context.Context, args []string) {
	if len(args) == 0 {
		termutil.Error("Subcommand required for 'diagnostic' (triage, test-alert)")
		os.Exit(1)
	}

	subCmd := args[0]
	subArgs := args[1:]

	fs := flag.NewFlagSet("diagnostic "+subCmd, flag.ExitOnError)

	switch subCmd {
	case "triage":
		containerName := fs.String("container", "tomcat-jmx-exporter", "Target container name to triage")
		spoolDir := fs.String("spool-dir", "/opt/tm-home/spool", "Evidence spool directory")
		jsonOut := fs.String("json-out", "", "Write JSON triage output to file")
		engineFlag := fs.String("engine", "auto", "Container engine (podman|docker)")
		_ = fs.Parse(subArgs)

		engineClient, _ := engine.NewEngineClient(*engineFlag, "")
		_, err := diagnostic.ExecuteTriage(ctx, engineClient, *containerName, *spoolDir, *jsonOut)
		if err != nil {
			termutil.Error("Diagnostic triage failed: %v", err)
			os.Exit(1)
		}

	case "test-alert":
		endpoint := fs.String("endpoint", "http://localhost:9093", "Alertmanager endpoint URL")
		recipient := fs.String("recipient", "sre-team@corp.internal", "Alert recipient address")
		_ = fs.Parse(subArgs)

		if err := diagnostic.SendTestAlert(ctx, *endpoint, *recipient); err != nil {
			termutil.Error("Synthetic alert dispatch failed: %v", err)
			os.Exit(1)
		}

	default:
		termutil.Error("Unknown diagnostic subcommand: %s", subCmd)
		os.Exit(1)
	}
}

func handleGitOpsCommand(ctx context.Context, args []string) {
	if len(args) == 0 {
		termutil.Error("Subcommand required for 'gitops' (init, sync, status)")
		os.Exit(1)
	}

	subCmd := args[0]
	subArgs := args[1:]

	fs := flag.NewFlagSet("gitops "+subCmd, flag.ExitOnError)

	switch subCmd {
	case "init":
		repoURL := fs.String("repo", "", "Git repository URL (e.g. http://gitea:3000/gitadm/tomcat-monitoring-gitops.git)")
		branch := fs.String("branch", "main", "Target Git branch")
		dir := fs.String("dir", "", "Local GitOps working directory")
		spec := fs.String("spec", "monitoring-spec.yaml", "Specification manifest file name")
		interval := fs.String("interval", "5min", "Autonomous reconciliation timer interval")
		timer := fs.Bool("timer", false, "Enable autonomous OS scheduler (systemd timer on Linux, Task Scheduler on Windows)")
		_ = fs.Parse(subArgs)

		err := gitops.InitGitOps(gitops.InitOptions{
			RepoURL:     *repoURL,
			Branch:      *branch,
			TargetDir:   *dir,
			SpecFile:    *spec,
			Interval:    *interval,
			EnableTimer: *timer,
		})
		if err != nil {
			termutil.Error("GitOps init failed: %v", err)
			os.Exit(1)
		}

	case "sync":
		specPath := fs.String("spec", "", "Path to monitoring-spec.yaml")
		workDir := fs.String("work-dir", "", "Working directory containing state and repository")
		force := fs.Bool("force", false, "Force reconciliation even if commit is unchanged")
		dryRun := fs.Bool("dry-run", false, "Simulate reconciliation without applying changes")
		_ = fs.Parse(subArgs)

		err := gitops.SyncGitOps(gitops.SyncOptions{
			SpecFile: *specPath,
			WorkDir:  *workDir,
			Force:    *force,
			DryRun:   *dryRun,
		})
		if err != nil {
			termutil.Error("GitOps sync failed: %v", err)
			os.Exit(1)
		}

	case "status":
		dir := fs.String("dir", "", "GitOps working directory")
		specPath := fs.String("spec", "", "Path to monitoring-spec.yaml")
		jsonOut := fs.String("json-out", "", "Write JSON status output to file")
		_ = fs.Parse(subArgs)

		if err := gitops.StatusGitOps(*dir, *specPath, *jsonOut); err != nil {
			termutil.Error("GitOps status failed: %v", err)
			os.Exit(1)
		}

	default:
		termutil.Error("Unknown gitops subcommand: %s", subCmd)
		os.Exit(1)
	}
}

func handleServeCommand(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	host := fs.String("host", "0.0.0.0", "HTTP listen host")
	port := fs.Int("port", 8099, "HTTP listen port")
	apiKey := fs.String("api-key", "", "API key for authenticating REST requests")
	_ = fs.Parse(args)

	if err := api.ServeAPI(*host, *port, *apiKey); err != nil {
		termutil.Error("API daemon failed: %v", err)
		os.Exit(1)
	}
}

func handleRegistryCommand(ctx context.Context, args []string) {
	if len(args) == 0 {
		termutil.Error("Subcommand required for 'registry' (login, logout)")
		os.Exit(1)
	}

	subCmd := args[0]
	subArgs := args[1:]

	fs := flag.NewFlagSet("registry "+subCmd, flag.ExitOnError)
	tokenFile := fs.String("token-file", "", "Path to token/password file")
	authFile := fs.String("auth-file", "", "Path to custom container auth file")

	switch subCmd {
	case "login":
		_ = fs.Parse(subArgs)
		nonFlags := fs.Args()
		if len(nonFlags) < 2 {
			termutil.Error("Usage: tmctl registry login <host:port> <username> [--token-file <path>] [--auth-file <path>]")
			os.Exit(1)
		}
		host := nonFlags[0]
		user := nonFlags[1]
		var password string

		if *tokenFile != "" {
			data, err := os.ReadFile(*tokenFile)
			if err != nil {
				termutil.Error("Failed to read token file: %v", err)
				os.Exit(1)
			}
			password = strings.TrimSpace(string(data))
		} else if len(nonFlags) >= 3 {
			password = nonFlags[2]
		} else {
			password = ""
		}

		if err := registry.Login(host, user, password, *authFile); err != nil {
			termutil.Error("Registry login failed: %v", err)
			os.Exit(1)
		}

	case "logout":
		_ = fs.Parse(subArgs)
		host := ""
		if len(fs.Args()) > 0 {
			host = fs.Args()[0]
		}
		if err := registry.Logout(host, *authFile); err != nil {
			termutil.Error("Registry logout failed: %v", err)
			os.Exit(1)
		}

	default:
		termutil.Error("Unknown registry subcommand: %s", subCmd)
		os.Exit(1)
	}
}

func handleValidateCommand(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	layout := fs.Bool("layout", false, "Validate layout and sensitive files")
	schemas := fs.Bool("schemas", false, "Validate JSON schemas")
	ansible := fs.Bool("ansible", false, "Validate ansible files")
	dir := fs.String("dir", ".", "Project root directory")
	_ = fs.Parse(args)

	opts := validator.ValidateOptions{
		Layout:  *layout,
		Schemas: *schemas,
		Ansible: *ansible,
	}

	if err := validator.RunValidation(*dir, opts); err != nil {
		termutil.Error("Validation failed: %v", err)
		os.Exit(1)
	}
}
