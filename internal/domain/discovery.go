package domain

import "time"

type DiscoveryConfigID string

type DiscoveryConfig struct {
	ID               DiscoveryConfigID `json:"id"`
	TenantID         TenantID          `json:"tenant_id"`
	Categories       []string          `json:"categories"`
	BaselineCriteria Criteria          `json:"baseline_criteria"`
	ScoringConfigID  ScoringConfigID   `json:"scoring_config_id"`
	Cadence          string            `json:"cadence"`
	Enabled          bool              `json:"enabled"`
	LastRunAt        *time.Time        `json:"last_run_at,omitempty"`
	NextRunAt        *time.Time        `json:"next_run_at,omitempty"`
}
