package domain

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// StrategyGoal.DaysRemaining
// ---------------------------------------------------------------------------

func TestStrategyGoal_DaysRemaining_FutureDate(t *testing.T) {
	g := StrategyGoal{
		TimeframeEnd: time.Now().Add(10 * 24 * time.Hour), // 10 days from now
	}
	days := g.DaysRemaining()
	if days < 9 || days > 10 {
		t.Errorf("expected ~10 days remaining, got %d", days)
	}
}

func TestStrategyGoal_DaysRemaining_PastDateReturnsZero(t *testing.T) {
	g := StrategyGoal{
		TimeframeEnd: time.Now().Add(-48 * time.Hour), // 2 days ago
	}
	days := g.DaysRemaining()
	if days != 0 {
		t.Errorf("expected 0 days remaining for past date, got %d", days)
	}
}

func TestStrategyGoal_DaysRemaining_ExactlyNow(t *testing.T) {
	g := StrategyGoal{
		TimeframeEnd: time.Now(),
	}
	days := g.DaysRemaining()
	if days != 0 {
		t.Errorf("expected 0 days remaining for now, got %d", days)
	}
}

// ---------------------------------------------------------------------------
// StrategyGoal.ProgressPct
// ---------------------------------------------------------------------------

func TestStrategyGoal_ProgressPct_Partial(t *testing.T) {
	g := StrategyGoal{
		TargetAmount:    2000,
		CurrentProgress: 500,
	}
	pct := g.ProgressPct()
	if pct != 25.0 {
		t.Errorf("expected 25%%, got %.2f%%", pct)
	}
}

func TestStrategyGoal_ProgressPct_Complete(t *testing.T) {
	g := StrategyGoal{
		TargetAmount:    2000,
		CurrentProgress: 2000,
	}
	pct := g.ProgressPct()
	if pct != 100.0 {
		t.Errorf("expected 100%%, got %.2f%%", pct)
	}
}

func TestStrategyGoal_ProgressPct_Over100Capped(t *testing.T) {
	g := StrategyGoal{
		TargetAmount:    2000,
		CurrentProgress: 5000,
	}
	pct := g.ProgressPct()
	if pct != 100.0 {
		t.Errorf("expected capped at 100%%, got %.2f%%", pct)
	}
}

func TestStrategyGoal_ProgressPct_ZeroTarget(t *testing.T) {
	g := StrategyGoal{
		TargetAmount:    0,
		CurrentProgress: 500,
	}
	pct := g.ProgressPct()
	if pct != 0 {
		t.Errorf("expected 0%% for zero target, got %.2f%%", pct)
	}
}

func TestStrategyGoal_ProgressPct_NegativeTarget(t *testing.T) {
	g := StrategyGoal{
		TargetAmount:    -100,
		CurrentProgress: 50,
	}
	pct := g.ProgressPct()
	if pct != 0 {
		t.Errorf("expected 0%% for negative target, got %.2f%%", pct)
	}
}

func TestStrategyGoal_ProgressPct_ZeroProgress(t *testing.T) {
	g := StrategyGoal{
		TargetAmount:    2000,
		CurrentProgress: 0,
	}
	pct := g.ProgressPct()
	if pct != 0.0 {
		t.Errorf("expected 0%%, got %.2f%%", pct)
	}
}
