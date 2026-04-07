package domain

import (
	"fmt"
	"time"
)

type CampaignID string

type CampaignType string

const (
	CampaignTypeDiscoveryRun CampaignType = "discovery_run"
	CampaignTypeManual       CampaignType = "manual"
	CampaignTypeExperiment   CampaignType = "experiment"
)

type CampaignStatus string

const (
	CampaignStatusPending   CampaignStatus = "pending"
	CampaignStatusRunning   CampaignStatus = "running"
	CampaignStatusCompleted CampaignStatus = "completed"
	CampaignStatusFailed    CampaignStatus = "failed"
)

type Campaign struct {
	ID              CampaignID      `json:"id"`
	TenantID        TenantID        `json:"tenant_id"`
	Type            CampaignType    `json:"type"`
	Criteria        Criteria        `json:"criteria"`
	ScoringConfigID ScoringConfigID `json:"scoring_config_id"`
	ExperimentID    *string         `json:"experiment_id,omitempty"`
	SourceFile      *string         `json:"source_file,omitempty"`
	ScanType        ScanType        `json:"scan_type"`
	Status          CampaignStatus  `json:"status"`
	CreatedBy       string          `json:"created_by"`
	TriggerType     TriggerType     `json:"trigger_type"`
	CreatedAt       time.Time       `json:"created_at"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
}

type TriggerType string

const (
	TriggerChat        TriggerType = "chat"
	TriggerDashboard   TriggerType = "dashboard"
	TriggerScheduler   TriggerType = "scheduler"
	TriggerSpreadsheet TriggerType = "spreadsheet"
)

type Criteria struct {
	Keywords          []string    `json:"keywords"`
	MinMonthlyRevenue *int        `json:"min_monthly_revenue,omitempty"`
	MinMarginPct      *float64    `json:"min_margin_pct,omitempty"`
	MaxWholesaleCost  *float64    `json:"max_wholesale_cost,omitempty"`
	MaxMOQ            *int        `json:"max_moq,omitempty"`
	PreferredBrands   []string    `json:"preferred_brands,omitempty"`
	BlockedBrands     []string    `json:"blocked_brands,omitempty"`
	Marketplace       string      `json:"marketplace"`
}

func (c *Campaign) Transition(to CampaignStatus) error {
	valid := map[CampaignStatus][]CampaignStatus{
		CampaignStatusPending: {CampaignStatusRunning, CampaignStatusFailed},
		CampaignStatusRunning: {CampaignStatusCompleted, CampaignStatusFailed},
	}
	for _, allowed := range valid[c.Status] {
		if to == allowed {
			c.Status = to
			if to == CampaignStatusCompleted || to == CampaignStatusFailed {
				now := time.Now()
				c.CompletedAt = &now
			}
			return nil
		}
	}
	return fmt.Errorf("%w: cannot transition campaign from %s to %s", ErrInvalidTransition, c.Status, to)
}
