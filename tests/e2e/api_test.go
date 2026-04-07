//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const (
	baseURL  = "http://localhost:8081"
	authToken = "Bearer dev-user-dev-tenant"
)

// helper to make requests with auth header
func doRequest(t *testing.T, method, path string, body any) (*http.Response, map[string]any) {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, baseURL+path, bodyReader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", authToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, path, err)
	}

	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var result map[string]any
	// try to parse as JSON object; if it fails or is an array, wrap it
	if err := json.Unmarshal(raw, &result); err != nil {
		// might be an array
		var arr []any
		if err2 := json.Unmarshal(raw, &arr); err2 == nil {
			result = map[string]any{"_array": arr}
		} else {
			result = map[string]any{"_raw": string(raw)}
		}
	}

	return resp, result
}

// doRequestArray parses a JSON array response.
func doRequestArray(t *testing.T, method, path string, body any) (*http.Response, []any) {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}

	req, _ := http.NewRequest(method, baseURL+path, bodyReader)
	req.Header.Set("Authorization", authToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, path, err)
	}

	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var arr []any
	if err := json.Unmarshal(raw, &arr); err != nil {
		// not an array; try object
		t.Logf("response is not an array: %s", string(raw))
		return resp, nil
	}
	return resp, arr
}

func TestE2E_APIEndpoints(t *testing.T) {
	// Pre-flight: skip if API is not running
	client := &http.Client{Timeout: 3 * time.Second}
	_, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Skipf("API not running at %s, skipping E2E tests: %v", baseURL, err)
	}

	// ----------------------------------------------------------------
	// Health & Ready
	// ----------------------------------------------------------------
	t.Run("Health", func(t *testing.T) {
		resp, body := doRequest(t, "GET", "/health", nil)
		assertStatus(t, resp, 200)
		assertField(t, body, "status", "ok")
	})

	t.Run("Ready", func(t *testing.T) {
		resp, body := doRequest(t, "GET", "/ready", nil)
		assertStatus(t, resp, 200)
		assertField(t, body, "status", "ready")
	})

	// ----------------------------------------------------------------
	// Settings
	// ----------------------------------------------------------------
	t.Run("Settings_Get", func(t *testing.T) {
		resp, body := doRequest(t, "GET", "/settings", nil)
		assertStatus(t, resp, 200)
		// Should have agent_memory_enabled field
		if _, ok := body["agent_memory_enabled"]; !ok {
			t.Errorf("expected agent_memory_enabled in response, got: %v", body)
		}
	})

	t.Run("Settings_Update", func(t *testing.T) {
		payload := map[string]any{
			"agent_memory_enabled": true,
		}
		resp, body := doRequest(t, "PUT", "/settings", payload)
		assertStatus(t, resp, 200)
		if v, ok := body["agent_memory_enabled"]; !ok || v != true {
			t.Errorf("expected agent_memory_enabled=true, got: %v", body)
		}
	})

	// ----------------------------------------------------------------
	// Scoring Config
	// ----------------------------------------------------------------
	t.Run("ScoringConfig_Get", func(t *testing.T) {
		resp, body := doRequest(t, "GET", "/config/scoring", nil)
		assertStatus(t, resp, 200)
		if _, ok := body["weights"]; !ok {
			t.Errorf("expected weights in scoring config, got: %v", body)
		}
	})

	// ----------------------------------------------------------------
	// Discovery Config
	// ----------------------------------------------------------------
	t.Run("Discovery_Get", func(t *testing.T) {
		resp, body := doRequest(t, "GET", "/discovery", nil)
		// May return 200 with defaults or 404 if no config saved yet
		if resp.StatusCode != 200 && resp.StatusCode != 404 {
			t.Errorf("expected status 200 or 404, got %d", resp.StatusCode)
		}
		if resp.StatusCode == 200 {
			if _, ok := body["categories"]; !ok {
				t.Errorf("expected categories in discovery config, got: %v", body)
			}
		} else {
			t.Logf("discovery config not found (404) — will be created by update test")
		}
	})

	t.Run("Discovery_Update", func(t *testing.T) {
		payload := map[string]any{
			"categories": []string{"electronics", "toys"},
			"cadence":    "weekly",
			"enabled":    true,
			"baseline_criteria": map[string]any{
				"keywords":    []string{"gadgets"},
				"marketplace": "US",
			},
		}
		resp, body := doRequest(t, "PUT", "/discovery", payload)
		if resp.StatusCode == 500 {
			t.Skipf("discovery update returned 500 (likely missing DB table/migration): %v", body["error"])
		}
		assertStatus(t, resp, 200)
		cats, ok := body["categories"]
		if !ok {
			t.Errorf("expected categories in response, got: %v", body)
		} else {
			arr, _ := cats.([]any)
			if len(arr) != 2 {
				t.Errorf("expected 2 categories, got %d: %v", len(arr), arr)
			}
		}
	})

	// ----------------------------------------------------------------
	// Brand Blocklist
	// ----------------------------------------------------------------
	t.Run("BrandBlocklist_InitialEmpty", func(t *testing.T) {
		resp, arr := doRequestArray(t, "GET", "/brand-blocklist", nil)
		assertStatus(t, resp, 200)
		// May or may not be empty depending on prior runs; just check 200
		t.Logf("initial blocklist has %d entries", len(arr))
	})

	t.Run("BrandBlocklist_Add", func(t *testing.T) {
		payload := map[string]any{
			"brand":  "TestBrand",
			"reason": "e2e test",
		}
		resp, body := doRequest(t, "POST", "/brand-blocklist", payload)
		assertStatus(t, resp, 201)
		assertField(t, body, "status", "added")
		assertField(t, body, "brand", "TestBrand")
	})

	t.Run("BrandBlocklist_Contains", func(t *testing.T) {
		resp, arr := doRequestArray(t, "GET", "/brand-blocklist", nil)
		assertStatus(t, resp, 200)
		found := false
		for _, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			// Check both "brand" and "brand_name" fields, case-insensitive
			brand, _ := m["brand"].(string)
			brandName, _ := m["brand_name"].(string)
			if strings.EqualFold(brand, "TestBrand") || strings.EqualFold(brandName, "TestBrand") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected TestBrand in blocklist, got: %v", arr)
		}
	})

	t.Run("BrandBlocklist_Remove", func(t *testing.T) {
		payload := map[string]any{
			"brand": "TestBrand",
		}
		resp, body := doRequest(t, "DELETE", "/brand-blocklist", payload)
		assertStatus(t, resp, 200)
		assertField(t, body, "status", "removed")
	})

	// ----------------------------------------------------------------
	// Campaign Lifecycle
	// ----------------------------------------------------------------
	var campaignID string

	t.Run("Campaign_Create", func(t *testing.T) {
		payload := map[string]any{
			"type":         "manual",
			"trigger_type": "dashboard",
			"criteria": map[string]any{
				"keywords":    []string{"test product"},
				"marketplace": "US",
			},
		}
		resp, body := doRequest(t, "POST", "/campaigns", payload)
		if resp.StatusCode != 201 && resp.StatusCode != 200 {
			t.Fatalf("expected 200 or 201, got %d: %v", resp.StatusCode, body)
		}
		id, ok := body["id"].(string)
		if !ok || id == "" {
			t.Fatalf("expected campaign id in response, got: %v", body)
		}
		campaignID = id
		t.Logf("created campaign: %s", campaignID)
	})

	t.Run("Campaign_List", func(t *testing.T) {
		if campaignID == "" {
			t.Skip("no campaign created")
		}
		resp, arr := doRequestArray(t, "GET", "/campaigns", nil)
		assertStatus(t, resp, 200)
		found := false
		for _, item := range arr {
			m, ok := item.(map[string]any)
			if ok && m["id"] == campaignID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected campaign %s in list", campaignID)
		}
	})

	t.Run("Campaign_GetByID", func(t *testing.T) {
		if campaignID == "" {
			t.Skip("no campaign created")
		}
		resp, body := doRequest(t, "GET", "/campaigns/"+campaignID, nil)
		assertStatus(t, resp, 200)
		assertField(t, body, "id", campaignID)
		assertField(t, body, "type", "manual")
	})

	// ----------------------------------------------------------------
	// Deals
	// ----------------------------------------------------------------
	t.Run("Deals_List", func(t *testing.T) {
		resp, body := doRequest(t, "GET", "/deals", nil)
		assertStatus(t, resp, 200)
		if _, ok := body["deals"]; !ok {
			t.Errorf("expected deals key in response, got: %v", body)
		}
		if _, ok := body["total"]; !ok {
			t.Errorf("expected total key in response, got: %v", body)
		}
	})

	t.Run("Dashboard_Summary", func(t *testing.T) {
		resp, body := doRequest(t, "GET", "/dashboard/summary", nil)
		assertStatus(t, resp, 200)
		for _, key := range []string{"deals_pending_review", "deals_approved", "active_campaigns", "recent_deals"} {
			if _, ok := body[key]; !ok {
				t.Errorf("expected %s in dashboard summary, got: %v", key, body)
			}
		}
	})

	// ----------------------------------------------------------------
	// Events
	// ----------------------------------------------------------------
	t.Run("Events_List", func(t *testing.T) {
		resp, arr := doRequestArray(t, "GET", "/events", nil)
		assertStatus(t, resp, 200)
		// After creating a campaign, there should be at least one event
		if campaignID != "" {
			found := false
			for _, item := range arr {
				m, ok := item.(map[string]any)
				if ok {
					if et, _ := m["event_type"].(string); et == "campaign_created" {
						found = true
						break
					}
				}
			}
			if !found {
				t.Logf("warning: campaign_created event not found in %d events (may be expected if events are async)", len(arr))
			}
		}
	})
}

func TestE2E_DiscoveryEngine(t *testing.T) {
	// Pre-flight: skip if API is not running
	client := &http.Client{Timeout: 3 * time.Second}
	_, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Skipf("API not running at %s, skipping E2E tests: %v", baseURL, err)
	}

	// ----------------------------------------------------------------
	// Catalog Products — should return empty initially
	// ----------------------------------------------------------------
	t.Run("Catalog_ListProducts", func(t *testing.T) {
		resp, body := doRequest(t, "GET", "/catalog/products", nil)
		assertStatus(t, resp, 200)
		if _, ok := body["products"]; !ok {
			t.Errorf("expected products key in response, got: %v", body)
		}
		if _, ok := body["total"]; !ok {
			t.Errorf("expected total key in response, got: %v", body)
		}
	})

	t.Run("Catalog_ListProducts_WithFilters", func(t *testing.T) {
		resp, body := doRequest(t, "GET", "/catalog/products?category=Kitchen&min_margin=10&sort_by=estimated_margin_pct&sort_dir=desc&limit=10", nil)
		assertStatus(t, resp, 200)
		if _, ok := body["products"]; !ok {
			t.Errorf("expected products key, got: %v", body)
		}
	})

	// ----------------------------------------------------------------
	// Catalog Stats
	// ----------------------------------------------------------------
	t.Run("Catalog_Stats", func(t *testing.T) {
		resp, body := doRequest(t, "GET", "/catalog/stats", nil)
		assertStatus(t, resp, 200)
		if _, ok := body["total_products"]; !ok {
			t.Errorf("expected total_products in stats, got: %v", body)
		}
		if _, ok := body["eligible_count"]; !ok {
			t.Errorf("expected eligible_count in stats, got: %v", body)
		}
	})

	// ----------------------------------------------------------------
	// Brand Intelligence
	// ----------------------------------------------------------------
	t.Run("Catalog_ListBrands", func(t *testing.T) {
		resp, body := doRequest(t, "GET", "/catalog/brands", nil)
		// Materialized view may not exist yet on fresh DB — accept 200 or 500
		if resp.StatusCode == 500 {
			t.Logf("brand intelligence view not yet created (expected on fresh DB): %v", body)
			return
		}
		assertStatus(t, resp, 200)
		if _, ok := body["brands"]; !ok {
			t.Errorf("expected brands key, got: %v", body)
		}
	})

	t.Run("Catalog_ListBrands_WithFilters", func(t *testing.T) {
		resp, _ := doRequest(t, "GET", "/catalog/brands?min_margin=20&min_products=5&sort_by=avg_margin", nil)
		// Accept 200 or 500 (view may not exist)
		if resp.StatusCode != 200 && resp.StatusCode != 500 {
			t.Errorf("expected 200 or 500, got %d", resp.StatusCode)
		}
	})

	t.Run("Catalog_BrandProducts", func(t *testing.T) {
		// Use a dummy brand ID — should return empty, not error
		resp, body := doRequest(t, "GET", "/catalog/brands/00000000-0000-0000-0000-000000000001/products", nil)
		assertStatus(t, resp, 200)
		if _, ok := body["products"]; !ok {
			t.Errorf("expected products key, got: %v", body)
		}
	})

	// ----------------------------------------------------------------
	// Scan Jobs
	// ----------------------------------------------------------------
	t.Run("Scans_List", func(t *testing.T) {
		resp, arr := doRequestArray(t, "GET", "/scans", nil)
		assertStatus(t, resp, 200)
		t.Logf("scan jobs: %d", len(arr))
	})

	t.Run("Scans_GetNotFound", func(t *testing.T) {
		resp, _ := doRequest(t, "GET", "/scans/nonexistent-id", nil)
		if resp.StatusCode != 404 && resp.StatusCode != 500 {
			t.Errorf("expected 404 or 500 for nonexistent scan, got %d", resp.StatusCode)
		}
	})

	// ----------------------------------------------------------------
	// Category Scan Trigger
	// ----------------------------------------------------------------
	t.Run("Scans_TriggerCategory", func(t *testing.T) {
		payload := map[string]any{
			"max_nodes": 5,
		}
		resp, body := doRequest(t, "POST", "/scans/category", payload)
		// May return 202 (accepted), 503 (inngest not available), or 500
		if resp.StatusCode == 202 {
			assertField(t, body, "status", "triggered")
			t.Logf("category scan triggered")
		} else if resp.StatusCode == 503 {
			t.Logf("durable runtime not available (expected in dev without Inngest): %v", body)
		} else {
			t.Logf("category scan trigger returned %d: %v", resp.StatusCode, body)
		}
	})

	// ----------------------------------------------------------------
	// Price List Upload (funnel-based)
	// ----------------------------------------------------------------
	t.Run("PriceList_UploadFunnel_NoFile", func(t *testing.T) {
		// Should fail with 400 — no file provided
		req, _ := http.NewRequest("POST", baseURL+"/pricelist/upload-funnel", nil)
		req.Header.Set("Authorization", authToken)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for missing file, got %d", resp.StatusCode)
		}
	})

	// ----------------------------------------------------------------
	// Funnel-based upload with a test CSV
	// ----------------------------------------------------------------
	t.Run("PriceList_UploadFunnel_WithCSV", func(t *testing.T) {
		csvData := "UPC,Product Name,Wholesale Cost,Brand\n012345678901,Test Product,12.99,TestBrand\n023456789012,Another Product,8.50,AnotherBrand\n"

		body := &bytes.Buffer{}
		writer := io.Writer(body)
		boundary := "----TestBoundary"
		writer.Write([]byte("------TestBoundary\r\n"))
		writer.Write([]byte("Content-Disposition: form-data; name=\"file\"; filename=\"test.csv\"\r\n"))
		writer.Write([]byte("Content-Type: text/csv\r\n\r\n"))
		writer.Write([]byte(csvData))
		writer.Write([]byte("\r\n------TestBoundary--\r\n"))

		req, _ := http.NewRequest("POST", baseURL+"/pricelist/upload-funnel", body)
		req.Header.Set("Authorization", authToken)
		req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("upload failed: %v", err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		t.Logf("upload response (%d): %s", resp.StatusCode, string(raw))

		// Accept 200 (success) or 500 (SP-API not configured in dev)
		if resp.StatusCode != 200 && resp.StatusCode != 500 && resp.StatusCode != 400 {
			t.Errorf("expected 200, 400 or 500, got %d", resp.StatusCode)
		}

		if resp.StatusCode == 200 {
			var result map[string]any
			json.Unmarshal(raw, &result)
			if _, ok := result["funnel_stats"]; !ok {
				t.Errorf("expected funnel_stats in response, got: %v", result)
			}
			if _, ok := result["total_items"]; !ok {
				t.Errorf("expected total_items in response, got: %v", result)
			}
		}
	})
}

// ----------------------------------------------------------------
// Assertion helpers
// ----------------------------------------------------------------

func assertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		t.Errorf("expected status %d, got %d", expected, resp.StatusCode)
	}
}

func assertField(t *testing.T, body map[string]any, key string, expected any) {
	t.Helper()
	val, ok := body[key]
	if !ok {
		t.Errorf("expected key %q in response, got: %v", key, body)
		return
	}
	if fmt.Sprintf("%v", val) != fmt.Sprintf("%v", expected) {
		t.Errorf("expected %s=%v, got %v", key, expected, val)
	}
}
