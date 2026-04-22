package tool

import (
	"context"
	"encoding/json"
	"maps"

	"github.com/example/claude-code-go/pkg/claudetypes"
)

// ============================================================================
// Tool interface — the contract every tool satisfies
// ============================================================================

// Tool is the interface every tool must implement.
// Implementations register themselves into a Registry at startup.
type Tool interface {
	// Name returns the unique tool name (e.g. "Bash", "Read", "Edit").
	Name() string

	// Schema returns the JSON Schema describing the tool's expected input.
	// This is sent to the model so it knows how to call the tool.
	Schema() JSONSchema

	// Call executes the tool with the given input and context.
	// It returns a channel of ToolEvents for streaming output.
	// Blocking tools send one result event and close the channel.
	Call(ctx context.Context, input json.RawMessage, tc ToolContext) (<-chan ToolEvent, error)
}

// JSONSchema is a JSON Schema object describing tool input.
type JSONSchema struct {
	Type        string              `json:"type"`
	Properties  map[string]Property `json:"properties,omitempty"`
	Required    []string            `json:"required,omitempty"`
	Description string              `json:"description,omitempty"`
	// Extra holds any additional JSON Schema fields.
	Extra map[string]any `json:"-"`
}

// Property describes a single property in a JSON Schema.
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	// Items describes array element types.
	Items *Property `json:"items,omitempty"`
}

// MarshalJSON implements custom JSON marshaling to include Extra fields.
func (s JSONSchema) MarshalJSON() ([]byte, error) {
	m := map[string]any{
		"type": s.Type,
	}
	if len(s.Properties) > 0 {
		m["properties"] = s.Properties
	}
	if len(s.Required) > 0 {
		m["required"] = s.Required
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	maps.Copy(m, s.Extra)
	return json.Marshal(m)
}

// ============================================================================
// ToolContext — context passed to each tool invocation
// ============================================================================

// ToolContext carries the ambient state a tool needs during execution.
type ToolContext struct {
	// ToolUseID is the unique ID for this specific tool_use block.
	ToolUseID string

	// Abort is a channel that signals the tool should stop.
	Abort <-chan struct{}

	// Messages is the full conversation history up to this point.
	Messages []claudetypes.Message

	// CWD is the current working directory.
	CWD string

	// PermissionMode is the current permission mode.
	PermissionMode claudetypes.PermissionMode

	// OnProgress is an optional callback for reporting incremental progress.
	OnProgress func(ToolEvent)

	// MaxOutputChars limits the output size. 0 means unlimited.
	MaxOutputChars int

	// IsNonInteractiveSession is true for headless/SDK mode.
	IsNonInteractiveSession bool
}

// ============================================================================
// ToolEvent — streaming output from tool execution
// ============================================================================

// ToolEventType identifies the kind of tool event.
type ToolEventType string

const (
	EventResult   ToolEventType = "result"
	EventProgress ToolEventType = "progress"
	EventError    ToolEventType = "error"
)

// ToolEvent is a single event from a tool's execution stream.
type ToolEvent struct {
	Type ToolEventType

	// Result carries the final tool output (EventResult).
	Result *ToolResult

	// Progress carries incremental status (EventProgress).
	Progress *ToolProgress

	// Error carries any error (EventError).
	Error error
}

// ToolResult is the final output of a tool invocation.
type ToolResult struct {
	// Output is the text result shown to the model.
	Output string

	// IsError marks this result as an error.
	IsError bool

	// NewMessages are additional messages to inject into the transcript.
	NewMessages []claudetypes.Message
}

// ToolProgress reports incremental progress from a long-running tool.
type ToolProgress struct {
	Message string
	Data    any
}

// ============================================================================
// Validation
// ============================================================================

// ValidationResult is returned by optional tool-level validation.
type ValidationResult struct {
	Valid   bool
	Message string
}

// ============================================================================
// Tool metadata
// ============================================================================

// Metadata describes a tool's static properties.
type Metadata struct {
	Name            string
	IsReadOnly      bool
	IsConcurrencySafe bool
	IsDestructive   bool
	IsMCP           bool
	MCPServerName   string
}
