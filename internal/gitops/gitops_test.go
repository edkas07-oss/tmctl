package gitops

import (
	"os"
	"path/filepath"
	"testing"
	"gopkg.in/yaml.v3"
)

func TestParseGitURL(t *testing.T) {
	tests := []struct {
		input   string
		baseURL string
		owner   string
		repo    string
	}{
		{"http://localhost:3000/gitadm/tomcat-monitoring-gitops.git", "http://localhost:3000", "gitadm", "tomcat-monitoring-gitops"},
		{"https://github.com/eddywiyatno/monitoring-gitops", "https://github.com", "eddywiyatno", "monitoring-gitops"},
	}

	for _, tt := range tests {
		b, o, r, err := ParseGitURL(tt.input)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", tt.input, err)
		}
		if b != tt.baseURL || o != tt.owner || r != tt.repo {
			t.Errorf("mismatch for %s: got (%s, %s, %s), want (%s, %s, %s)", tt.input, b, o, r, tt.baseURL, tt.owner, tt.repo)
		}
	}
}

func TestMonitoringSpecValidation(t *testing.T) {
	starter := StarterMonitoringSpec()
	if err := starter.Validate(); err != nil {
		t.Fatalf("starter spec should be valid: %v", err)
	}

	yamlStr := GenerateStarterYAML()
	var parsed MonitoringSpec
	if err := yaml.Unmarshal([]byte(yamlStr), &parsed); err != nil {
		t.Fatalf("failed to parse starter YAML: %v", err)
	}

	if parsed.Metadata.Topology != "all-in-one" {
		t.Errorf("expected topology all-in-one, got %s", parsed.Metadata.Topology)
	}
}

func TestGitOpsStatePersistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gitops-state-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	stateFile := filepath.Join(tempDir, "state.json")
	initial := &GitOpsState{
		LastCommit:     "a1b2c3d",
		CommitMessage:  "feat: update prometheus",
		LastSyncTime:   "2026-09-26T12:00:00Z",
		Status:         "In-Sync",
		ActiveServices: []string{"prometheus", "alertmanager"},
	}

	if err := SaveState(stateFile, initial); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	loaded, err := LoadState(stateFile)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if loaded.LastCommit != initial.LastCommit || loaded.Status != initial.Status {
		t.Errorf("loaded state mismatch: got %+v, want %+v", loaded, initial)
	}
}
