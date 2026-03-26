package port

import (
	"context"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type DurableRuntime interface {
	TriggerCampaignProcessing(ctx context.Context, campaignID domain.CampaignID, tenantID domain.TenantID) error
	TriggerDiscoveryRun(ctx context.Context, tenantID domain.TenantID) error
}
