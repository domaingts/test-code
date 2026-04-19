package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

type closureTransport struct {
	fn func(req *http.Request) (*http.Response, error)
}

func (t *closureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.fn(req)
}

func TestMessageRequest_params_mapsFields(t *testing.T) {
	temperature := 0.4
	req := MessageRequest{
		Model:     anthropic.ModelClaudeSonnet4_6,
		MaxTokens: 512,
		Messages: []anthropic.MessageParam{{
			Role: anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{{
				OfText: &anthropic.TextBlockParam{Text: "hello"},
			}},
		}},
		System: []anthropic.TextBlockParam{{Text: "system"}},
		Tools: []anthropic.ToolUnionParam{{
			OfTool: &anthropic.ToolParam{
				Name: "lookup",
				InputSchema: anthropic.ToolInputSchemaParam{
					Type:       "object",
					Properties: map[string]any{"query": map[string]any{"type": "string"}},
					Required:   []string{"query"},
				},
			},
		}},
		ToolChoice: anthropic.ToolChoiceUnionParam{
			OfTool: &anthropic.ToolChoiceToolParam{Name: "lookup"},
		},
		MetadataUserID: "session-json-string",
		Temperature:    &temperature,
	}

	params := req.params()
	if params.Model != req.Model {
		t.Fatalf("Model = %q, want %q", params.Model, req.Model)
	}
	if params.MaxTokens != req.MaxTokens {
		t.Fatalf("MaxTokens = %d, want %d", params.MaxTokens, req.MaxTokens)
	}
	if len(params.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(params.Messages))
	}
	if len(params.System) != 1 || params.System[0].Text != "system" {
		t.Fatalf("System = %#v, want single system block", params.System)
	}
	if len(params.Tools) != 1 || params.Tools[0].OfTool == nil || params.Tools[0].OfTool.Name != "lookup" {
		t.Fatalf("Tools = %#v, want tool named lookup", params.Tools)
	}
	if name := params.ToolChoice.GetName(); name == nil || *name != "lookup" {
		t.Fatalf("ToolChoice name = %v, want lookup", name)
	}
	if !params.Metadata.UserID.Valid() || params.Metadata.UserID.Value != "session-json-string" {
		t.Fatalf("Metadata.UserID = %#v, want session-json-string", params.Metadata.UserID)
	}
	if !params.Temperature.Valid() || params.Temperature.Value != temperature {
		t.Fatalf("Temperature = %#v, want %v", params.Temperature, temperature)
	}
}

func TestMessageRequest_params_omitsTemperatureWhenThinkingEnabled(t *testing.T) {
	temperature := 0.4
	req := MessageRequest{
		Model:       anthropic.ModelClaudeSonnet4_6,
		MaxTokens:   1024,
		Temperature: &temperature,
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{
				Display: anthropic.ThinkingConfigAdaptiveDisplaySummarized,
			},
		},
	}

	params := req.params()
	if params.Thinking.OfAdaptive == nil {
		t.Fatalf("Thinking = %#v, want adaptive thinking", params.Thinking)
	}
	if params.Temperature.Valid() {
		t.Fatalf("Temperature should be omitted when thinking is enabled, got %#v", params.Temperature)
	}
}

func TestMessageRequest_requestOptions_addsClientRequestID(t *testing.T) {
	req := MessageRequest{ClientRequestID: "req-123"}
	options := req.requestOptions()
	if len(options) != 1 {
		t.Fatalf("len(options) = %d, want 1", len(options))
	}
}

func TestNewClaudeClient_appliesDefaultHeaders(t *testing.T) {
	var capturedHeaders http.Header
	client := NewClaudeClient(Options{
		APIKey:               "test-key",
		UserAgent:            "claude-code-go-test",
		SessionID:            "session-123",
		RemoteContainerID:    "container-123",
		RemoteSessionID:      "remote-session-123",
		ClientApp:            "sdk-test",
		AdditionalProtection: true,
		Headers: map[string]string{
			"Authorization": "Bearer custom-token",
			"x-custom":      "custom-value",
		},
		HTTPClient: &http.Client{
			Transport: &closureTransport{
				fn: func(req *http.Request) (*http.Response, error) {
					capturedHeaders = req.Header.Clone()
					return jsonResponse(map[string]any{
						"id":            "msg_test",
						"type":          "message",
						"role":          "assistant",
						"content":       []map[string]any{{"type": "text", "text": "hi"}},
						"model":         "claude-sonnet-4-6",
						"stop_reason":   "end_turn",
						"stop_sequence": nil,
						"usage":         map[string]any{"input_tokens": 1, "output_tokens": 1},
					})
				},
			},
		},
	})

	_, err := client.CreateMessage(context.Background(), MessageRequest{
		Model:     anthropic.ModelClaudeSonnet4_6,
		MaxTokens: 16,
		Messages: []anthropic.MessageParam{{
			Role: anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{{
				OfText: &anthropic.TextBlockParam{Text: "hello"},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("CreateMessage() error = %v", err)
	}

	tests := map[string]string{
		"x-app":                             "cli",
		"User-Agent":                        "claude-code-go-test",
		"X-Claude-Code-Session-Id":          "session-123",
		"x-claude-remote-container-id":      "container-123",
		"x-claude-remote-session-id":        "remote-session-123",
		"x-client-app":                      "sdk-test",
		"x-anthropic-additional-protection": "true",
		"x-custom":                          "custom-value",
		"Authorization":                     "Bearer custom-token",
	}
	for key, want := range tests {
		if got := capturedHeaders.Get(key); got != want {
			t.Fatalf("header %q = %q, want %q", key, got, want)
		}
	}
}

func TestStreamMessage_accumulatesFinalMessageAndClientRequestID(t *testing.T) {
	var capturedHeaders http.Header
	client := NewClaudeClient(Options{
		APIKey: "test-key",
		HTTPClient: &http.Client{
			Transport: &closureTransport{
				fn: func(req *http.Request) (*http.Response, error) {
					capturedHeaders = req.Header.Clone()
					return sseResponse(stringsJoin(
						sseEvent("message_start", `{"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-6","stop_reason":"end_turn","stop_sequence":"","usage":{"input_tokens":1,"output_tokens":0}}}`),
						sseEvent("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":"Hello"}}`),
						sseEvent("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}`),
						sseEvent("content_block_stop", `{"type":"content_block_stop","index":0}`),
						sseEvent("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":""},"usage":{"output_tokens":2}}`),
						sseEvent("message_stop", `{"type":"message_stop"}`),
					)), nil
				},
			},
		},
	})

	stream, err := client.StreamMessage(context.Background(), MessageRequest{
		Model:           anthropic.ModelClaudeSonnet4_6,
		MaxTokens:       16,
		ClientRequestID: "req-123",
		Messages: []anthropic.MessageParam{{
			Role: anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{{
				OfText: &anthropic.TextBlockParam{Text: "hello"},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("StreamMessage() error = %v", err)
	}
	defer stream.Close()

	count := 0
	for stream.Next() {
		count++
	}
	if stream.Err() != nil {
		t.Fatalf("stream.Err() = %v", stream.Err())
	}
	if count != 6 {
		t.Fatalf("stream event count = %d, want 6", count)
	}
	if got := capturedHeaders.Get("x-client-request-id"); got != "req-123" {
		t.Fatalf("x-client-request-id = %q, want req-123", got)
	}

	final := stream.FinalMessage()
	if final.ID != "msg_test" {
		t.Fatalf("final.ID = %q, want msg_test", final.ID)
	}
	if len(final.Content) != 1 || final.Content[0].Text != "Hello world" {
		t.Fatalf("final.Content = %#v, want single text block", final.Content)
	}
	if final.StopReason != "end_turn" {
		t.Fatalf("final.StopReason = %q, want end_turn", final.StopReason)
	}
	if final.Usage.OutputTokens != 2 {
		t.Fatalf("final.Usage.OutputTokens = %d, want 2", final.Usage.OutputTokens)
	}
}

func jsonResponse(body map[string]any) (*http.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewReader(payload)),
	}, nil
}

func sseResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(bytes.NewReader([]byte(body))),
	}
}

func sseEvent(name string, data string) string {
	return "event: " + name + "\n" + "data: " + data + "\n\n"
}

func stringsJoin(parts ...string) string {
	return strings.Join(parts, "")
}

func TestNewClaudeClient_requestTimeoutOptionUsesHTTPClient(t *testing.T) {
	client := NewClaudeClient(Options{
		APIKey:         "test-key",
		RequestTimeout: 50 * time.Millisecond,
		HTTPClient: &http.Client{
			Transport: &closureTransport{
				fn: func(req *http.Request) (*http.Response, error) {
					<-req.Context().Done()
					return nil, req.Context().Err()
				},
			},
		},
	})

	_, err := client.CreateMessage(context.Background(), MessageRequest{
		Model:     anthropic.ModelClaudeSonnet4_6,
		MaxTokens: 16,
		Messages: []anthropic.MessageParam{{
			Role: anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{{
				OfText: &anthropic.TextBlockParam{Text: "hello"},
			}},
		}},
	})
	if err == nil {
		t.Fatal("CreateMessage() error = nil, want timeout error")
	}
}
