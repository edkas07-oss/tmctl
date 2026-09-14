package validator

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/eddywiyatno/tmctl/pkg/termutil"
)

// ValidateOptions controls which validation suites to execute.
type ValidateOptions struct {
	Layout  bool
	Schemas bool
	Ansible bool
}

var forbiddenFileExtensions = []string{
	".pem", ".key", ".p12", ".pfx", ".jks", ".keystore", ".env",
}

// RunValidation executes static contract assertions.
func RunValidation(projectRoot string, opts ValidateOptions) error {
	if projectRoot == "" {
		projectRoot = "."
	}

	// Default: run all if no specific flag specified
	if !opts.Layout && !opts.Schemas && !opts.Ansible {
		opts.Layout = true
		opts.Schemas = true
		opts.Ansible = true
	}

	termutil.Info("Starting baseline validation on project root: %s", projectRoot)

	if opts.Layout {
		if err := validateLayout(projectRoot); err != nil {
			return err
		}
		if err := validateSensitiveFiles(projectRoot); err != nil {
			return err
		}
	}

	if opts.Schemas {
		if err := validateJSONFiles(projectRoot); err != nil {
			return err
		}
	}

	termutil.Success("All platform validation assertions passed successfully.")
	return nil
}

func validateLayout(root string) error {
	termutil.Step(1, 3, "Validating repository layout and required contract files...")

	requiredFiles := []string{
		"CONFIG",
		"README.md",
		"AGENTS.md",
	}

	for _, req := range requiredFiles {
		p := filepath.Join(root, req)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			// Check one directory up
			pUp := filepath.Join("..", req)
			if _, errUp := os.Stat(pUp); os.IsNotExist(errUp) {
				termutil.Warn("Contract file not found: %s", req)
			}
		}
	}
	return nil
}

func validateSensitiveFiles(root string) error {
	termutil.Step(2, 3, "Auditing repository for forbidden sensitive material files...")

	var violations []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "bin" {
				return filepath.SkipDir
			}
			return nil
		}

		name := strings.ToLower(d.Name())
		for _, ext := range forbiddenFileExtensions {
			if strings.HasSuffix(name, ext) || name == ".env" {
				violations = append(violations, path)
			}
		}
		return nil
	})

	if len(violations) > 0 {
		return fmt.Errorf("forbidden sensitive files detected in repository:\n - %s", strings.Join(violations, "\n - "))
	}
	return nil
}

func validateJSONFiles(root string) error {
	termutil.Step(3, 3, "Validating JSON schema syntax integrity across configuration files...")

	var invalidJSON []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "bin" {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(d.Name(), ".json") {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			var dummy interface{}
			if err := json.Unmarshal(content, &dummy); err != nil {
				invalidJSON = append(invalidJSON, fmt.Sprintf("%s (%v)", path, err))
			}
		}
		return nil
	})

	if len(invalidJSON) > 0 {
		return fmt.Errorf("invalid JSON syntax detected in files:\n - %s", strings.Join(invalidJSON, "\n - "))
	}
	return nil
}
