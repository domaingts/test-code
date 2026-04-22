package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/example/claude-code-go/pkg/claudetypes"
)

// AnthropicClient implements Client using the Anthropic SDK.
type AnthropicClient struct {
	client anthropic.Client
	config Config
}

// NewAnthropic creates a new Anthropic-backed LLM client.
func NewAnthropic(cfg Config) *AnthropicClient {
	opts := []option.RequestOption{}
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}

	client := anthropic.NewClient(opts...)
	return &AnthropicClient{
		client: client,
		config: cfg,
	}
}

// Send implements Client.Send for non-streaming requests.
func (c *AnthropicClient) Send(ctx context.Context, req Request) (*Response, error) {
	params := c.buildParams(req)

	msg, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic api: %w", err)
	}

	return c.convertResponse(msg), nil
}

// Stream implements Client.Stream for streaming requests.
func (c *AnthropicClient) Stream(ctx context.Context, req Request) (<-chan StreamEvent, error) {
	params := c.buildParams(req)

	stream := c.client.Messages.NewStreaming(ctx, params)

	ch := make(chan StreamEvent, 16)

	go func() {
		defer close(ch)

		for stream.Next() {
			evt := stream.Current()
			converted := c.convertStreamEvent(evt)
			select {
			case ch <- converted:
			case <-ctx.Done():
				ch <- StreamEvent{Type: EventError, Error: ctx.Err()}
				return
			}
		}

		if err := stream.Err(); err != nil {
			ch <- StreamEvent{Type: EventError, Error: fmt.Errorf("stream error: %w", err)}
		}
	}()

	return ch, nil
}

// buildParams converts our Request into Anthropic SDK params.
func (c *AnthropicClient) buildParams(req Request) anthropic.MessageNewParams {
	params := anthropic.MessageNewParams{
		MaxTokens: req.MaxTokens,
	}

	if req.Model != "" {
		params.Model = anthropic.Model(req.Model)
	} else if c.config.Model != "" {
		params.Model = anthropic.Model(c.config.Model)
	}

	// System prompt
	if len(req.System) > 0 {
		for _, b := range req.System {
			switch block := b.(type) {
			case claudetypes.TextBlock:
				params.System = append(params.System, anthropic.TextBlockParam{Text: block.Text})
			}
		}
	}

	// Messages
	for _, m := range req.Messages {
		switch msg := m.(type) {
		case claudetypes.UserMessage:
			params.Messages = append(params.Messages, c.userMsgToParam(msg))
		case claudetypes.AssistantMessage:
			params.Messages = append(params.Messages, c.assistantMsgToParam(msg))
		}
	}

	// Tools
	for _, t := range req.Tools {
		params.Tools = append(params.Tools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.String(t.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: t.Schema,
				},
			},
		})
	}

	// Thinking
	if req.Thinking != nil && req.Thinking.Type == "enabled" {
		params.Thinking = anthropic.ThinkingConfigParamOfEnabled(req.Thinking.BudgetTokens)
	}

	if req.Temperature != nil {
		params.Temperature = anthropic.Float(*req.Temperature)
	}

	return params
}

func (c *AnthropicClient) userMsgToParam(msg claudetypes.UserMessage) anthropic.MessageParam {
	var content []anthropic.ContentBlockParamUnion
	for _, b := range msg.Content {
		switch block := b.(type) {
		case claudetypes.TextBlock:
			content = append(content, anthropic.NewTextBlock(block.Text))
		case claudetypes.ToolResultBlock:
			// Concatenate text content into a single string for the tool result
			var resultText string
			for _, rb := range block.Content {
				if tb, ok := rb.(claudetypes.TextBlock); ok {
					if resultText != "" {
						resultText += "\n"
					}
					resultText += tb.Text
				}
			}
			content = append(content, anthropic.NewToolResultBlock(
				block.ToolUseID,
				resultText,
				block.IsError,
			))
		}
	}
	return anthropic.NewUserMessage(content...)
}

func (c *AnthropicClient) assistantMsgToParam(msg claudetypes.AssistantMessage) anthropic.MessageParam {
	var content []anthropic.ContentBlockParamUnion
	for _, b := range msg.Message {
		switch block := b.(type) {
		case claudetypes.TextBlock:
			content = append(content, anthropic.NewTextBlock(block.Text))
		case claudetypes.ToolUseBlock:
			content = append(content, anthropic.NewToolUseBlock(block.ID, block.Input, block.Name))
		case claudetypes.ThinkingBlock:
			content = append(content, anthropic.NewTextBlock(block.Thinking))
		}
	}
	return anthropic.NewAssistantMessage(content...)
}

func (c *AnthropicClient) convertResponse(msg *anthropic.Message) *Response {
	usage := convertUsageFromSDK(msg.Usage)
	return &Response{
		Message: claudetypes.AssistantMessage{
			Message: convertContentBlocksFromSDK(msg.Content),
			Usage:   usage,
		},
		Usage: usage,
	}
}

func convertContentBlocksFromSDK(blocks []anthropic.ContentBlockUnion) []claudetypes.ContentBlock {
	var result []claudetypes.ContentBlock
	for _, b := range blocks {
		switch b.Type {
		case "text":
			result = append(result, claudetypes.TextBlock{Text: b.Text})
		case "tool_use":
			result = append(result, claudetypes.ToolUseBlock{
				ID:    b.ID,
				Name:  b.Name,
				Input: parseJSONMap(b.Input),
			})
		case "thinking":
			result = append(result, claudetypes.ThinkingBlock{Thinking: b.Thinking})
		}
	}
	return result
}

func parseJSONMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return m
}

func convertUsageFromSDK(u anthropic.Usage) claudetypes.Usage {
	return claudetypes.Usage{
		InputTokens:              u.InputTokens,
		OutputTokens:             u.OutputTokens,
		CacheReadInputTokens:     u.CacheReadInputTokens,
		CacheCreationInputTokens: u.CacheCreationInputTokens,
	}
}

func convertDeltaUsageFromSDK(u anthropic.MessageDeltaUsage) claudetypes.Usage {
	return claudetypes.Usage{
		InputTokens:              u.InputTokens,
		OutputTokens:             u.OutputTokens,
		CacheReadInputTokens:     u.CacheReadInputTokens,
		CacheCreationInputTokens: u.CacheCreationInputTokens,
	}
}

// convertStreamEvent converts an Anthropic SDK stream event to our StreamEvent.
func (c *AnthropicClient) convertStreamEvent(evt anthropic.MessageStreamEventUnion) StreamEvent {
	switch evt.Type {
	case "message_start":
		return StreamEvent{
			Type: EventMessageStart,
			MessageStart: &MessageStartData{
				ID:    evt.Message.ID,
				Role:  "assistant",
				Model: string(evt.Message.Model),
				Usage: convertUsageFromSDK(evt.Message.Usage),
			},
		}

	case "content_block_start":
		data := &ContentBlockStartData{
			Index: int(evt.Index),
			Type:  evt.ContentBlock.Type,
		}
		if evt.ContentBlock.Type == "tool_use" {
			data.ToolUseID = evt.ContentBlock.ID
			data.ToolUseName = evt.ContentBlock.Name
		}
		if evt.ContentBlock.Type == "thinking" {
			data.Thinking = evt.ContentBlock.Thinking
		}
		return StreamEvent{
			Type:              EventContentStart,
			ContentBlockStart: data,
		}

	case "content_block_delta":
		data := &ContentBlockDeltaData{
			Index: int(evt.Index),
			Type:  evt.Delta.Type,
		}
		if evt.Delta.Type == "text_delta" {
			data.Text = evt.Delta.Text
		}
		if evt.Delta.Type == "input_json_delta" {
			data.JSON = evt.Delta.PartialJSON
		}
		if evt.Delta.Type == "thinking_delta" {
			data.Thinking = evt.Delta.Thinking
		}
		return StreamEvent{
			Type:              EventContentDelta,
			ContentBlockDelta: data,
		}

	case "content_block_stop":
		return StreamEvent{
			Type:             EventContentStop,
			ContentBlockStop: &ContentBlockStopData{Index: int(evt.Index)},
		}

	case "message_delta":
		data := &MessageDeltaData{
			StopReason: string(evt.Delta.StopReason),
			Usage:      convertDeltaUsageFromSDK(evt.Usage),
		}
		return StreamEvent{
			Type:         EventMessageDelta,
			MessageDelta: data,
			Usage:        &data.Usage,
		}

	case "message_stop":
		return StreamEvent{
			Type: EventMessageStop,
		}

	default:
		return StreamEvent{
			Type:  EventError,
			Error: fmt.Errorf("unknown stream event type: %s", evt.Type),
		}
	}
}

// ============================================================================
// Retry logic
// ============================================================================

// isRetryable returns true if the error is transient and the request should be retried.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	if apiErr, ok := errors.AsType[*anthropic.Error](err); ok {
		switch apiErr.StatusCode {
		case 429, 500, 502, 503, 529:
			return true
		}
	}

	return errors.Is(err, context.DeadlineExceeded)
}

// defaultMaxRetries is used when Config.MaxRetries is not set.
const defaultMaxRetries = 3

// WithRetry wraps a Client with retry logic on transient errors.
type WithRetry struct {
	inner         Client
	maxRetries    int
	fallbackModel string
}

// NewWithRetry wraps an inner client with retry behavior.
func NewWithRetry(inner Client, maxRetries int, fallbackModel string) *WithRetry {
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}
	return &WithRetry{
		inner:         inner,
		maxRetries:    maxRetries,
		fallbackModel: fallbackModel,
	}
}

// Send delegates to the inner client with retry logic.
func (r *WithRetry) Send(ctx context.Context, req Request) (*Response, error) {
	var lastErr error
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		if attempt > 0 {
			delay := backoffDelay(attempt)
			slog.DebugContext(ctx, "retrying LLM request",
				"attempt", attempt,
				"delay", delay,
				"error", lastErr,
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}

			// Try fallback model on last attempt
			if attempt == r.maxRetries && r.fallbackModel != "" {
				req.Model = r.fallbackModel
			}
		}

		resp, err := r.inner.Send(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		if !isRetryable(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("exhausted %d retries: %w", r.maxRetries, lastErr)
}

// Stream delegates to the inner client. Retries are handled at the request level.
func (r *WithRetry) Stream(ctx context.Context, req Request) (<-chan StreamEvent, error) {
	return r.inner.Stream(ctx, req)
}

// backoffDelay returns an exponential backoff delay.
func backoffDelay(attempt int) time.Duration {
	base := time.Duration(1<<min(attempt, 6)) * time.Second
	return base + time.Duration(attempt)*100*time.Millisecond
}
