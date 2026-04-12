package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/claude"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// AGUIHandler implements the AG-UI protocol for CopilotKit integration.
// It streams SSE events back following the AG-UI event spec.
type AGUIHandler struct {
	apiKey   string
	model    string
	toolkit  *claude.ConciergeToolkit
	profiles port.SellerProfileRepo
	fps      port.EligibilityFingerprintRepo
}

// NewAGUIHandler creates the AG-UI protocol handler.
func NewAGUIHandler(
	apiKey string,
	toolkit *claude.ConciergeToolkit,
	profiles port.SellerProfileRepo,
	fps port.EligibilityFingerprintRepo,
) *AGUIHandler {
	return &AGUIHandler{
		apiKey:   apiKey,
		model:    "claude-sonnet-4-5-20250929",
		toolkit:  toolkit,
		profiles: profiles,
		fps:      fps,
	}
}

// aguiRequest is the incoming POST body from CopilotKit's runtime.
type aguiRequest struct {
	ThreadID string        `json:"threadId"`
	RunID    string        `json:"runId"`
	Messages []aguiMessage `json:"messages"`
	Tools    []aguiTool    `json:"tools"`
}

type aguiMessage struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

type aguiTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Run handles POST /api/copilotkit — the main AG-UI streaming endpoint.
func (h *AGUIHandler) Run(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	if ac == nil || ac.TenantID == "" {
		response.Error(w, http.StatusForbidden, "no tenant context")
		return
	}

	if h.apiKey == "" {
		response.Error(w, http.StatusServiceUnavailable, "AG-UI not configured: missing Anthropic API key")
		return
	}

	var req aguiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	runID := req.RunID
	if runID == "" {
		runID = port.UUIDGenerator{}.New()
	}

	// Emit RUN_STARTED
	writeSSE(w, flusher, "RUN_STARTED", map[string]string{"runId": runID})

	// Build Claude messages from AG-UI request
	systemPrompt := h.buildSystemPrompt(r.Context(), ac.TenantID)
	messages := h.convertMessages(req.Messages)

	// Inject tenant ID into context for tool execution
	ctx := claude.WithTenantID(r.Context(), ac.TenantID)

	// Build tool params — use our concierge toolkit
	tools := h.buildToolParams()

	// Create Claude client per-request (stateless)
	client := anthropic.NewClient(option.WithAPIKey(h.apiKey))

	// ReAct loop: reason -> tool call -> observe -> repeat (max 10 turns)
	const maxTurns = 10
	for turn := 0; turn < maxTurns; turn++ {
		slog.Debug("agui: ReAct turn", "turn", turn, "tenant", ac.TenantID)

		resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     h.model,
			MaxTokens: 4096,
			System: []anthropic.TextBlockParam{
				{Text: systemPrompt},
			},
			Messages: messages,
			Tools:    tools,
		})
		if err != nil {
			slog.Error("agui: claude API error", "error", err, "turn", turn)
			writeSSE(w, flusher, "RUN_ERROR", map[string]string{
				"runId":   runID,
				"message": "Claude API error: " + err.Error(),
			})
			writeSSE(w, flusher, "RUN_FINISHED", map[string]string{"runId": runID})
			return
		}

		// Process response content blocks
		var assistantBlocks []anthropic.ContentBlockParamUnion
		hasToolUse := resp.StopReason == "tool_use"

		for _, block := range resp.Content {
			switch block.Type {
			case "text":
				if block.Text != "" {
					msgID := port.UUIDGenerator{}.New()
					writeSSE(w, flusher, "TEXT_MESSAGE_START", map[string]string{"messageId": msgID})
					writeSSE(w, flusher, "TEXT_MESSAGE_CONTENT", map[string]any{
						"messageId": msgID,
						"delta":     block.Text,
					})
					writeSSE(w, flusher, "TEXT_MESSAGE_END", map[string]string{"messageId": msgID})
				}
				assistantBlocks = append(assistantBlocks, anthropic.NewTextBlock(block.Text))

			case "tool_use":
				toolCallID := block.ID
				writeSSE(w, flusher, "TOOL_CALL_START", map[string]string{
					"toolCallId":   toolCallID,
					"toolCallName": block.Name,
				})

				// Emit tool call args
				argsJSON, _ := json.Marshal(block.Input)
				writeSSE(w, flusher, "TOOL_CALL_ARGS", map[string]any{
					"toolCallId": toolCallID,
					"delta":      string(argsJSON),
				})

				writeSSE(w, flusher, "TOOL_CALL_END", map[string]string{"toolCallId": toolCallID})

				assistantBlocks = append(assistantBlocks, anthropic.ContentBlockParamUnion{
					OfToolUse: &anthropic.ToolUseBlockParam{
						ID:    block.ID,
						Name:  block.Name,
						Input: block.Input,
					},
				})
			}
		}

		// Append assistant message to conversation
		messages = append(messages, anthropic.NewAssistantMessage(assistantBlocks...))

		// If Claude wants to use tools, execute them and continue the loop
		if hasToolUse {
			var toolResults []anthropic.ContentBlockParamUnion
			for _, block := range resp.Content {
				if block.Type == "tool_use" {
					slog.Info("agui: executing tool", "tool", block.Name, "turn", turn)

					inputJSON, _ := json.Marshal(block.Input)
					result, err := h.toolkit.Execute(ctx, block.Name, inputJSON)
					if err != nil {
						result = fmt.Sprintf("Tool error: %s", err)
					}

					toolResults = append(toolResults, anthropic.NewToolResultBlock(block.ID, result, false))
				}
			}
			messages = append(messages, anthropic.NewUserMessage(toolResults...))
			continue // Next ReAct turn
		}

		// No tool calls — Claude is done
		break
	}

	writeSSE(w, flusher, "RUN_FINISHED", map[string]string{"runId": runID})
}

// convertMessages transforms AG-UI messages into Claude API message params.
func (h *AGUIHandler) convertMessages(msgs []aguiMessage) []anthropic.MessageParam {
	var out []anthropic.MessageParam
	for _, m := range msgs {
		switch m.Role {
		case "user":
			out = append(out, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		case "assistant":
			out = append(out, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		}
	}
	return out
}

// buildToolParams converts our concierge toolkit to Claude API tool params.
func (h *AGUIHandler) buildToolParams() []anthropic.ToolUnionParam {
	conciergeTools := h.toolkit.GetTools()
	tools := make([]anthropic.ToolUnionParam, 0, len(conciergeTools))
	for _, t := range conciergeTools {
		properties, _ := t.InputSchema["properties"]
		required, _ := t.InputSchema["required"].([]string)

		tools = append(tools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.String(t.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: properties,
					Required:   required,
				},
			},
		})
	}
	return tools
}

// buildSystemPrompt creates the concierge system prompt with tenant context.
func (h *AGUIHandler) buildSystemPrompt(ctx context.Context, tenantID domain.TenantID) string {
	prompt := `You are an FBA wholesale concierge for an Amazon seller. You help them find profitable products they can list and sell on Amazon via wholesale/arbitrage.

IMPORTANT: You have tools that query REAL data from the seller's account. Use them — don't guess.
- ALWAYS call get_assessment_summary first to understand the seller's current situation
- Use get_eligible_products to show products they can list NOW
- Use get_ungatable_products to show products they can apply for approval
- Use search_products to find new products on Amazon
- Use check_eligibility to verify if a specific ASIN can be listed

Guidelines:
- Be concise and actionable — this is a seller who wants to make money, not read essays
- When discussing products, always reference ASIN, price, estimated margin, and eligibility status
- For "ungatable" products (status: can apply), tell them they can request approval via Seller Central
- Provide the Seller Central approval URL when available
- Prioritize high-margin, low-competition products
- Never auto-execute critical actions — always ask for confirmation
- Base your answers on the seller's ACTUAL DATA below, not general knowledge
`

	profile, err := h.profiles.Get(ctx, tenantID)
	if err == nil && profile != nil {
		prompt += fmt.Sprintf("\n## Seller Profile\nArchetype: %s\n", profile.Archetype)
	}

	prompt += "\nUse your tools to get real-time data. Don't rely on cached information.\n"
	return prompt
}

// writeSSE writes a single SSE event to the response writer and flushes.
func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, data any) {
	payload, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
	flusher.Flush()
}
