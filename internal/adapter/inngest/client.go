package inngest

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/inngest/inngestgo"
	"github.com/inngest/inngestgo/step"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

// CampaignRequestedEvent is the event payload for triggering campaign processing.
type CampaignRequestedEvent struct {
	CampaignID string `json:"campaign_id"`
	TenantID   string `json:"tenant_id"`
}

// DurableRuntime implements port.DurableRuntime using Inngest.
type DurableRuntime struct {
	client inngestgo.Client
}

// NewDurableRuntime creates the Inngest client and registers workflow functions.
func NewDurableRuntime(pipelineSvc *service.PipelineService) (*DurableRuntime, error) {
	client, err := inngestgo.NewClient(inngestgo.ClientOpts{
		AppID: "fba-orchestrator",
	})
	if err != nil {
		return nil, fmt.Errorf("create inngest client: %w", err)
	}

	// Register the campaign processing function
	// LLM pipeline calls can take 5-10 minutes for a full campaign
	retries := 1
	inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{
			ID:      "process-campaign",
			Name:    "Process Campaign Research Pipeline",
			Retries: &retries,
		},
		inngestgo.EventTrigger("campaign/requested", nil),
		func(ctx context.Context, input inngestgo.Input[CampaignRequestedEvent]) (any, error) {
			data := input.Event.Data

			slog.Info("inngest: starting campaign processing",
				"campaign_id", data.CampaignID,
				"tenant_id", data.TenantID,
			)

			// Give the pipeline enough time — LLM calls are slow
			// Each agent call can take 10-60s, with 6 stages × N candidates
			pipelineCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			defer cancel()

			// Run the pipeline as a durable step
			_, err := step.Run(pipelineCtx, "run-research-pipeline", func(ctx context.Context) (string, error) {
				err := pipelineSvc.RunCampaign(
					ctx,
					domain.CampaignID(data.CampaignID),
					domain.TenantID(data.TenantID),
				)
				if err != nil {
					return "", err
				}
				return "completed", nil
			})

			if err != nil {
				slog.Error("inngest: campaign processing failed",
					"campaign_id", data.CampaignID,
					"error", err,
				)
				return nil, err
			}

			slog.Info("inngest: campaign processing completed",
				"campaign_id", data.CampaignID,
			)
			return map[string]string{"status": "completed"}, nil
		},
	)

	return &DurableRuntime{client: client}, nil
}

// TriggerCampaignProcessing sends an event to Inngest to start the campaign workflow.
func (r *DurableRuntime) TriggerCampaignProcessing(ctx context.Context, campaignID domain.CampaignID, tenantID domain.TenantID) error {
	_, err := r.client.Send(ctx, inngestgo.Event{
		Name: "campaign/requested",
		Data: map[string]any{
			"campaign_id": string(campaignID),
			"tenant_id":   string(tenantID),
		},
	})
	if err != nil {
		return fmt.Errorf("send inngest event: %w", err)
	}

	slog.Info("inngest: campaign processing event sent",
		"campaign_id", campaignID,
		"tenant_id", tenantID,
	)
	return nil
}

// TriggerDiscoveryRun sends an event to Inngest for scheduled discovery.
func (r *DurableRuntime) TriggerDiscoveryRun(ctx context.Context, tenantID domain.TenantID) error {
	slog.Info("inngest: discovery run not yet implemented", "tenant_id", tenantID)
	return nil
}

// Handler returns the HTTP handler for Inngest to call our functions.
func (r *DurableRuntime) Handler() http.Handler {
	return r.client.Serve()
}
