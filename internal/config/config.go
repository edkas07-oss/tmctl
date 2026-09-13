package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// StackConfig represents the unified platform configuration.
type StackConfig struct {
	PlatformName        string
	NetworkName         string
	ContainerEngine     string
	SocketPath          string
	DiagnosticURL       string
	BearerToken         string
	RegistryURL         string
	RegistryNamespace   string
	RegistryTLSVerify   bool
	ImagePullPolicy     string
	RegistryAuthFile    string

	TomcatHTTPPort      int
	TomcatJMXPort       int
	PrometheusPort      int
	AlertmanagerPort    int
	DiagnosticPort      int
	MailpitHTTPPort     int
	MailpitSMTPPort     int
	PostfixPort         int

	TomcatLogVolume           string
	DiagnosticDataVolume      string
	PrometheusConfigVolume    string
	PrometheusTruststoreVolume string
	PrometheusDataVolume      string
	AlertmanagerConfigVolume  string
	AlertmanagerTruststoreVolume string
	AlertmanagerDataVolume    string

	DefaultSpoolDir     string
	DefaultSecretsDir   string
	DefaultTLSDir       string
	DefaultJMXTLSDir    string

	ProjectRoot         string
}

var envVarPattern = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)

// DefaultConfig returns default configuration parameters.
func DefaultConfig() *StackConfig {
	homeDir, _ := os.UserHomeDir()
	return &StackConfig{
		PlatformName:        "tomcat-monitoring",
		NetworkName:         "devops-lab",
		DiagnosticURL:       "https://localhost:8443",
		BearerToken:         "test-token-12345",
		RegistryURL:         "localhost",
		RegistryNamespace:   "",
		RegistryTLSVerify:   true,
		ImagePullPolicy:     "IfNotPresent",
		RegistryAuthFile:    "",

		TomcatHTTPPort:      8083,
		TomcatJMXPort:       9404,
		PrometheusPort:      9090,
		AlertmanagerPort:    9093,
		DiagnosticPort:      8443,
		MailpitHTTPPort:     8025,
		MailpitSMTPPort:     1025,
		PostfixPort:         587,

		TomcatLogVolume:           "tomcat_logs",
		DiagnosticDataVolume:      "diagnostic_data",
		PrometheusConfigVolume:    "prometheus_config",
		PrometheusTruststoreVolume: "prometheus_truststore",
		PrometheusDataVolume:      "prometheus_data",
		AlertmanagerConfigVolume:  "alertmanager_config",
		AlertmanagerTruststoreVolume: "alertmanager_truststore",
		AlertmanagerDataVolume:    "alertmanager_data",

		DefaultSpoolDir:     filepath.Join(homeDir, ".local", "share", "tomcat-monitoring", "spool"),
		DefaultSecretsDir:   filepath.Join(homeDir, ".local", "share", "tomcat-monitoring", "diagnostic-service-secrets"),
		DefaultTLSDir:       filepath.Join(homeDir, ".local", "share", "tomcat-monitoring", "diagnostic-service-tls"),
		DefaultJMXTLSDir:    filepath.Join(homeDir, ".local", "share", "tomcat-monitoring", "jmx-exporter-tls"),
	}
}

// LoadConfig attempts to locate CONFIG file or fallback to defaults + environment variables.
func LoadConfig(configPath string) (*StackConfig, error) {
	cfg := DefaultConfig()

	// Locate root if not provided
	if configPath == "" {
		candidates := []string{
			"CONFIG",
			"../CONFIG",
			"../../CONFIG",
			filepath.Join("tomcat-monitoring", "CONFIG"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				configPath = c
				break
			}
		}
	}

	if configPath != "" {
		if abs, err := filepath.Abs(configPath); err == nil {
			cfg.ProjectRoot = filepath.Dir(abs)
		}
		if err := parseConfigFile(configPath, cfg); err != nil {
			// If file specified explicitly and fails, return err; if default candidate fails, log warning
			return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
		}
	}

	// Apply Environment Variable Overrides
	applyEnvOverrides(cfg)

	return cfg, nil
}

func parseConfigFile(path string, cfg *StackConfig) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		matches := envVarPattern.FindStringSubmatch(line)
		if len(matches) == 3 {
			key := matches[1]
			val := cleanValue(matches[2])
			assignConfigKey(cfg, key, val)
		}
	}
	return scanner.Err()
}

func cleanValue(raw string) string {
	val := strings.TrimSpace(raw)
	// Strip surrounding double quotes or single quotes first
	if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
		(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
		if len(val) >= 2 {
			val = val[1 : len(val)-1]
		}
	}
	// Handle bash variable expansion like "${VAR:-default}" or "${VAR}"
	if strings.HasPrefix(val, "${") && strings.HasSuffix(val, "}") {
		inner := val[2 : len(val)-1]
		if idx := strings.Index(inner, ":-"); idx != -1 {
			envKey := inner[:idx]
			defaultVal := inner[idx+2:]
			if envVal := os.Getenv(envKey); envVal != "" {
				return envVal
			}
			return defaultVal
		}
		if envVal := os.Getenv(inner); envVal != "" {
			return envVal
		}
		return ""
	}
	return val
}

func assignConfigKey(cfg *StackConfig, key, val string) {
	switch key {
	case "PLATFORM_NAME":
		if val != "" {
			cfg.PlatformName = val
		}
	case "NETWORK_NAME":
		if val != "" {
			cfg.NetworkName = val
		}
	case "CONTAINER_ENGINE":
		if val != "" {
			cfg.ContainerEngine = val
		}
	case "SOCKET_PATH":
		if val != "" {
			cfg.SocketPath = val
		}
	case "DIAGNOSTIC_URL":
		if val != "" {
			cfg.DiagnosticURL = val
		}
	case "BEARER_TOKEN":
		if val != "" {
			cfg.BearerToken = val
		}
	case "REGISTRY_URL":
		if val != "" {
			cfg.RegistryURL = val
		}
	case "REGISTRY_NAMESPACE":
		cfg.RegistryNamespace = val
	case "REGISTRY_TLS_VERIFY":
		cfg.RegistryTLSVerify = (val != "false" && val != "0")
	case "IMAGE_PULL_POLICY":
		if val != "" {
			cfg.ImagePullPolicy = val
		}
	case "REGISTRY_AUTH_FILE":
		cfg.RegistryAuthFile = val
	case "TOMCAT_HTTP_PORT":
		if p, err := strconv.Atoi(val); err == nil {
			cfg.TomcatHTTPPort = p
		}
	case "TOMCAT_JMX_PORT":
		if p, err := strconv.Atoi(val); err == nil {
			cfg.TomcatJMXPort = p
		}
	case "PROMETHEUS_PORT":
		if p, err := strconv.Atoi(val); err == nil {
			cfg.PrometheusPort = p
		}
	case "ALERTMANAGER_PORT":
		if p, err := strconv.Atoi(val); err == nil {
			cfg.AlertmanagerPort = p
		}
	case "DIAGNOSTIC_PORT":
		if p, err := strconv.Atoi(val); err == nil {
			cfg.DiagnosticPort = p
		}
	case "MAILPIT_HTTP_PORT":
		if p, err := strconv.Atoi(val); err == nil {
			cfg.MailpitHTTPPort = p
		}
	case "MAILPIT_SMTP_PORT":
		if p, err := strconv.Atoi(val); err == nil {
			cfg.MailpitSMTPPort = p
		}
	case "POSTFIX_PORT":
		if p, err := strconv.Atoi(val); err == nil {
			cfg.PostfixPort = p
		}
	case "TOMCAT_LOG_VOLUME":
		if val != "" {
			cfg.TomcatLogVolume = val
		}
	case "DIAGNOSTIC_DATA_VOLUME":
		if val != "" {
			cfg.DiagnosticDataVolume = val
		}
	case "PROMETHEUS_CONFIG_VOLUME":
		if val != "" {
			cfg.PrometheusConfigVolume = val
		}
	case "PROMETHEUS_TRUSTSTORE_VOLUME":
		if val != "" {
			cfg.PrometheusTruststoreVolume = val
		}
	case "PROMETHEUS_DATA_VOLUME":
		if val != "" {
			cfg.PrometheusDataVolume = val
		}
	case "ALERTMANAGER_CONFIG_VOLUME":
		if val != "" {
			cfg.AlertmanagerConfigVolume = val
		}
	case "ALERTMANAGER_TRUSTSTORE_VOLUME":
		if val != "" {
			cfg.AlertmanagerTruststoreVolume = val
		}
	case "ALERTMANAGER_DATA_VOLUME":
		if val != "" {
			cfg.AlertmanagerDataVolume = val
		}
	case "DEFAULT_SPOOL_DIR":
		if val != "" {
			cfg.DefaultSpoolDir = val
		}
	case "DEFAULT_SECRETS_DIR":
		if val != "" {
			cfg.DefaultSecretsDir = val
		}
	case "DEFAULT_TLS_DIR":
		if val != "" {
			cfg.DefaultTLSDir = val
		}
	case "DEFAULT_JMX_TLS_DIR":
		if val != "" {
			cfg.DefaultJMXTLSDir = val
		}
	}
}

func applyEnvOverrides(cfg *StackConfig) {
	if v := os.Getenv("CONTAINER_ENGINE"); v != "" {
		cfg.ContainerEngine = v
	}
	if v := os.Getenv("SOCKET_PATH"); v != "" {
		cfg.SocketPath = v
	}
	if v := os.Getenv("DIAGNOSTIC_URL"); v != "" {
		cfg.DiagnosticURL = v
	}
	if v := os.Getenv("BEARER_TOKEN"); v != "" {
		cfg.BearerToken = v
	}
	if v := os.Getenv("REGISTRY_URL"); v != "" {
		cfg.RegistryURL = v
	}
	if v := os.Getenv("REGISTRY_AUTH_FILE"); v != "" {
		cfg.RegistryAuthFile = v
	}
}
