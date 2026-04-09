package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// AssessmentService orchestrates the seller account assessment flow.
// Runs the 300-ASIN eligibility scan, classifies archetype, builds fingerprint.
type AssessmentService struct {
	profiles     port.SellerProfileRepo
	fingerprints port.EligibilityFingerprintRepo
	spapi        port.ProductSearcher
	sharedCatalog *SharedCatalogService
	idGen        port.IDGenerator
}

func NewAssessmentService(
	profiles port.SellerProfileRepo,
	fingerprints port.EligibilityFingerprintRepo,
	spapi port.ProductSearcher,
	sharedCatalog *SharedCatalogService,
	idGen port.IDGenerator,
) *AssessmentService {
	return &AssessmentService{
		profiles:     profiles,
		fingerprints: fingerprints,
		spapi:        spapi,
		sharedCatalog: sharedCatalog,
		idGen:        idGen,
	}
}

// StartAssessment creates a seller profile and begins the eligibility scan.
func (s *AssessmentService) StartAssessment(ctx context.Context, tenantID domain.TenantID, accountAgeDays int, activeListings int, statedCapital float64) (*domain.SellerProfile, error) {
	now := time.Now()
	profile := &domain.SellerProfile{
		ID:               s.idGen.New(),
		TenantID:         tenantID,
		AccountAgeDays:   accountAgeDays,
		ActiveListings:   activeListings,
		StatedCapital:    statedCapital,
		AssessmentStatus: domain.AssessmentStatusRunning,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	// Classify archetype
	profile.Archetype = domain.ClassifyArchetype(accountAgeDays, activeListings, statedCapital)

	if err := s.profiles.Create(ctx, profile); err != nil {
		return nil, err
	}

	slog.Info("assessment: started",
		"tenant_id", tenantID,
		"archetype", profile.Archetype,
		"account_age_days", accountAgeDays,
		"active_listings", activeListings)

	return profile, nil
}

// RunEligibilityScan executes the 300-ASIN probe scan and builds the fingerprint.
// Each probe calls SP-API CheckListingEligibility and records the result in both
// the tenant eligibility table and the shared catalog.
func (s *AssessmentService) RunEligibilityScan(ctx context.Context, tenantID domain.TenantID, probes []domain.AssessmentProbe) (*domain.EligibilityFingerprint, error) {
	if s.spapi == nil {
		return nil, nil
	}

	fingerprintID := s.idGen.New()
	var brandResults []domain.BrandProbeResult
	categoryMap := make(map[string]*domain.CategoryEligibility)

	slog.Info("assessment: scanning eligibility", "tenant_id", tenantID, "probes", len(probes))

	// Batch check eligibility — 1 ASIN at a time (SP-API restriction endpoint)
	for i, probe := range probes {
		if i > 0 && i%50 == 0 {
			slog.Info("assessment: progress", "checked", i, "total", len(probes))
		}

		restrictions, err := s.spapi.CheckListingEligibility(ctx, []string{probe.ASIN}, "US")
		if err != nil {
			slog.Warn("assessment: eligibility check failed", "asin", probe.ASIN, "error", err)
			continue
		}

		eligible := true
		reason := ""
		if len(restrictions) > 0 && !restrictions[0].Allowed {
			eligible = false
			reason = restrictions[0].Reason
		}

		// Record probe result
		result := domain.BrandProbeResult{
			ASIN:     probe.ASIN,
			Brand:    probe.Brand,
			Category: probe.Category,
			Tier:     probe.Tier,
			Eligible: eligible,
			Reason:   reason,
		}
		brandResults = append(brandResults, result)

		// Update category stats
		cat, exists := categoryMap[probe.Category]
		if !exists {
			cat = &domain.CategoryEligibility{Category: probe.Category}
			categoryMap[probe.Category] = cat
		}
		cat.ProbeCount++
		if eligible {
			cat.OpenCount++
		} else {
			cat.GatedCount++
		}

		// Also record in shared catalog's tenant eligibility
		if s.sharedCatalog != nil {
			te := &domain.TenantEligibility{
				TenantID:  tenantID,
				ASIN:      probe.ASIN,
				Eligible:  eligible,
				Reason:    reason,
				CheckedAt: time.Now(),
			}
			s.sharedCatalog.eligibility.Set(ctx, te)
		}
	}

	// Calculate category open rates
	var categories []domain.CategoryEligibility
	for _, cat := range categoryMap {
		if cat.ProbeCount > 0 {
			cat.OpenRate = float64(cat.OpenCount) / float64(cat.ProbeCount) * 100
		}
		categories = append(categories, *cat)
	}

	// Build fingerprint
	totalEligible := 0
	totalRestricted := 0
	for _, r := range brandResults {
		if r.Eligible {
			totalEligible++
		} else {
			totalRestricted++
		}
	}

	overallOpenRate := float64(0)
	if len(brandResults) > 0 {
		overallOpenRate = float64(totalEligible) / float64(len(brandResults)) * 100
	}

	// Confidence based on probe coverage (300 probes = 1.0, less = proportionally lower)
	confidence := float64(len(brandResults)) / 300.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	fp := &domain.EligibilityFingerprint{
		ID:              fingerprintID,
		TenantID:        tenantID,
		Categories:      categories,
		BrandResults:    brandResults,
		TotalProbes:     len(brandResults),
		TotalEligible:   totalEligible,
		TotalRestricted: totalRestricted,
		OverallOpenRate: overallOpenRate,
		Confidence:      confidence,
		AssessedAt:      time.Now(),
	}

	// Persist
	if err := s.fingerprints.Create(ctx, fp); err != nil {
		return nil, err
	}
	if err := s.fingerprints.SaveProbeResults(ctx, fingerprintID, tenantID, brandResults); err != nil {
		slog.Warn("assessment: failed to save probe results", "error", err)
	}
	if err := s.fingerprints.SaveCategoryEligibilities(ctx, fingerprintID, tenantID, categories); err != nil {
		slog.Warn("assessment: failed to save category eligibilities", "error", err)
	}

	slog.Info("assessment: scan complete",
		"tenant_id", tenantID,
		"total_probes", len(brandResults),
		"eligible", totalEligible,
		"restricted", totalRestricted,
		"open_rate", overallOpenRate,
		"confidence", confidence)

	return fp, nil
}

// CompleteAssessment marks the assessment as complete and updates the profile.
func (s *AssessmentService) CompleteAssessment(ctx context.Context, tenantID domain.TenantID) error {
	profile, err := s.profiles.Get(ctx, tenantID)
	if err != nil {
		return err
	}
	now := time.Now()
	profile.AssessmentStatus = domain.AssessmentStatusCompleted
	profile.AssessedAt = &now
	profile.UpdatedAt = now
	return s.profiles.Update(ctx, profile)
}

// FailAssessment marks the assessment as failed.
func (s *AssessmentService) FailAssessment(ctx context.Context, tenantID domain.TenantID) error {
	profile, err := s.profiles.Get(ctx, tenantID)
	if err != nil {
		return err
	}
	profile.AssessmentStatus = domain.AssessmentStatusFailed
	profile.UpdatedAt = time.Now()
	return s.profiles.Update(ctx, profile)
}

// GetProfile returns the seller profile for a tenant.
func (s *AssessmentService) GetProfile(ctx context.Context, tenantID domain.TenantID) (*domain.SellerProfile, error) {
	return s.profiles.Get(ctx, tenantID)
}

// GetFingerprint returns the eligibility fingerprint for a tenant.
func (s *AssessmentService) GetFingerprint(ctx context.Context, tenantID domain.TenantID) (*domain.EligibilityFingerprint, error) {
	return s.fingerprints.Get(ctx, tenantID)
}
