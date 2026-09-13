package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryLoginAndLogout(t *testing.T) {
	tmpDir := t.TempDir()
	authFile := filepath.Join(tmpDir, "config.json")

	// Test Login
	err := Login("harbor.internal.corp:5000", "admin", "SecureSecret123", authFile)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	data, err := os.ReadFile(authFile)
	if err != nil {
		t.Fatalf("Failed to read authfile: %v", err)
	}

	var cfg AuthConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Failed to unmarshal authfile: %v", err)
	}

	if _, ok := cfg.Auths["harbor.internal.corp:5000"]; !ok {
		t.Errorf("expected entry for harbor.internal.corp:5000 in auths map")
	}

	// Test Logout
	err = Logout("harbor.internal.corp:5000", authFile)
	if err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	dataAfter, _ := os.ReadFile(authFile)
	var cfgAfter AuthConfig
	_ = json.Unmarshal(dataAfter, &cfgAfter)
	if _, ok := cfgAfter.Auths["harbor.internal.corp:5000"]; ok {
		t.Errorf("entry still exists after logout")
	}
}
