// Package claudetypes defines the shared data types used across all modules.
//
// These are Go equivalents of the TypeScript types in
// ClaudeCode/src/types/message.ts, src/types/permissions.ts, and
// src/entrypoints/agentSdkTypes.ts.  M1 contains no logic — only type
// definitions, constants, and constructor helpers.
package claudetypes

import "time"

// ============================================================================
// Message — conversation transcript entries
// ============================================================================

// Message is the sealed interface for all transcript messages.
// Every message variant implements this interface via the unexported isMessage
// method, preventing external implementations.
type Message interface {
	isMessage()
	// GetUUID returns the unique identifier for this message.
	GetUUID() string
	// GetTimestamp returns when this message was created.
	GetTimestamp() time.Time
	// GetIsMeta reports whether this message is metadata-only (not shown to user).
	GetIsMeta() bool
}

// MessageBase contains fields shared by all message types.
type MessageBase struct {
	UUID      string
	Timestamp time.Time
	IsMeta    bool
	IsVirtual bool
}

func (MessageBase) isMessage() {}

func (m MessageBase) GetUUID() string     { return m.UUID }
func (m MessageBase) GetTimestamp() time.Time { return m.Timestamp }
func (m MessageBase) GetIsMeta() bool     { return m.IsMeta }

// MessageOrigin identifies how a user message was produced.
type MessageOrigin string

const (
	OriginKeyboard       MessageOrigin = "keyboard"
	OriginSendUser       MessageOrigin = "SendUserMessage"
	OriginQueue          MessageOrigin = "queue"
	OriginHook           MessageOrigin = "hook"
	OriginAgent          MessageOrigin = "agent"
)

// ============================================================================
// Content blocks — used by UserMessage and AssistantMessage
// ============================================================================

// ContentBlock is a sealed interface for Anthropic content blocks.
type ContentBlock interface {
	isContentBlock()
}

// TextBlock represents plain text content.
type TextBlock struct {
	Text string
}

func (TextBlock) isContentBlock() {}

// ToolUseBlock represents a request from the assistant to use a tool.
type ToolUseBlock struct {
	ID    string
	Name  string
	Input map[string]any
}

func (ToolUseBlock) isContentBlock() {}

// ToolResultBlock represents the result of a tool invocation.
type ToolResultBlock struct {
	ToolUseID string
	Content   []ContentBlock
	IsError   bool
}

func (ToolResultBlock) isContentBlock() {}

// ThinkingBlock represents extended thinking content.
type ThinkingBlock struct {
	Thinking string
}

func (ThinkingBlock) isContentBlock() {}

// ============================================================================
// Usage — token accounting
// ============================================================================

// Usage tracks token counts for an API call.
type Usage struct {
	InputTokens              int64
	OutputTokens             int64
	CacheReadInputTokens     int64
	CacheCreationInputTokens int64
}

// IsZero reports whether no tokens were consumed.
func (u Usage) IsZero() bool {
	return u.InputTokens == 0 && u.OutputTokens == 0 &&
		u.CacheReadInputTokens == 0 && u.CacheCreationInputTokens == 0
}

// TotalInput returns the total input-side tokens (direct + cache creation).
func (u Usage) TotalInput() int64 {
	return u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
}

// ============================================================================
// AssistantMessage
// ============================================================================

// AssistantMessage represents a response from the model.
type AssistantMessage struct {
	MessageBase
	Message           []ContentBlock // parsed content blocks
	RequestID         string
	Usage             Usage
	IsAPIErrorMessage bool
	APIError          error
	Error             error
	ErrorDetails      string
	AdvisorModel      string
	CostUSD           float64
}

// ============================================================================
// UserMessage
// ============================================================================

// UserMessage represents input from the user or a tool result.
type UserMessage struct {
	MessageBase
	Content               []ContentBlock
	IsVisibleInTranscript bool
	IsCompactSummary      bool
	ToolUseResult         any
	MCPMeta               any
	ImagePasteIDs         []int
	SourceToolAssistantUUID string
	PermissionMode        string
	Origin                MessageOrigin
	SummarizeMetadata     *CompactMetadata
}

// CompactMetadata stores information about a compaction operation.
type CompactMetadata struct {
	MessagesSummarized        int
	UserContext               string
	Direction                 string // "forward" or "backward"
	Trigger                   string
	PreTokens                 int
	PreCompactDiscoveredTools []string
}

// ============================================================================
// SystemMessage variants
// ============================================================================

// SystemMessageLevel classifies the severity of a system message.
type SystemMessageLevel string

const (
	SystemLevelInfo       SystemMessageLevel = "info"
	SystemLevelWarning    SystemMessageLevel = "warning"
	SystemLevelSuggestion SystemMessageLevel = "suggestion"
)

// SystemMessageSubtype identifies the specific system message variant.
type SystemMessageSubtype string

const (
	SubtypeInformational          SystemMessageSubtype = "informational"
	SubtypeAPIError               SystemMessageSubtype = "api_error"
	SubtypeLocalCommand           SystemMessageSubtype = "local_command"
	SubtypePermissionRetry        SystemMessageSubtype = "permission_retry"
	SubtypeBridgeStatus           SystemMessageSubtype = "bridge_status"
	SubtypeScheduledTaskFire      SystemMessageSubtype = "scheduled_task_fire"
	SubtypeStopHookSummary        SystemMessageSubtype = "stop_hook_summary"
	SubtypeTurnDuration           SystemMessageSubtype = "turn_duration"
	SubtypeAwaySummary            SystemMessageSubtype = "away_summary"
	SubtypeMemorySaved            SystemMessageSubtype = "memory_saved"
	SubtypeCompactBoundary        SystemMessageSubtype = "compact_boundary"
	SubtypeMicrocompactBoundary   SystemMessageSubtype = "microcompact_boundary"
	SubtypeAgentsKilled           SystemMessageSubtype = "agents_killed"
	SubtypeAPIMetrics             SystemMessageSubtype = "api_metrics"
	SubtypeFileSnapshot           SystemMessageSubtype = "file_snapshot"
	SubtypeThinking               SystemMessageSubtype = "thinking"
)

// SystemMessage represents a system-level message in the transcript.
// The Subtype field identifies the specific variant; use the type assertion
// helpers (e.g. AsInformational) to access variant-specific fields.
type SystemMessage struct {
	MessageBase
	Subtype SystemMessageSubtype
	Content string
	Level   SystemMessageLevel

	// Fields used by specific subtypes (zero-value-safe).
	ToolUseID             string
	PreventContinuation   bool
	Commands              []string
	URL                   string
	UpgradeNudge          string
	HookCount             int
	HookInfos             []StopHookInfo
	HookErrors            []string
	StopReason            string
	HasOutput             bool
	HookLabel             string
	TotalDurationMs       int64
	DurationMs            int64
	BudgetTokens          int64
	BudgetLimit           int64
	BudgetNudges          int64
	MessageCount          int
	WrittenPaths          []string
	CompactMeta           *CompactMetadata
	APIMetrics            *APIMetrics
}

// StopHookInfo describes the result of a single stop hook execution.
type StopHookInfo struct {
	HookName              string
	Output                string
	Error                 string
	DurationMs            int64
	PreventedContinuation bool
}

// APIMetrics contains per-turn performance metrics.
type APIMetrics struct {
	TTFTMs                 int64
	OTPS                   float64
	IsP50                  bool
	HookDurationMs         int64
	TurnDurationMs         int64
	ToolDurationMs         int64
	ClassifierDurationMs   int64
	ToolCount              int
	HookCount              int
	ClassifierCount        int
	ConfigWriteCount       int
}

// ============================================================================
// Other message types
// ============================================================================

// AttachmentMessage carries a file or data attachment.
type AttachmentMessage struct {
	MessageBase
	AttachmentType string
	Attachment     map[string]any
}

// ProgressMessage reports incremental progress of a long-running tool.
type ProgressMessage struct {
	MessageBase
	Data           any
	ToolUseID      string
	ParentToolUseID string
}

// TombstoneMessage marks a deleted message in the transcript.
type TombstoneMessage struct {
	MessageBase
}

// ToolUseSummaryMessage summarizes a tool invocation for display.
type ToolUseSummaryMessage struct {
	MessageBase
}

// StreamEventMessage wraps a raw streaming event from the API.
type StreamEventMessage struct {
	MessageBase
	Event any
}

// RequestStartEventMessage marks the beginning of an API request.
type RequestStartEventMessage struct {
	MessageBase
}

// HookResultMessage carries the output of a hook execution.
type HookResultMessage struct {
	MessageBase
	Attachment any
}
