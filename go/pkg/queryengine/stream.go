package queryengine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/example/claude-code-go/pkg/claudetypes"
	"github.com/example/claude-code-go/pkg/llm"
)

// streamAndCollect drains an LLM stream and assembles a complete turn result.
func (e *engine) streamAndCollect(ctx context.Context, req llm.Request) (turnResult, error) {
	eventCh, err := e.cfg.Client.Stream(ctx, req)
	if err != nil {
		return turnResult{}, fmt.Errorf("stream init: %w", err)
	}

	var blocks []collectedBlock
	var currentBlock *collectedBlock
	var usage claudetypes.Usage
	var stopReason string

	for evt := range eventCh {
		switch evt.Type {
		case llm.EventMessageStart:
			// Initial metadata — nothing to capture for MVP

		case llm.EventContentStart:
			start := evt.ContentBlockStart
			cb := collectedBlock{Type: start.Type}
			if start.Type == "tool_use" {
				cb.ToolUse = &claudetypes.ToolUseBlock{
					ID:    start.ToolUseID,
					Name:  start.ToolUseName,
					Input: map[string]any{},
				}
			}
			blocks = append(blocks, cb)
			currentBlock = &blocks[len(blocks)-1]

		case llm.EventContentDelta:
			if currentBlock == nil {
				continue
			}
			delta := evt.ContentBlockDelta
			switch delta.Type {
			case "text_delta":
				if currentBlock.Text == nil {
					s := ""
					currentBlock.Text = &s
				}
				*currentBlock.Text += delta.Text
			case "input_json_delta":
				currentBlock.JSONBuf.WriteString(delta.JSON)
			case "thinking_delta":
				if currentBlock.Thinking == nil {
					s := ""
					currentBlock.Thinking = &s
				}
				*currentBlock.Thinking += delta.Thinking
			}

		case llm.EventContentStop:
			// Finalize tool_use input from accumulated JSON
			if currentBlock != nil && currentBlock.Type == "tool_use" && currentBlock.ToolUse != nil {
				raw := currentBlock.JSONBuf.String()
				if raw != "" {
					var input map[string]any
					if err := json.Unmarshal([]byte(raw), &input); err == nil {
						currentBlock.ToolUse.Input = input
					}
				}
			}
			currentBlock = nil

		case llm.EventMessageDelta:
			if evt.MessageDelta != nil {
				stopReason = evt.MessageDelta.StopReason
				usage = evt.MessageDelta.Usage
			}

		case llm.EventMessageStop:
			// Stream complete

		case llm.EventError:
			return turnResult{}, evt.Error
		}
	}

	// Assemble content blocks and extract tool_use blocks
	contentBlocks := make([]claudetypes.ContentBlock, 0, len(blocks))
	var toolUseBlocks []claudetypes.ToolUseBlock

	for _, b := range blocks {
		switch b.Type {
		case "text":
			text := ""
			if b.Text != nil {
				text = *b.Text
			}
			contentBlocks = append(contentBlocks, claudetypes.TextBlock{Text: text})
		case "tool_use":
			if b.ToolUse != nil {
				contentBlocks = append(contentBlocks, *b.ToolUse)
				toolUseBlocks = append(toolUseBlocks, *b.ToolUse)
			}
		case "thinking":
			thinking := ""
			if b.Thinking != nil {
				thinking = *b.Thinking
			}
			contentBlocks = append(contentBlocks, claudetypes.ThinkingBlock{Thinking: thinking})
		}
	}

	assistantMsg := claudetypes.AssistantMessage{
		MessageBase: claudetypes.MessageBase{
			UUID:      generateUUID(),
			Timestamp: time.Now(),
		},
		Message: contentBlocks,
		Usage:   usage,
	}

	return turnResult{
		AssistantMsg: assistantMsg,
		ToolUse:      toolUseBlocks,
		StopReason:   stopReason,
		Usage:        usage,
	}, nil
}
