package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/eddywiyatno/tmctl/internal/buildinfo"
	"github.com/eddywiyatno/tmctl/internal/config"
	"github.com/eddywiyatno/tmctl/internal/engine"
	"github.com/eddywiyatno/tmctl/internal/orchestrator"
	"github.com/eddywiyatno/tmctl/internal/registry"
	"github.com/eddywiyatno/tmctl/internal/rules"
	"github.com/eddywiyatno/tmctl/internal/validator"
	"github.com/eddywiyatno/tmctl/pkg/termutil"
)

func printHelp() {
	fmt.Printf(`tmctl — Unified Cross-Platform Operator CLI for Tomcat Monitoring

Usage:
  tmctl <command> [subcommand] [flags] [args...]

Available Commands:
  stack       Manage container lifecycle (deploy, status, clean)
  rules       Manage AI diagnostic rulepacks (ingest, export)
  registry    Manage container registry authentication (login, logout)
  validate    Validate platform configuration and contracts
  version     Display version and build information

Stack Subcommands:
  tmctl stack deploy [--target <name>] [--env <name>] [--engine <podman|docker>] [--config <path>]
  tmctl stack status [--config <path>]
  tmctl stack clean  [--all] [--config <path>]

Rules Subcommands:
  tmctl rules ingest <path/to/rulepack.json> [--token <bearer_token>] [--url <endpoint>]
  tmctl rules export [--category <name>] [--output <path.json>] [--categories] [--token <token>]

Registry Subcommands:
  tmctl registry login <host:port> <username> [--token-file <path>] [--auth-file <path>]
  tmctl registry logout [host:port] [--auth-file <path>]

Validation Subcommands:
  tmctl validate [--layout] [--schemas] [--ansible] [--dir <path>]

Examples:
  tmctl stack deploy --target tomcat
  tmctl stack deploy --target all --engine podman
  tmctl stack status
  tmctl rules ingest config/rules/custom-rules.json
  tmctl rules export --categories
  tmctl registry login registry.internal.corp:5000 admin
  tmctl validate

Use "tmctl <command> --help" for more information about a command.
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

	case "rules":
		handleRulesCommand(ctx, os.Args[2:])

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
		termutil.Error("Subcommand required for 'stack' (deploy, status, clean)")
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

func handleRulesCommand(ctx context.Context, args []string) {
	if len(args) == 0 {
		termutil.Error("Subcommand required for 'rules' (ingest, export)")
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

	default:
		termutil.Error("Unknown rules subcommand: %s", subCmd)
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
			// Prompt or default to empty
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
