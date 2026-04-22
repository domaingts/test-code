package todowrite

import (
	"context"
	"encoding/json"
	"fmt"

	toolpkg "github.com/example/claude-code-go/pkg/tool"
)

// Tool implements the "TodoWrite" tool — manages an in-session todo list.
type Tool struct {
	// Todos stores the current todo list keyed by session/agent ID.
	Todos map[string][]TodoItem
}

func (t *Tool) Name() string { return "TodoWrite" }

func (t *Tool) Schema() toolpkg.JSONSchema {
	return toolpkg.JSONSchema{
		Type: "object",
		Properties: map[string]toolpkg.Property{
			"todos": {
				Type:        "array",
				Description: "The updated todo list",
				Items: &toolpkg.Property{
					Type: "object",
				},
			},
		},
		Required: []string{"todos"},
	}
}

// TodoItem represents a single todo entry.
type TodoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`     // "pending", "in_progress", "completed"
	ActiveForm string `json:"activeForm"` // e.g. "Running tests"
}

type input struct {
	Todos []TodoItem `json:"todos"`
}

type output struct {
	OldTodos []TodoItem `json:"oldTodos"`
	NewTodos []TodoItem `json:"newTodos"`
}

func (t *Tool) Call(_ context.Context, raw json.RawMessage, tc toolpkg.ToolContext) (<-chan toolpkg.ToolEvent, error) {
	var in input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}

	ch := make(chan toolpkg.ToolEvent, 1)

	key := tc.ToolUseID
	if key == "" {
		key = "default"
	}

	if t.Todos == nil {
		t.Todos = make(map[string][]TodoItem)
	}

	oldTodos := t.Todos[key]

	// If all items are completed, clear the list
	allDone := true
	for _, todo := range in.Todos {
		if todo.Status != "completed" {
			allDone = false
			break
		}
	}
	if allDone && len(in.Todos) > 0 {
		t.Todos[key] = nil
	} else {
		t.Todos[key] = in.Todos
	}

	out := output{
		OldTodos: oldTodos,
		NewTodos: in.Todos,
	}

	data, _ := json.Marshal(out)
	ch <- toolpkg.ToolEvent{
		Type:   toolpkg.EventResult,
		Result: &toolpkg.ToolResult{Output: string(data)},
	}
	close(ch)
	return ch, nil
}

var _ toolpkg.Tool = (*Tool)(nil)
