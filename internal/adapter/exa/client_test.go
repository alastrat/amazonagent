package exa

import (
	"context"
	"testing"
)

func TestClient_Search_MockFallback(t *testing.T) {
	client := NewClient("")
	if client.IsConfigured() {
		t.Error("expected unconfigured client")
	}

	results, err := client.Search(context.Background(), "kitchen gadgets amazon", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected mock results")
	}
	if len(results) > 5 {
		t.Errorf("expected max 5 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Title == "" || r.URL == "" {
			t.Error("result missing title or URL")
		}
	}
}
