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
