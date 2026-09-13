package orchestrator

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/eddywiyatno/tomcat-monitoring/tmctl/internal/config"
	"github.com/eddywiyatno/tomcat-monitoring/tmctl/internal/engine"
)

// WorkloadSpecBuilder builds container specifications for platform services.
type WorkloadSpecBuilder struct {
	cfg *config.StackConfig
}

// NewWorkloadSpecBuilder creates a builder with provided config.
func NewWorkloadSpecBuilder(cfg *config.StackConfig) *WorkloadSpecBuilder {
	return &WorkloadSpecBuilder{cfg: cfg}
}

func (b *WorkloadSpecBuilder) formatImage(name, defaultTag string) string {
	reg := b.cfg.RegistryURL
	ns := b.cfg.RegistryNamespace
	if ns != "" {
		return fmt.Sprintf("%s/%s/%s:%s", reg, ns, name, defaultTag)
	}
	return fmt.Sprintf("%s/%s:%s", reg, name, defaultTag)
}

func (b *WorkloadSpecBuilder) getVolumeMode(readOnly bool, shared bool) string {
	// SELinux volume relabeling on Linux
	if runtime.GOOS == "linux" {
		if readOnly {
			return "ro,z"
		}
		return "z"
	}
	if readOnly {
		return "ro"
	}
	return "rw"
}

// BuildTomcatSpec creates Tomcat JMX Exporter container spec.
func (b *WorkloadSpecBuilder) BuildTomcatSpec() engine.ContainerSpec {
	repoRoot := b.cfg.ProjectRoot
	configPath := filepath.Join(repoRoot, "config", "jmx-exporter", "jmx-exporter.yml")
	keystorePath := filepath.Join(b.cfg.DefaultJMXTLSDir, "keystore.p12")
	passPath := filepath.Join(b.cfg.DefaultJMXTLSDir, "keystore-password")

	return engine.ContainerSpec{
		Name:          "tomcat-jmx-exporter",
		Image:         b.formatImage("tomcat-jmx-exporter", "1.0.0"),
		Network:       b.cfg.NetworkName,
		NetworkAlias:  "tomcat-jmx-exporter",
		RestartPolicy: "on-failure:5",
		Ports: []engine.PortBinding{
			{HostPort: b.cfg.TomcatHTTPPort, ContainerPort: 8080, Protocol: "tcp"},
			{HostPort: b.cfg.TomcatJMXPort, ContainerPort: 9404, Protocol: "tcp"},
		},
		Volumes: []engine.VolumeMount{
			{Source: configPath, Target: "/etc/tomcat-jmx-exporter/config.yml", ReadOnly: true, Mode: "ro"},
			{Source: keystorePath, Target: "/run/secrets/tomcat-jmx-exporter/keystore.p12", ReadOnly: true, Mode: "ro"},
			{Source: passPath, Target: "/run/secrets/tomcat-jmx-exporter/keystore-password", ReadOnly: true, Mode: "ro"},
			{Source: b.cfg.TomcatLogVolume, Target: "/usr/local/tomcat/logs", ReadOnly: false, Mode: b.getVolumeMode(false, true), IsVolume: true},
		},
	}
}

// BuildPrometheusSpec creates Prometheus container spec.
func (b *WorkloadSpecBuilder) BuildPrometheusSpec() engine.ContainerSpec {
	return engine.ContainerSpec{
		Name:          "prometheus",
		Image:         b.formatImage("prometheus", "1.0.0"),
		Network:       b.cfg.NetworkName,
		NetworkAlias:  "prometheus",
		RestartPolicy: "on-failure:5",
		Ports: []engine.PortBinding{
			{HostPort: b.cfg.PrometheusPort, ContainerPort: 9090, Protocol: "tcp"},
		},
		Volumes: []engine.VolumeMount{
			{Source: b.cfg.PrometheusConfigVolume, Target: "/etc/prometheus", ReadOnly: true, Mode: "ro", IsVolume: true},
			{Source: b.cfg.PrometheusTruststoreVolume, Target: "/etc/prometheus/ssl", ReadOnly: true, Mode: "ro", IsVolume: true},
			{Source: b.cfg.PrometheusDataVolume, Target: "/prometheus", ReadOnly: false, Mode: b.getVolumeMode(false, true), IsVolume: true},
		},
	}
}

// BuildAlertmanagerSpec creates Alertmanager container spec.
func (b *WorkloadSpecBuilder) BuildAlertmanagerSpec() engine.ContainerSpec {
	return engine.ContainerSpec{
		Name:          "alertmanager",
		Image:         b.formatImage("alertmanager", "1.0.0"),
		Network:       b.cfg.NetworkName,
		NetworkAlias:  "alertmanager",
		RestartPolicy: "on-failure:5",
		Ports: []engine.PortBinding{
			{HostPort: b.cfg.AlertmanagerPort, ContainerPort: 9093, Protocol: "tcp"},
		},
		Volumes: []engine.VolumeMount{
			{Source: b.cfg.AlertmanagerConfigVolume, Target: "/etc/alertmanager", ReadOnly: true, Mode: "ro", IsVolume: true},
			{Source: b.cfg.AlertmanagerTruststoreVolume, Target: "/etc/alertmanager/secrets", ReadOnly: true, Mode: "ro", IsVolume: true},
			{Source: b.cfg.AlertmanagerDataVolume, Target: "/alertmanager", ReadOnly: false, Mode: b.getVolumeMode(false, true), IsVolume: true},
		},
	}
}

// BuildDiagnosticSpec creates Diagnostic Service container spec.
func (b *WorkloadSpecBuilder) BuildDiagnosticSpec() engine.ContainerSpec {
	repoRoot := b.cfg.ProjectRoot
	configPath := filepath.Join(repoRoot, "config", "diagnostic-service", "application.json")
	targetsPath := filepath.Join(repoRoot, "config", "diagnostic-service", "targets.json")
	bearerToken := filepath.Join(b.cfg.DefaultSecretsDir, "bearer-token")
	smtpUser := filepath.Join(b.cfg.DefaultSecretsDir, "smtp-username")
	smtpPass := filepath.Join(b.cfg.DefaultSecretsDir, "smtp-password")
	serverCrt := filepath.Join(b.cfg.DefaultTLSDir, "server.crt")
	serverKey := filepath.Join(b.cfg.DefaultTLSDir, "server.key")
	postfixCA := filepath.Join(b.cfg.DefaultTLSDir, "postfix-ca.crt")

	volROZ := b.getVolumeMode(true, true)
	volZ := b.getVolumeMode(false, true)

	return engine.ContainerSpec{
		Name:          "diagnostic-service",
		Image:         b.formatImage("tomcat-diagnostic-service", "latest"),
		Network:       b.cfg.NetworkName,
		NetworkAlias:  "diagnostic-service",
		RestartPolicy: "on-failure:5",
		Env: []string{
			"NODE_EXTRA_CA_CERTS=/run/tomcat-diagnostic/tls/postfix-ca.crt",
		},
		Ports: []engine.PortBinding{
			{HostPort: b.cfg.DiagnosticPort, ContainerPort: b.cfg.DiagnosticPort, Protocol: "tcp"},
		},
		Volumes: []engine.VolumeMount{
			{Source: configPath, Target: "/run/tomcat-diagnostic/application.json", ReadOnly: true, Mode: volROZ},
			{Source: targetsPath, Target: "/run/tomcat-diagnostic/config/targets.json", ReadOnly: true, Mode: volROZ},
			{Source: bearerToken, Target: "/run/tomcat-diagnostic/secrets/bearer-token", ReadOnly: true, Mode: volROZ},
			{Source: smtpUser, Target: "/run/tomcat-diagnostic/secrets/smtp-username", ReadOnly: true, Mode: volROZ},
			{Source: smtpPass, Target: "/run/tomcat-diagnostic/secrets/smtp-password", ReadOnly: true, Mode: volROZ},
			{Source: serverCrt, Target: "/run/tomcat-diagnostic/tls/server.crt", ReadOnly: true, Mode: volROZ},
			{Source: serverKey, Target: "/run/tomcat-diagnostic/tls/server.key", ReadOnly: true, Mode: volROZ},
			{Source: postfixCA, Target: "/run/tomcat-diagnostic/tls/postfix-ca.crt", ReadOnly: true, Mode: volROZ},
			{Source: b.cfg.DefaultSpoolDir, Target: "/run/tomcat-diagnostic/spool", ReadOnly: true, Mode: volROZ},
			{Source: b.cfg.TomcatLogVolume, Target: "/run/tomcat-diagnostic/logs", ReadOnly: true, Mode: volROZ, IsVolume: true},
			{Source: b.cfg.DiagnosticDataVolume, Target: "/var/lib/tomcat-diagnostic", ReadOnly: false, Mode: volZ, IsVolume: true},
		},
	}
}

// BuildMailpitSpec creates Mailpit container spec.
func (b *WorkloadSpecBuilder) BuildMailpitSpec() engine.ContainerSpec {
	return engine.ContainerSpec{
		Name:          "mailpit",
		Image:         "ghcr.io/axllent/mailpit:v1.31.0",
		Network:       b.cfg.NetworkName,
		NetworkAlias:  "mailpit",
		RestartPolicy: "on-failure:5",
		Ports: []engine.PortBinding{
			{HostPort: b.cfg.MailpitHTTPPort, ContainerPort: 8025, Protocol: "tcp"},
			{HostPort: b.cfg.MailpitSMTPPort, ContainerPort: 1025, Protocol: "tcp"},
		},
	}
}

// BuildPostfixSpec creates Postfix Enterprise Relay container spec.
func (b *WorkloadSpecBuilder) BuildPostfixSpec() engine.ContainerSpec {
	return engine.ContainerSpec{
		Name:          "postfix-relay",
		Image:         b.formatImage("postfix-relay", "latest"),
		Network:       b.cfg.NetworkName,
		NetworkAlias:  "postfix-relay",
		RestartPolicy: "on-failure:5",
		Ports: []engine.PortBinding{
			{HostPort: b.cfg.PostfixPort, ContainerPort: 587, Protocol: "tcp"},
		},
	}
}
