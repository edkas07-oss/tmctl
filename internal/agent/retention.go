package agent

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PruneReport contains statistics of the retention pruning cycle.
type PruneReport struct {
	StaleTmpPruned  int `json:"stale_tmp_pruned"`
	StaleJsonPruned int `json:"stale_json_pruned"`
	QuotaPruned     int `json:"quota_pruned"`
	TotalRemaining  int `json:"total_remaining"`
}

// PruneSpool cleans stale temporary files, expires old json files, and enforces FIFO quota.
func PruneSpool(spoolDir string, maxAgeHours, maxFiles, staleTmpMinutes int) (*PruneReport, error) {
	report := &PruneReport{}

	if _, err := os.Stat(spoolDir); os.IsNotExist(err) {
		return report, nil
	}

	now := time.Now()

	// 1. Clean stale .tmp files older than staleTmpMinutes
	staleTmpCutoff := now.Add(-time.Duration(staleTmpMinutes) * time.Minute)
	tmpEntries, err := filepath.Glob(filepath.Join(spoolDir, "*.tmp"))
	if err == nil {
		for _, f := range tmpEntries {
			fi, err := os.Stat(f)
			if err == nil && fi.ModTime().Before(staleTmpCutoff) {
				if os.Remove(f) == nil {
					report.StaleTmpPruned++
				}
			}
		}
	}

	// 2. Clean stale .json files older than maxAgeHours
	staleJsonCutoff := now.Add(-time.Duration(maxAgeHours) * time.Hour)
	jsonEntries, err := filepath.Glob(filepath.Join(spoolDir, "*.json"))
	if err == nil {
		for _, f := range jsonEntries {
			fi, err := os.Stat(f)
			if err == nil && fi.ModTime().Before(staleJsonCutoff) {
				if os.Remove(f) == nil {
					report.StaleJsonPruned++
				}
			}
		}
	}

	// 3. Enforce FIFO quota if remaining .json files exceed maxFiles
	activeEntries, err := filepath.Glob(filepath.Join(spoolDir, "*.json"))
	if err != nil {
		return report, err
	}

	if maxFiles > 0 && len(activeEntries) > maxFiles {
		type fileInfo struct {
			path    string
			modTime time.Time
		}
		var files []fileInfo
		for _, p := range activeEntries {
			// Skip schema files or subdirectories
			if strings.HasSuffix(p, ".schema.json") {
				continue
			}
			fi, err := os.Stat(p)
			if err == nil && !fi.IsDir() {
				files = append(files, fileInfo{path: p, modTime: fi.ModTime()})
			}
		}

		// Sort by ModTime ascending (oldest first)
		sort.Slice(files, func(i, j int) bool {
			return files[i].modTime.Before(files[j].modTime)
		})

		excess := len(files) - maxFiles
		for i := 0; i < excess; i++ {
			if os.Remove(files[i].path) == nil {
				report.QuotaPruned++
			}
		}
	}

	// Count active remaining .json files
	finalEntries, err := filepath.Glob(filepath.Join(spoolDir, "*.json"))
	if err == nil {
		report.TotalRemaining = len(finalEntries)
	}

	return report, nil
}
