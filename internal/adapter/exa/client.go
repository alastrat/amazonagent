package exa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// Client implements port.WebSearcher using Exa's neural search API.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) IsConfigured() bool {
	return c.apiKey != ""
}

type searchRequest struct {
	Query      string `json:"query"`
	NumResults int    `json:"numResults"`
	Type       string `json:"type,omitempty"`
}

type searchResponse struct {
	Results []struct {
		Title string  `json:"title"`
		URL   string  `json:"url"`
		Text  string  `json:"text"`
		Score float64 `json:"score"`
	} `json:"results"`
}

func (c *Client) Search(ctx context.Context, query string, numResults int) ([]port.WebSearchResult, error) {
	if !c.IsConfigured() {
		slog.Warn("exa: not configured, returning mock data")
		return mockSearch(query, numResults), nil
	}

	body, _ := json.Marshal(searchRequest{
		Query:      query,
		NumResults: numResults,
		Type:       "neural",
	})

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.exa.ai/search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	var results []port.WebSearchResult
	for _, r := range result.Results {
		results = append(results, port.WebSearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Text,
			Score:   r.Score,
		})
	}

	slog.Info("exa: search complete", "query", query, "results", len(results))
	return results, nil
}

func mockSearch(_ string, numResults int) []port.WebSearchResult {
	mocks := []port.WebSearchResult{
		{Title: "Top 10 Kitchen Gadgets for 2026", URL: "https://example.com/kitchen-review", Snippet: "Comprehensive review of the best kitchen gadgets.", Score: 0.95},
		{Title: "Amazon FBA Kitchen Products Trend Analysis", URL: "https://example.com/fba-trends", Snippet: "Kitchen category showing 15% YoY growth.", Score: 0.89},
		{Title: "Wholesale Kitchen Suppliers Directory", URL: "https://example.com/wholesale", Snippet: "List of authorized wholesale distributors.", Score: 0.82},
		{Title: "Reddit: Best kitchen gadgets under $30", URL: "https://reddit.com/r/BuyItForLife", Snippet: "Community discussion about durable kitchen gadgets.", Score: 0.78},
		{Title: "Kitchen Gadgets Market Size Report 2026", URL: "https://example.com/market-report", Snippet: "Global kitchen gadgets market projected to reach $25B.", Score: 0.75},
	}
	if numResults > len(mocks) {
		numResults = len(mocks)
	}
	return mocks[:numResults]
}
