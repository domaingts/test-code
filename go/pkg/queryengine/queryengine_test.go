package queryengine

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/example/claude-code-go/pkg/claudetypes"
	"github.com/example/claude-code-go/pkg/llm"
	"github.com/example/claude-code-go/pkg/permission"
	"github.com/example/claude-code-go/pkg/session"
	"github.com/example/claude-code-go/pkg/tool"
)

// --- Mock infrastructure ---

type mockClient struct {
	streamFunc func(context.Context, llm.Request) (<-chan llm.StreamEvent, error)
}

func (m *mockClient) Stream(ctx context.Context, req llm.Request) (<-chan llm.StreamEvent, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, req)
	}
	return newMockStream(nil...), nil
}

func (m *mockClient) Send(ctx context.Context, req llm.Request) (*llm.Response, error) {
	return nil, errors.New("not implemented")
}


// newMockStream creates a channel that emits the given events and closes.
func newMockStream(events ...llm.StreamEvent) <-chan llm.StreamEvent {
	ch := make(chan llm.StreamEvent, len(events))
	for _, e := range events {
		ch <- e
	}
	close(ch)
	return ch
}

// textStreamEvents builds a sequence of stream events for a text-only response.
func textStreamEvents(text string, stopReason string) []llm.StreamEvent {
	return []llm.StreamEvent{
		{Type: llm.EventMessageStart, MessageStart: &llm.MessageStartData{Role: "assistant"}},
		{Type: llm.EventContentStart, ContentBlockStart: &llm.ContentBlockStartData{Index: 0, Type: "text"}},
		{Type: llm.EventContentDelta, ContentBlockDelta: &llm.ContentBlockDeltaData{Index: 0, Type: "text_delta", Text: text}},
		{Type: llm.EventContentStop, ContentBlockStop: &llm.ContentBlockStopData{Index: 0}},
		{Type: llm.EventMessageDelta, MessageDelta: &llm.MessageDeltaData{StopReason: stopReason}},
		{Type: llm.EventMessageStop},
	}
}

// toolUseStreamEvents builds a sequence for a tool_use response.
func toolUseStreamEvents(toolID, toolName, inputJSON string) []llm.StreamEvent {
	return []llm.StreamEvent{
		{Type: llm.EventMessageStart, MessageStart: &llm.MessageStartData{Role: "assistant"}},
		{Type: llm.EventContentStart, ContentBlockStart: &llm.ContentBlockStartData{Index: 0, Type: "tool_use", ToolUseID: toolID, ToolUseName: toolName}},
		{Type: llm.EventContentDelta, ContentBlockDelta: &llm.ContentBlockDeltaData{Index: 0, Type: "input_json_delta", JSON: inputJSON}},
		{Type: llm.EventContentStop, ContentBlockStop: &llm.ContentBlockStopData{Index: 0}},
		{Type: llm.EventMessageDelta, MessageDelta: &llm.MessageDeltaData{StopReason: "tool_use"}},
		{Type: llm.EventMessageStop},
	}
}

type mockTool struct {
	name   string
	result string
	isErr  bool
	err    error
}

func (m *mockTool) Name() string { return m.name }
func (m *mockTool) Schema() tool.JSONSchema {
	return tool.JSONSchema{Type: "object", Description: "mock tool"}
}
func (m *mockTool) Call(ctx context.Context, input json.RawMessage, tc tool.ToolContext) (<-chan tool.ToolEvent, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan tool.ToolEvent, 1)
	ch <- tool.ToolEvent{
		Type:   tool.EventResult,
		Result: &tool.ToolResult{Output: m.result, IsError: m.isErr},
	}
	close(ch)
	return ch, nil
}

type mockDecider struct {
	behavior permission.Behavior
	reason   string
}

func (m *mockDecider) CanUse(toolName string, input json.RawMessage, mode claudetypes.PermissionMode) (decision permission.Decision, err error) {
	return permission.Decision{Behavior: m.behavior, Reason: m.reason}, nil
}

type mockStore struct {
	mu       sync.Mutex
	messages []claudetypes.Message
}

func (m *mockStore) Load(sessionID string) ([]claudetypes.Message, error) { return nil, nil }
func (m *mockStore) Append(sessionID string, msgs []claudetypes.Message) error {
	m.mu.Lock()
	m.messages = append(m.messages, msgs...)
	m.mu.Unlock()
	return nil
}
func (m *mockStore) List() ([]session.SessionInfo, error) { return nil, nil }
func (m *mockStore) Delete(sessionID string) error { return nil }

func (m *mockStore) messageCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

// SessionInfo is imported from session package but we need a local alias for mockStore.
// Since mockStore.List() returns session.SessionInfo, we need the import.
// But the mock is defined inline, so we'll use the concrete type.

// --- Tests ---

func TestRun_singleTurnTextOnly(t *testing.T) {
	client := &mockClient{
		streamFunc: func(ctx context.Context, req llm.Request) (<-chan llm.StreamEvent, error) {
			return newMockStream(textStreamEvents("Hello there!", "end_turn")...), nil
		},
	}

	reg := tool.NewRegistry()
	eng, err := New(Config{Client: client, Tools: reg, CWD: "/tmp"})
	if err != nil {
		t.Fatal(err)
	}

	input := claudetypes.UserMessage{
		MessageBase: claudetypes.MessageBase{UUID: "u1"},
		Content:     []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "hi"}},
	}

	events := collectEvents(eng.Run(context.Background(), input))

	// Should have: user msg echo + assistant msg + result = 3 events
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}

	// First: user message echo
	if _, ok := events[0].(claudetypes.SDKMessageEvent); !ok {
		t.Error("event[0] should be SDKMessageEvent (user echo)")
	}

	// Second: assistant message
	evt1, ok := events[1].(claudetypes.SDKMessageEvent)
	if !ok {
		t.Fatal("event[1] should be SDKMessageEvent (assistant)")
	}
	assistant, ok := evt1.Message.(claudetypes.AssistantMessage)
	if !ok {
		t.Fatal("event[1] message should be AssistantMessage")
	}
	if len(assistant.Message) != 1 {
		t.Fatalf("assistant has %d content blocks, want 1", len(assistant.Message))
	}
	textBlock, ok := assistant.Message[0].(claudetypes.TextBlock)
	if !ok {
		t.Fatal("content block should be TextBlock")
	}
	if textBlock.Text != "Hello there!" {
		t.Errorf("text = %q, want %q", textBlock.Text, "Hello there!")
	}

	// Third: result
	result, ok := events[2].(claudetypes.SDKResultEvent)
	if !ok {
		t.Fatal("event[2] should be SDKResultEvent")
	}
	if result.IsError {
		t.Error("result should not be an error")
	}
	if result.NumTurns != 1 {
		t.Errorf("NumTurns = %d, want 1", result.NumTurns)
	}
}

func TestRun_singleTurnWithToolUse(t *testing.T) {
	turn := 0
	client := &mockClient{
		streamFunc: func(ctx context.Context, req llm.Request) (<-chan llm.StreamEvent, error) {
			turn++
			if turn == 1 {
				return newMockStream(toolUseStreamEvents("tool1", "Read", `{"file_path":"test.go"}`)...), nil
			}
			return newMockStream(textStreamEvents("Done reading!", "end_turn")...), nil
		},
	}

	reg := tool.NewRegistry()
	reg.Register(&mockTool{name: "Read", result: "file contents"})

	eng, _ := New(Config{Client: client, Tools: reg, CWD: "/tmp"})

	input := claudetypes.UserMessage{
		MessageBase: claudetypes.MessageBase{UUID: "u1"},
		Content:     []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "read test.go"}},
	}

	events := collectEvents(eng.Run(context.Background(), input))

	// Events: user echo, assistant (tool_use), tool_result msg, assistant (text), result = 5
	if len(events) != 5 {
		t.Fatalf("got %d events, want 5", len(events))
	}

	// Check tool_result message (event[2])
	evt2, ok := events[2].(claudetypes.SDKMessageEvent)
	if !ok {
		t.Fatal("event[2] should be SDKMessageEvent")
	}
	toolResultMsg, ok := evt2.Message.(claudetypes.UserMessage)
	if !ok {
		t.Fatal("event[2] should be UserMessage (tool result)")
	}
	if len(toolResultMsg.Content) != 1 {
		t.Fatalf("tool result content has %d blocks, want 1", len(toolResultMsg.Content))
	}
	trBlock, ok := toolResultMsg.Content[0].(claudetypes.ToolResultBlock)
	if !ok {
		t.Fatal("content block should be ToolResultBlock")
	}
	if trBlock.ToolUseID != "tool1" {
		t.Errorf("ToolUseID = %q, want tool1", trBlock.ToolUseID)
	}
	if trBlock.IsError {
		t.Error("tool result should not be an error")
	}

	// Final result
	result := events[4].(claudetypes.SDKResultEvent)
	if result.NumTurns != 2 {
		t.Errorf("NumTurns = %d, want 2", result.NumTurns)
	}
}

func TestRun_maxTurnsTermination(t *testing.T) {
	client := &mockClient{
		streamFunc: func(ctx context.Context, req llm.Request) (<-chan llm.StreamEvent, error) {
			// Always return a tool_use to force continuation
			return newMockStream(toolUseStreamEvents("t1", "Read", `{}`)...), nil
		},
	}

	reg := tool.NewRegistry()
	reg.Register(&mockTool{name: "Read", result: "x"})

	eng, _ := New(Config{Client: client, Tools: reg, CWD: "/tmp", MaxTurns: 3})

	input := claudetypes.UserMessage{
		MessageBase: claudetypes.MessageBase{UUID: "u1"},
		Content:     []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "go"}},
	}

	events := collectEvents(eng.Run(context.Background(), input))

	// Last event should be an error result (max turns)
	last := events[len(events)-1]
	result, ok := last.(claudetypes.SDKResultEvent)
	if !ok {
		t.Fatal("last event should be SDKResultEvent")
	}
	if !result.IsError {
		t.Error("should be error (max turns reached)")
	}
	if result.NumTurns != 3 {
		t.Errorf("NumTurns = %d, want 3", result.NumTurns)
	}
}

func TestRun_contextCancellation(t *testing.T) {
	streamStarted := make(chan struct{})
	client := &mockClient{
		streamFunc: func(ctx context.Context, req llm.Request) (<-chan llm.StreamEvent, error) {
			ch := make(chan llm.StreamEvent)
			close(streamStarted)
			// Block until context is cancelled
			go func() {
				<-ctx.Done()
				close(ch)
			}()
			return ch, nil
		},
	}

	reg := tool.NewRegistry()
	eng, _ := New(Config{Client: client, Tools: reg, CWD: "/tmp"})

	ctx, cancel := context.WithCancel(context.Background())
	input := claudetypes.UserMessage{
		MessageBase: claudetypes.MessageBase{UUID: "u1"},
		Content:     []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "hi"}},
	}

	events := make([]claudetypes.SDKEvent, 0)
	done := make(chan struct{})
	go func() {
		for evt := range eng.Run(ctx, input) {
			events = append(events, evt)
		}
		close(done)
	}()

	// Wait for stream to start, then cancel
	<-streamStarted
	cancel()

	// Drain remaining events
	<-done

	// Should have at least the user echo; no SDKResultEvent
	for _, evt := range events {
		if _, ok := evt.(claudetypes.SDKResultEvent); ok {
			t.Error("should not have a result event after cancellation")
		}
	}
}

func TestRun_permissionDeny(t *testing.T) {
	client := &mockClient{
		streamFunc: func(ctx context.Context, req llm.Request) (<-chan llm.StreamEvent, error) {
			return newMockStream(toolUseStreamEvents("t1", "Bash", `{"command":"rm -rf /"}`)...), nil
		},
	}

	reg := tool.NewRegistry()
	reg.Register(&mockTool{name: "Bash", result: "ok"})

	decider := &mockDecider{behavior: permission.BehaviorDeny, reason: "dangerous command"}

	eng, _ := New(Config{
		Client:   client,
		Tools:    reg,
		Decider:  decider,
		CWD:      "/tmp",
		MaxTurns: 2, // prevent infinite loop
	})

	input := claudetypes.UserMessage{
		MessageBase: claudetypes.MessageBase{UUID: "u1"},
		Content:     []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "delete everything"}},
	}

	events := collectEvents(eng.Run(context.Background(), input))

	// Find the tool_result event — should be an error
	for _, evt := range events {
		if me, ok := evt.(claudetypes.SDKMessageEvent); ok {
			if um, ok := me.Message.(claudetypes.UserMessage); ok {
				for _, block := range um.Content {
					if tr, ok := block.(claudetypes.ToolResultBlock); ok {
						if !tr.IsError {
							t.Error("tool result should be an error for denied permission")
						}
						return
					}
				}
			}
		}
	}
	t.Error("no tool_result found in events")
}

func TestRun_toolNotFound(t *testing.T) {
	client := &mockClient{
		streamFunc: func(ctx context.Context, req llm.Request) (<-chan llm.StreamEvent, error) {
			return newMockStream(toolUseStreamEvents("t1", "Nonexistent", `{}`)...), nil
		},
	}

	reg := tool.NewRegistry()
	eng, _ := New(Config{Client: client, Tools: reg, CWD: "/tmp", MaxTurns: 2})

	input := claudetypes.UserMessage{
		MessageBase: claudetypes.MessageBase{UUID: "u1"},
		Content:     []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "do something"}},
	}

	events := collectEvents(eng.Run(context.Background(), input))

	// Should have a tool_result with IsError=true
	for _, evt := range events {
		if me, ok := evt.(claudetypes.SDKMessageEvent); ok {
			if um, ok := me.Message.(claudetypes.UserMessage); ok {
				for _, block := range um.Content {
					if tr, ok := block.(claudetypes.ToolResultBlock); ok {
						if !tr.IsError {
							t.Error("tool result should be an error for missing tool")
						}
						return
					}
				}
			}
		}
	}
	t.Error("no tool_result found in events")
}

func TestRun_streamError(t *testing.T) {
	client := &mockClient{
		streamFunc: func(ctx context.Context, req llm.Request) (<-chan llm.StreamEvent, error) {
			ch := make(chan llm.StreamEvent, 2)
			ch <- llm.StreamEvent{Type: llm.EventMessageStart, MessageStart: &llm.MessageStartData{Role: "assistant"}}
			ch <- llm.StreamEvent{Type: llm.EventError, Error: errors.New("API rate limit")}
			close(ch)
			return ch, nil
		},
	}

	reg := tool.NewRegistry()
	eng, _ := New(Config{Client: client, Tools: reg, CWD: "/tmp"})

	input := claudetypes.UserMessage{
		MessageBase: claudetypes.MessageBase{UUID: "u1"},
		Content:     []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "hi"}},
	}

	events := collectEvents(eng.Run(context.Background(), input))

	// Should have user echo + error event (no result)
	foundError := false
	for _, evt := range events {
		if ee, ok := evt.(claudetypes.SDKErrorEvent); ok {
			foundError = true
			if ee.Error.Error() != "API rate limit" {
				t.Errorf("error message = %q, want API rate limit", ee.Error.Error())
			}
		}
	}
	if !foundError {
		t.Error("should have SDKErrorEvent")
	}
}

func TestRun_withSessionStore(t *testing.T) {
	client := &mockClient{
		streamFunc: func(ctx context.Context, req llm.Request) (<-chan llm.StreamEvent, error) {
			return newMockStream(textStreamEvents("ok", "end_turn")...), nil
		},
	}

	reg := tool.NewRegistry()
	store := &mockStore{}

	eng, _ := New(Config{Client: client, Tools: reg, Store: store, CWD: "/tmp"})

	input := claudetypes.UserMessage{
		MessageBase: claudetypes.MessageBase{UUID: "u1"},
		Content:     []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "hello"}},
	}

	collectEvents(eng.Run(context.Background(), input))

	// Store should have: user message + assistant message = 2
	if store.messageCount() != 2 {
		t.Errorf("store has %d messages, want 2", store.messageCount())
	}
}

func TestBuildRequest(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(&mockTool{name: "Read", result: ""})
	reg.Register(&mockTool{name: "Write", result: ""})

	eng := &engine{cfg: Config{
		Client:      &mockClient{},
		Tools:       reg,
		Model:       "claude-sonnet-4-20250514",
		MaxTokens:   4096,
		SystemPrompt: []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "system"}},
	}}

	msgs := []claudetypes.Message{
		claudetypes.UserMessage{
			MessageBase: claudetypes.MessageBase{UUID: "u1"},
			Content:     []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "hi"}},
		},
	}

	req := eng.buildRequest(msgs)

	if req.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want claude-sonnet-4-20250514", req.Model)
	}
	if req.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want 4096", req.MaxTokens)
	}
	if len(req.Messages) != 1 {
		t.Errorf("Messages = %d, want 1", len(req.Messages))
	}
	if len(req.Tools) != 2 {
		t.Errorf("Tools = %d, want 2", len(req.Tools))
	}
	if !req.Stream {
		t.Error("Stream should be true")
	}

	// Check tool specs
	names := map[string]bool{}
	for _, ts := range req.Tools {
		names[ts.Name] = true
	}
	if !names["Read"] || !names["Write"] {
		t.Errorf("tool names = %v, want Read and Write", names)
	}
}

func TestRun_emptyTools(t *testing.T) {
	callCount := 0
	client := &mockClient{
		streamFunc: func(ctx context.Context, req llm.Request) (<-chan llm.StreamEvent, error) {
			callCount++
			// Should not receive any tools in the request
			if len(req.Tools) != 0 {
				t.Errorf("expected 0 tools, got %d", len(req.Tools))
			}
			return newMockStream(textStreamEvents("no tools needed", "end_turn")...), nil
		},
	}

	reg := tool.NewRegistry() // empty
	eng, _ := New(Config{Client: client, Tools: reg, CWD: "/tmp"})

	input := claudetypes.UserMessage{
		MessageBase: claudetypes.MessageBase{UUID: "u1"},
		Content:     []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "hi"}},
	}

	events := collectEvents(eng.Run(context.Background(), input))

	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}
	if len(events) != 3 {
		t.Errorf("events = %d, want 3", len(events))
	}
}

// --- Helpers ---

func collectEvents(ch <-chan claudetypes.SDKEvent) []claudetypes.SDKEvent {
	var events []claudetypes.SDKEvent
	for evt := range ch {
		events = append(events, evt)
	}
	return events
}
