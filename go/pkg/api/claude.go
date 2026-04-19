package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
)

type Options struct {
	APIKey               string
	BaseURL              string
	MaxRetries           int
	RequestTimeout       time.Duration
	UserAgent            string
	SessionID            string
	RemoteContainerID    string
	RemoteSessionID      string
	ClientApp            string
	AdditionalProtection bool
	Headers              map[string]string
	HTTPClient           option.HTTPClient
}

type MessageRequest struct {
	Model           anthropic.Model
	MaxTokens       int64
	Messages        []anthropic.MessageParam
	System          []anthropic.TextBlockParam
	Tools           []anthropic.ToolUnionParam
	ToolChoice      anthropic.ToolChoiceUnionParam
	Thinking        anthropic.ThinkingConfigParamUnion
	MetadataUserID  string
	Temperature     *float64
	ClientRequestID string
}

type ClaudeClient struct {
	client anthropic.Client
}

type MessageStream struct {
	stream  *ssestream.Stream[anthropic.MessageStreamEventUnion]
	current anthropic.MessageStreamEventUnion
	final   anthropic.Message
	err     error
}

func NewClaudeClient(opts Options) ClaudeClient {
	return ClaudeClient{
		client: anthropic.NewClient(clientOptions(opts)...),
	}
}

func (c ClaudeClient) CreateMessage(ctx context.Context, req MessageRequest) (anthropic.Message, error) {
	message, err := c.client.Messages.New(ctx, req.params(), req.requestOptions()...)
	if err != nil {
		return anthropic.Message{}, err
	}
	if message == nil {
		return anthropic.Message{}, nil
	}
	return *message, nil
}

func (c ClaudeClient) StreamMessage(ctx context.Context, req MessageRequest) (*MessageStream, error) {
	stream := c.client.Messages.NewStreaming(ctx, req.params(), req.requestOptions()...)
	return &MessageStream{stream: stream}, nil
}

func (s *MessageStream) Next() bool {
	if s.err != nil || s.stream == nil {
		return false
	}
	if !s.stream.Next() {
		return false
	}

	s.current = s.stream.Current()
	if err := (&s.final).Accumulate(s.current); err != nil {
		s.err = fmt.Errorf("accumulate message stream event: %w", err)
		if closeErr := s.stream.Close(); closeErr != nil {
			s.err = errors.Join(s.err, closeErr)
		}
		return false
	}

	return true
}

func (s *MessageStream) Current() anthropic.MessageStreamEventUnion {
	return s.current
}

func (s *MessageStream) Err() error {
	if s.err != nil {
		return s.err
	}
	if s.stream == nil {
		return nil
	}
	return s.stream.Err()
}

func (s *MessageStream) FinalMessage() anthropic.Message {
	return s.final
}

func (s *MessageStream) Close() error {
	if s.stream == nil {
		return nil
	}
	return s.stream.Close()
}

func clientOptions(opts Options) []option.RequestOption {
	requestOptions := []option.RequestOption{}
	if opts.APIKey != "" {
		requestOptions = append(requestOptions, option.WithAPIKey(opts.APIKey))
	}
	if opts.BaseURL != "" {
		requestOptions = append(requestOptions, option.WithBaseURL(opts.BaseURL))
	}
	requestOptions = append(requestOptions, option.WithMaxRetries(opts.MaxRetries))
	if opts.RequestTimeout > 0 {
		requestOptions = append(requestOptions, option.WithRequestTimeout(opts.RequestTimeout))
	}
	if opts.HTTPClient != nil {
		requestOptions = append(requestOptions, option.WithHTTPClient(opts.HTTPClient))
	}

	requestOptions = append(requestOptions, option.WithHeader("x-app", "cli"))
	if opts.UserAgent != "" {
		requestOptions = append(requestOptions, option.WithHeader("User-Agent", opts.UserAgent))
	}
	if opts.SessionID != "" {
		requestOptions = append(requestOptions, option.WithHeader("X-Claude-Code-Session-Id", opts.SessionID))
	}
	for key, value := range opts.Headers {
		requestOptions = append(requestOptions, option.WithHeader(key, value))
	}
	if opts.RemoteContainerID != "" {
		requestOptions = append(requestOptions, option.WithHeader("x-claude-remote-container-id", opts.RemoteContainerID))
	}
	if opts.RemoteSessionID != "" {
		requestOptions = append(requestOptions, option.WithHeader("x-claude-remote-session-id", opts.RemoteSessionID))
	}
	if opts.ClientApp != "" {
		requestOptions = append(requestOptions, option.WithHeader("x-client-app", opts.ClientApp))
	}
	if opts.AdditionalProtection {
		requestOptions = append(requestOptions, option.WithHeader("x-anthropic-additional-protection", "true"))
	}

	return requestOptions
}

func (r MessageRequest) params() anthropic.MessageNewParams {
	params := anthropic.MessageNewParams{
		Model:     r.Model,
		MaxTokens: r.MaxTokens,
		Messages:  r.Messages,
	}
	if len(r.System) > 0 {
		params.System = r.System
	}
	if len(r.Tools) > 0 {
		params.Tools = r.Tools
	}
	if hasToolChoice(r.ToolChoice) {
		params.ToolChoice = r.ToolChoice
	}
	thinkingEnabled := hasEnabledThinking(r.Thinking)
	if thinkingEnabled {
		params.Thinking = r.Thinking
	}
	if r.MetadataUserID != "" {
		params.Metadata = anthropic.MetadataParam{UserID: anthropic.String(r.MetadataUserID)}
	}
	if r.Temperature != nil && !thinkingEnabled {
		params.Temperature = anthropic.Float(*r.Temperature)
	}
	return params
}

func hasToolChoice(choice anthropic.ToolChoiceUnionParam) bool {
	return choice.OfAuto != nil || choice.OfAny != nil || choice.OfTool != nil || choice.OfNone != nil
}

func hasEnabledThinking(thinking anthropic.ThinkingConfigParamUnion) bool {
	return thinking.OfAdaptive != nil || thinking.OfEnabled != nil
}

func (r MessageRequest) requestOptions() []option.RequestOption {
	if r.ClientRequestID == "" {
		return nil
	}
	return []option.RequestOption{option.WithHeader("x-client-request-id", r.ClientRequestID)}
}
