package service_test

import (
	"context"
	"testing"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

func TestScoringService_EnsureDefault_CreatesWhenNone(t *testing.T) {
	repo := &memScoringConfigRepo{active: nil}
	idGen := &campSeqIDGen{}

	svc := service.NewScoringService(repo, idGen)

	err := svc.EnsureDefault(context.Background(), "tenant-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.active == nil {
		t.Fatal("expected default config to be created")
	}
	if repo.active.Version != 1 {
		t.Errorf("expected version 1, got %d", repo.active.Version)
	}
	if repo.active.Weights.Demand != 0.25 {
		t.Errorf("expected default demand weight 0.25, got %f", repo.active.Weights.Demand)
	}
	if repo.active.CreatedBy != "system" {
		t.Errorf("expected created_by system, got %s", repo.active.CreatedBy)
	}
}

func TestScoringService_EnsureDefault_SkipsWhenExists(t *testing.T) {
	existing := &domain.ScoringConfig{
		ID:       "existing-1",
		TenantID: "tenant-1",
		Version:  5,
		Active:   true,
	}
	repo := &memScoringConfigRepo{active: existing}
	idGen := &campSeqIDGen{}

	svc := service.NewScoringService(repo, idGen)

	err := svc.EnsureDefault(context.Background(), "tenant-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not have changed the existing config
	if repo.active.ID != "existing-1" {
		t.Error("should not replace existing config")
	}
	if repo.active.Version != 5 {
		t.Error("should not change version of existing config")
	}
}

func TestScoringService_Update(t *testing.T) {
	existing := &domain.ScoringConfig{
		ID:       "sc-1",
		TenantID: "tenant-1",
		Version:  1,
		Weights:  domain.DefaultScoringWeights(),
		Active:   true,
	}
	repo := &memScoringConfigRepo{active: existing}
	idGen := &campSeqIDGen{}

	svc := service.NewScoringService(repo, idGen)

	newWeights := domain.ScoringWeights{
		Demand:      0.30,
		Competition: 0.20,
		Margin:      0.25,
		Risk:        0.15,
		Sourcing:    0.10,
	}
	newThresholds := domain.Thresholds{
		MinOverall:      7,
		MinPerDimension: 5,
	}

	updated, err := svc.Update(context.Background(), "tenant-1", newWeights, newThresholds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("expected version 2, got %d", updated.Version)
	}
	if updated.Weights.Demand != 0.30 {
		t.Errorf("expected demand 0.30, got %f", updated.Weights.Demand)
	}
	if updated.Thresholds.MinOverall != 7 {
		t.Errorf("expected min_overall 7, got %d", updated.Thresholds.MinOverall)
	}
	if updated.CreatedBy != "user" {
		t.Errorf("expected created_by user, got %s", updated.CreatedBy)
	}
}
