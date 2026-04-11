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
	sellerID     string
	httpClient   *http.Client
	rateLimiter  *AdaptiveRateLimiter

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

func NewClient(clientID, clientSecret, refreshToken, marketplace, sellerID string) *Client {
	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		refreshToken: refreshToken,
		marketplace:  marketplace,
		sellerID:     sellerID,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		rateLimiter:  NewAdaptiveRateLimiter(),
	}
}

// NewClientFromCredentials constructs a per-tenant SP-API client from explicit credentials.
// This is the same as NewClient but named to clarify it takes stored (decrypted) credentials
// rather than config/env vars.
func NewClientFromCredentials(clientID, clientSecret, refreshToken, marketplace, sellerID string) *Client {
	return NewClient(clientID, clientSecret, refreshToken, marketplace, sellerID)
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
	return c.apiRequestWithRL(ctx, method, endpoint, body, "")
}

func (c *Client) apiRequestWithRL(ctx context.Context, method, endpoint string, body io.Reader, rlEndpoint string) (*http.Response, error) {
	// Apply rate limiting if endpoint category is specified
	if rlEndpoint != "" && c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx, rlEndpoint); err != nil {
			return nil, fmt.Errorf("rate limiter: %w", err)
		}
	}

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

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Report throttling to adaptive rate limiter
	if resp.StatusCode == 429 && rlEndpoint != "" && c.rateLimiter != nil {
		c.rateLimiter.ReportThrottle(rlEndpoint)
	}

	return resp, nil
}

func (c *Client) SearchProducts(ctx context.Context, keywords []string, marketplace string) ([]port.ProductSearchResult, error) {
	if !c.IsConfigured() {
		slog.Warn("sp-api: not configured, returning mock data")
		return mockProductSearch(keywords), nil
	}

	query := strings.Join(keywords, " ")
	endpoint := fmt.Sprintf("/catalog/2022-04-01/items?marketplaceIds=%s&keywords=%s&pageSize=20&includedData=summaries,salesRanks,attributes",
		marketplaceID(marketplace), url.QueryEscape(query))

	resp, err := c.apiRequestWithRL(ctx, "GET", endpoint, nil, "catalog_search")
	if err != nil {
		return nil, fmt.Errorf("search products: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Use flexible parsing since SP-API response format varies
	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	var products []port.ProductSearchResult
	items, _ := raw["items"].([]any)
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}

		p := port.ProductSearchResult{}
		p.ASIN, _ = item["asin"].(string)
		if p.ASIN == "" {
			continue
		}

		// Parse summaries
		if summaries, ok := item["summaries"].([]any); ok && len(summaries) > 0 {
			if s, ok := summaries[0].(map[string]any); ok {
				p.Title, _ = s["itemName"].(string)
				p.Brand, _ = s["brandName"].(string)
				if p.Brand == "" {
					p.Brand, _ = s["brand"].(string)
				}
				// itemClassification can be string or object
				switch cls := s["itemClassification"].(type) {
				case string:
					p.Category = cls
				case map[string]any:
					p.Category, _ = cls["displayName"].(string)
				}
			}
		}

		// Log brand extraction for diagnostic
		if p.Brand != "" {
			slog.Debug("sp-api: brand found", "asin", p.ASIN, "brand", p.Brand)
		}

		// Parse sales ranks
		if ranks, ok := item["salesRanks"].([]any); ok {
			for _, rawRank := range ranks {
				if r, ok := rawRank.(map[string]any); ok {
					if classRanks, ok := r["classificationRanks"].([]any); ok && len(classRanks) > 0 {
						if cr, ok := classRanks[0].(map[string]any); ok {
							if rank, ok := cr["rank"].(float64); ok {
								p.BSRRank = int(rank)
							}
							p.BSRCategory, _ = cr["title"].(string)
						}
					}
					if displayRanks, ok := r["displayGroupRanks"].([]any); ok && len(displayRanks) > 0 {
						if dr, ok := displayRanks[0].(map[string]any); ok {
							if rank, ok := dr["rank"].(float64); ok && p.BSRRank == 0 {
								p.BSRRank = int(rank)
							}
							if p.BSRCategory == "" {
								p.BSRCategory, _ = dr["title"].(string)
							}
						}
					}
				}
			}
		}

		// Try to extract price from attributes
		if attrs, ok := item["attributes"].(map[string]any); ok {
			if listPrice, ok := attrs["list_price"].([]any); ok && len(listPrice) > 0 {
				if lp, ok := listPrice[0].(map[string]any); ok {
					if amount, ok := lp["value"].(float64); ok {
						p.AmazonPrice = amount
					}
				}
			}
		}

		products = append(products, p)
	}

	// Enrich with pricing data from competitive pricing API (for products still missing price)
	c.enrichPricing(ctx, products, marketplace)

	// Count brands found
	brandsFound := 0
	for _, p := range products {
		if p.Brand != "" {
			brandsFound++
		}
	}
	slog.Info("sp-api: search complete", "keywords", keywords, "results", len(products), "with_brand", brandsFound)
	return products, nil
}

// SearchByBrowseNode searches for products in a specific Amazon browse node (category).
// Returns up to 20 products per call. Use pageToken for pagination (max ~10 pages = 200 products).
// Returns: products, nextPageToken (empty if no more pages), error.
func (c *Client) SearchByBrowseNode(ctx context.Context, nodeID string, marketplace string, pageToken string) ([]port.ProductSearchResult, string, error) {
	if !c.IsConfigured() {
		slog.Warn("sp-api: not configured, returning mock browse node data")
		return mockProductSearch([]string{"browse-" + nodeID}), "", nil
	}

	endpoint := fmt.Sprintf("/catalog/2022-04-01/items?marketplaceIds=%s&classificationIds=%s&pageSize=20&includedData=summaries,salesRanks,dimensions,identifiers",
		marketplaceID(marketplace), url.QueryEscape(nodeID))
	if pageToken != "" {
		endpoint += "&pageToken=" + url.QueryEscape(pageToken)
	}

	resp, err := c.apiRequestWithRL(ctx, "GET", endpoint, nil, "catalog_search")
	if err != nil {
		return nil, "", fmt.Errorf("browse node search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("browse node search failed (status %d): %s", resp.StatusCode, string(body))
	}

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, "", fmt.Errorf("decode response: %w", err)
	}

	var products []port.ProductSearchResult
	items, _ := raw["items"].([]any)
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}

		p := port.ProductSearchResult{}
		p.ASIN, _ = item["asin"].(string)
		if p.ASIN == "" {
			continue
		}

		if summaries, ok := item["summaries"].([]any); ok && len(summaries) > 0 {
			if s, ok := summaries[0].(map[string]any); ok {
				p.Title, _ = s["itemName"].(string)
				p.Brand, _ = s["brandName"].(string)
				if p.Brand == "" {
					p.Brand, _ = s["brand"].(string)
				}
				switch cls := s["itemClassification"].(type) {
				case string:
					p.Category = cls
				case map[string]any:
					p.Category, _ = cls["displayName"].(string)
				}
			}
		}

		if ranks, ok := item["salesRanks"].([]any); ok {
			for _, rawRank := range ranks {
				if r, ok := rawRank.(map[string]any); ok {
					if classRanks, ok := r["classificationRanks"].([]any); ok && len(classRanks) > 0 {
						if cr, ok := classRanks[0].(map[string]any); ok {
							if rank, ok := cr["rank"].(float64); ok {
								p.BSRRank = int(rank)
							}
							p.BSRCategory, _ = cr["title"].(string)
						}
					}
				}
			}
		}

		if attrs, ok := item["attributes"].(map[string]any); ok {
			if listPrice, ok := attrs["list_price"].([]any); ok && len(listPrice) > 0 {
				if lp, ok := listPrice[0].(map[string]any); ok {
					if amount, ok := lp["value"].(float64); ok {
						p.AmazonPrice = amount
					}
				}
			}
		}

		products = append(products, p)
	}

	// Extract next page token
	nextPageToken := ""
	if pagination, ok := raw["pagination"].(map[string]any); ok {
		nextPageToken, _ = pagination["nextToken"].(string)
	}

	// Enrich with pricing
	c.enrichPricing(ctx, products, marketplace)

	slog.Info("sp-api: browse node search complete", "node", nodeID, "results", len(products), "has_next", nextPageToken != "")
	return products, nextPageToken, nil
}

// enrichPricing calls the SP-API competitive pricing endpoint to get real prices.
func (c *Client) enrichPricing(ctx context.Context, products []port.ProductSearchResult, marketplace string) {
	// Batch ASINs that need pricing (max 20 per request)
	var asinsNeedPricing []string
	asinIndex := make(map[string]int)
	for i, p := range products {
		if p.ASIN != "" && p.AmazonPrice <= 0 {
			asinsNeedPricing = append(asinsNeedPricing, p.ASIN)
			asinIndex[p.ASIN] = i
		}
	}

	if len(asinsNeedPricing) == 0 {
		return
	}

	// Build comma-separated ASIN list for batch request
	asinParam := strings.Join(asinsNeedPricing, ",")
	endpoint := fmt.Sprintf("/products/pricing/v0/competitivePrice?MarketplaceId=%s&Asins=%s&ItemType=Asin",
		marketplaceID(marketplace), asinParam)

	resp, err := c.apiRequestWithRL(ctx, "GET", endpoint, nil, "competitive_pricing")
	if err != nil {
		slog.Warn("sp-api: competitive pricing request failed", "error", err)
		return
	}
	defer resp.Body.Close()

	var raw map[string]any
	json.NewDecoder(resp.Body).Decode(&raw)

	payload, ok := raw["payload"].([]any)
	if !ok {
		return
	}

	for _, rawItem := range payload {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		asin, _ := item["ASIN"].(string)
		idx, exists := asinIndex[asin]
		if !exists {
			continue
		}

		prod, ok := item["Product"].(map[string]any)
		if !ok {
			continue
		}

		cp, ok := prod["CompetitivePricing"].(map[string]any)
		if !ok {
			continue
		}

		// Extract price from competitive prices
		if prices, ok := cp["CompetitivePrices"].([]any); ok && len(prices) > 0 {
			if price, ok := prices[0].(map[string]any); ok {
				if priceData, ok := price["Price"].(map[string]any); ok {
					if lp, ok := priceData["ListingPrice"].(map[string]any); ok {
						if amount, ok := lp["Amount"].(float64); ok {
							products[idx].AmazonPrice = amount
							slog.Info("sp-api: got price", "asin", asin, "price", amount)
						}
					}
				}
			}
		}

		// Extract seller count
		if listings, ok := cp["NumberOfOfferListings"].([]any); ok {
			for _, l := range listings {
				if lm, ok := l.(map[string]any); ok {
					if cond, _ := lm["condition"].(string); cond == "New" {
						if count, ok := lm["Count"].(float64); ok {
							products[idx].SellerCount = int(count)
						}
					}
				}
			}
		}
	}
}

func (c *Client) GetProductDetails(ctx context.Context, asins []string, marketplace string) ([]port.ProductSearchResult, error) {
	if !c.IsConfigured() {
		return mockProductDetails(asins), nil
	}

	// Use competitive pricing to get price + seller count per ASIN
	products := make([]port.ProductSearchResult, len(asins))
	for i, asin := range asins {
		products[i] = port.ProductSearchResult{ASIN: asin}
	}

	asinParam := strings.Join(asins, ",")
	endpoint := fmt.Sprintf("/products/pricing/v0/competitivePrice?MarketplaceId=%s&Asins=%s&ItemType=Asin",
		marketplaceID(marketplace), asinParam)

	resp, err := c.apiRequestWithRL(ctx, "GET", endpoint, nil, "competitive_pricing")
	if err != nil {
		slog.Warn("sp-api: competitive pricing failed in GetProductDetails", "error", err)
		return products, nil
	}
	defer resp.Body.Close()

	var raw map[string]any
	json.NewDecoder(resp.Body).Decode(&raw)

	asinIndex := make(map[string]int)
	for i, asin := range asins {
		asinIndex[asin] = i
	}

	if payload, ok := raw["payload"].([]any); ok {
		for _, rawItem := range payload {
			item, ok := rawItem.(map[string]any)
			if !ok {
				continue
			}
			asin, _ := item["ASIN"].(string)
			idx, exists := asinIndex[asin]
			if !exists {
				continue
			}

			prod, _ := item["Product"].(map[string]any)
			if prod == nil {
				continue
			}
			cp, _ := prod["CompetitivePricing"].(map[string]any)
			if cp == nil {
				continue
			}

			if prices, ok := cp["CompetitivePrices"].([]any); ok && len(prices) > 0 {
				if price, ok := prices[0].(map[string]any); ok {
					if pd, ok := price["Price"].(map[string]any); ok {
						if lp, ok := pd["ListingPrice"].(map[string]any); ok {
							if amount, ok := lp["Amount"].(float64); ok {
								products[idx].AmazonPrice = amount
							}
						}
					}
				}
			}

			if listings, ok := cp["NumberOfOfferListings"].([]any); ok {
				for _, l := range listings {
					if lm, ok := l.(map[string]any); ok {
						if cond, _ := lm["condition"].(string); cond == "New" {
							if count, ok := lm["Count"].(float64); ok {
								products[idx].SellerCount = int(count)
							}
						}
					}
				}
			}

			slog.Info("sp-api: product details", "asin", asin, "price", products[idx].AmazonPrice, "sellers", products[idx].SellerCount)
		}
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

func (c *Client) CheckListingEligibility(ctx context.Context, asins []string, marketplace string) ([]port.ListingRestriction, error) {
	if !c.IsConfigured() || c.sellerID == "" {
		// No seller ID — can't check, assume all allowed
		var results []port.ListingRestriction
		for _, asin := range asins {
			results = append(results, port.ListingRestriction{ASIN: asin, Allowed: true, Status: port.EligibilityEligible})
		}
		return results, nil
	}

	var results []port.ListingRestriction
	for _, asin := range asins {
		endpoint := fmt.Sprintf("/listings/2021-08-01/restrictions?asin=%s&sellerId=%s&marketplaceIds=%s&conditionType=new_new&reasonLocale=en_US",
			asin, c.sellerID, marketplaceID(marketplace))

		resp, err := c.apiRequestWithRL(ctx, "GET", endpoint, nil, "listing_restrictions")
		if err != nil {
			slog.Warn("sp-api: eligibility check failed", "asin", asin, "error", err)
			results = append(results, port.ListingRestriction{ASIN: asin, Allowed: true, Status: port.EligibilityEligible}) // fail open
			continue
		}

		var raw map[string]any
		json.NewDecoder(resp.Body).Decode(&raw)
		resp.Body.Close()

		restrictions, _ := raw["restrictions"].([]any)
		if len(restrictions) == 0 {
			results = append(results, port.ListingRestriction{ASIN: asin, Allowed: true, Status: port.EligibilityEligible})
			slog.Debug("sp-api: eligible", "asin", asin)
		} else {
			reason := "Restricted"
			reasonCode := ""
			approvalURL := ""
			status := port.EligibilityRestricted

			if r, ok := restrictions[0].(map[string]any); ok {
				if reasons, ok := r["reasons"].([]any); ok {
					for _, rr := range reasons {
						if rm, ok := rr.(map[string]any); ok {
							if msg, ok := rm["message"].(string); ok && msg != "" {
								reason = msg
							}
							if rc, ok := rm["reasonCode"].(string); ok {
								reasonCode = rc
							}
							// Check for approval links
							if links, ok := rm["links"].([]any); ok {
								for _, l := range links {
									if lm, ok := l.(map[string]any); ok {
										if res, ok := lm["resource"].(string); ok && res != "" {
											approvalURL = res
										}
									}
								}
							}
						}
					}
				}
			}

			// APPROVAL_REQUIRED with a link = ungatable (seller can apply)
			if reasonCode == "APPROVAL_REQUIRED" || approvalURL != "" {
				status = port.EligibilityUngatable
			}

			results = append(results, port.ListingRestriction{
				ASIN:        asin,
				Allowed:     false,
				Reason:      reason,
				ReasonCode:  reasonCode,
				ApprovalURL: approvalURL,
				Status:      status,
			})
			slog.Info("sp-api: not eligible", "asin", asin, "reason", reason, "reasonCode", reasonCode, "status", status, "approvalURL", approvalURL)
		}
	}

	return results, nil
}

func (c *Client) LookupByIdentifier(ctx context.Context, identifiers []string, idType string, marketplace string) ([]port.ProductSearchResult, error) {
	if !c.IsConfigured() {
		slog.Warn("sp-api: not configured, returning mock identifier lookup")
		return mockIdentifierLookup(identifiers, idType), nil
	}

	// SP-API catalog items endpoint accepts up to 20 identifiers per request
	var allProducts []port.ProductSearchResult
	for i := 0; i < len(identifiers); i += 20 {
		end := i + 20
		if end > len(identifiers) {
			end = len(identifiers)
		}
		batch := identifiers[i:end]

		idParam := strings.Join(batch, ",")
		endpoint := fmt.Sprintf("/catalog/2022-04-01/items?marketplaceIds=%s&identifiers=%s&identifiersType=%s&pageSize=20&includedData=summaries,salesRanks,attributes",
			marketplaceID(marketplace), url.QueryEscape(idParam), url.QueryEscape(idType))

		resp, err := c.apiRequestWithRL(ctx, "GET", endpoint, nil, "catalog_items")
		if err != nil {
			slog.Warn("sp-api: identifier lookup failed", "error", err)
			continue
		}

		var raw map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		items, _ := raw["items"].([]any)

		// Diagnostic: log brand availability from identifier lookup
		if len(items) > 0 {
			brandsFound := 0
			for _, ri := range items {
				if itm, ok := ri.(map[string]any); ok {
					if sums, ok := itm["summaries"].([]any); ok && len(sums) > 0 {
						if s, ok := sums[0].(map[string]any); ok {
							if b, _ := s["brandName"].(string); b != "" {
								brandsFound++
							}
						}
					}
				}
			}
			slog.Info("sp-api: identifier lookup brand check", "total", len(items), "with_brand", brandsFound)
			// Log first item's raw summaries for debugging
			if first, ok := items[0].(map[string]any); ok {
				if sums, ok := first["summaries"].([]any); ok && len(sums) > 0 {
					sumJSON, _ := json.Marshal(sums[0])
					slog.Info("sp-api: first item summaries", "raw", string(sumJSON))
				}
			}
		}

		for _, rawItem := range items {
			item, ok := rawItem.(map[string]any)
			if !ok {
				continue
			}

			p := port.ProductSearchResult{}
			p.ASIN, _ = item["asin"].(string)
			if p.ASIN == "" {
				continue
			}

			if summaries, ok := item["summaries"].([]any); ok && len(summaries) > 0 {
				if s, ok := summaries[0].(map[string]any); ok {
					p.Title, _ = s["itemName"].(string)
					// SP-API uses "brandName" in keyword search but "brand" in identifier lookup
					p.Brand, _ = s["brandName"].(string)
					if p.Brand == "" {
						p.Brand, _ = s["brand"].(string)
					}
				}
			}

			if ranks, ok := item["salesRanks"].([]any); ok {
				for _, rawRank := range ranks {
					if r, ok := rawRank.(map[string]any); ok {
						if classRanks, ok := r["classificationRanks"].([]any); ok && len(classRanks) > 0 {
							if cr, ok := classRanks[0].(map[string]any); ok {
								if rank, ok := cr["rank"].(float64); ok {
									p.BSRRank = int(rank)
								}
								p.BSRCategory, _ = cr["title"].(string)
							}
						}
					}
				}
			}

			allProducts = append(allProducts, p)
		}
	}

	// Enrich with pricing data
	c.enrichPricing(ctx, allProducts, marketplace)

	slog.Info("sp-api: identifier lookup complete", "identifiers", len(identifiers), "results", len(allProducts))
	return allProducts, nil
}

func mockIdentifierLookup(identifiers []string, _ string) []port.ProductSearchResult {
	// Return mock matches for roughly half of the identifiers
	var results []port.ProductSearchResult
	mockProducts := []struct {
		title string
		brand string
		price float64
		bsr   int
	}{
		{"Premium Kitchen Knife Set", "ChefMaster", 34.99, 2500},
		{"Organic Green Tea 100pk", "TeaLeaf", 18.99, 8200},
		{"Wireless Mouse Ergonomic", "TechGrip", 22.99, 4100},
		{"Yoga Mat Extra Thick", "FlexFit", 28.99, 3300},
	}
	for i, id := range identifiers {
		if i%2 == 0 && i/2 < len(mockProducts) {
			mp := mockProducts[i/2]
			results = append(results, port.ProductSearchResult{
				ASIN:        fmt.Sprintf("B0MOCK%s", id[len(id)-4:]),
				Title:       mp.title,
				Brand:       mp.brand,
				AmazonPrice: mp.price,
				BSRRank:     mp.bsr,
				SellerCount: 5 + rand.Intn(10),
			})
		}
	}
	return results
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
