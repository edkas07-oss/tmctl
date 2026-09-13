package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.PlatformName != "tomcat-monitoring" {
		t.Errorf("expected platform tomcat-monitoring, got %s", cfg.PlatformName)
	}
	if cfg.TomcatHTTPPort != 8083 {
		t.Errorf("expected tomcat port 8083, got %d", cfg.TomcatHTTPPort)
	}
	if cfg.PrometheusPort != 9090 {
		t.Errorf("expected prometheus port 9090, got %d", cfg.PrometheusPort)
	}
	if cfg.AlertmanagerPort != 9093 {
		t.Errorf("expected alertmanager port 9093, got %d", cfg.AlertmanagerPort)
	}
}

func TestParseConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "CONFIG")
	content := `
PLATFORM_NAME="custom-platform"
NETWORK_NAME="custom-net"
TOMCAT_HTTP_PORT=8888
CONTAINER_ENGINE="podman"
`
	if err := os.WriteFile(confPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(confPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.PlatformName != "custom-platform" {
		t.Errorf("expected custom-platform, got %s", cfg.PlatformName)
	}
	if cfg.NetworkName != "custom-net" {
		t.Errorf("expected custom-net, got %s", cfg.NetworkName)
	}
	if cfg.TomcatHTTPPort != 8888 {
		t.Errorf("expected port 8888, got %d", cfg.TomcatHTTPPort)
	}
}
