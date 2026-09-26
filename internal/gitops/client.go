package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GiteaCommitSummary represents the minimal commit structure returned by Gitea REST API.
type GiteaCommitSummary struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

// LoadConfig reads gitops-config.json from directory if it exists.
func LoadConfig(dir string) (*GitOpsConfig, error) {
	configPath := filepath.Join(dir, "gitops-config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read gitops config '%s': %w", configPath, err)
	}

	var cfg GitOpsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid json in gitops config '%s': %w", configPath, err)
	}

	return &cfg, nil
}

// SaveConfig writes the GitOpsConfig to gitops-config.json in the specified directory.
func SaveConfig(dir string, cfg *GitOpsConfig) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory '%s': %w", dir, err)
	}

	cfg.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	configPath := filepath.Join(dir, "gitops-config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize gitops config: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}

// ParseGitURL parses a Git HTTP/HTTPS repository URL into base URL, owner, and repository name.
func ParseGitURL(rawURL string) (baseURL, owner, repo string, err error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", "", "", fmt.Errorf("invalid URL: %w", err)
	}

	baseURL = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathParts) < 2 {
		return "", "", "", fmt.Errorf("invalid git repository path in URL: '%s' (expected /<owner>/<repo>)", u.Path)
	}

	owner = pathParts[0]
	repo = strings.TrimSuffix(pathParts[1], ".git")

	return baseURL, owner, repo, nil
}

// FetchCommitInfoHTTP queries Gitea REST API to get the latest commit SHA and message.
func FetchCommitInfoHTTP(repoURL, branch string) (sha string, message string, err error) {
	baseURL, owner, repo, err := ParseGitURL(repoURL)
	if err != nil {
		return "", "", err
	}

	if branch == "" {
		branch = "main"
	}

	apiURL := fmt.Sprintf("%s/api/v1/repos/%s/%s/commits?limit=1&sha=%s", baseURL, owner, repo, url.QueryEscape(branch))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create http request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to connect to Gitea API at '%s': %w", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("Gitea API returned HTTP %d for '%s': %s", resp.StatusCode, apiURL, strings.TrimSpace(string(body)))
	}

	var commits []GiteaCommitSummary
	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		return "", "", fmt.Errorf("failed to decode commit summary json: %w", err)
	}

	if len(commits) == 0 {
		return "", "", fmt.Errorf("no commits found in repository '%s/%s' on branch '%s'", owner, repo, branch)
	}

	sha = commits[0].SHA
	if len(sha) > 7 {
		sha = sha[:7]
	}
	message = strings.TrimSpace(commits[0].Commit.Message)

	return sha, message, nil
}

// FetchSpecFileHTTP downloads the raw monitoring-spec.yaml from Gitea raw endpoint.
func FetchSpecFileHTTP(repoURL, branch, specName string) ([]byte, error) {
	baseURL, owner, repo, err := ParseGitURL(repoURL)
	if err != nil {
		return nil, err
	}

	if branch == "" {
		branch = "main"
	}
	if specName == "" {
		specName = "monitoring-spec.yaml"
	}

	// First try standard raw endpoint: /<owner>/<repo>/raw/branch/<branch>/<specName>
	rawURL := fmt.Sprintf("%s/%s/%s/raw/branch/%s/%s", baseURL, owner, repo, url.PathEscape(branch), specName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch raw spec file from '%s': %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return io.ReadAll(resp.Body)
	}

	// Fallback to API raw endpoint: /api/v1/repos/<owner>/<repo>/raw/<specName>?ref=<branch>
	apiRawURL := fmt.Sprintf("%s/api/v1/repos/%s/%s/raw/%s?ref=%s", baseURL, owner, repo, specName, url.QueryEscape(branch))
	reqAPI, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiRawURL, nil)
	respAPI, errAPI := http.DefaultClient.Do(reqAPI)
	if errAPI == nil {
		defer respAPI.Body.Close()
		if respAPI.StatusCode == http.StatusOK {
			return io.ReadAll(respAPI.Body)
		}
	}

	return nil, fmt.Errorf("failed to fetch spec file '%s' from repository '%s/%s' (HTTP %d)", specName, owner, repo, resp.StatusCode)
}
