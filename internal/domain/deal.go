package domain

import (
	"fmt"
	"time"
)

type DealID string

type DealStatus string

const (
	DealStatusDiscovered  DealStatus = "discovered"
	DealStatusAnalyzing   DealStatus = "analyzing"
	DealStatusNeedsReview DealStatus = "needs_review"
	DealStatusApproved    DealStatus = "approved"
	DealStatusRejected    DealStatus = "rejected"
	DealStatusSourcing    DealStatus = "sourcing"
	DealStatusProcuring   DealStatus = "procuring"
	DealStatusListing     DealStatus = "listing"
	DealStatusLive        DealStatus = "live"
	DealStatusMonitoring  DealStatus = "monitoring"
	DealStatusReorder     DealStatus = "reorder"
	DealStatusArchived    DealStatus = "archived"
)

var validDealTransitions = map[DealStatus][]DealStatus{
	DealStatusDiscovered:  {DealStatusAnalyzing},
	DealStatusAnalyzing:   {DealStatusNeedsReview, DealStatusRejected},
	DealStatusNeedsReview: {DealStatusApproved, DealStatusRejected},
	DealStatusApproved:    {DealStatusSourcing, DealStatusArchived},
	DealStatusSourcing:    {DealStatusProcuring, DealStatusArchived},
	DealStatusProcuring:   {DealStatusListing, DealStatusArchived},
	DealStatusListing:     {DealStatusLive, DealStatusArchived},
	DealStatusLive:        {DealStatusMonitoring, DealStatusArchived},
	DealStatusMonitoring:  {DealStatusReorder, DealStatusArchived},
	DealStatusReorder:     {DealStatusProcuring, DealStatusArchived},
}

type Deal struct {
	ID              DealID     `json:"id"`
	TenantID        TenantID   `json:"tenant_id"`
	CampaignID      CampaignID `json:"campaign_id"`
	ASIN            string     `json:"asin"`
	Title           string     `json:"title"`
	Brand           string     `json:"brand"`
	Category        string     `json:"category"`
	Status          DealStatus `json:"status"`
	Scores          DealScores `json:"scores"`
	Evidence        Evidence   `json:"evidence"`
	ReviewerVerdict string     `json:"reviewer_verdict"`
	IterationCount  int        `json:"iteration_count"`
	SupplierID      *string    `json:"supplier_id,omitempty"`
	ListingID       *string    `json:"listing_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type DealScores struct {
	Demand              int     `json:"demand"`
	Competition         int     `json:"competition"`
	Margin              int     `json:"margin"`
	Risk                int     `json:"risk"`
	SourcingFeasibility int     `json:"sourcing_feasibility"`
	Overall             float64 `json:"overall"`
}

type Evidence struct {
	Demand      AgentEvidence `json:"demand"`
	Competition AgentEvidence `json:"competition"`
	Margin      AgentEvidence `json:"margin"`
	Risk        AgentEvidence `json:"risk"`
	Sourcing    AgentEvidence `json:"sourcing"`
}

type AgentEvidence struct {
	Reasoning string         `json:"reasoning"`
	Data      map[string]any `json:"data"`
}

func (d *Deal) Transition(to DealStatus) error {
	for _, allowed := range validDealTransitions[d.Status] {
		if to == allowed {
			d.Status = to
			d.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("%w: cannot transition deal from %s to %s", ErrInvalidTransition, d.Status, to)
}
