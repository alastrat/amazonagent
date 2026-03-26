package domain

import (
	"testing"
	"time"
)

func TestCampaign_Transition_PendingToRunning(t *testing.T) {
	c := &Campaign{Status: CampaignStatusPending}
	if err := c.Transition(CampaignStatusRunning); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c.Status != CampaignStatusRunning {
		t.Errorf("expected running, got %s", c.Status)
	}
	if c.CompletedAt != nil {
		t.Error("expected CompletedAt to be nil for running status")
	}
}

func TestCampaign_Transition_PendingToFailed(t *testing.T) {
	c := &Campaign{Status: CampaignStatusPending}
	if err := c.Transition(CampaignStatusFailed); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c.Status != CampaignStatusFailed {
		t.Errorf("expected failed, got %s", c.Status)
	}
	if c.CompletedAt == nil {
		t.Error("expected CompletedAt to be set for failed status")
	}
}

func TestCampaign_Transition_RunningToCompleted(t *testing.T) {
	c := &Campaign{Status: CampaignStatusRunning}
	if err := c.Transition(CampaignStatusCompleted); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c.Status != CampaignStatusCompleted {
		t.Errorf("expected completed, got %s", c.Status)
	}
	if c.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
	if time.Since(*c.CompletedAt) > time.Second {
		t.Error("CompletedAt should be recent")
	}
}

func TestCampaign_Transition_RunningToFailed(t *testing.T) {
	c := &Campaign{Status: CampaignStatusRunning}
	if err := c.Transition(CampaignStatusFailed); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c.Status != CampaignStatusFailed {
		t.Errorf("expected failed, got %s", c.Status)
	}
	if c.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestCampaign_Transition_InvalidFromPending(t *testing.T) {
	c := &Campaign{Status: CampaignStatusPending}
	err := c.Transition(CampaignStatusCompleted)
	if err == nil {
		t.Fatal("expected error for pending → completed")
	}
	if c.Status != CampaignStatusPending {
		t.Error("status should not change on invalid transition")
	}
}

func TestCampaign_Transition_InvalidFromCompleted(t *testing.T) {
	c := &Campaign{Status: CampaignStatusCompleted}
	err := c.Transition(CampaignStatusRunning)
	if err == nil {
		t.Fatal("expected error for completed → running")
	}
}

func TestCampaign_Transition_InvalidFromFailed(t *testing.T) {
	c := &Campaign{Status: CampaignStatusFailed}
	err := c.Transition(CampaignStatusRunning)
	if err == nil {
		t.Fatal("expected error for failed → running")
	}
}
