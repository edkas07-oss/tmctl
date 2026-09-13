package rules

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/eddywiyatno/tomcat-monitoring/tmctl/internal/config"
	"github.com/eddywiyatno/tomcat-monitoring/tmctl/pkg/termutil"
)

// IngestRules reads rules from a file or stdin and posts them to Diagnostic Service.
func IngestRules(ctx context.Context, cfg *config.StackConfig, filePath, tokenOverride, urlOverride string) error {
	var payload []byte
	var err error

	if filePath == "-" {
		payload, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
	} else {
		payload, err = os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", filePath, err)
		}
	}

	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return fmt.Errorf("empty payload provided")
	}

	targetURL := cfg.DiagnosticURL
	if urlOverride != "" {
		targetURL = urlOverride
	}
	authToken := cfg.BearerToken
	if tokenOverride != "" {
		authToken = tokenOverride
	}

	endpoint := strings.TrimRight(targetURL, "/") + "/api/v1/rules"

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Determine if array or single object
	if bytes.HasPrefix(trimmed, []byte("[")) {
		var ruleList []json.RawMessage
		if err := json.Unmarshal(trimmed, &ruleList); err != nil {
			return fmt.Errorf("invalid JSON array format: %w", err)
		}

		termutil.Info("Detected Batch Rulepack: %d rules to process.", len(ruleList))

		createdCount := 0
		conflictCount := 0
		failedCount := 0

		for i, r := range ruleList {
			var meta struct {
				Branch   string `json:"branch"`
				RuleName string `json:"ruleName"`
			}
			_ = json.Unmarshal(r, &meta)
			branch := meta.Branch
			if branch == "" {
				branch = fmt.Sprintf("Rule #%d", i+1)
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(r))
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+authToken)
			req.Header.Set("Content-Type", "application/json")

			resp, err := httpClient.Do(req)
			if err != nil {
				termutil.Error("Failed to send rule %s: %v", branch, err)
				failedCount++
				continue
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			switch resp.StatusCode {
			case http.StatusCreated:
				termutil.Success("Rule %s (%s) ingested successfully (201 Created).", branch, meta.RuleName)
				createdCount++
			case http.StatusConflict:
				termutil.Warn("Rule %s (%s) already exists (409 Conflict). Skipped.", branch, meta.RuleName)
				conflictCount++
			default:
				termutil.Error("Rule %s (%s) rejected (%d): %s", branch, meta.RuleName, resp.StatusCode, string(body))
				failedCount++
			}
		}

		fmt.Println()
		termutil.Info("=== Ingestion Summary ===")
		fmt.Printf("Total Processed : %d\n", len(ruleList))
		fmt.Printf("Created (New)   : %d\n", createdCount)
		fmt.Printf("Skipped (Exist) : %d\n", conflictCount)
		fmt.Printf("Failed/Rejected : %d\n", failedCount)

		if failedCount > 0 {
			return fmt.Errorf("batch ingestion completed with %d failures", failedCount)
		}
		return nil
	}

	// Single rule object
	var meta struct {
		Branch   string `json:"branch"`
		RuleName string `json:"ruleName"`
	}
	if err := json.Unmarshal(trimmed, &meta); err != nil {
		return fmt.Errorf("invalid JSON rule object: %w", err)
	}

	termutil.Info("Ingesting rule %s (%s) to %s...", meta.Branch, meta.RuleName, endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(trimmed))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+authToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connection to diagnostic service failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusCreated:
		termutil.Success("Rule %s (%s) ingested successfully (201 Created).", meta.Branch, meta.RuleName)
		var pretty bytes.Buffer
		if json.Indent(&pretty, body, "", "  ") == nil {
			fmt.Println(pretty.String())
		}
		return nil
	case http.StatusConflict:
		termutil.Warn("Rule %s (%s) already exists (409 Conflict).", meta.Branch, meta.RuleName)
		var pretty bytes.Buffer
		if json.Indent(&pretty, body, "", "  ") == nil {
			fmt.Println(pretty.String())
		}
		return nil
	default:
		return fmt.Errorf("ingestion rejected by Diagnostic Service (%d): %s", resp.StatusCode, string(body))
	}
}
