package diagnostic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/eddywiyatno/tmctl/internal/agent"
	"github.com/eddywiyatno/tmctl/internal/engine"
	"github.com/eddywiyatno/tmctl/pkg/termutil"
)

// TriageResult represents the structured outcome of diagnostic triage.
type TriageResult struct {
	Timestamp      string   `json:"timestamp"`
	Target         string   `json:"target"`
	ContainerState string   `json:"container_state"`
	ExitCode       int      `json:"exit_code"`
	OOMKilled      bool     `json:"oom_killed"`
	Diagnosis      string   `json:"diagnosis"`
	Severity       string   `json:"severity"` // CRITICAL, WARNING, INFO
	Remediation    []string `json:"remediation"`
	EvidenceFiles  []string `json:"evidence_files,omitempty"`
}

// ExecuteTriage inspects the live container state or reads recent evidence from the spool directory.
func ExecuteTriage(ctx context.Context, engineClient engine.EngineClient, targetContainer, spoolDir, jsonOut string) (*TriageResult, error) {
	termutil.Header("Executing Tomcat Diagnostic & Crash Triage")

	if targetContainer == "" {
		targetContainer = "tomcat-jmx-exporter"
	}
	if spoolDir == "" {
		spoolDir = "/opt/tm-home/spool"
	}

	termutil.Info("Target Container : %s", targetContainer)
	termutil.Info("Spool Directory  : %s", spoolDir)

	result := &TriageResult{
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Target:      targetContainer,
		Severity:    "INFO",
		Diagnosis:   "Container is running normally with no abnormal crash indicators detected.",
		Remediation: []string{"Continue routine telemetry monitoring."},
	}

	// 1. Inspect live container if engineClient available
	if engineClient != nil {
		inspect, err := engineClient.InspectContainer(ctx, targetContainer)
		if err == nil && inspect != nil {
			if inspect.State.Running {
				result.ContainerState = "running"
			} else {
				result.ContainerState = "exited"
			}
			result.ExitCode = inspect.State.ExitCode
			result.OOMKilled = inspect.State.OOMKilled
		}
	}

	// 2. Read latest spool records from spoolDir
	if _, err := os.Stat(spoolDir); err == nil {
		entries, _ := filepath.Glob(filepath.Join(spoolDir, "*.json"))
		if len(entries) > 0 {
			sort.Slice(entries, func(i, j int) bool {
				fi1, _ := os.Stat(entries[i])
				fi2, _ := os.Stat(entries[j])
				if fi1 != nil && fi2 != nil {
					return fi1.ModTime().After(fi2.ModTime())
				}
				return false
			})

			// Take latest 5 evidence files
			limit := 5
			if len(entries) < limit {
				limit = len(entries)
			}
			for i := 0; i < limit; i++ {
				result.EvidenceFiles = append(result.EvidenceFiles, filepath.Base(entries[i]))

				// Parse event record
				data, rErr := os.ReadFile(entries[i])
				if rErr == nil {
					var rec agent.EventRecord
					if json.Unmarshal(data, &rec) == nil {
						if rec.Type == "runtime_oom" {
							if valMap, ok := rec.Value.(map[string]interface{}); ok {
								if oom, ok := valMap["oomKilled"].(bool); ok && oom {
									result.OOMKilled = true
								}
								if ec, ok := valMap["exitCode"].(float64); ok {
									result.ExitCode = int(ec)
								}
							}
						}
					}
				}
			}
		}
	}

	// 3. Deterministic Heuristic Evaluation
	if result.OOMKilled || result.ExitCode == 137 {
		result.Severity = "CRITICAL"
		result.Diagnosis = "Container terminated by OS Cgroup Out-Of-Memory (OOM) Killer (Exit Code 137)."
		result.Remediation = []string{
			"Increase container memory limit (--memory or spec.stack.memoryLimit)",
			"Adjust JVM maximum heap size (-Xmx) to allow at least 25% headroom for native memory / metaspace",
			"Check Tomcat catalina.out for Java OutOfMemoryError: Java heap space",
		}
	} else if result.ExitCode == 143 {
		result.Severity = "WARNING"
		result.Diagnosis = "Container received SIGTERM (Exit Code 143) — graceful shutdown or orchestration restart."
		result.Remediation = []string{
			"Verify if deployment rollout or rolling restart was scheduled by operator or GitOps reconciler.",
		}
	} else if result.ExitCode == 134 {
		result.Severity = "CRITICAL"
		result.Diagnosis = "Container terminated with SIGABRT (Exit Code 134) — JVM fatal error or abort signal."
		result.Remediation = []string{
			"Inspect hs_err_pid.log in container logs volume for core dump and fatal native library failure.",
		}
	} else if result.ExitCode != 0 && result.ContainerState == "exited" {
		result.Severity = "CRITICAL"
		result.Diagnosis = fmt.Sprintf("Container terminated with non-zero exit code (%d).", result.ExitCode)
		result.Remediation = []string{
			"Inspect container logs: 'podman logs " + targetContainer + "' or 'docker logs " + targetContainer + "'",
			"Verify port binding availability and mounted volume file permissions.",
		}
	}

	// 4. Console Output
	termutil.Info("Container State  : %s", result.ContainerState)
	termutil.Info("Exit Code        : %d", result.ExitCode)
	termutil.Info("OOM Killed       : %t", result.OOMKilled)
	if result.Severity == "CRITICAL" {
		termutil.Error("Triage Diagnosis : [%s] %s", result.Severity, result.Diagnosis)
	} else if result.Severity == "WARNING" {
		termutil.Warn("Triage Diagnosis : [%s] %s", result.Severity, result.Diagnosis)
	} else {
		termutil.Success("Triage Diagnosis : [%s] %s", result.Severity, result.Diagnosis)
	}

	termutil.Info("Remediation Plan :")
	for _, step := range result.Remediation {
		termutil.Bullet("%s", step)
	}

	if len(result.EvidenceFiles) > 0 {
		termutil.Info("Correlated Evidence Files (%d): %s", len(result.EvidenceFiles), strings.Join(result.EvidenceFiles, ", "))
	}

	// 5. JSON Export if requested
	if jsonOut != "" {
		data, _ := json.MarshalIndent(result, "", "  ")
		if err := os.WriteFile(jsonOut, data, 0644); err != nil {
			return result, fmt.Errorf("failed to write triage json to %s: %w", jsonOut, err)
		}
		termutil.Success("Saved triage report to: %s", jsonOut)
	}

	return result, nil
}
