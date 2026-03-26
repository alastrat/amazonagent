package domain

import (
	"testing"
	"time"
)

func TestDeal_Transition_DiscoveredToAnalyzing(t *testing.T) {
	d := &Deal{Status: DealStatusDiscovered, UpdatedAt: time.Now().Add(-time.Hour)}
	before := d.UpdatedAt
	if err := d.Transition(DealStatusAnalyzing); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if d.Status != DealStatusAnalyzing {
		t.Errorf("expected analyzing, got %s", d.Status)
	}
	if !d.UpdatedAt.After(before) {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestDeal_Transition_AnalyzingToNeedsReview(t *testing.T) {
	d := &Deal{Status: DealStatusAnalyzing}
	if err := d.Transition(DealStatusNeedsReview); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if d.Status != DealStatusNeedsReview {
		t.Errorf("expected needs_review, got %s", d.Status)
	}
}

func TestDeal_Transition_AnalyzingToRejected(t *testing.T) {
	d := &Deal{Status: DealStatusAnalyzing}
	if err := d.Transition(DealStatusRejected); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if d.Status != DealStatusRejected {
		t.Errorf("expected rejected, got %s", d.Status)
	}
}

func TestDeal_Transition_NeedsReviewToApproved(t *testing.T) {
	d := &Deal{Status: DealStatusNeedsReview}
	if err := d.Transition(DealStatusApproved); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if d.Status != DealStatusApproved {
		t.Errorf("expected approved, got %s", d.Status)
	}
}

func TestDeal_Transition_NeedsReviewToRejected(t *testing.T) {
	d := &Deal{Status: DealStatusNeedsReview}
	if err := d.Transition(DealStatusRejected); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if d.Status != DealStatusRejected {
		t.Errorf("expected rejected, got %s", d.Status)
	}
}

func TestDeal_Transition_ApprovedToSourcing(t *testing.T) {
	d := &Deal{Status: DealStatusApproved}
	if err := d.Transition(DealStatusSourcing); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if d.Status != DealStatusSourcing {
		t.Errorf("expected sourcing, got %s", d.Status)
	}
}

func TestDeal_Transition_ApprovedToArchived(t *testing.T) {
	d := &Deal{Status: DealStatusApproved}
	if err := d.Transition(DealStatusArchived); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if d.Status != DealStatusArchived {
		t.Errorf("expected archived, got %s", d.Status)
	}
}

func TestDeal_Transition_ReorderToProcuring(t *testing.T) {
	d := &Deal{Status: DealStatusReorder}
	if err := d.Transition(DealStatusProcuring); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if d.Status != DealStatusProcuring {
		t.Errorf("expected procuring, got %s", d.Status)
	}
}

func TestDeal_Transition_InvalidDiscoveredToApproved(t *testing.T) {
	d := &Deal{Status: DealStatusDiscovered}
	err := d.Transition(DealStatusApproved)
	if err == nil {
		t.Fatal("expected error for discovered → approved")
	}
	if d.Status != DealStatusDiscovered {
		t.Error("status should not change on invalid transition")
	}
}

func TestDeal_Transition_InvalidRejectedToApproved(t *testing.T) {
	d := &Deal{Status: DealStatusRejected}
	err := d.Transition(DealStatusApproved)
	if err == nil {
		t.Fatal("expected error for rejected → approved (rejected is terminal)")
	}
}

func TestDeal_Transition_InvalidNeedsReviewToLive(t *testing.T) {
	d := &Deal{Status: DealStatusNeedsReview}
	err := d.Transition(DealStatusLive)
	if err == nil {
		t.Fatal("expected error for needs_review → live (must go through intermediate states)")
	}
}

func TestDeal_Transition_FullLifecycle(t *testing.T) {
	d := &Deal{Status: DealStatusDiscovered, UpdatedAt: time.Now()}

	steps := []DealStatus{
		DealStatusAnalyzing,
		DealStatusNeedsReview,
		DealStatusApproved,
		DealStatusSourcing,
		DealStatusProcuring,
		DealStatusListing,
		DealStatusLive,
		DealStatusMonitoring,
		DealStatusArchived,
	}

	for _, next := range steps {
		if err := d.Transition(next); err != nil {
			t.Fatalf("transition to %s failed: %v", next, err)
		}
		if d.Status != next {
			t.Fatalf("expected %s, got %s", next, d.Status)
		}
	}
}
