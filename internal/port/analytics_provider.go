package port

import "context"

type AnalyticsProvider interface {
	CaptureEvent(ctx context.Context, distinctID string, eventName string, properties map[string]any) error
	IsFeatureEnabled(ctx context.Context, flagKey string, distinctID string) (bool, error)
}
