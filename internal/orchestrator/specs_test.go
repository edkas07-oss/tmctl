package orchestrator

import (
	"testing"

	"github.com/eddywiyatno/tmctl/internal/config"
)

func TestWorkloadSpecs(t *testing.T) {
	cfg := config.DefaultConfig()
	builder := NewWorkloadSpecBuilder(cfg)

	tomcatSpec := builder.BuildTomcatSpec()
	if tomcatSpec.Name != "tomcat-jmx-exporter" {
		t.Errorf("expected tomcat-jmx-exporter, got %s", tomcatSpec.Name)
	}
	if len(tomcatSpec.Ports) != 2 {
		t.Errorf("expected 2 ports for tomcat, got %d", len(tomcatSpec.Ports))
	}

	promSpec := builder.BuildPrometheusSpec()
	if promSpec.Name != "prometheus" {
		t.Errorf("expected prometheus, got %s", promSpec.Name)
	}

	amSpec := builder.BuildAlertmanagerSpec()
	if amSpec.Name != "alertmanager" {
		t.Errorf("expected alertmanager, got %s", amSpec.Name)
	}

	diagSpec := builder.BuildDiagnosticSpec()
	if diagSpec.Name != "diagnostic-service" {
		t.Errorf("expected diagnostic-service, got %s", diagSpec.Name)
	}
}
