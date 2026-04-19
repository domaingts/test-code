package claudetypes

import (
	"testing"
	"time"
)

func TestMessageInterface(t *testing.T) {
	msgs := []Message{
		UserMessage{
			MessageBase: MessageBase{UUID: "1", Timestamp: time.Now()},
			Content:     []ContentBlock{TextBlock{Text: "hello"}},
		},
		AssistantMessage{
			MessageBase: MessageBase{UUID: "2", Timestamp: time.Now()},
			Message:     []ContentBlock{TextBlock{Text: "hi"}},
			Usage:       Usage{InputTokens: 10, OutputTokens: 5},
		},
		SystemMessage{
			MessageBase: MessageBase{UUID: "3", Timestamp: time.Now(), IsMeta: true},
			Subtype:     SubtypeInformational,
			Content:     "info",
			Level:       SystemLevelInfo,
		},
		AttachmentMessage{
			MessageBase: MessageBase{UUID: "4", Timestamp: time.Now()},
			AttachmentType: "file",
		},
		ProgressMessage{
			MessageBase: MessageBase{UUID: "5", Timestamp: time.Now()},
			ToolUseID:   "tool1",
		},
		TombstoneMessage{
			MessageBase: MessageBase{UUID: "6", Timestamp: time.Now()},
		},
		HookResultMessage{
			MessageBase: MessageBase{UUID: "7", Timestamp: time.Now()},
		},
	}

	for _, m := range msgs {
		if m.GetUUID() == "" {
			t.Errorf("%T has empty UUID", m)
		}
	}

	// Verify sealed interface prevents external implementations
	var _ Message = UserMessage{}
	var _ Message = AssistantMessage{}
	var _ Message = SystemMessage{}
}

func TestUsage(t *testing.T) {
	u := Usage{InputTokens: 100, CacheReadInputTokens: 50, OutputTokens: 25}
	if u.TotalInput() != 150 {
		t.Errorf("TotalInput() = %d, want 150", u.TotalInput())
	}
	if u.IsZero() {
		t.Error("IsZero() should be false")
	}
	if !(Usage{}).IsZero() {
		t.Error("zero Usage should be IsZero()")
	}
}

func TestSDKEventInterface(t *testing.T) {
	events := []SDKEvent{
		SDKMessageEvent{Message: UserMessage{
			MessageBase: MessageBase{UUID: "1", Timestamp: time.Now()},
		}},
		SDKResultEvent{
			Usage:    Usage{InputTokens: 10},
			NumTurns: 3,
		},
		SDKErrorEvent{Message: "fail"},
	}

	kinds := []string{EventMessage, EventResult, EventError}
	for i, e := range events {
		if e.Kind() != kinds[i] {
			t.Errorf("event %d: Kind() = %q, want %q", i, e.Kind(), kinds[i])
		}
	}
}

func TestPermissionMode(t *testing.T) {
	modes := []PermissionMode{
		ModeDefault, ModeAcceptEdits, ModeBypassPermissions,
		ModeDontAsk, ModePlan, ModeAuto, ModeBubble,
	}
	if len(modes) != 7 {
		t.Errorf("expected 7 permission modes, got %d", len(modes))
	}
	if len(ExternalPermissionModes) != 5 {
		t.Errorf("expected 5 external modes, got %d", len(ExternalPermissionModes))
	}
}

func TestHookEvents(t *testing.T) {
	if len(AllHookEvents) == 0 {
		t.Error("AllHookEvents should not be empty")
	}
	seen := make(map[HookEvent]bool)
	for _, h := range AllHookEvents {
		if seen[h] {
			t.Errorf("duplicate hook event: %s", h)
		}
		seen[h] = true
	}
}
