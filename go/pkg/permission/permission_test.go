package permission

import (
	"encoding/json"
	"testing"

	"github.com/example/claude-code-go/pkg/claudetypes"
)

// --- Wildcard matching tests ---

func TestMatchWildcard(t *testing.T) {
	cases := []struct {
		pattern string
		input   string
		want    bool
	}{
		{"git *", "git add", true},
		{"git *", "git", true}, // trailing optional
		{"git *", "git status --short", true},
		{"npm run *", "npm run build", true},
		{"npm run *", "npm", false},
		{"exact match", "exact match", true},
		{"exact match", "different", false},
		{"*", "anything", true},
		{"*test*", "my test file", true},
		{`literal\*star`, "literal*star", true},
	}

	for _, tc := range cases {
		got := MatchWildcard(tc.pattern, tc.input)
		if got != tc.want {
			t.Errorf("MatchWildcard(%q, %q) = %v, want %v", tc.pattern, tc.input, got, tc.want)
		}
	}
}

// --- Rule parsing tests ---

func TestParseRule(t *testing.T) {
	cases := []struct {
		entry    RuleEntry
		wantType RuleType
	}{
		{RuleEntry{ToolName: "Bash"}, RuleExact},                             // whole tool
		{RuleEntry{ToolName: "Bash", RuleContent: "npm install"}, RuleExact}, // exact
		{RuleEntry{ToolName: "Bash", RuleContent: "npm:*"}, RulePrefix},      // legacy prefix
		{RuleEntry{ToolName: "Bash", RuleContent: "npm run *"}, RuleWildcard}, // wildcard
		{RuleEntry{ToolName: "Edit", RuleContent: "*.go"}, RuleWildcard},
	}

	for _, tc := range cases {
		parsed := ParseRule(tc.entry)
		if parsed.Type != tc.wantType {
			t.Errorf("ParseRule(%+v).Type = %q, want %q", tc.entry, parsed.Type, tc.wantType)
		}
	}
}

func TestParsedRule_Matches(t *testing.T) {
	cases := []struct {
		rule    RuleEntry
		tool    string
		content string
		want    bool
	}{
		// Whole-tool match
		{RuleEntry{ToolName: "Bash"}, "Bash", "anything", true},
		{RuleEntry{ToolName: "Bash"}, "Edit", "anything", false},

		// Exact match
		{RuleEntry{ToolName: "Bash", RuleContent: "npm install"}, "Bash", "npm install", true},
		{RuleEntry{ToolName: "Bash", RuleContent: "npm install"}, "Bash", "npm run build", false},

		// Prefix match (legacy)
		{RuleEntry{ToolName: "Bash", RuleContent: "npm:*"}, "Bash", "npm install", true},
		{RuleEntry{ToolName: "Bash", RuleContent: "npm:*"}, "Bash", "npm run build", true},
		{RuleEntry{ToolName: "Bash", RuleContent: "npm:*"}, "Bash", "pip install", false},
		{RuleEntry{ToolName: "Bash", RuleContent: "npm:*"}, "Bash", "npm", true}, // bare "npm" matches

		// Wildcard match
		{RuleEntry{ToolName: "Bash", RuleContent: "git *"}, "Bash", "git add", true},
		{RuleEntry{ToolName: "Bash", RuleContent: "git *"}, "Bash", "git", true},
		{RuleEntry{ToolName: "Bash", RuleContent: "npm run *"}, "Bash", "npm run build", true},
	}

	for _, tc := range cases {
		parsed := ParseRule(tc.rule)
		got := parsed.Matches(tc.tool, tc.content)
		if got != tc.want {
			t.Errorf("ParseRule(%+v).Matches(%q, %q) = %v, want %v",
				tc.rule, tc.tool, tc.content, got, tc.want)
		}
	}
}

// --- Decider tests ---

func jsonInput(m map[string]any) json.RawMessage {
	data, _ := json.Marshal(m)
	return data
}

func TestDecider_denyRule(t *testing.T) {
	d := New(Context{
		Mode: claudetypes.ModeDefault,
		CWD:  "/tmp",
		DenyRules: []RuleEntry{
			{ToolName: "Bash", RuleContent: "rm -rf *"},
		},
	})

	dec, err := d.CanUse("Bash", jsonInput(map[string]any{"command": "rm -rf /"}), claudetypes.ModeDefault)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Behavior != BehaviorDeny {
		t.Errorf("Behavior = %q, want deny", dec.Behavior)
	}
}

func TestDecider_denyWholeTool(t *testing.T) {
	d := New(Context{
		Mode: claudetypes.ModeDefault,
		CWD:  "/tmp",
		DenyRules: []RuleEntry{
			{ToolName: "Agent"},
		},
	})

	dec, _ := d.CanUse("Agent", jsonInput(map[string]any{"prompt": "do stuff"}), claudetypes.ModeDefault)
	if dec.Behavior != BehaviorDeny {
		t.Errorf("Behavior = %q, want deny", dec.Behavior)
	}
}

func TestDecider_bypassMode(t *testing.T) {
	d := New(Context{
		Mode: claudetypes.ModeDefault,
		CWD:  "/tmp",
	})

	dec, _ := d.CanUse("Bash", jsonInput(map[string]any{"command": "anything"}), claudetypes.ModeBypassPermissions)
	if dec.Behavior != BehaviorAllow {
		t.Errorf("Behavior = %q, want allow (bypass)", dec.Behavior)
	}
}

func TestDecider_bypassStillDeniesExplicitDenyRules(t *testing.T) {
	d := New(Context{
		Mode: claudetypes.ModeDefault,
		CWD:  "/tmp",
		DenyRules: []RuleEntry{
			{ToolName: "Bash", RuleContent: "rm -rf *"},
		},
	})

	dec, _ := d.CanUse("Bash", jsonInput(map[string]any{"command": "rm -rf /"}), claudetypes.ModeBypassPermissions)
	if dec.Behavior != BehaviorDeny {
		t.Errorf("Behavior = %q, want deny (bypass still honors deny rules)", dec.Behavior)
	}
}

func TestDecider_allowRule(t *testing.T) {
	d := New(Context{
		Mode: claudetypes.ModeDefault,
		CWD:  "/tmp",
		AllowRules: []RuleEntry{
			{ToolName: "Read"},
		},
	})

	dec, _ := d.CanUse("Read", jsonInput(map[string]any{"file_path": "/etc/passwd"}), claudetypes.ModeDefault)
	if dec.Behavior != BehaviorAllow {
		t.Errorf("Behavior = %q, want allow", dec.Behavior)
	}
}

func TestDecider_allowContentRule(t *testing.T) {
	d := New(Context{
		Mode: claudetypes.ModeDefault,
		CWD:  "/tmp",
		AllowRules: []RuleEntry{
			{ToolName: "Bash", RuleContent: "npm *"},
		},
	})

	dec, _ := d.CanUse("Bash", jsonInput(map[string]any{"command": "npm install"}), claudetypes.ModeDefault)
	if dec.Behavior != BehaviorAllow {
		t.Errorf("Behavior = %q, want allow", dec.Behavior)
	}

	dec, _ = d.CanUse("Bash", jsonInput(map[string]any{"command": "rm -rf /"}), claudetypes.ModeDefault)
	if dec.Behavior != BehaviorAsk {
		t.Errorf("Behavior = %q, want ask (no matching allow)", dec.Behavior)
	}
}

func TestDecider_acceptEditsMode(t *testing.T) {
	d := New(Context{
		Mode: claudetypes.ModeDefault,
		CWD:  "/home/user/project",
	})

	dec, _ := d.CanUse("Edit", jsonInput(map[string]any{
		"file_path": "/home/user/project/src/main.go",
	}), claudetypes.ModeAcceptEdits)
	if dec.Behavior != BehaviorAllow {
		t.Errorf("Behavior = %q, want allow (acceptEdits)", dec.Behavior)
	}

	// Should not allow edits outside CWD
	dec, _ = d.CanUse("Edit", jsonInput(map[string]any{
		"file_path": "/etc/config",
	}), claudetypes.ModeAcceptEdits)
	if dec.Behavior != BehaviorAsk {
		t.Errorf("Behavior = %q, want ask (outside CWD)", dec.Behavior)
	}
}

func TestDecider_acceptEditsDoesNotApplyInDefaultMode(t *testing.T) {
	d := New(Context{
		Mode: claudetypes.ModeDefault,
		CWD:  "/home/user/project",
	})

	dec, _ := d.CanUse("Edit", jsonInput(map[string]any{
		"file_path": "/home/user/project/src/main.go",
	}), claudetypes.ModeDefault)
	if dec.Behavior != BehaviorAsk {
		t.Errorf("Behavior = %q, want ask (not acceptEdits mode)", dec.Behavior)
	}
}

func TestDecider_askRule(t *testing.T) {
	d := New(Context{
		Mode: claudetypes.ModeDefault,
		CWD:  "/tmp",
		AskRules: []RuleEntry{
			{ToolName: "Bash", RuleContent: "git *"},
		},
	})

	dec, _ := d.CanUse("Bash", jsonInput(map[string]any{"command": "git push"}), claudetypes.ModeDefault)
	if dec.Behavior != BehaviorAsk {
		t.Errorf("Behavior = %q, want ask", dec.Behavior)
	}
	if dec.AskUser == nil {
		t.Error("AskUser should be non-nil for ask decisions")
	}
}

func TestDecider_denyTakesPrecedenceOverAllow(t *testing.T) {
	d := New(Context{
		Mode: claudetypes.ModeDefault,
		CWD:  "/tmp",
		AllowRules: []RuleEntry{
			{ToolName: "Bash", RuleContent: "npm *"},
		},
		DenyRules: []RuleEntry{
			{ToolName: "Bash", RuleContent: "npm publish *"},
		},
	})

	dec, _ := d.CanUse("Bash", jsonInput(map[string]any{"command": "npm publish"}), claudetypes.ModeDefault)
	if dec.Behavior != BehaviorDeny {
		t.Errorf("Behavior = %q, want deny (deny takes precedence)", dec.Behavior)
	}

	dec, _ = d.CanUse("Bash", jsonInput(map[string]any{"command": "npm install"}), claudetypes.ModeDefault)
	if dec.Behavior != BehaviorAllow {
		t.Errorf("Behavior = %q, want allow (npm install matches allow, not deny)", dec.Behavior)
	}
}

func TestDecider_defaultAsk(t *testing.T) {
	d := New(Context{
		Mode: claudetypes.ModeDefault,
		CWD:  "/tmp",
	})

	dec, _ := d.CanUse("Bash", jsonInput(map[string]any{"command": "echo hello"}), claudetypes.ModeDefault)
	if dec.Behavior != BehaviorAsk {
		t.Errorf("Behavior = %q, want ask (default)", dec.Behavior)
	}
}
