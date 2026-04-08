package spapi

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// EndpointRates defines the initial rate limits per SP-API endpoint.
var EndpointRates = map[string]float64{
	"catalog_search":       1.5,
	"catalog_items":        1.5,
	"competitive_pricing":  7.0,
	"listing_restrictions": 3.5,
	"product_fees":         8.0,
	"browse_tree":          0.8,
}

// AdaptiveRateLimiter manages per-endpoint rate limits with automatic
// backoff on throttling and gradual recovery.
type AdaptiveRateLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*endpointLimiter
}

type endpointLimiter struct {
	limiter     *rate.Limiter
	initialRate float64
	currentRate float64
	lastThrottle time.Time
}

func NewAdaptiveRateLimiter() *AdaptiveRateLimiter {
	rl := &AdaptiveRateLimiter{
		limiters: make(map[string]*endpointLimiter),
	}
	for endpoint, r := range EndpointRates {
		rl.limiters[endpoint] = &endpointLimiter{
			limiter:     rate.NewLimiter(rate.Limit(r), int(r)+1),
			initialRate: r,
			currentRate: r,
		}
	}
	return rl
}

// Wait blocks until the rate limiter allows a request to the given endpoint.
func (rl *AdaptiveRateLimiter) Wait(ctx context.Context, endpoint string) error {
	rl.mu.RLock()
	el, ok := rl.limiters[endpoint]
	rl.mu.RUnlock()

	if !ok {
		// Unknown endpoint — create a conservative default
		rl.mu.Lock()
		el = &endpointLimiter{
			limiter:     rate.NewLimiter(rate.Limit(1.0), 2),
			initialRate: 1.0,
			currentRate: 1.0,
		}
		rl.limiters[endpoint] = el
		rl.mu.Unlock()
	}

	// Check if we should recover rate (30 seconds since last throttle)
	rl.mu.Lock()
	if !el.lastThrottle.IsZero() && time.Since(el.lastThrottle) > 30*time.Second {
		newRate := el.currentRate * 1.1
		if newRate > el.initialRate {
			newRate = el.initialRate
		}
		if newRate != el.currentRate {
			el.currentRate = newRate
			el.limiter.SetLimit(rate.Limit(newRate))
			slog.Debug("rate-limiter: recovering", "endpoint", endpoint, "rate", newRate)
		}
	}
	rl.mu.Unlock()

	return el.limiter.Wait(ctx)
}

// ReportThrottle is called when a 429 response is received.
// Halves the current rate, with a floor of 0.5 req/sec.
func (rl *AdaptiveRateLimiter) ReportThrottle(endpoint string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	el, ok := rl.limiters[endpoint]
	if !ok {
		return
	}

	newRate := el.currentRate / 2.0
	if newRate < 0.5 {
		newRate = 0.5
	}
	el.currentRate = newRate
	el.lastThrottle = time.Now()
	el.limiter.SetLimit(rate.Limit(newRate))

	slog.Warn("rate-limiter: throttled, backing off",
		"endpoint", endpoint,
		"new_rate", newRate,
		"initial_rate", el.initialRate,
	)
}
