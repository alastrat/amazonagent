package inngest

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type DurableRuntime struct {
	eventKey string
	dev      bool
}

func NewDurableRuntime(eventKey string, dev bool) *DurableRuntime {
	return &DurableRuntime{eventKey: eventKey, dev: dev}
}

func (r *DurableRuntime) TriggerCampaignProcessing(ctx context.Context, campaignID domain.CampaignID, tenantID domain.TenantID) error {
	slog.Info("triggering campaign processing workflow",
		"campaign_id", campaignID,
		"tenant_id", tenantID,
	)

	if r.dev {
		slog.Warn("Inngest dev mode — workflow trigger is a no-op")
		return nil
	}

	return fmt.Errorf("Inngest campaign processing not yet implemented")
}

func (r *DurableRuntime) TriggerDiscoveryRun(ctx context.Context, tenantID domain.TenantID) error {
	slog.Info("triggering discovery run", "tenant_id", tenantID)

	if r.dev {
		slog.Warn("Inngest dev mode — discovery trigger is a no-op")
		return nil
	}

	return fmt.Errorf("Inngest discovery run not yet implemented")
}
