package queryengine

import (
	"strings"

	"github.com/example/claude-code-go/pkg/claudetypes"
)

// engine is the concrete Engine implementation.
type engine struct {
	cfg Config
}

// turnResult captures what a single turn produces.
type turnResult struct {
	AssistantMsg claudetypes.AssistantMessage
	ToolUse      []claudetypes.ToolUseBlock
	StopReason   string // "end_turn", "max_tokens", "tool_use"
	Usage        claudetypes.Usage
}

// collectedBlock accumulates a single content block during streaming.
type collectedBlock struct {
	Type     string // "text", "tool_use", "thinking"
	Text     *string
	ToolUse  *claudetypes.ToolUseBlock
	Thinking *string
	JSONBuf  strings.Builder // for input_json_delta
}
