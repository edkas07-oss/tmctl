package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecordValidationAndWriting(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tmctl-agent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	now := time.Now().UTC()
	record := NewEventRecord(
		"container_state",
		"test/tomcat-01",
		1,
		StatusCollected,
		StrengthDirect,
		ContainerStateValue{State: "running", Container: "tomcat-jmx-exporter"},
		now,
	)

	if err := ValidateRecord(record); err != nil {
		t.Fatalf("record validation failed: %v", err)
	}

	writtenPath, err := WriteRecord(tempDir, record, 16384)
	if err != nil {
		t.Fatalf("failed to write record: %v", err)
	}

	if _, err := os.Stat(writtenPath); os.IsNotExist(err) {
		t.Fatalf("written file does not exist: %s", writtenPath)
	}

	// Test retention pruning
	report, err := PruneSpool(tempDir, 24, 100, 60)
	if err != nil {
		t.Fatalf("prune spool failed: %v", err)
	}

	if report.TotalRemaining != 1 {
		t.Fatalf("expected 1 remaining file, got %d", report.TotalRemaining)
	}
}

func TestRetentionQuotaEnforcement(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tmctl-retention-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create 5 dummy files
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		rec := NewEventRecord("test", "target", 1, StatusCollected, StrengthDirect, ContainerStateValue{State: "test"}, now)
		_, err := WriteRecord(tempDir, rec, 16384)
		if err != nil {
			t.Fatalf("failed to write record: %v", err)
		}
		time.Sleep(2 * time.Millisecond) // ensure different timestamps
	}

	// Prune to max 2 files
	report, err := PruneSpool(tempDir, 24, 2, 60)
	if err != nil {
		t.Fatalf("prune failed: %v", err)
	}

	if report.QuotaPruned != 3 {
		t.Errorf("expected 3 quota pruned files, got %d", report.QuotaPruned)
	}

	files, _ := filepath.Glob(filepath.Join(tempDir, "*.json"))
	if len(files) != 2 {
		t.Errorf("expected 2 files remaining, got %d", len(files))
	}
}
