package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// AgentRuntime implements port.ConversationalRuntime using the Claude API directly.
// Supports tool calling via the ReAct pattern: reason → call tool → observe → repeat.
type AgentRuntime struct {
	client  anthropic.Client
	toolkit *ConciergeToolkit
	model   string
	sessions map[string]*session
}

type session struct {
	systemPrompt string
	messages     []anthropic.MessageParam
	tenantID     domain.TenantID
}

// NewAgentRuntime creates a Claude API-based agent runtime.
// Dependencies are port-based — no concrete types leak from the adapter.
func NewAgentRuntime(apiKey string, deps ToolDeps) *AgentRuntime {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	toolkit := NewConciergeToolkit(deps)
	return &AgentRuntime{
		client:   client,
		toolkit:  toolkit,
		model:    "claude-sonnet-4-5-20250929",
		sessions: make(map[string]*session),
	}
}

// RunAgent implements port.AgentRuntime for backward compatibility with the pipeline.
func (r *AgentRuntime) RunAgent(ctx context.Context, task domain.AgentTask) (*domain.AgentOutput, error) {
	start := time.Now()

	resp, err := r.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     r.model,
		MaxTokens: 4096,
		System: []anthropic.TextBlockParam{
			{Text: task.SystemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(buildAgentMessage(task))),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("claude API: %w", err)
	}

	raw := extractText(resp)
	structured := parseJSON(raw)

	return &domain.AgentOutput{
		Structured: structured,
		Raw:        raw,
		TokensUsed: int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

// StartSession creates a persistent conversation session.
func (r *AgentRuntime) StartSession(_ context.Context, tenantID domain.TenantID, config port.SessionConfig) (string, error) {
	sessionID := fmt.Sprintf("claude-session-%s", tenantID)

	r.sessions[sessionID] = &session{
		systemPrompt: config.SystemPrompt,
		messages:     []anthropic.MessageParam{},
		tenantID:     tenantID,
	}

	slog.Info("claude: session started", "tenant_id", tenantID, "session_id", sessionID)
	return sessionID, nil
}

// SendMessage sends a user message, runs the ReAct tool loop, and returns the final response.
func (r *AgentRuntime) SendMessage(ctx context.Context, sessionID string, message string) (*domain.AgentOutput, error) {
	sess, ok := r.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	start := time.Now()
	totalTokens := 0

	// Add user message to history
	sess.messages = append(sess.messages, anthropic.NewUserMessage(anthropic.NewTextBlock(message)))

	// Inject tenant ID into context for tool execution
	ctx = WithTenantID(ctx, sess.tenantID)

	// Build tool definitions for Claude API
	tools := r.buildToolParams()

	// ReAct loop: reason → tool call → observe → repeat (max 10 turns)
	const maxTurns = 10
	for turn := 0; turn < maxTurns; turn++ {
		slog.Debug("claude: ReAct turn", "turn", turn, "session", sessionID, "messages", len(sess.messages))

		resp, err := r.client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     r.model,
			MaxTokens: 4096,
			System: []anthropic.TextBlockParam{
				{Text: sess.systemPrompt},
			},
			Messages: sess.messages,
			Tools:    tools,
		})
		if err != nil {
			return nil, fmt.Errorf("claude API turn %d: %w", turn, err)
		}

		totalTokens += int(resp.Usage.InputTokens + resp.Usage.OutputTokens)

		// Convert response content blocks to params for history
		var assistantBlocks []anthropic.ContentBlockParamUnion
		for _, block := range resp.Content {
			switch block.Type {
			case "text":
				assistantBlocks = append(assistantBlocks, anthropic.NewTextBlock(block.Text))
			case "tool_use":
				assistantBlocks = append(assistantBlocks, anthropic.ContentBlockParamUnion{
					OfToolUse: &anthropic.ToolUseBlockParam{
						ID:    block.ID,
						Name:  block.Name,
						Input: block.Input,
					},
				})
			}
		}
		sess.messages = append(sess.messages, anthropic.NewAssistantMessage(assistantBlocks...))

		// Check if Claude wants to use tools
		if resp.StopReason == "tool_use" {
			var toolResults []anthropic.ContentBlockParamUnion
			for _, block := range resp.Content {
				if block.Type == "tool_use" {
					slog.Info("claude: tool call", "tool", block.Name, "turn", turn)

					inputJSON, _ := json.Marshal(block.Input)
					result, err := r.toolkit.Execute(ctx, block.Name, inputJSON)
					if err != nil {
						result = fmt.Sprintf("Tool error: %s", err)
					}

					toolResults = append(toolResults, anthropic.NewToolResultBlock(block.ID, result, false))
				}
			}
			sess.messages = append(sess.messages, anthropic.NewUserMessage(toolResults...))
			continue // Next turn — Claude processes tool results
		}

		// No tool calls — Claude is done. Extract final text.
		finalText := extractText(resp)

		slog.Info("claude: response complete",
			"session", sessionID,
			"turns", turn+1,
			"tokens", totalTokens,
			"duration_ms", time.Since(start).Milliseconds(),
		)

		return &domain.AgentOutput{
			Structured: map[string]any{"message": finalText},
			Raw:        finalText,
			TokensUsed: totalTokens,
			DurationMs: time.Since(start).Milliseconds(),
		}, nil
	}

	return &domain.AgentOutput{
		Structured: map[string]any{"message": "I've reached my reasoning limit. Please try a more specific question."},
		Raw:        "Max turns exhausted",
		TokensUsed: totalTokens,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

// EndSession cleans up a session.
func (r *AgentRuntime) EndSession(_ context.Context, sessionID string) error {
	delete(r.sessions, sessionID)
	slog.Info("claude: session ended", "session_id", sessionID)
	return nil
}

// buildToolParams converts toolkit tools to Claude API tool params.
func (r *AgentRuntime) buildToolParams() []anthropic.ToolUnionParam {
	conciergeTools := r.toolkit.GetTools()
	tools := make([]anthropic.ToolUnionParam, 0, len(conciergeTools))
	for _, t := range conciergeTools {
		// Build InputSchema from the tool's schema map
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

func extractText(resp *anthropic.Message) string {
	for _, block := range resp.Content {
		if block.Type == "text" {
			return block.Text
		}
	}
	return ""
}

func parseJSON(text string) map[string]any {
	var m map[string]any
	if err := json.Unmarshal([]byte(text), &m); err == nil {
		return m
	}
	return map[string]any{"message": text, "reasoning": text}
}

func buildAgentMessage(task domain.AgentTask) string {
	inputJSON, _ := json.MarshalIndent(task.Input, "", "  ")
	return fmt.Sprintf("Input data:\n%s\n\nRespond with valid JSON only.", string(inputJSON))
}
