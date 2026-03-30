package firecrawl

import (
	"context"
	"testing"
)

func TestClient_Scrape_MockFallback(t *testing.T) {
	client := NewClient("")
	if client.IsConfigured() {
		t.Error("expected unconfigured client")
	}

	page, err := client.Scrape(context.Background(), "https://example.com/wholesale")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.URL != "https://example.com/wholesale" {
		t.Errorf("expected URL to match, got %s", page.URL)
	}
	if page.Content == "" {
		t.Error("expected non-empty content")
	}
}

func TestClient_ScrapeMultiple_MockFallback(t *testing.T) {
	client := NewClient("")
	pages, err := client.ScrapeMultiple(context.Background(), []string{"https://a.com", "https://b.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) != 2 {
		t.Errorf("expected 2 pages, got %d", len(pages))
	}
}
