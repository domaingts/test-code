package llm

import (
	"context"

	"github.com/example/claude-code-go/pkg/claudetypes"
)

// ============================================================================
// Tool specification — describes tools available to the model
// ============================================================================

// ToolSpec describes a tool that the model can invoke.
type ToolSpec struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Schema is a JSON Schema for the tool's input.
	Schema map[string]any `json:"input_schema"`
}

// ============================================================================
// Thinking config — extended thinking
// ============================================================================

// ThinkingConfig controls extended thinking behavior.
type ThinkingConfig struct {
	Type         string `json:"type,omitempty"` // "enabled" or "disabled"
	BudgetTokens int64  `json:"budget_tokens,omitempty"`
}

// ============================================================================
// Request — what we send to the model
// ============================================================================

// Request represents a single API call to the model.
type Request struct {
	Model       string
	MaxTokens   int64
	System      []claudetypes.ContentBlock
	Messages    []claudetypes.Message
	Tools       []ToolSpec
	Thinking    *ThinkingConfig
	Temperature *float64
	Stream      bool // if true, use streaming; otherwise non-streaming
}

// ============================================================================
// Stream events — emitted by the streaming channel
// ============================================================================

// StreamEventType identifies the kind of streaming event.
type StreamEventType string

const (
	EventMessageStart    StreamEventType = "message_start"
	EventContentStart    StreamEventType = "content_block_start"
	EventContentDelta    StreamEventType = "content_block_delta"
	EventContentStop     StreamEventType = "content_block_stop"
	EventMessageDelta    StreamEventType = "message_delta"
	EventMessageStop     StreamEventType = "message_stop"
	EventError           StreamEventType = "error"
	EventUsage           StreamEventType = "usage"
)

// StreamEvent is a single event from the streaming API.
type StreamEvent struct {
	Type StreamEventType

	// MessageStart carries the initial message metadata.
	MessageStart *MessageStartData
	// ContentBlockStart carries the start of a content block.
	ContentBlockStart *ContentBlockStartData
	// ContentBlockDelta carries incremental content.
	ContentBlockDelta *ContentBlockDeltaData
	// ContentBlockStop marks the end of a content block.
	ContentBlockStop *ContentBlockStopData
	// MessageDelta carries final message metadata (stop reason, usage).
	MessageDelta *MessageDeltaData
	// Usage carries token usage (emitted at message stop or message delta).
	Usage *claudetypes.Usage
	// Error carries any error that occurred during streaming.
	Error error
}

// MessageStartData contains data for a message_start event.
type MessageStartData struct {
	ID      string
	Role    string
	Model   string
	Usage   claudetypes.Usage
}

// ContentBlockStartData contains data for a content_block_start event.
type ContentBlockStartData struct {
	Index int
	Type  string // "text", "tool_use", "thinking"
	// For tool_use blocks
	ToolUseID   string
	ToolUseName string
	// For thinking blocks
	Thinking string
}

// ContentBlockDeltaData contains incremental content for a content_block_delta event.
type ContentBlockDeltaData struct {
	Index int
	Type  string // "text_delta", "input_json_delta", "thinking_delta"
	// Content
	Text       string
	JSON       string // partial JSON for tool input
	Thinking   string
}

// ContentBlockStopData marks the end of a content block.
type ContentBlockStopData struct {
	Index int
}

// MessageDeltaData contains final metadata for a message_delta event.
type MessageDeltaData struct {
	StopReason string // "end_turn", "max_tokens", "stop_sequence", "tool_use"
	Usage      claudetypes.Usage
}

// ============================================================================
// Response — the complete result of a non-streaming call
// ============================================================================

// Response is the result of a non-streaming API call.
type Response struct {
	Message claudetypes.AssistantMessage
	Usage   claudetypes.Usage
}

// ============================================================================
// Client — the LLM abstraction
// ============================================================================

// Client is the abstraction over the LLM backend.
// Implementations include AnthropicDirect, and future Bedrock/Vertex backends.
type Client interface {
	// Stream sends a request and returns a channel of streaming events.
	// The channel is closed when the stream completes or an error occurs.
	// The caller must drain the channel to completion.
	Stream(ctx context.Context, req Request) (<-chan StreamEvent, error)

	// Send sends a non-streaming request and returns the complete response.
	Send(ctx context.Context, req Request) (*Response, error)
}

// ============================================================================
// Config — how to construct a client
// ============================================================================

// Config holds configuration for creating an LLM client.
type Config struct {
	// APIKey for direct Anthropic access.
	APIKey string
	// BaseURL overrides the API base URL (for proxies).
	BaseURL string
	// Model is the default model to use.
	Model string
	// FallbackModel is tried if the primary model fails.
	FallbackModel string
	// MaxRetries is the number of retry attempts on transient errors.
	MaxRetries int
	// Timeout per request in seconds. 0 means no timeout.
	Timeout int
}
