package rules

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/eddywiyatno/tmctl/internal/config"
	"github.com/eddywiyatno/tmctl/pkg/termutil"
)

// ExportOptions holds parameters for exporting rules.
type ExportOptions struct {
	Category       string
	SpecificBranch string
	ListCategories bool
	OutputFile     string
	TokenOverride  string
	URLOverride    string
}

// ExportRules retrieves active rules from Diagnostic Service and prints or saves them.
func ExportRules(ctx context.Context, cfg *config.StackConfig, opts ExportOptions) error {
	targetURL := cfg.DiagnosticURL
	if opts.URLOverride != "" {
		targetURL = opts.URLOverride
	}
	authToken := cfg.BearerToken
	if opts.TokenOverride != "" {
		authToken = opts.TokenOverride
	}

	endpoint := strings.TrimRight(targetURL, "/") + "/api/v1/rules"
	if opts.SpecificBranch != "" {
		endpoint = fmt.Sprintf("%s/%s", endpoint, url.PathEscape(opts.SpecificBranch))
	} else if opts.Category != "" {
		endpoint = fmt.Sprintf("%s?category=%s", endpoint, url.QueryEscape(opts.Category))
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+authToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to contact Diagnostic Service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("export rules failed (%d): %s", resp.StatusCode, string(body))
	}

	if opts.ListCategories {
		return displayCategories(body)
	}

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err != nil {
		pretty.Write(body)
	}

	if opts.OutputFile != "" {
		if err := os.WriteFile(opts.OutputFile, pretty.Bytes(), 0644); err != nil {
			return fmt.Errorf("failed to write rules to %s: %w", opts.OutputFile, err)
		}
		termutil.Success("Rules exported successfully to %s", opts.OutputFile)
		return nil
	}

	fmt.Println(pretty.String())
	return nil
}

func displayCategories(rawJSON []byte) error {
	var data struct {
		Rules []struct {
			Branch   string `json:"branch"`
			Category string `json:"category"`
			RuleName string `json:"ruleName"`
		} `json:"rules"`
	}

	if err := json.Unmarshal(rawJSON, &data); err != nil {
		return fmt.Errorf("failed to parse rules response for categories: %w", err)
	}

	catMap := make(map[string][]string)
	for _, r := range data.Rules {
		cat := r.Category
		if cat == "" {
			cat = "general"
		}
		catMap[cat] = append(catMap[cat], r.Branch)
	}

	termutil.Info("=== Active Rulepack Categories in Diagnostic Engine ===")
	var cats []string
	for c := range catMap {
		cats = append(cats, c)
	}
	sort.Strings(cats)

	for _, c := range cats {
		branches := strings.Join(catMap[c], ", ")
		fmt.Printf("• %-24s : %d rules (%s)\n", c, len(catMap[c]), branches)
	}
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("Total Categories : %d\n", len(cats))
	fmt.Printf("Total Rules      : %d\n", len(data.Rules))
	return nil
}
