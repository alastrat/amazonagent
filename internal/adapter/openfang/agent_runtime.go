package openfang

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

// AgentRuntime implements port.AgentRuntime by calling OpenFang's HTTP API.
// It spawns persistent agents on first use and reuses them across calls.
type AgentRuntime struct {
	apiURL     string
	apiKey     string
	httpClient *http.Client

	mu       sync.Mutex
	agentIDs map[string]string // agentName -> OpenFang agent UUID
}

func NewAgentRuntime(apiURL, apiKey string) *AgentRuntime {
	return &AgentRuntime{
		apiURL: apiURL,
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // LLM calls can be slow
		},
		agentIDs: make(map[string]string),
	}
}

// RunAgent sends a message to the appropriate OpenFang agent and returns the structured response.
func (r *AgentRuntime) RunAgent(ctx context.Context, task domain.AgentTask) (*domain.AgentOutput, error) {
	start := time.Now()

	// Get or spawn the agent for this role
	agentID, err := r.getOrSpawnAgent(ctx, task.AgentName, task.SystemPrompt)
	if err != nil {
		return nil, fmt.Errorf("get/spawn agent %q: %w", task.AgentName, err)
	}

	// Build the message to send
	message := buildAgentMessage(task)

	slog.Info("openfang: sending message to agent",
		"agent", task.AgentName,
		"agent_id", agentID,
	)

	// Send blocking message
	resp, err := r.sendMessage(ctx, agentID, message)
	if err != nil {
		return nil, fmt.Errorf("send message to %q: %w", task.AgentName, err)
	}

	// Parse the response into structured output
	structured, err := parseAgentResponse(resp)
	if err != nil {
		slog.Warn("openfang: failed to parse structured output, returning raw",
			"agent", task.AgentName,
			"error", err,
		)
		// Return raw response as best-effort
		structured = map[string]any{
			"raw_response": resp.GetResponse(),
			"reasoning":    resp.GetResponse(),
		}
	}

	return &domain.AgentOutput{
		Structured: structured,
		Raw:        resp.GetResponse(),
		TokensUsed: resp.InputTokens + resp.OutputTokens,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

// getOrSpawnAgent returns an existing agent ID or spawns a new one.
func (r *AgentRuntime) getOrSpawnAgent(ctx context.Context, agentName, systemPrompt string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if id, ok := r.agentIDs[agentName]; ok {
		return id, nil
	}

	// Spawn a new agent with the role's system prompt
	id, err := r.spawnAgent(ctx, agentName, systemPrompt)
	if err != nil {
		return "", err
	}

	r.agentIDs[agentName] = id
	slog.Info("openfang: spawned agent", "name", agentName, "id", id)
	return id, nil
}

// spawnAgent creates a new agent in OpenFang.
func (r *AgentRuntime) spawnAgent(ctx context.Context, name, systemPrompt string) (string, error) {
	manifest := fmt.Sprintf(`name = "%s"
version = "0.1.0"
module = "builtin:chat"
system_prompt = """%s"""
`, name, systemPrompt)

	body, err := json.Marshal(map[string]string{
		"manifest_toml": manifest,
	})
	if err != nil {
		return "", fmt.Errorf("marshal spawn request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", r.apiURL+"/api/agents", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	r.setHeaders(req)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("spawn agent failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		AgentID string `json:"agent_id"`
		ID      string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode spawn response: %w", err)
	}

	id := result.AgentID
	if id == "" {
		id = result.ID
	}
	if id == "" {
		return "", fmt.Errorf("no agent_id in spawn response")
	}

	return id, nil
}

// messageResponse is the OpenFang message response format.
type messageResponse struct {
	Response     string  `json:"response"`
	Answer       string  `json:"answer"` // fallback field name
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	Iterations   int     `json:"iterations"`
}

func (m *messageResponse) GetResponse() string {
	if m.Response != "" {
		return m.Response
	}
	return m.Answer
}

// sendMessage sends a blocking message to an agent.
func (r *AgentRuntime) sendMessage(ctx context.Context, agentID, message string) (*messageResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"message": message,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", r.apiURL+"/api/agents/"+agentID+"/message", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	r.setHeaders(req)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("message failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result messageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

func (r *AgentRuntime) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if r.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.apiKey)
	}
}

// buildAgentMessage constructs the message to send to the agent.
func buildAgentMessage(task domain.AgentTask) string {
	var msg string

	// Strong JSON-only instruction at the top
	msg += "You MUST respond with ONLY a valid JSON object. No markdown code fences, no explanation, no text before or after. Just raw JSON.\n\n"

	// Add input data — keep it compact to reduce token usage
	if len(task.Input) > 0 {
		inputJSON, _ := json.Marshal(task.Input)
		msg += "Input:\n" + string(inputJSON) + "\n\n"
	}

	// Add upstream context if provided
	if len(task.Context) > 0 {
		msg += "Context from previous agents:\n"
		for _, ctx := range task.Context {
			ctxJSON, _ := json.Marshal(ctx.Facts)
			msg += fmt.Sprintf("- %s: %s", ctx.AgentName, string(ctxJSON))
			if len(ctx.Flags) > 0 {
				msg += fmt.Sprintf(" flags=%v", ctx.Flags)
			}
			msg += "\n"
		}
		msg += "\n"
	}

	// Add output schema — use agent-specific schemas for better compliance
	schema := task.OutputSchema
	if len(schema) == 0 {
		schema = defaultSchemaFor(task.AgentName)
	}
	if len(schema) > 0 {
		schemaJSON, _ := json.Marshal(schema)
		msg += "Required JSON output format:\n" + string(schemaJSON) + "\n\n"
	}

	msg += "RESPOND WITH ONLY THE JSON OBJECT. NO OTHER TEXT."

	return msg
}

// defaultSchemaFor returns the expected output schema for each agent type.
func defaultSchemaFor(agentName string) map[string]any {
	switch agentName {
	case "sourcing":
		return map[string]any{
			"candidates": []map[string]any{
				{"asin": "string", "title": "string", "brand": "string", "category": "string", "amazon_price": 0.0, "bsr_rank": 0, "seller_count": 0},
			},
		}
	case "gating":
		return map[string]any{
			"passed":     true,
			"risk_score": 5,
			"flags":      []string{},
			"reasoning":  "string",
		}
	case "profitability":
		return map[string]any{
			"amazon_price":   0.0,
			"wholesale_cost": 0.0,
			"net_margin_pct": 0.0,
			"roi_pct":        0.0,
			"reasoning":      "string",
		}
	case "demand":
		return map[string]any{
			"demand_score":      7,
			"competition_score": 6,
			"bsr_rank":          5000,
			"monthly_units":     500,
			"reasoning":         "string",
		}
	case "supplier":
		return map[string]any{
			"suppliers": []map[string]any{
				{"company": "string", "unit_price": 0.0, "moq": 100, "lead_time_days": 14, "authorized": true},
			},
			"outreach_draft": "string",
			"reasoning":      "string",
		}
	case "reviewer":
		return map[string]any{
			"opportunity_viability": 7,
			"execution_confidence":  7,
			"sourcing_feasibility":  7,
			"reasoning":             "string",
		}
	default:
		return nil
	}
}

// parseAgentResponse attempts to extract a JSON object from the agent's response.
func parseAgentResponse(resp *messageResponse) (map[string]any, error) {
	answer := resp.GetResponse()

	// Try direct JSON parse first
	var result map[string]any
	if err := json.Unmarshal([]byte(answer), &result); err == nil {
		return result, nil
	}

	// Try to extract JSON from markdown code blocks
	jsonStr := extractJSON(answer)
	if jsonStr != "" {
		if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
			return result, nil
		}
	}

	return nil, fmt.Errorf("no valid JSON found in response")
}

// extractJSON pulls JSON from markdown code blocks or raw text.
func extractJSON(text string) string {
	// Try ```json ... ``` blocks
	start := -1
	for i := 0; i < len(text)-6; i++ {
		if text[i:i+7] == "```json" {
			start = i + 7
			// Skip whitespace/newline after ```json
			for start < len(text) && (text[start] == '\n' || text[start] == '\r' || text[start] == ' ') {
				start++
			}
			break
		}
		if text[i:i+3] == "```" && start == -1 {
			start = i + 3
			for start < len(text) && (text[start] == '\n' || text[start] == '\r' || text[start] == ' ') {
				start++
			}
		}
	}

	if start >= 0 {
		end := len(text)
		for i := start; i < len(text)-2; i++ {
			if text[i:i+3] == "```" {
				end = i
				break
			}
		}
		return text[start:end]
	}

	// Try to find first { ... last }
	firstBrace := -1
	lastBrace := -1
	for i, c := range text {
		if c == '{' && firstBrace == -1 {
			firstBrace = i
		}
		if c == '}' {
			lastBrace = i
		}
	}
	if firstBrace >= 0 && lastBrace > firstBrace {
		return text[firstBrace : lastBrace+1]
	}

	return ""
}
