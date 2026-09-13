package validator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateSensitiveFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Should pass on clean directory
	opts := ValidateOptions{Layout: false, Schemas: false, Ansible: false}
	err := validateSensitiveFiles(tmpDir)
	if err != nil {
		t.Errorf("clean directory failed validation: %v", err)
	}

	// Create sensitive file
	badFile := filepath.Join(tmpDir, "secret.key")
	_ = os.WriteFile(badFile, []byte("forbidden"), 0600)

	err = validateSensitiveFiles(tmpDir)
	if err == nil {
		t.Errorf("expected error when sensitive .key file is present, got nil")
	}

	_ = opts
}

func TestValidateJSONFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Valid JSON
	goodFile := filepath.Join(tmpDir, "valid.json")
	_ = os.WriteFile(goodFile, []byte(`{"key": "value"}`), 0644)

	err := validateJSONFiles(tmpDir)
	if err != nil {
		t.Errorf("valid JSON failed: %v", err)
	}

	// Invalid JSON
	badFile := filepath.Join(tmpDir, "broken.json")
	_ = os.WriteFile(badFile, []byte(`{invalid json`), 0644)

	err = validateJSONFiles(tmpDir)
	if err == nil {
		t.Errorf("expected error on broken JSON, got nil")
	}
}
