package ops

import "time"

type ActionKind string

const (
	ActionRun   ActionKind = "run"
	ActionWrite ActionKind = "write"
)

type ActionStatus string

const (
	StatusPending  ActionStatus = "pending"
	StatusApproved ActionStatus = "approved"
	StatusDenied   ActionStatus = "denied"
	StatusFailed   ActionStatus = "failed"
)

type PolicyDecision struct {
	Allowed  bool
	Decision string
	Reason   string
}

type Action struct {
	ID             string       `json:"id"`
	RequestedBy    string       `json:"requested_by"`
	Kind           ActionKind   `json:"kind"`
	Path           string       `json:"path,omitempty"`
	Command        string       `json:"command,omitempty"`
	Content        string       `json:"content,omitempty"`
	Status         ActionStatus `json:"status"`
	PolicyDecision string       `json:"policy_decision"`
	PolicyReason   string       `json:"policy_reason"`
	Override       bool         `json:"override,omitempty"`
	OverrideReason string       `json:"override_reason,omitempty"`
	Result         string       `json:"result,omitempty"`
	Error          string       `json:"error,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}
