package claudetypes

// ============================================================================
// SDK Events — output from the query engine
// ============================================================================

// SDKEvent is the sealed interface for events emitted by the query engine.
// Consumers use type switches to handle specific event kinds.
type SDKEvent interface {
	isSDKEvent()
	// Kind returns the event type identifier.
	Kind() string
}

// EventKind constants identify SDK event types.
const (
	EventMessage   = "message"
	EventResult    = "result"
	EventError     = "error"
	EventStream    = "stream"
	EventRequest   = "request"
)

// SDKMessageEvent wraps a transcript message as an SDK event.
type SDKMessageEvent struct {
	Message Message
}

func (SDKMessageEvent) isSDKEvent()  {}
func (SDKMessageEvent) Kind() string { return EventMessage }

// SDKResultEvent carries the final result of a query.
type SDKResultEvent struct {
	Result    Message // the final assistant message
	Usage     Usage
	NumTurns  int
	IsError   bool
}

func (SDKResultEvent) isSDKEvent()  {}
func (SDKResultEvent) Kind() string { return EventResult }

// SDKErrorEvent reports a non-recoverable error.
type SDKErrorEvent struct {
	Error   error
	Message string
}

func (SDKErrorEvent) isSDKEvent()  {}
func (SDKErrorEvent) Kind() string { return EventError }

// ============================================================================
// Hook events — lifecycle hooks registered by SDK consumers
// ============================================================================

// HookEvent identifies which hook is firing.
type HookEvent string

const (
	HookPreToolUse        HookEvent = "PreToolUse"
	HookPostToolUse       HookEvent = "PostToolUse"
	HookPostToolUseFailure HookEvent = "PostToolUseFailure"
	HookNotification      HookEvent = "Notification"
	HookUserPromptSubmit  HookEvent = "UserPromptSubmit"
	HookSessionStart      HookEvent = "SessionStart"
	HookSessionEnd        HookEvent = "SessionEnd"
	HookStop              HookEvent = "Stop"
	HookStopFailure       HookEvent = "StopFailure"
	HookSubagentStart     HookEvent = "SubagentStart"
	HookSubagentStop      HookEvent = "SubagentStop"
	HookPreCompact        HookEvent = "PreCompact"
	HookPostCompact       HookEvent = "PostCompact"
	HookPermissionRequest HookEvent = "PermissionRequest"
	HookPermissionDenied  HookEvent = "PermissionDenied"
	HookSetup             HookEvent = "Setup"
	HookTeammateIdle      HookEvent = "TeammateIdle"
	HookTaskCreated       HookEvent = "TaskCreated"
	HookTaskCompleted     HookEvent = "TaskCompleted"
	HookElicitation       HookEvent = "Elicitation"
	HookElicitationResult HookEvent = "ElicitationResult"
	HookConfigChange      HookEvent = "ConfigChange"
	HookWorktreeCreate    HookEvent = "WorktreeCreate"
	HookWorktreeRemove    HookEvent = "WorktreeRemove"
	HookInstructionsLoaded HookEvent = "InstructionsLoaded"
	HookCwdChanged        HookEvent = "CwdChanged"
	HookFileChanged       HookEvent = "FileChanged"
)

// AllHookEvents lists every hook event in declaration order.
var AllHookEvents = []HookEvent{
	HookPreToolUse, HookPostToolUse, HookPostToolUseFailure,
	HookNotification, HookUserPromptSubmit, HookSessionStart,
	HookSessionEnd, HookStop, HookStopFailure, HookSubagentStart,
	HookSubagentStop, HookPreCompact, HookPostCompact,
	HookPermissionRequest, HookPermissionDenied, HookSetup,
	HookTeammateIdle, HookTaskCreated, HookTaskCompleted,
	HookElicitation, HookElicitationResult, HookConfigChange,
	HookWorktreeCreate, HookWorktreeRemove, HookInstructionsLoaded,
	HookCwdChanged, HookFileChanged,
}

// ============================================================================
// Exit reasons — why a session ended
// ============================================================================

// ExitReason identifies why the CLI session exited.
type ExitReason string

const (
	ExitClear                    ExitReason = "clear"
	ExitResume                   ExitReason = "resume"
	ExitLogout                   ExitReason = "logout"
	ExitPromptInputExit          ExitReason = "prompt_input_exit"
	ExitOther                    ExitReason = "other"
	ExitBypassPermissionsDisabled ExitReason = "bypass_permissions_disabled"
)

// AllExitReasons lists every exit reason.
var AllExitReasons = []ExitReason{
	ExitClear, ExitResume, ExitLogout,
	ExitPromptInputExit, ExitOther, ExitBypassPermissionsDisabled,
}
