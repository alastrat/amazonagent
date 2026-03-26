package domain

import "time"

type ScoringConfigID string

type ScoringConfig struct {
	ID         ScoringConfigID `json:"id"`
	TenantID   TenantID        `json:"tenant_id"`
	Version    int             `json:"version"`
	Weights    ScoringWeights  `json:"weights"`
	Thresholds Thresholds      `json:"thresholds"`
	CreatedBy  string          `json:"created_by"`
	Active     bool            `json:"active"`
	CreatedAt  time.Time       `json:"created_at"`
}

type ScoringWeights struct {
	Demand      float64 `json:"demand"`
	Competition float64 `json:"competition"`
	Margin      float64 `json:"margin"`
	Risk        float64 `json:"risk"`
	Sourcing    float64 `json:"sourcing"`
}

type Thresholds struct {
	MinOverall      int `json:"min_overall"`
	MinPerDimension int `json:"min_per_dimension"`
}

func DefaultScoringWeights() ScoringWeights {
	return ScoringWeights{
		Demand:      0.25,
		Competition: 0.20,
		Margin:      0.25,
		Risk:        0.15,
		Sourcing:    0.15,
	}
}

func DefaultThresholds() Thresholds {
	return Thresholds{
		MinOverall:      8,
		MinPerDimension: 6,
	}
}
