package tool

import (
	"context"
	"encoding/json"
	"testing"
)

// --- Mock tool ---

type mockTool struct {
	name   string
	schema JSONSchema
}

func (m *mockTool) Name() string { return m.name }
func (m *mockTool) Schema() JSONSchema { return m.schema }
func (m *mockTool) Call(_ context.Context, _ json.RawMessage, tc ToolContext) (<-chan ToolEvent, error) {
	ch := make(chan ToolEvent, 1)
	ch <- ToolEvent{Type: EventResult, Result: &ToolResult{Output: "ok"}}
	close(ch)
	return ch, nil
}

// --- Registry tests ---

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	tool := &mockTool{name: "Test", schema: JSONSchema{Type: "object"}}

	reg.Register(tool)

	got, ok := reg.Get("Test")
	if !ok {
		t.Fatal("expected to find Test")
	}
	if got.Name() != "Test" {
		t.Errorf("Name() = %q, want Test", got.Name())
	}
}

func TestRegistry_Get_missing(t *testing.T) {
	reg := NewRegistry()

	_, ok := reg.Get("NonExistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestRegistry_Alias(t *testing.T) {
	reg := NewRegistry()
	tool := &mockTool{name: "Read", schema: JSONSchema{Type: "object"}}
	reg.Register(tool)
	reg.RegisterAlias("Cat", "Read")

	got, ok := reg.Get("Cat")
	if !ok {
		t.Fatal("expected to find alias Cat")
	}
	if got.Name() != "Read" {
		t.Errorf("alias resolved to %q, want Read", got.Name())
	}
}

func TestRegistry_All(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockTool{name: "A"})
	reg.Register(&mockTool{name: "B"})
	reg.Register(&mockTool{name: "C"})
	reg.RegisterAlias("D", "A") // alias shouldn't duplicate

	all := reg.All()
	if len(all) != 3 {
		t.Errorf("All() returned %d tools, want 3", len(all))
	}
}

func TestRegistry_Names(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockTool{name: "Bash"})
	reg.Register(&mockTool{name: "Read"})
	reg.Register(&mockTool{name: "Edit"})

	names := reg.Names()
	if len(names) != 3 {
		t.Fatalf("Names() returned %d, want 3", len(names))
	}
	// Should be sorted
	if names[0] != "Bash" || names[1] != "Edit" || names[2] != "Read" {
		t.Errorf("Names() = %v, want [Bash Edit Read]", names)
	}
}

func TestRegistry_Len(t *testing.T) {
	reg := NewRegistry()
	if reg.Len() != 0 {
		t.Errorf("Len() = %d, want 0", reg.Len())
	}
	reg.Register(&mockTool{name: "X"})
	reg.Register(&mockTool{name: "Y"})
	if reg.Len() != 2 {
		t.Errorf("Len() = %d, want 2", reg.Len())
	}
}

func TestRegistry_Replace(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockTool{name: "X", schema: JSONSchema{Description: "v1"}})
	reg.Register(&mockTool{name: "X", schema: JSONSchema{Description: "v2"}})

	got, _ := reg.Get("X")
	if got.Schema().Description != "v2" {
		t.Errorf("Description = %q, want v2", got.Schema().Description)
	}
	if reg.Len() != 1 {
		t.Errorf("Len() = %d, want 1 (replaced, not duplicated)", reg.Len())
	}
}

func TestMustGet(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockTool{name: "X"})

	// Should not panic
	_ = reg.MustGet("X")

	// Should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing tool")
		}
	}()
	_ = reg.MustGet("Missing")
}

// --- Tool call tests ---

func TestMockTool_Call(t *testing.T) {
	tool := &mockTool{name: "Echo", schema: JSONSchema{Type: "object"}}
	ch, err := tool.Call(context.Background(), nil, ToolContext{ToolUseID: "test"})
	if err != nil {
		t.Fatal(err)
	}

	evt := <-ch
	if evt.Type != EventResult {
		t.Errorf("event type = %q, want result", evt.Type)
	}
	if evt.Result.Output != "ok" {
		t.Errorf("output = %q, want ok", evt.Result.Output)
	}
}

func TestToolContext_abort(t *testing.T) {
	aborted := false
	tool := &callCapture{
		name: "Abortable",
		fn: func(_ context.Context, tc ToolContext) (<-chan ToolEvent, error) {
			select {
			case <-tc.Abort:
				aborted = true
			default:
			}
			ch := make(chan ToolEvent, 1)
			close(ch)
			return ch, nil
		},
	}

	abort := make(chan struct{})
	close(abort) // pre-cancelled
	_, err := tool.Call(context.Background(), nil, ToolContext{ToolUseID: "t", Abort: abort})
	if err != nil {
		t.Fatal(err)
	}
	if !aborted {
		t.Error("tool should have seen the abort signal")
	}
}

// --- JSON Schema tests ---

func TestJSONSchema_MarshalJSON(t *testing.T) {
	schema := JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"path": {Type: "string", Description: "file path"},
			"line": {Type: "integer"},
		},
		Required: []string{"path"},
	}

	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m["type"] != "object" {
		t.Errorf("type = %v, want object", m["type"])
	}
	if m["required"] == nil {
		t.Error("required should be present")
	}
}

func TestJSONSchema_Extra(t *testing.T) {
	schema := JSONSchema{
		Type: "object",
		Extra: map[string]any{
			"additionalProperties": false,
		},
	}

	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m["additionalProperties"] != false {
		t.Error("extra fields should be included")
	}
}

// --- Helper ---

type callCapture struct {
	name string
	fn   func(context.Context, ToolContext) (<-chan ToolEvent, error)
	schema JSONSchema
}

func (c *callCapture) Name() string        { return c.name }
func (c *callCapture) Schema() JSONSchema  { return c.schema }
func (c *callCapture) Call(ctx context.Context, _ json.RawMessage, tc ToolContext) (<-chan ToolEvent, error) {
	return c.fn(ctx, tc)
}

// Verify interface compliance
var _ Tool = (*mockTool)(nil)
var _ Tool = (*callCapture)(nil)
