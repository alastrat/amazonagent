package posthog

import (
	"context"
	"log/slog"
)

type AnalyticsProvider struct {
	apiKey string
	host   string
	isDev  bool
}

func NewAnalyticsProvider(apiKey, host string, isDev bool) *AnalyticsProvider {
	return &AnalyticsProvider{apiKey: apiKey, host: host, isDev: isDev}
}

func (p *AnalyticsProvider) CaptureEvent(ctx context.Context, distinctID string, eventName string, properties map[string]any) error {
	if p.isDev || p.apiKey == "" {
		slog.Debug("posthog capture (no-op)", "event", eventName, "distinct_id", distinctID)
		return nil
	}

	slog.Info("posthog capture", "event", eventName, "distinct_id", distinctID)
	return nil
}

func (p *AnalyticsProvider) IsFeatureEnabled(ctx context.Context, flagKey string, distinctID string) (bool, error) {
	if p.isDev || p.apiKey == "" {
		return false, nil
	}

	return false, nil
}
