package agent

import (
	"fmt"
	"time"
)

var validStatuses = map[string]bool{
	StatusCollected:       true,
	StatusNotFound:        true,
	StatusNoData:          true,
	StatusUnavailable:     true,
	StatusTimeout:         true,
	StatusUnauthorized:    true,
	StatusNotConfigured:   true,
	StatusInvalidResponse: true,
	StatusFailed:          true,
	StatusPartial:         true,
}

var validStrengths = map[string]bool{
	StrengthDirect:        true,
	StrengthSupporting:    true,
	StrengthContextual:    true,
	StrengthDefinitive:    true,
	StrengthInconclusive:  true,
	StrengthHeuristic:     true,
	StrengthContradictory: true,
}

// ValidateRecord validates an EventRecord against event-record-v1 rules.
func ValidateRecord(r *EventRecord) error {
	if r == nil {
		return fmt.Errorf("record cannot be nil")
	}
	if r.SchemaVersion < 1 {
		return fmt.Errorf("schema_version must be >= 1, got %d", r.SchemaVersion)
	}
	if r.Type == "" {
		return fmt.Errorf("type is required and cannot be empty")
	}
	if r.TargetID == "" {
		return fmt.Errorf("target_id is required and cannot be empty")
	}
	if r.ObservedAt == "" {
		return fmt.Errorf("observed_at is required")
	}
	if _, err := time.Parse(time.RFC3339, r.ObservedAt); err != nil {
		return fmt.Errorf("observed_at must be RFC3339 date-time format: %w", err)
	}
	if !validStatuses[r.Status] {
		return fmt.Errorf("invalid status enum value: %s", r.Status)
	}
	if !validStrengths[r.Strength] {
		return fmt.Errorf("invalid strength enum value: %s", r.Strength)
	}
	if r.Value == nil {
		return fmt.Errorf("value field cannot be nil")
	}
	return nil
}
