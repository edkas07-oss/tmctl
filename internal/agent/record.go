package agent

import (
	"encoding/json"
	"time"
)

// Status enum values for event-record-v1.
const (
	StatusCollected       = "collected"
	StatusNotFound        = "not_found"
	StatusNoData          = "no_data"
	StatusUnavailable     = "unavailable"
	StatusTimeout         = "timeout"
	StatusUnauthorized    = "unauthorized"
	StatusNotConfigured   = "not_configured"
	StatusInvalidResponse = "invalid_response"
	StatusFailed          = "failed"
	StatusPartial         = "partial"
)

// Strength enum values for event-record-v1.
const (
	StrengthDirect        = "direct"
	StrengthSupporting    = "supporting"
	StrengthContextual    = "contextual"
	StrengthDefinitive    = "definitive"
	StrengthInconclusive  = "inconclusive"
	StrengthHeuristic     = "heuristic"
	StrengthContradictory = "contradictory"
)

// EventRecord represents the canonical data model for event-record-v1.schema.json.
type EventRecord struct {
	SchemaVersion int         `json:"schema_version,omitempty"`
	Type          string      `json:"type"`
	TargetID      string      `json:"target_id"`
	Generation    *int        `json:"generation"`
	ObservedAt    string      `json:"observed_at"`
	Status        string      `json:"status"`
	Strength      string      `json:"strength"`
	Value         interface{} `json:"value"`
	Redacted      bool        `json:"redacted"`
}

// ContainerStateValue represents payload for container_state event type.
type ContainerStateValue struct {
	State     string `json:"state"`
	Container string `json:"container,omitempty"`
}

// RuntimeOOMValue represents payload for runtime_oom event type.
type RuntimeOOMValue struct {
	OOMKilled bool `json:"oomKilled"`
	ExitCode  int  `json:"exitCode"`
}

// CollectorStatusValue represents payload for collector_status event type.
type CollectorStatusValue struct {
	Error string `json:"error"`
}

// NewEventRecord constructs an EventRecord initialized with schema defaults.
func NewEventRecord(recordType, targetID string, generation int, status, strength string, value interface{}, observedAt time.Time) *EventRecord {
	gen := generation
	return &EventRecord{
		SchemaVersion: 1,
		Type:          recordType,
		TargetID:      targetID,
		Generation:    &gen,
		ObservedAt:    observedAt.UTC().Format(time.RFC3339),
		Status:        status,
		Strength:      strength,
		Value:         value,
		Redacted:      false,
	}
}

// MarshalIndent serializes the EventRecord to indented JSON.
func (r *EventRecord) MarshalIndent() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
