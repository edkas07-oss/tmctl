package gitops

import (
	"fmt"
)

// MonitoringSpec represents the declarative configuration in monitoring-spec.yaml.
type MonitoringSpec struct {
	Version  string              `yaml:"version" json:"version"`
	Metadata MonitoringMetadata  `yaml:"metadata" json:"metadata"`
	Spec     MonitoringStackSpec `yaml:"spec" json:"spec"`
}

// MonitoringMetadata contains identifying metadata for the monitoring node.
type MonitoringMetadata struct {
	Environment string `yaml:"environment" json:"environment"`
	Tier        string `yaml:"tier" json:"tier"`
	Topology    string `yaml:"topology" json:"topology"` // all-in-one, agent-only, central-hub
}

// AgentSpec defines host-level telemetry collector parameters.
type AgentSpec struct {
	Enabled          bool     `yaml:"enabled" json:"enabled"`
	TargetContainer  string   `yaml:"targetContainer" json:"targetContainer"`
	TargetContainers []string `yaml:"targetContainers,omitempty" json:"targetContainers,omitempty"`
	SpoolDir         string   `yaml:"spoolDir" json:"spoolDir"`
	RetentionHours   int      `yaml:"retentionHours" json:"retentionHours"`
	MaxFiles         int      `yaml:"maxFiles" json:"maxFiles"`
}

// ServiceContainerSpec defines container image, port, and resource constraints.
type ServiceContainerSpec struct {
	Image       string `yaml:"image" json:"image"`
	Port        int    `yaml:"port,omitempty" json:"port,omitempty"`
	MemoryLimit string `yaml:"memoryLimit,omitempty" json:"memoryLimit,omitempty"`
	Enabled     bool   `yaml:"enabled" json:"enabled"`
}

// StackServicesSpec defines the set of monitoring services.
type StackServicesSpec struct {
	DiagnosticService ServiceContainerSpec `yaml:"diagnosticService" json:"diagnosticService"`
	Prometheus        ServiceContainerSpec `yaml:"prometheus" json:"prometheus"`
	Alertmanager      ServiceContainerSpec `yaml:"alertmanager" json:"alertmanager"`
	PostfixRelay      ServiceContainerSpec `yaml:"postfixRelay" json:"postfixRelay"`
}

// RulesSpec defines diagnostic rules auto-ingestion.
type RulesSpec struct {
	AutoIngest   bool   `yaml:"autoIngest" json:"autoIngest"`
	RulepackPath string `yaml:"rulepackPath" json:"rulepackPath"`
	Endpoint     string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
}

// MonitoringStackSpec defines the overall monitoring stack specification.
type MonitoringStackSpec struct {
	Engine string            `yaml:"engine,omitempty" json:"engine,omitempty"` // podman / docker
	Agent  AgentSpec         `yaml:"agent" json:"agent"`
	Stack  StackServicesSpec `yaml:"stack" json:"stack"`
	Rules  RulesSpec         `yaml:"rules" json:"rules"`
}

// GitOpsConfig stores local GitOps repository binding details.
type GitOpsConfig struct {
	RepoURL   string `json:"repo_url"`
	Branch    string `json:"branch"`
	SpecFile  string `json:"spec_file"`
	TargetDir string `json:"target_dir"`
	Interval  string `json:"interval"`
	UpdatedAt string `json:"updated_at"`
}

// GitOpsState tracks the reconciliation history and active commit.
type GitOpsState struct {
	LastCommit     string   `json:"last_commit"`
	CommitMessage  string   `json:"commit_message"`
	LastSyncTime   string   `json:"last_sync_time"`
	Status         string   `json:"status"` // In-Sync, Drift-Detected, Error
	ActiveServices []string `json:"active_services"`
	Error          string   `json:"error,omitempty"`
}

// Validate verifies structural constraints of MonitoringSpec.
func (s *MonitoringSpec) Validate() error {
	if s.Version == "" {
		return fmt.Errorf("version is required in monitoring-spec.yaml")
	}
	if s.Metadata.Environment == "" {
		return fmt.Errorf("metadata.environment is required")
	}
	if s.Metadata.Topology == "" {
		s.Metadata.Topology = "all-in-one"
	}
	if s.Spec.Agent.Enabled && s.Spec.Agent.TargetContainer == "" {
		s.Spec.Agent.TargetContainer = "tomcat-jmx-exporter"
	}
	if s.Spec.Agent.SpoolDir == "" {
		s.Spec.Agent.SpoolDir = "/opt/tm-home/spool"
	}
	if s.Spec.Agent.RetentionHours <= 0 {
		s.Spec.Agent.RetentionHours = 24
	}
	return nil
}

// StarterMonitoringSpec generates a default starter manifest.
func StarterMonitoringSpec() *MonitoringSpec {
	return &MonitoringSpec{
		Version: "1.0",
		Metadata: MonitoringMetadata{
			Environment: "production",
			Tier:        "monitoring",
			Topology:    "all-in-one",
		},
		Spec: MonitoringStackSpec{
			Engine: "auto",
			Agent: AgentSpec{
				Enabled:         true,
				TargetContainer: "tomcat-jmx-exporter",
				SpoolDir:        "/opt/tm-home/spool",
				RetentionHours:  24,
				MaxFiles:        1000,
			},
			Stack: StackServicesSpec{
				DiagnosticService: ServiceContainerSpec{
					Image:       "localhost:5000/tomcat-diagnostic-service:1.0.0",
					Port:        3000,
					MemoryLimit: "512M",
					Enabled:     true,
				},
				Prometheus: ServiceContainerSpec{
					Image:       "prom/prometheus:v2.45.0",
					Port:        9090,
					MemoryLimit: "1G",
					Enabled:     true,
				},
				Alertmanager: ServiceContainerSpec{
					Image:       "prom/alertmanager:v0.25.0",
					Port:        9093,
					MemoryLimit: "256M",
					Enabled:     true,
				},
				PostfixRelay: ServiceContainerSpec{
					Image:   "localhost:5000/postfix-relay:1.0.0",
					Port:    587,
					Enabled: true,
				},
			},
			Rules: RulesSpec{
				AutoIngest:   true,
				RulepackPath: "rules/enterprise-rulepack.json",
			},
		},
	}
}

// GenerateStarterYAML returns string representation of default spec.
func GenerateStarterYAML() string {
	return `version: "1.0"
metadata:
  environment: "production"
  tier: "monitoring"
  topology: "all-in-one"

spec:
  engine: "auto"
  agent:
    enabled: true
    targetContainer: "tomcat-jmx-exporter"
    spoolDir: "/opt/tm-home/spool"
    retentionHours: 24
    maxFiles: 1000

  stack:
    diagnosticService:
      image: "localhost:5000/tomcat-diagnostic-service:1.0.0"
      port: 3000
      memoryLimit: "512M"
      enabled: true
    prometheus:
      image: "prom/prometheus:v2.45.0"
      port: 9090
      memoryLimit: "1G"
      enabled: true
    alertmanager:
      image: "prom/alertmanager:v0.25.0"
      port: 9093
      memoryLimit: "256M"
      enabled: true
    postfixRelay:
      image: "localhost:5000/postfix-relay:1.0.0"
      port: 587
      enabled: true

  rules:
    autoIngest: true
    rulepackPath: "rules/enterprise-rulepack.json"
`
}
