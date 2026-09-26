package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// EnsureSpoolDir creates the spool directory with 0700 permissions if not present.
func EnsureSpoolDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create spool directory %s: %w", dir, err)
	}
	_ = os.Chmod(dir, 0700)
	return nil
}

// WriteRecord writes an EventRecord atomically (.tmp -> .json) with 0600 permissions.
func WriteRecord(spoolDir string, record *EventRecord, maxBytes int64) (string, error) {
	if err := EnsureSpoolDir(spoolDir); err != nil {
		return "", err
	}

	if err := ValidateRecord(record); err != nil {
		return "", fmt.Errorf("invalid event record: %w", err)
	}

	data, err := record.MarshalIndent()
	if err != nil {
		return "", fmt.Errorf("failed to marshal event record: %w", err)
	}

	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return "", fmt.Errorf("record payload exceeds max limit of %d bytes (actual: %d bytes)", maxBytes, len(data))
	}

	timestamp := time.Now().UnixNano()
	tmpFile := filepath.Join(spoolDir, fmt.Sprintf("%d_%s.tmp", timestamp, record.Type))
	finalFile := filepath.Join(spoolDir, fmt.Sprintf("%d_%s.json", timestamp, record.Type))

	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write temporary spool file %s: %w", tmpFile, err)
	}
	_ = os.Chmod(tmpFile, 0600)

	// Verify written file size
	fi, err := os.Stat(tmpFile)
	if err != nil {
		_ = os.Remove(tmpFile)
		return "", fmt.Errorf("failed to stat temporary spool file %s: %w", tmpFile, err)
	}
	if maxBytes > 0 && fi.Size() > maxBytes {
		_ = os.Remove(tmpFile)
		return "", fmt.Errorf("written record exceeds max limit of %d bytes (actual: %d bytes)", maxBytes, fi.Size())
	}

	// Atomic rename
	if err := os.Rename(tmpFile, finalFile); err != nil {
		_ = os.Remove(tmpFile)
		return "", fmt.Errorf("failed to atomically rename %s to %s: %w", tmpFile, finalFile, err)
	}

	_ = os.Chmod(finalFile, 0600)
	return finalFile, nil
}
