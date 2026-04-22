package session

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/example/claude-code-go/pkg/claudetypes"
)

// ============================================================================
// JSONL entry types — each line in a .jsonl session file
// ============================================================================

// EntryType identifies the kind of JSONL entry.
type EntryType string

const (
	EntryUser       EntryType = "user"
	EntryAssistant  EntryType = "assistant"
	EntrySystem     EntryType = "system"
	EntryAttachment EntryType = "attachment"
	EntrySummary    EntryType = "summary"
)

// TranscriptEntry is the JSONL line format for transcript messages.
// This matches the TS SerializedMessage / TranscriptMessage shape
// so sessions are interoperable between the TS and Go binaries.
type TranscriptEntry struct {
	Type        EntryType `json:"type"`
	UUID        string    `json:"uuid"`
	ParentUUID  string    `json:"parentUuid"`
	Timestamp   string    `json:"timestamp"` // ISO 8601
	SessionID   string    `json:"sessionId"`
	CWD         string    `json:"cwd"`
	Version     string    `json:"version,omitempty"`
	IsSidechain bool      `json:"isSidechain,omitempty"`

	// Role for user/assistant messages
	Role string `json:"role,omitempty"`

	// Content for assistant messages (array of content blocks)
	Message []json.RawMessage `json:"message,omitempty"`

	// Content for user messages (array of content blocks or string)
	Content json.RawMessage `json:"content,omitempty"`

	// System message fields
	Subtype string `json:"subtype,omitempty"`
	Level   string `json:"level,omitempty"`
	// System content (single string)
	Data string `json:"data,omitempty"`

	// Usage for assistant messages
	Usage *UsageJSON `json:"usage,omitempty"`

	// Raw preserves any additional fields not explicitly handled.
	Raw map[string]json.RawMessage `json:"-"`
}

// UsageJSON is the JSON representation of token usage.
type UsageJSON struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

// ============================================================================
// Serialization: claudetypes.Message → TranscriptEntry
// ============================================================================

// MessageToEntry converts a claudetypes.Message into a JSONL-ready TranscriptEntry.
func MessageToEntry(msg claudetypes.Message, sessionID, cwd, parentUUID string) (TranscriptEntry, error) {
	entry := TranscriptEntry{
		UUID:       msg.GetUUID(),
		ParentUUID: parentUUID,
		Timestamp:  msg.GetTimestamp().UTC().Format(time.RFC3339Nano),
		SessionID:  sessionID,
		CWD:        cwd,
		Version:    "1.0",
	}

	switch m := msg.(type) {
	case claudetypes.UserMessage:
		entry.Type = EntryUser
		entry.Role = "user"
		content, err := marshalContentBlocks(m.Content)
		if err != nil {
			return entry, fmt.Errorf("marshal user content: %w", err)
		}
		entry.Content = content

	case claudetypes.AssistantMessage:
		entry.Type = EntryAssistant
		entry.Role = "assistant"
		entry.Message = marshalContentBlocksRaw(m.Message)
		entry.Usage = &UsageJSON{
			InputTokens:              m.Usage.InputTokens,
			OutputTokens:             m.Usage.OutputTokens,
			CacheReadInputTokens:     m.Usage.CacheReadInputTokens,
			CacheCreationInputTokens: m.Usage.CacheCreationInputTokens,
		}

	case claudetypes.SystemMessage:
		entry.Type = EntrySystem
		entry.Subtype = string(m.Subtype)
		entry.Level = string(m.Level)
		entry.Data = m.Content

	case claudetypes.AttachmentMessage:
		entry.Type = EntryAttachment

	default:
		entry.Type = EntrySystem
		entry.Subtype = "unknown"
	}

	return entry, nil
}

func marshalContentBlocks(blocks []claudetypes.ContentBlock) (json.RawMessage, error) {
	items := make([]json.RawMessage, 0, len(blocks))
	for _, b := range blocks {
		data, err := marshalContentBlock(b)
		if err != nil {
			return nil, err
		}
		items = append(items, data)
	}
	return json.Marshal(items)
}

func marshalContentBlock(b claudetypes.ContentBlock) (json.RawMessage, error) {
	switch block := b.(type) {
	case claudetypes.TextBlock:
		return json.Marshal(map[string]any{"type": "text", "text": block.Text})
	case claudetypes.ToolUseBlock:
		return json.Marshal(map[string]any{
			"type":  "tool_use",
			"id":    block.ID,
			"name":  block.Name,
			"input": block.Input,
		})
	case claudetypes.ToolResultBlock:
		content, _ := marshalContentBlocks(block.Content)
		return json.Marshal(map[string]any{
			"type":       "tool_result",
			"tool_use_id": block.ToolUseID,
			"content":    json.RawMessage(content),
			"is_error":   block.IsError,
		})
	case claudetypes.ThinkingBlock:
		return json.Marshal(map[string]any{"type": "thinking", "thinking": block.Thinking})
	default:
		return json.Marshal(map[string]any{"type": "unknown"})
	}
}

func marshalContentBlocksRaw(blocks []claudetypes.ContentBlock) []json.RawMessage {
	items := make([]json.RawMessage, 0, len(blocks))
	for _, b := range blocks {
		data, err := marshalContentBlock(b)
		if err != nil {
			continue
		}
		items = append(items, data)
	}
	return items
}

// ============================================================================
// Deserialization: TranscriptEntry → claudetypes.Message
// ============================================================================

// EntryToMessage converts a JSONL TranscriptEntry back into a claudetypes.Message.
func EntryToMessage(entry TranscriptEntry) (claudetypes.Message, error) {
	ts, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)

	base := claudetypes.MessageBase{
		UUID:      entry.UUID,
		Timestamp: ts,
	}

	switch entry.Type {
	case EntryUser:
		blocks, err := parseContentBlocks(entry.Content)
		if err != nil {
			return nil, fmt.Errorf("parse user content: %w", err)
		}
		return claudetypes.UserMessage{
			MessageBase: base,
			Content:     blocks,
		}, nil

	case EntryAssistant:
		blocks := parseContentBlocksRaw(entry.Message)
		var usage claudetypes.Usage
		if entry.Usage != nil {
			usage = claudetypes.Usage{
				InputTokens:              entry.Usage.InputTokens,
				OutputTokens:             entry.Usage.OutputTokens,
				CacheReadInputTokens:     entry.Usage.CacheReadInputTokens,
				CacheCreationInputTokens: entry.Usage.CacheCreationInputTokens,
			}
		}
		return claudetypes.AssistantMessage{
			MessageBase: base,
			Message:     blocks,
			Usage:       usage,
		}, nil

	case EntrySystem:
		return claudetypes.SystemMessage{
			MessageBase: base,
			Subtype:     claudetypes.SystemMessageSubtype(entry.Subtype),
			Level:       claudetypes.SystemMessageLevel(entry.Level),
			Content:     entry.Data,
		}, nil

	case EntryAttachment:
		return claudetypes.AttachmentMessage{
			MessageBase: base,
		}, nil

	default:
		return claudetypes.SystemMessage{
			MessageBase: base,
			Subtype:     "unknown",
			Content:     entry.Data,
		}, nil
	}
}

func parseContentBlocks(raw json.RawMessage) ([]claudetypes.ContentBlock, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		// Single string content (legacy format)
		var s string
		if err2 := json.Unmarshal(raw, &s); err2 == nil {
			return []claudetypes.ContentBlock{claudetypes.TextBlock{Text: s}}, nil
		}
		return nil, err
	}

	var blocks []claudetypes.ContentBlock
	for _, item := range items {
		block, err := parseContentBlock(item)
		if err != nil {
			continue
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

func parseContentBlock(raw json.RawMessage) (claudetypes.ContentBlock, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return claudetypes.TextBlock{Text: string(raw)}, nil
	}

	typeVal, _ := unquoteJSON(m["type"])

	switch typeVal {
	case "text":
		var text string
		json.Unmarshal(m["text"], &text)
		return claudetypes.TextBlock{Text: text}, nil

	case "tool_use":
		var id, name string
		json.Unmarshal(m["id"], &id)
		json.Unmarshal(m["name"], &name)
		var input map[string]any
		json.Unmarshal(m["input"], &input)
		return claudetypes.ToolUseBlock{ID: id, Name: name, Input: input}, nil

	case "tool_result":
		var toolUseID string
		json.Unmarshal(m["tool_use_id"], &toolUseID)
		var isError bool
		json.Unmarshal(m["is_error"], &isError)
		blocks, _ := parseContentBlocks(m["content"])
		return claudetypes.ToolResultBlock{
			ToolUseID: toolUseID,
			Content:   blocks,
			IsError:   isError,
		}, nil

	case "thinking":
		var thinking string
		json.Unmarshal(m["thinking"], &thinking)
		return claudetypes.ThinkingBlock{Thinking: thinking}, nil

	default:
		return claudetypes.TextBlock{Text: string(raw)}, nil
	}
}

func parseContentBlocksRaw(items []json.RawMessage) []claudetypes.ContentBlock {
	var blocks []claudetypes.ContentBlock
	for _, item := range items {
		block, err := parseContentBlock(item)
		if err != nil {
			continue
		}
		blocks = append(blocks, block)
	}
	return blocks
}

func unquoteJSON(raw json.RawMessage) (string, error) {
	var s string
	err := json.Unmarshal(raw, &s)
	return s, err
}
