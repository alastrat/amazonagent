package service_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

// --- Mocks ---

type memCampaignRepo struct {
	campaigns map[domain.CampaignID]*domain.Campaign
}

func newMemCampaignRepo() *memCampaignRepo {
	return &memCampaignRepo{campaigns: make(map[domain.CampaignID]*domain.Campaign)}
}

func (r *memCampaignRepo) Create(_ context.Context, c *domain.Campaign) error {
	r.campaigns[c.ID] = c
	return nil
}

func (r *memCampaignRepo) GetByID(_ context.Context, _ domain.TenantID, id domain.CampaignID) (*domain.Campaign, error) {
	c, ok := r.campaigns[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (r *memCampaignRepo) List(_ context.Context, _ domain.TenantID, _ port.CampaignFilter) ([]domain.Campaign, error) {
	var all []domain.Campaign
	for _, c := range r.campaigns {
		all = append(all, *c)
	}
	return all, nil
}

func (r *memCampaignRepo) Update(_ context.Context, c *domain.Campaign) error {
	r.campaigns[c.ID] = c
	return nil
}

type memScoringConfigRepo struct {
	active *domain.ScoringConfig
}

func (r *memScoringConfigRepo) Create(_ context.Context, sc *domain.ScoringConfig) error {
	if sc.Active {
		r.active = sc
	}
	return nil
}

func (r *memScoringConfigRepo) GetActive(_ context.Context, _ domain.TenantID) (*domain.ScoringConfig, error) {
	if r.active == nil {
		return nil, domain.ErrNotFound
	}
	return r.active, nil
}

func (r *memScoringConfigRepo) GetByID(_ context.Context, _ domain.TenantID, id domain.ScoringConfigID) (*domain.ScoringConfig, error) {
	if r.active != nil && r.active.ID == id {
		return r.active, nil
	}
	return nil, domain.ErrNotFound
}

func (r *memScoringConfigRepo) SetActive(_ context.Context, _ domain.TenantID, _ domain.ScoringConfigID) error {
	return nil
}

type mockDurableRuntime struct {
	triggered bool
}

func (m *mockDurableRuntime) TriggerCampaignProcessing(_ context.Context, _ domain.CampaignID, _ domain.TenantID) error {
	m.triggered = true
	return nil
}

func (m *mockDurableRuntime) TriggerDiscoveryRun(_ context.Context, _ domain.TenantID) error {
	return nil
}

// Reuse from deal_service_test.go pattern
type campSeqIDGen struct{ counter int }

func (g *campSeqIDGen) New() string {
	g.counter++
	return fmt.Sprintf("id-%d", g.counter)
}

type campMemEventRepo struct{}

func (r *campMemEventRepo) Create(_ context.Context, _ *domain.DomainEvent) error { return nil }
func (r *campMemEventRepo) List(_ context.Context, _ domain.TenantID, _ port.EventFilter) ([]domain.DomainEvent, error) {
	return nil, nil
}

// --- Tests ---

func TestCampaignService_Create(t *testing.T) {
	campRepo := newMemCampaignRepo()
	scoringRepo := &memScoringConfigRepo{
		active: &domain.ScoringConfig{
			ID:       "sc-1",
			TenantID: "tenant-1",
			Version:  1,
			Weights:  domain.DefaultScoringWeights(),
			Active:   true,
		},
	}
	idGen := &campSeqIDGen{}
	eventSvc := service.NewEventService(&campMemEventRepo{}, nil, idGen)
	durable := &mockDurableRuntime{}

	svc := service.NewCampaignService(campRepo, scoringRepo, eventSvc, durable, nil, idGen)

	campaign, err := svc.Create(context.Background(), service.CreateCampaignInput{
		TenantID:    "tenant-1",
		Type:        domain.CampaignTypeManual,
		TriggerType: domain.TriggerDashboard,
		Criteria: domain.Criteria{
			Keywords:    []string{"kitchen gadgets"},
			Marketplace: "US",
		},
		CreatedBy: "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if campaign.Status != domain.CampaignStatusPending {
		t.Errorf("expected pending, got %s", campaign.Status)
	}
	if campaign.TenantID != "tenant-1" {
		t.Errorf("expected tenant-1, got %s", campaign.TenantID)
	}
	if campaign.ScoringConfigID != "sc-1" {
		t.Errorf("expected scoring config sc-1, got %s", campaign.ScoringConfigID)
	}
	if campaign.Criteria.Keywords[0] != "kitchen gadgets" {
		t.Errorf("expected keyword 'kitchen gadgets', got %v", campaign.Criteria.Keywords)
	}
	// Pipeline runs async in goroutine — not directly testable here
	// The campaign should still be created successfully
}

func TestCampaignService_Create_NoScoringConfig(t *testing.T) {
	campRepo := newMemCampaignRepo()
	scoringRepo := &memScoringConfigRepo{active: nil} // no active config
	idGen := &campSeqIDGen{}
	eventSvc := service.NewEventService(&campMemEventRepo{}, nil, idGen)

	svc := service.NewCampaignService(campRepo, scoringRepo, eventSvc, nil, nil, idGen)

	_, err := svc.Create(context.Background(), service.CreateCampaignInput{
		TenantID:    "tenant-1",
		Type:        domain.CampaignTypeManual,
		TriggerType: domain.TriggerDashboard,
		Criteria:    domain.Criteria{Marketplace: "US"},
		CreatedBy:   "user-1",
	})
	if err == nil {
		t.Fatal("expected error when no scoring config exists")
	}
}

func TestCampaignService_Create_NilDurable(t *testing.T) {
	campRepo := newMemCampaignRepo()
	scoringRepo := &memScoringConfigRepo{
		active: &domain.ScoringConfig{ID: "sc-1", TenantID: "tenant-1", Active: true},
	}
	idGen := &campSeqIDGen{}
	eventSvc := service.NewEventService(&campMemEventRepo{}, nil, idGen)

	svc := service.NewCampaignService(campRepo, scoringRepo, eventSvc, nil, nil, idGen)

	campaign, err := svc.Create(context.Background(), service.CreateCampaignInput{
		TenantID:    "tenant-1",
		Type:        domain.CampaignTypeManual,
		TriggerType: domain.TriggerDashboard,
		Criteria:    domain.Criteria{Marketplace: "US"},
		CreatedBy:   "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if campaign == nil {
		t.Fatal("expected campaign to be created even with nil durable runtime")
	}
}
