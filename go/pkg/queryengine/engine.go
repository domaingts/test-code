package queryengine

import (
	"context"
	"crypto/rand"
	"fmt"

	"github.com/example/claude-code-go/pkg/claudetypes"
	"github.com/example/claude-code-go/pkg/llm"
	"github.com/example/claude-code-go/pkg/permission"
	"github.com/example/claude-code-go/pkg/session"
	"github.com/example/claude-code-go/pkg/tool"
)

// Config holds configuration for the query engine.
type Config struct {
	// Client is the LLM client (required).
	Client llm.Client
	// Tools is the tool registry (required).
	Tools *tool.Registry
	// Decider evaluates permission checks (optional, nil = no permission checks).
	Decider permission.Decider
	// Store persists session transcripts (optional, nil = no persistence).
	Store session.Store
	// CWD is the current working directory.
	CWD string
	// Model is the model to use.
	Model string
	// MaxTokens is the max tokens per response (default 8192).
	MaxTokens int64
	// MaxTurns is the max number of turns before terminating (default 100).
	MaxTurns int
	// PermissionMode controls permission behavior.
	PermissionMode claudetypes.PermissionMode
	// SystemPrompt is the system prompt content.
	SystemPrompt []claudetypes.ContentBlock
}

// Engine is the query engine interface.
type Engine interface {
	// Run executes the engine with the given user input and returns a channel
	// of SDK events. The channel is closed when execution completes.
	Run(ctx context.Context, input claudetypes.UserMessage) <-chan claudetypes.SDKEvent
}

// New creates a new Engine from the given config.
func New(cfg Config) (Engine, error) {
	if cfg.Client == nil {
		return nil, fmt.Errorf("queryengine: Client is required")
	}
	if cfg.Tools == nil {
		return nil, fmt.Errorf("queryengine: Tools is required")
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 8192
	}
	if cfg.MaxTurns <= 0 {
		cfg.MaxTurns = 100
	}
	return &engine{cfg: cfg}, nil
}

// generateUUID creates a random UUID v4 string.
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
