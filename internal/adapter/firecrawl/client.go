package firecrawl

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

// Client implements port.WebScraper using Firecrawl's API.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) IsConfigured() bool {
	return c.apiKey != ""
}

type scrapeRequest struct {
	URL     string   `json:"url"`
	Formats []string `json:"formats"`
}

type scrapeResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Markdown string            `json:"markdown"`
		Metadata map[string]string `json:"metadata"`
	} `json:"data"`
}

func (c *Client) Scrape(ctx context.Context, targetURL string) (*port.ScrapedPage, error) {
	if !c.IsConfigured() {
		slog.Warn("firecrawl: not configured, returning mock data")
		return mockScrape(targetURL), nil
	}

	body, _ := json.Marshal(scrapeRequest{URL: targetURL, Formats: []string{"markdown"}})

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.firecrawl.dev/v1/scrape", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scrape request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("scrape failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result scrapeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("scrape unsuccessful for %s", targetURL)
	}

	title := result.Data.Metadata["title"]
	if title == "" {
		title = targetURL
	}

	return &port.ScrapedPage{URL: targetURL, Title: title, Content: result.Data.Markdown}, nil
}

func (c *Client) ScrapeMultiple(ctx context.Context, urls []string) ([]port.ScrapedPage, error) {
	var pages []port.ScrapedPage
	for _, u := range urls {
		page, err := c.Scrape(ctx, u)
		if err != nil {
			slog.Warn("firecrawl: scrape failed", "url", u, "error", err)
			continue
		}
		pages = append(pages, *page)
	}
	return pages, nil
}

func mockScrape(targetURL string) *port.ScrapedPage {
	return &port.ScrapedPage{
		URL:     targetURL,
		Title:   "Scraped: " + targetURL,
		Content: fmt.Sprintf("# Mock Scraped Content\n\nContent for %s.\n\n## Wholesale Information\n- Company: Example Distribution LLC\n- Contact: sales@example.com\n- MOQ: 100 units\n- Lead time: 7-14 days", targetURL),
	}
}
