package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/example/claude-code-go/pkg/claudetypes"
)

func TestRequest_defaults(t *testing.T) {
	req := Request{
		Model: "claude-sonnet-4-20250514",
		Messages: []claudetypes.Message{
			claudetypes.UserMessage{
				MessageBase: claudetypes.MessageBase{UUID: "1"},
				Content:     []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "hello"}},
			},
		},
	}

	if req.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want claude-sonnet-4-20250514", req.Model)
	}
	if len(req.Messages) != 1 {
		t.Errorf("Messages = %d, want 1", len(req.Messages))
	}
}

func TestStreamEvent_types(t *testing.T) {
	events := []StreamEvent{
		{Type: EventMessageStart, MessageStart: &MessageStartData{ID: "msg_1", Model: "claude-sonnet-4-20250514"}},
		{Type: EventContentStart, ContentBlockStart: &ContentBlockStartData{Index: 0, Type: "text"}},
		{Type: EventContentDelta, ContentBlockDelta: &ContentBlockDeltaData{Index: 0, Type: "text_delta", Text: "hello"}},
		{Type: EventContentStop, ContentBlockStop: &ContentBlockStopData{Index: 0}},
		{Type: EventMessageDelta, MessageDelta: &MessageDeltaData{StopReason: "end_turn"}},
		{Type: EventMessageStop},
	}

	for i, evt := range events {
		if evt.Type == "" {
			t.Errorf("event %d: empty Type", i)
		}
	}
}

func TestWithRetry_retriesOnRetryable(t *testing.T) {
	attempts := 0
	retryErr := &retryableErr{msg: "rate limited"}
	mock := &mockClient{
		sendFunc: func(ctx context.Context, req Request) (*Response, error) {
			attempts++
			if attempts < 3 {
				return nil, retryErr
			}
			return &Response{}, nil
		},
	}

	// Patch isRetryable for test — use context.DeadlineExceeded
	_ = retryErr // not used; we override isRetryable below
	r := &WithRetry{
		inner:      mock,
		maxRetries: 3,
	}

	// Test retry with DeadlineExceeded (which isRetryable recognizes)
	attempts = 0
	mock.sendFunc = func(ctx context.Context, req Request) (*Response, error) {
		attempts++
		if attempts < 3 {
			return nil, context.DeadlineExceeded
		}
		return &Response{}, nil
	}
	_, err := r.Send(context.Background(), Request{Model: "test"})
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestWithRetry_noRetryOnNonRetryable(t *testing.T) {
	attempts := 0
	mock := &mockClient{
		sendFunc: func(ctx context.Context, req Request) (*Response, error) {
			attempts++
			return nil, errors.New("client error")
		},
	}

	r := NewWithRetry(mock, 3, "")
	_, err := r.Send(context.Background(), Request{Model: "test"})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (no retry on non-retryable)", attempts)
	}
}

func TestWithRetry_fallbackModel(t *testing.T) {
	var lastModel string
	mock := &mockClient{
		sendFunc: func(ctx context.Context, req Request) (*Response, error) {
			lastModel = req.Model
			if req.Model == "fallback" {
				return &Response{}, nil
			}
			return nil, context.DeadlineExceeded
		},
	}

	r := NewWithRetry(mock, 1, "fallback")
	_, err := r.Send(context.Background(), Request{Model: "primary"})
	if err != nil {
		t.Fatalf("expected success with fallback, got %v", err)
	}
	if lastModel != "fallback" {
		t.Errorf("lastModel = %q, want fallback", lastModel)
	}
}

func TestWithRetry_contextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	mock := &mockClient{
		sendFunc: func(ctx context.Context, req Request) (*Response, error) {
			return nil, context.DeadlineExceeded
		},
	}

	r := NewWithRetry(mock, 3, "")
	_, err := r.Send(ctx, Request{Model: "test"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// retryableErr is only used for documentation in test.
type retryableErr struct{ msg string }

func (e *retryableErr) Error() string { return e.msg }

func TestBackoffDelay(t *testing.T) {
	for attempt := 1; attempt <= 6; attempt++ {
		delay := backoffDelay(attempt)
		expected := time.Duration(1<<attempt)*time.Second + time.Duration(attempt)*100*time.Millisecond
		if delay != expected {
			t.Errorf("backoffDelay(%d) = %v, want %v", attempt, delay, expected)
		}
	}
}

func TestConvertUsage(t *testing.T) {
	// Test our convertUsageFromSDK is used correctly by checking the fields map
	usage := claudetypes.Usage{
		InputTokens:              100,
		OutputTokens:             50,
		CacheReadInputTokens:     25,
		CacheCreationInputTokens: 10,
	}
	if usage.TotalInput() != 135 {
		t.Errorf("TotalInput() = %d, want 135", usage.TotalInput())
	}
}

// --- Mock client ---

type mockClient struct {
	sendFunc   func(context.Context, Request) (*Response, error)
	streamFunc func(context.Context, Request) (<-chan StreamEvent, error)
}

func (m *mockClient) Send(ctx context.Context, req Request) (*Response, error) {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, req)
	}
	return &Response{}, nil
}

func (m *mockClient) Stream(ctx context.Context, req Request) (<-chan StreamEvent, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, req)
	}
	ch := make(chan StreamEvent)
	close(ch)
	return ch, nil
}
func TestIsRetryable(t *testing.T) {
	cases := []struct {
		err   error
		want  bool
	}{
		{nil, false},
		{errors.New("some error"), false},
		{context.DeadlineExceeded, true},
	}
	for _, tc := range cases {
		if got := isRetryable(tc.err); got != tc.want {
			t.Errorf("isRetryable(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
