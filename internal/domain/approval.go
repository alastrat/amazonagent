package domain

import "time"

type ApprovalID string

type ApprovalDecision string

const (
	ApprovalDecisionApproved ApprovalDecision = "approved"
	ApprovalDecisionRejected ApprovalDecision = "rejected"
)

type Approval struct {
	ID          ApprovalID        `json:"id"`
	TenantID    TenantID          `json:"tenant_id"`
	EntityType  string            `json:"entity_type"`
	EntityID    string            `json:"entity_id"`
	RequestedBy string            `json:"requested_by"`
	DecidedBy   *string           `json:"decided_by,omitempty"`
	Decision    *ApprovalDecision `json:"decision,omitempty"`
	Reason      *string           `json:"reason,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	DecidedAt   *time.Time        `json:"decided_at,omitempty"`
}
