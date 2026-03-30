package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/api"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/handler"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

// --- Test infrastructure ---

type testIDGen struct{ counter int }

func (g *testIDGen) New() string {
	g.counter++
	return fmt.Sprintf("test-id-%d", g.counter)
}

type testAuthProvider struct{}

func (p *testAuthProvider) ValidateToken(_ context.Context, _ string) (*port.AuthContext, error) {
	return &port.AuthContext{
		UserID:   "test-user",
		TenantID: "test-tenant",
		Role:     domain.RoleOwner,
	}, nil
}

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

type memDiscoveryConfigRepo struct {
	config *domain.DiscoveryConfig
}

func (r *memDiscoveryConfigRepo) Get(_ context.Context, _ domain.TenantID) (*domain.DiscoveryConfig, error) {
	if r.config == nil {
		return nil, domain.ErrNotFound
	}
	return r.config, nil
}

func (r *memDiscoveryConfigRepo) Upsert(_ context.Context, dc *domain.DiscoveryConfig) error {
	r.config = dc
	return nil
}

// setupRouter creates a fully wired test router with in-memory backends
func setupRouter(dealRepo *memDealRepo) (*httptest.Server, *memDealRepo) {
	if dealRepo == nil {
		dealRepo = newMemDealRepo()
	}
	campRepo := newMemCampaignRepo()
	eventRepo := &memEventRepo{}
	scoringRepo := &memScoringConfigRepo{
		active: &domain.ScoringConfig{
			ID: "sc-1", TenantID: "test-tenant", Version: 1,
			Weights: domain.DefaultScoringWeights(), Active: true,
		},
	}
	discoveryRepo := &memDiscoveryConfigRepo{}
	authProvider := &testAuthProvider{}
	idGen := &testIDGen{}

	eventSvc := service.NewEventService(eventRepo, nil, idGen)
	dealSvc := service.NewDealService(dealRepo, eventSvc, idGen)
	campaignSvc := service.NewCampaignService(campRepo, scoringRepo, eventSvc, nil, nil, idGen)
	scoringSvc := service.NewScoringService(scoringRepo, idGen)
	discoverySvc := service.NewDiscoveryService(discoveryRepo)

	handlers := api.Handlers{
		Health:    handler.NewHealthHandler(),
		Campaign:  handler.NewCampaignHandler(campaignSvc),
		Deal:      handler.NewDealHandler(dealSvc),
		Scoring:   handler.NewScoringHandler(scoringSvc),
		Discovery: handler.NewDiscoveryHandler(discoverySvc),
		Event:     handler.NewEventHandler(eventSvc),
		Dashboard: handler.NewDashboardHandler(campaignSvc, dealSvc),
	}

	router := api.NewRouter(handlers, authProvider, idGen)
	server := httptest.NewServer(router)
	return server, dealRepo
}

// --- Tests ---

func TestHealthEndpoint(t *testing.T) {
	srv, _ := setupRouter(nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("expected ok, got %s", body["status"])
	}
}

func TestDashboardSummary(t *testing.T) {
	srv, _ := setupRouter(nil)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/dashboard/summary", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["deals_pending_review"] != float64(0) {
		t.Errorf("expected 0 pending, got %v", body["deals_pending_review"])
	}
}

func TestDashboardSummary_NoAuth(t *testing.T) {
	srv, _ := setupRouter(nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/dashboard/summary") // no auth header
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestDeals_ListEmpty(t *testing.T) {
	srv, _ := setupRouter(nil)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/deals", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["total"] != float64(0) {
		t.Errorf("expected 0 total, got %v", body["total"])
	}
}

func TestDeals_ApproveFlow(t *testing.T) {
	dealRepo := newMemDealRepo()
	deal := &domain.Deal{
		ID:        "deal-1",
		TenantID:  "test-tenant",
		ASIN:      "B0TEST001",
		Title:     "Test Product",
		Status:    domain.DealStatusNeedsReview,
		Scores:    domain.DealScores{Overall: 8.5},
		UpdatedAt: time.Now(),
	}
	dealRepo.Create(context.Background(), deal)

	srv, _ := setupRouter(dealRepo)
	defer srv.Close()

	// Get deal
	req, _ := http.NewRequest("GET", srv.URL+"/deals/deal-1", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get deal failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 for get deal, got %d", resp.StatusCode)
	}

	// Approve deal
	req, _ = http.NewRequest("POST", srv.URL+"/deals/deal-1/approve", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("approve deal failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var approved domain.Deal
	json.NewDecoder(resp.Body).Decode(&approved)
	if approved.Status != domain.DealStatusApproved {
		t.Errorf("expected approved, got %s", approved.Status)
	}
}

func TestDeals_RejectFlow(t *testing.T) {
	dealRepo := newMemDealRepo()
	deal := &domain.Deal{
		ID:        "deal-2",
		TenantID:  "test-tenant",
		ASIN:      "B0TEST002",
		Status:    domain.DealStatusNeedsReview,
		UpdatedAt: time.Now(),
	}
	dealRepo.Create(context.Background(), deal)

	srv, _ := setupRouter(dealRepo)
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL+"/deals/deal-2/reject",
		strings.NewReader(`{"reason":"too risky"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("reject deal failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var rejected domain.Deal
	json.NewDecoder(resp.Body).Decode(&rejected)
	if rejected.Status != domain.DealStatusRejected {
		t.Errorf("expected rejected, got %s", rejected.Status)
	}
}

func TestCampaigns_CreateAndList(t *testing.T) {
	srv, _ := setupRouter(nil)
	defer srv.Close()

	// Create campaign
	body := `{"type":"manual","trigger_type":"dashboard","criteria":{"keywords":["test"],"marketplace":"US"}}`
	req, _ := http.NewRequest("POST", srv.URL+"/campaigns", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create campaign failed: %v", err)
	}

	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var created domain.Campaign
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	if created.Type != domain.CampaignTypeManual {
		t.Errorf("expected manual, got %s", created.Type)
	}
	if created.Status != domain.CampaignStatusPending {
		t.Errorf("expected pending, got %s", created.Status)
	}

	// List campaigns
	req, _ = http.NewRequest("GET", srv.URL+"/campaigns", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list campaigns failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var campaigns []domain.Campaign
	json.NewDecoder(resp.Body).Decode(&campaigns)
	if len(campaigns) != 1 {
		t.Errorf("expected 1 campaign, got %d", len(campaigns))
	}
}
