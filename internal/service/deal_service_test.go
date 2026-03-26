package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type memDealRepo struct {
	deals map[domain.DealID]*domain.Deal
}

func newMemDealRepo() *memDealRepo {
	return &memDealRepo{deals: make(map[domain.DealID]*domain.Deal)}
}

func (r *memDealRepo) Create(_ context.Context, d *domain.Deal) error {
	r.deals[d.ID] = d
	return nil
}

func (r *memDealRepo) CreateBatch(_ context.Context, deals []domain.Deal) error {
	for i := range deals {
		r.deals[deals[i].ID] = &deals[i]
	}
	return nil
}

func (r *memDealRepo) GetByID(_ context.Context, _ domain.TenantID, id domain.DealID) (*domain.Deal, error) {
	d, ok := r.deals[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return d, nil
}

func (r *memDealRepo) List(_ context.Context, _ domain.TenantID, _ port.DealFilter) ([]domain.Deal, int, error) {
	var all []domain.Deal
	for _, d := range r.deals {
		all = append(all, *d)
	}
	return all, len(all), nil
}

func (r *memDealRepo) Update(_ context.Context, d *domain.Deal) error {
	r.deals[d.ID] = d
	return nil
}

type memEventRepo struct{}

func (r *memEventRepo) Create(_ context.Context, _ *domain.DomainEvent) error { return nil }
func (r *memEventRepo) List(_ context.Context, _ domain.TenantID, _ port.EventFilter) ([]domain.DomainEvent, error) {
	return nil, nil
}

type seqIDGen struct{ counter int }

func (g *seqIDGen) New() string {
	g.counter++
	return fmt.Sprintf("id-%d", g.counter)
}

func TestDealService_CreateFromResearch(t *testing.T) {
	repo := newMemDealRepo()
	idGen := &seqIDGen{}
	eventSvc := service.NewEventService(&memEventRepo{}, nil, idGen)
	svc := service.NewDealService(repo, eventSvc, idGen)

	result := &domain.ResearchResult{
		CampaignID: "camp-1",
		Candidates: []domain.CandidateResult{
			{
				ASIN:            "B0TEST001",
				Title:           "Test Product",
				Brand:           "TestBrand",
				Category:        "Kitchen",
				Scores:          domain.DealScores{Demand: 9, Competition: 8, Margin: 9, Risk: 8, SourcingFeasibility: 8, Overall: 8.5},
				ReviewerVerdict: "PASS",
				IterationCount:  1,
			},
		},
	}

	deals, err := svc.CreateFromResearch(context.Background(), "tenant-1", result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deals) != 1 {
		t.Fatalf("expected 1 deal, got %d", len(deals))
	}
	if deals[0].ASIN != "B0TEST001" {
		t.Errorf("expected ASIN B0TEST001, got %s", deals[0].ASIN)
	}
	if deals[0].Status != domain.DealStatusNeedsReview {
		t.Errorf("expected status needs_review, got %s", deals[0].Status)
	}
}

func TestDealService_Approve(t *testing.T) {
	repo := newMemDealRepo()
	idGen := &seqIDGen{}
	eventSvc := service.NewEventService(&memEventRepo{}, nil, idGen)
	svc := service.NewDealService(repo, eventSvc, idGen)

	deal := &domain.Deal{
		ID:        "deal-1",
		TenantID:  "tenant-1",
		ASIN:      "B0TEST001",
		Status:    domain.DealStatusNeedsReview,
		UpdatedAt: time.Now(),
	}
	repo.Create(context.Background(), deal)

	approved, err := svc.Approve(context.Background(), "tenant-1", "deal-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if approved.Status != domain.DealStatusApproved {
		t.Errorf("expected status approved, got %s", approved.Status)
	}
}

func TestDealService_Approve_InvalidTransition(t *testing.T) {
	repo := newMemDealRepo()
	idGen := &seqIDGen{}
	eventSvc := service.NewEventService(&memEventRepo{}, nil, idGen)
	svc := service.NewDealService(repo, eventSvc, idGen)

	deal := &domain.Deal{
		ID:        "deal-1",
		TenantID:  "tenant-1",
		Status:    domain.DealStatusDiscovered,
		UpdatedAt: time.Now(),
	}
	repo.Create(context.Background(), deal)

	_, err := svc.Approve(context.Background(), "tenant-1", "deal-1", "user-1")
	if err == nil {
		t.Fatal("expected error for invalid transition")
	}
}
