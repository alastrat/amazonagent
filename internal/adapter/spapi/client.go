package spapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// Client implements port.ProductSearcher using Amazon SP-API.
type Client struct {
	clientID     string
	clientSecret string
	refreshToken string
	marketplace  string
	httpClient   *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

func NewClient(clientID, clientSecret, refreshToken, marketplace string) *Client {
	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		refreshToken: refreshToken,
		marketplace:  marketplace,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) IsConfigured() bool {
	return c.clientID != "" && c.clientSecret != "" && c.refreshToken != ""
}

func (c *Client) getAccessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {c.refreshToken},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.amazon.com/auth/o2/token",
		strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	c.accessToken = tokenResp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)
	slog.Info("sp-api: obtained access token", "expires_in", tokenResp.ExpiresIn)
	return c.accessToken, nil
}

func (c *Client) apiRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Response, error) {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	baseURL := "https://sellingpartnerapi-na.amazon.com"
	if c.marketplace == "UK" || c.marketplace == "EU" {
		baseURL = "https://sellingpartnerapi-eu.amazon.com"
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("x-amz-access-token", token)
	req.Header.Set("Content-Type", "application/json")

	return c.httpClient.Do(req)
}

func (c *Client) SearchProducts(ctx context.Context, keywords []string, marketplace string) ([]port.ProductSearchResult, error) {
	if !c.IsConfigured() {
		slog.Warn("sp-api: not configured, returning mock data")
		return mockProductSearch(keywords), nil
	}

	query := strings.Join(keywords, " ")
	endpoint := fmt.Sprintf("/catalog/2022-04-01/items?marketplaceIds=%s&keywords=%s&pageSize=20",
		marketplaceID(marketplace), url.QueryEscape(query))

	resp, err := c.apiRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("search products: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []struct {
			ASIN      string `json:"asin"`
			Summaries []struct {
				BrandName      string `json:"brandName"`
				ItemName       string `json:"itemName"`
				Classification struct {
					DisplayName string `json:"displayName"`
				} `json:"itemClassification"`
			} `json:"summaries"`
			SalesRanks []struct {
				Rank             int    `json:"rank"`
				DisplayGroupName string `json:"displayGroupName"`
			} `json:"salesRanks"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	var products []port.ProductSearchResult
	for _, item := range result.Items {
		p := port.ProductSearchResult{ASIN: item.ASIN}
		if len(item.Summaries) > 0 {
			p.Title = item.Summaries[0].ItemName
			p.Brand = item.Summaries[0].BrandName
			p.Category = item.Summaries[0].Classification.DisplayName
		}
		if len(item.SalesRanks) > 0 {
			p.BSRRank = item.SalesRanks[0].Rank
			p.BSRCategory = item.SalesRanks[0].DisplayGroupName
		}
		products = append(products, p)
	}

	slog.Info("sp-api: search complete", "keywords", keywords, "results", len(products))
	return products, nil
}

func (c *Client) GetProductDetails(ctx context.Context, asins []string, marketplace string) ([]port.ProductSearchResult, error) {
	if !c.IsConfigured() {
		return mockProductDetails(asins), nil
	}
	var products []port.ProductSearchResult
	for _, asin := range asins {
		products = append(products, port.ProductSearchResult{ASIN: asin})
	}
	return products, nil
}

func (c *Client) EstimateFees(ctx context.Context, asin string, price float64, marketplace string) (*port.ProductFeeEstimate, error) {
	if !c.IsConfigured() {
		return mockFeeEstimate(asin, price), nil
	}
	// TODO: implement real SP-API fee estimate endpoint
	return mockFeeEstimate(asin, price), nil
}

func marketplaceID(marketplace string) string {
	switch marketplace {
	case "US":
		return "ATVPDKIKX0DER"
	case "UK":
		return "A1F83G8C2ARO7P"
	case "EU", "DE":
		return "A1PA6795UKMFR9"
	default:
		return "ATVPDKIKX0DER"
	}
}

func mockProductSearch(_ []string) []port.ProductSearchResult {
	products := []port.ProductSearchResult{
		{ASIN: "B0CX23V5KK", Title: "Stainless Steel Kitchen Utensil Set 12-Piece", Brand: "HomeChef Pro", Category: "Kitchen & Dining", AmazonPrice: 29.99, BSRRank: 3421, SellerCount: 8, ReviewCount: 1247, Rating: 4.5},
		{ASIN: "B0D1FG89NM", Title: "Silicone Baking Mat Set (3 Pack)", Brand: "BakeRight", Category: "Kitchen & Dining", AmazonPrice: 14.99, BSRRank: 8932, SellerCount: 12, ReviewCount: 3891, Rating: 4.7},
		{ASIN: "B0BY7K3PQR", Title: "Bamboo Cutting Board with Juice Groove", Brand: "EcoBoard", Category: "Kitchen & Dining", AmazonPrice: 24.99, BSRRank: 5612, SellerCount: 6, ReviewCount: 892, Rating: 4.4},
		{ASIN: "B0C2JN45ST", Title: "Electric Milk Frother Handheld", Brand: "FrothMaster", Category: "Kitchen & Dining", AmazonPrice: 12.99, BSRRank: 1203, SellerCount: 15, ReviewCount: 5621, Rating: 4.6},
		{ASIN: "B0BN4R67WX", Title: "Cast Iron Skillet 10-Inch Pre-Seasoned", Brand: "IronForge", Category: "Kitchen & Dining", AmazonPrice: 34.99, BSRRank: 2890, SellerCount: 9, ReviewCount: 2341, Rating: 4.8},
		{ASIN: "B0CR9T23YZ", Title: "Adjustable Dumbbell Set 25lb", Brand: "FitCore", Category: "Sports & Outdoors", AmazonPrice: 49.99, BSRRank: 4521, SellerCount: 7, ReviewCount: 1823, Rating: 4.5},
		{ASIN: "B0DQ6N45EF", Title: "Wireless Earbuds Noise Cancelling", Brand: "SoundPeak", Category: "Electronics", AmazonPrice: 39.99, BSRRank: 1567, SellerCount: 18, ReviewCount: 8932, Rating: 4.4},
		{ASIN: "B0CT3P78GH", Title: "Portable Phone Charger 20000mAh", Brand: "JuiceBox", Category: "Electronics", AmazonPrice: 27.99, BSRRank: 3456, SellerCount: 14, ReviewCount: 6721, Rating: 4.5},
	}
	n := 5 + rand.Intn(4)
	if n > len(products) {
		n = len(products)
	}
	perm := rand.Perm(len(products))
	var result []port.ProductSearchResult
	for i := 0; i < n; i++ {
		result = append(result, products[perm[i]])
	}
	return result
}

func mockProductDetails(asins []string) []port.ProductSearchResult {
	all := mockProductSearch(nil)
	var result []port.ProductSearchResult
	for _, asin := range asins {
		for _, p := range all {
			if p.ASIN == asin {
				result = append(result, p)
				break
			}
		}
	}
	return result
}

func mockFeeEstimate(asin string, price float64) *port.ProductFeeEstimate {
	referral := price * 0.15
	fba := 3.22 + rand.Float64()*2.0
	return &port.ProductFeeEstimate{
		ASIN:        asin,
		ReferralFee: referral,
		FBAFee:      fba,
		TotalFees:   referral + fba,
	}
}
