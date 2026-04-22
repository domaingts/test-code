package queryengine

import (
	"context"
	"time"

	"github.com/example/claude-code-go/pkg/claudetypes"
)

// Run executes the query engine loop and returns a channel of SDK events.
// The channel is closed when the engine completes or encounters an error.
func (e *engine) Run(ctx context.Context, input claudetypes.UserMessage) <-chan claudetypes.SDKEvent {
	events := make(chan claudetypes.SDKEvent, 16)

	go func() {
		defer close(events)

		sessionID := generateUUID()
		var messages []claudetypes.Message

		// Load prior messages from store if configured
		if e.cfg.Store != nil {
			if prior, err := e.cfg.Store.Load(sessionID); err == nil && prior != nil {
				messages = append(messages, prior...)
			}
		}

		// Append the user input message
		messages = append(messages, input)
		if err := emit(ctx, events, claudetypes.SDKMessageEvent{Message: input}); err != nil {
			return
		}

		// Persist user message
		if e.cfg.Store != nil {
			e.cfg.Store.Append(sessionID, []claudetypes.Message{input})
		}

		// Main turn loop
		totalUsage := claudetypes.Usage{}
		maxTurns := e.cfg.MaxTurns
		if maxTurns <= 0 {
			maxTurns = 100
		}

		var lastAssistant claudetypes.AssistantMessage

		for turn := 0; turn < maxTurns; turn++ {
			// Build request
			req := e.buildRequest(messages)

			// Stream and collect
			result, err := e.streamAndCollect(ctx, req)
			if err != nil {
				emit(ctx, events, claudetypes.SDKErrorEvent{Error: err})
				return
			}

			// Accumulate usage
			totalUsage = addUsage(totalUsage, result.Usage)

			// Emit assistant message
			if err := emit(ctx, events, claudetypes.SDKMessageEvent{Message: result.AssistantMsg}); err != nil {
				return
			}
			lastAssistant = result.AssistantMsg

			// Persist assistant message
			messages = append(messages, result.AssistantMsg)
			if e.cfg.Store != nil {
				e.cfg.Store.Append(sessionID, []claudetypes.Message{result.AssistantMsg})
			}

			// If no tool_use, we're done
			if len(result.ToolUse) == 0 {
				emit(ctx, events, claudetypes.SDKResultEvent{
					Result:   lastAssistant,
					Usage:    totalUsage,
					NumTurns: turn + 1,
				})
				return
			}

			// Execute tools
			toolResults := e.executeTools(ctx, result.ToolUse, messages)

			// Build tool_result user message
			toolResultMsg := claudetypes.UserMessage{
				MessageBase: claudetypes.MessageBase{
					UUID:      generateUUID(),
					Timestamp: time.Now(),
				},
				Content: toolResultsToContentBlocks(toolResults),
			}

			// Emit tool result message
			if err := emit(ctx, events, claudetypes.SDKMessageEvent{Message: toolResultMsg}); err != nil {
				return
			}

			messages = append(messages, toolResultMsg)
			if e.cfg.Store != nil {
				e.cfg.Store.Append(sessionID, []claudetypes.Message{toolResultMsg})
			}
		}

		// Max turns reached
		emit(ctx, events, claudetypes.SDKResultEvent{
			IsError:  true,
			Usage:    totalUsage,
			NumTurns: maxTurns,
		})
	}()

	return events
}

// emit sends an event to the channel, respecting context cancellation.
func emit(ctx context.Context, ch chan<- claudetypes.SDKEvent, evt claudetypes.SDKEvent) error {
	select {
	case ch <- evt:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// addUsage combines two Usage values.
func addUsage(a, b claudetypes.Usage) claudetypes.Usage {
	return claudetypes.Usage{
		InputTokens:              a.InputTokens + b.InputTokens,
		OutputTokens:             a.OutputTokens + b.OutputTokens,
		CacheReadInputTokens:     a.CacheReadInputTokens + b.CacheReadInputTokens,
		CacheCreationInputTokens: a.CacheCreationInputTokens + b.CacheCreationInputTokens,
	}
}

// toolResultsToContentBlocks converts tool results to content blocks for a UserMessage.
func toolResultsToContentBlocks(results []claudetypes.ToolResultBlock) []claudetypes.ContentBlock {
	blocks := make([]claudetypes.ContentBlock, len(results))
	for i, r := range results {
		blocks[i] = r
	}
	return blocks
}
