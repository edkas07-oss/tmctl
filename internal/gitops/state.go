package gitops

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadState reads the GitOpsState from state.json in the specified path.
func LoadState(statePath string) (*GitOpsState, error) {
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &GitOpsState{Status: "Never-Synced"}, nil
		}
		return nil, fmt.Errorf("failed to read state file '%s': %w", statePath, err)
	}

	var state GitOpsState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("invalid json in state file '%s': %w", statePath, err)
	}

	return &state, nil
}

// SaveState writes the GitOpsState to state.json in the specified path.
func SaveState(statePath string, state *GitOpsState) error {
	dir := filepath.Dir(statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory '%s': %w", dir, err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize state: %w", err)
	}

	return os.WriteFile(statePath, data, 0644)
}
