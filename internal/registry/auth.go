package registry

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/eddywiyatno/tomcat-monitoring/tmctl/pkg/termutil"
)

// AuthConfig holds basic registry authentication format.
type AuthConfig struct {
	Auths map[string]struct {
		Auth string `json:"auth"`
	} `json:"auths"`
}

// Login performs registry login by updating or creating an isolated authfile.
func Login(registryHost, username, password, authFile string) error {
	if registryHost == "" {
		return fmt.Errorf("registry host cannot be empty")
	}
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	if authFile == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("could not determine home dir: %w", err)
		}
		authFile = filepath.Join(homeDir, ".docker", "config.json")
	}

	if err := os.MkdirAll(filepath.Dir(authFile), 0700); err != nil {
		return fmt.Errorf("failed to create directory for authfile %s: %w", authFile, err)
	}

	cfg := AuthConfig{
		Auths: make(map[string]struct {
			Auth string `json:"auth"`
		}),
	}

	if data, err := os.ReadFile(authFile); err == nil {
		_ = json.Unmarshal(data, &cfg)
		if cfg.Auths == nil {
			cfg.Auths = make(map[string]struct {
				Auth string `json:"auth"`
			})
		}
	}

	token := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
	cfg.Auths[registryHost] = struct {
		Auth string `json:"auth"`
	}{
		Auth: token,
	}

	outBytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode auth configuration: %w", err)
	}

	if err := os.WriteFile(authFile, outBytes, 0600); err != nil {
		return fmt.Errorf("failed to write authfile %s: %w", authFile, err)
	}

	termutil.Success("Successfully logged in to registry %s (Authfile: %s)", registryHost, authFile)
	return nil
}

// Logout removes registry credentials from the authfile.
func Logout(registryHost, authFile string) error {
	if authFile == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("could not determine home dir: %w", err)
		}
		authFile = filepath.Join(homeDir, ".docker", "config.json")
	}

	if _, err := os.Stat(authFile); os.IsNotExist(err) {
		termutil.Info("Authfile %s does not exist. Already logged out.", authFile)
		return nil
	}

	data, err := os.ReadFile(authFile)
	if err != nil {
		return fmt.Errorf("failed to read authfile: %w", err)
	}

	var cfg AuthConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse authfile: %w", err)
	}

	if registryHost != "" && cfg.Auths != nil {
		delete(cfg.Auths, registryHost)
	} else {
		cfg.Auths = make(map[string]struct {
			Auth string `json:"auth"`
		})
	}

	outBytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if strings.HasPrefix(authFile, "/tmp/") || strings.HasPrefix(authFile, os.TempDir()) {
		_ = os.Remove(authFile)
	} else {
		_ = os.WriteFile(authFile, outBytes, 0600)
	}

	termutil.Success("Successfully logged out from registry %s", registryHost)
	return nil
}
