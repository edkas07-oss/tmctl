package diagnostic

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/eddywiyatno/tmctl/internal/agent"
)

func TestExecuteTriageWithOOMRecord(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "diagnostic-triage-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	now := time.Now().UTC()
	oomRec := agent.NewEventRecord(
		"runtime_oom",
		"test/tomcat-01",
		1,
		agent.StatusCollected,
		agent.StrengthDirect,
		agent.RuntimeOOMValue{
			OOMKilled: true,
			ExitCode:  137,
		},
		now,
	)

	_, err = agent.WriteRecord(tempDir, oomRec, 16384)
	if err != nil {
		t.Fatalf("failed to write record: %v", err)
	}

	res, err := ExecuteTriage(context.Background(), nil, "tomcat-test", tempDir, "")
	if err != nil {
		t.Fatalf("triage execution failed: %v", err)
	}

	if res.Severity != "CRITICAL" {
		t.Errorf("expected CRITICAL severity, got %s", res.Severity)
	}
	if !res.OOMKilled {
		t.Errorf("expected OOMKilled true, got false")
	}
	if res.ExitCode != 137 {
		t.Errorf("expected exit code 137, got %d", res.ExitCode)
	}
}
