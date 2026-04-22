package queryengine

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/example/claude-code-go/pkg/claudetypes"
	"github.com/example/claude-code-go/pkg/permission"
	"github.com/example/claude-code-go/pkg/tool"
)

// executeTools runs each tool_use block sequentially and collects results.
func (e *engine) executeTools(
	ctx context.Context,
	tools []claudetypes.ToolUseBlock,
	messages []claudetypes.Message,
) []claudetypes.ToolResultBlock {
	results := make([]claudetypes.ToolResultBlock, 0, len(tools))
	for _, tu := range tools {
		result := e.executeOneTool(ctx, tu, messages)
		results = append(results, result)
	}
	return results
}

// executeOneTool handles a single tool invocation: lookup, permission check, execution.
func (e *engine) executeOneTool(
	ctx context.Context,
	tu claudetypes.ToolUseBlock,
	messages []claudetypes.Message,
) claudetypes.ToolResultBlock {
	// Look up tool
	t, ok := e.cfg.Tools.Get(tu.Name)
	if !ok {
		return claudetypes.ToolResultBlock{
			ToolUseID: tu.ID,
			Content:   []claudetypes.ContentBlock{claudetypes.TextBlock{Text: fmt.Sprintf("tool %q not found", tu.Name)}},
			IsError:   true,
		}
	}

	// Serialize input for permission check
	inputJSON, err := json.Marshal(tu.Input)
	if err != nil {
		return claudetypes.ToolResultBlock{
			ToolUseID: tu.ID,
			Content:   []claudetypes.ContentBlock{claudetypes.TextBlock{Text: fmt.Sprintf("failed to marshal input: %v", err)}},
			IsError:   true,
		}
	}

	// Permission check
	if e.cfg.Decider != nil {
		dec, err := e.cfg.Decider.CanUse(tu.Name, inputJSON, e.cfg.PermissionMode)
		if err != nil {
			return claudetypes.ToolResultBlock{
				ToolUseID: tu.ID,
				Content:   []claudetypes.ContentBlock{claudetypes.TextBlock{Text: fmt.Sprintf("permission error: %v", err)}},
				IsError:   true,
			}
		}
		switch dec.Behavior {
		case permission.BehaviorAllow:
			// proceed
		case permission.BehaviorDeny:
			reason := dec.Reason
			if reason == "" {
				reason = "permission denied"
			}
			return claudetypes.ToolResultBlock{
				ToolUseID: tu.ID,
				Content:   []claudetypes.ContentBlock{claudetypes.TextBlock{Text: reason}},
				IsError:   true,
			}
		case permission.BehaviorAsk:
			// MVP: treat ask as deny
			return claudetypes.ToolResultBlock{
				ToolUseID: tu.ID,
				Content:   []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "permission required (ask mode not yet supported)"}},
				IsError:   true,
			}
		}
	}

	// Execute tool
	tc := tool.ToolContext{
		ToolUseID:      tu.ID,
		Messages:       messages,
		CWD:            e.cfg.CWD,
		PermissionMode: e.cfg.PermissionMode,
	}

	eventCh, err := t.Call(ctx, inputJSON, tc)
	if err != nil {
		return claudetypes.ToolResultBlock{
			ToolUseID: tu.ID,
			Content:   []claudetypes.ContentBlock{claudetypes.TextBlock{Text: fmt.Sprintf("tool execution error: %v", err)}},
			IsError:   true,
		}
	}

	// Drain the event channel, collect the final result
	var finalResult *tool.ToolResult
	for evt := range eventCh {
		switch evt.Type {
		case tool.EventResult:
			finalResult = evt.Result
		case tool.EventError:
			return claudetypes.ToolResultBlock{
				ToolUseID: tu.ID,
				Content:   []claudetypes.ContentBlock{claudetypes.TextBlock{Text: fmt.Sprintf("tool error: %v", evt.Error)}},
				IsError:   true,
			}
		}
	}

	if finalResult == nil {
		return claudetypes.ToolResultBlock{
			ToolUseID: tu.ID,
			Content:   []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "tool returned no result"}},
			IsError:   true,
		}
	}

	return claudetypes.ToolResultBlock{
		ToolUseID: tu.ID,
		Content:   []claudetypes.ContentBlock{claudetypes.TextBlock{Text: finalResult.Output}},
		IsError:   finalResult.IsError,
	}
}
