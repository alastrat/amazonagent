package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

// ---------------------------------------------------------------------------
// Mock runtime
// ---------------------------------------------------------------------------

type mockRuntime struct {
	mu        sync.Mutex
	callCount int
	outputs   []*domain.AgentOutput // returned in order; last one repeats
	errs      []error               // parallel to outputs; nil = no error
}

func (m *mockRuntime) RunAgent(_ context.Context, _ domain.AgentTask) (*domain.AgentOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.callCount
	m.callCount++

	var err error
	if idx < len(m.errs) {
		err = m.errs[idx]
	}
	if err != nil {
		return nil, err
	}

	if idx < len(m.outputs) {
		return m.outputs[idx], nil
	}
	// Repeat last output
	if len(m.outputs) > 0 {
		return m.outputs[len(m.outputs)-1], nil
	}
	return &domain.AgentOutput{Structured: map[string]any{}, Raw: "default"}, nil
}

func (m *mockRuntime) calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestRunAgentLoop_SingleShot(t *testing.T) {
	rt := &mockRuntime{
		outputs: []*domain.AgentOutput{
			{Structured: map[string]any{"result": "ok"}, Raw: "done"},
		},
	}
	def := &domain.AgentDefinition{
		Name:     "test-single",
		MaxTurns: 1,
	}
	task := domain.AgentTask{
		AgentName: "test-single",
		Input:     map[string]any{"_tenant_id": "t1"},
	}

	output, err := RunAgentLoop(context.Background(), rt, def, task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Raw != "done" {
		t.Errorf("Raw = %q, want %q", output.Raw, "done")
	}
	if rt.calls() != 1 {
		t.Errorf("RunAgent called %d times, want 1", rt.calls())
	}
}

func TestRunAgentLoop_MultiTurn_StopsOnCondition(t *testing.T) {
	rt := &mockRuntime{
		outputs: []*domain.AgentOutput{
			{Structured: map[string]any{"confidence": 50.0}, Raw: "turn0"},
			{Structured: map[string]any{"confidence": 90.0}, Raw: "turn1"},
			{Structured: map[string]any{"confidence": 95.0}, Raw: "turn2-should-not-reach"},
		},
	}
	def := &domain.AgentDefinition{
		Name:          "test-multi",
		MaxTurns:      3,
		CanSelfRefine: true,
		StopCondition: ConfidenceThreshold(80),
	}
	task := domain.AgentTask{
		AgentName: "test-multi",
		Input:     map[string]any{"_tenant_id": "t1"},
	}

	output, err := RunAgentLoop(context.Background(), rt, def, task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Raw != "turn1" {
		t.Errorf("Raw = %q, want %q (should stop at turn 1)", output.Raw, "turn1")
	}
	if rt.calls() != 2 {
		t.Errorf("RunAgent called %d times, want 2", rt.calls())
	}
}

func TestRunAgentLoop_MultiTurn_ExhaustsMaxTurns(t *testing.T) {
	rt := &mockRuntime{
		outputs: []*domain.AgentOutput{
			{Structured: map[string]any{"confidence": 30.0}, Raw: "low"},
		},
	}
	def := &domain.AgentDefinition{
		Name:          "test-exhaust",
		MaxTurns:      3,
		CanSelfRefine: true,
		StopCondition: ConfidenceThreshold(99), // never met
	}
	task := domain.AgentTask{
		AgentName: "test-exhaust",
		Input:     map[string]any{"_tenant_id": "t1"},
	}

	output, err := RunAgentLoop(context.Background(), rt, def, task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output == nil {
		t.Fatal("expected non-nil output after exhausting max turns")
	}
	if rt.calls() != 3 {
		t.Errorf("RunAgent called %d times, want 3", rt.calls())
	}
}

func TestRunAgentLoop_PreRunHookAbort(t *testing.T) {
	rt := &mockRuntime{
		outputs: []*domain.AgentOutput{
			{Raw: "should-not-reach"},
		},
	}
	def := &domain.AgentDefinition{
		Name:     "test-prehook",
		MaxTurns: 1,
		Hooks: &domain.AgentHooks{
			PreRun: []domain.HookFunc{
				func(_ context.Context, _ string, _ domain.TenantID) error {
					return errors.New("quota exceeded")
				},
			},
		},
	}
	task := domain.AgentTask{
		AgentName: "test-prehook",
		Input:     map[string]any{"_tenant_id": "t1"},
	}

	_, err := RunAgentLoop(context.Background(), rt, def, task)
	if err == nil {
		t.Fatal("expected error from pre-run hook")
	}
	if rt.calls() != 0 {
		t.Errorf("RunAgent called %d times, want 0 (pre-run hook should abort)", rt.calls())
	}
}

func TestRunAgentLoop_PostRunHookCalled(t *testing.T) {
	rt := &mockRuntime{
		outputs: []*domain.AgentOutput{
			{Raw: "result", TokensUsed: 500},
		},
	}

	var hookAgentName string
	var hookOutput *domain.AgentOutput
	var hookDuration int64

	def := &domain.AgentDefinition{
		Name:     "test-posthook",
		MaxTurns: 1,
		Hooks: &domain.AgentHooks{
			PostRun: []domain.PostRunHookFunc{
				func(_ context.Context, agentName string, output *domain.AgentOutput, durationMs int64) error {
					hookAgentName = agentName
					hookOutput = output
					hookDuration = durationMs
					return nil
				},
			},
		},
	}
	task := domain.AgentTask{
		AgentName: "test-posthook",
		Input:     map[string]any{"_tenant_id": "t1"},
	}

	_, err := RunAgentLoop(context.Background(), rt, def, task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hookAgentName != "test-posthook" {
		t.Errorf("hook agent name = %q, want %q", hookAgentName, "test-posthook")
	}
	if hookOutput == nil || hookOutput.Raw != "result" {
		t.Errorf("hook output not passed correctly")
	}
	if hookDuration < 0 {
		t.Errorf("hook duration = %d, expected >= 0", hookDuration)
	}
}

func TestRunAgentLoop_ErrorHookFires(t *testing.T) {
	rt := &mockRuntime{
		errs: []error{errors.New("runtime failure")},
	}

	var hookErr error
	def := &domain.AgentDefinition{
		Name:     "test-errhook",
		MaxTurns: 1,
		Hooks: &domain.AgentHooks{
			OnError: []domain.ErrorHookFunc{
				func(_ context.Context, _ string, err error) error {
					hookErr = err
					return nil
				},
			},
		},
	}
	task := domain.AgentTask{
		AgentName: "test-errhook",
		Input:     map[string]any{"_tenant_id": "t1"},
	}

	_, err := RunAgentLoop(context.Background(), rt, def, task)
	if err == nil {
		t.Fatal("expected error from runtime failure")
	}
	if hookErr == nil {
		t.Fatal("expected error hook to be called")
	}
	if hookErr.Error() != "runtime failure" {
		t.Errorf("hook error = %q, want %q", hookErr.Error(), "runtime failure")
	}
}

func TestRunAgentLoop_RefinementContextInjected(t *testing.T) {
	var capturedTasks []domain.AgentTask

	// Custom runtime that captures tasks
	rt := &captureRuntime{
		tasks: &capturedTasks,
		output: &domain.AgentOutput{
			Structured: map[string]any{"confidence": 30.0},
			Raw:        "previous output",
		},
	}

	def := &domain.AgentDefinition{
		Name:          "test-refine",
		MaxTurns:      2,
		CanSelfRefine: true,
		StopCondition: ConfidenceThreshold(99), // never met
	}
	task := domain.AgentTask{
		AgentName: "test-refine",
		Input:     map[string]any{"_tenant_id": "t1"},
	}

	_, err := RunAgentLoop(context.Background(), rt, def, task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedTasks) < 2 {
		t.Fatalf("expected 2 tasks captured, got %d", len(capturedTasks))
	}

	// First turn should not have refinement context
	if _, ok := capturedTasks[0].Input["_refinement_context"]; ok {
		t.Error("first turn should not have refinement context")
	}

	// Second turn should have refinement context
	rc, ok := capturedTasks[1].Input["_refinement_context"]
	if !ok {
		t.Fatal("second turn should have _refinement_context")
	}
	rcStr, ok := rc.(string)
	if !ok || rcStr == "" {
		t.Error("refinement context should be a non-empty string")
	}

	turn, ok := capturedTasks[1].Input["_turn"]
	if !ok || turn != 1 {
		t.Errorf("expected _turn=1, got %v", turn)
	}
}

// captureRuntime records tasks and returns a fixed output.
type captureRuntime struct {
	tasks  *[]domain.AgentTask
	output *domain.AgentOutput
}

func (c *captureRuntime) RunAgent(_ context.Context, task domain.AgentTask) (*domain.AgentOutput, error) {
	// Deep copy Input to avoid mutation
	cp := domain.AgentTask{
		AgentName:    task.AgentName,
		SystemPrompt: task.SystemPrompt,
		Input:        make(map[string]any),
	}
	for k, v := range task.Input {
		cp.Input[k] = v
	}
	*c.tasks = append(*c.tasks, cp)
	return c.output, nil
}
