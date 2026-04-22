package permission

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/example/claude-code-go/pkg/claudetypes"
)

// ============================================================================
// SimpleDecider — rule-based permission decider
// ============================================================================

// SimpleDecider evaluates permission rules against tool invocations.
// It checks deny rules first, then tool-specific checks, then allow rules,
// falling back to "ask".
type SimpleDecider struct {
	ctx Context
}

// New creates a SimpleDecider from the given context.
func New(ctx Context) *SimpleDecider {
	return &SimpleDecider{ctx: ctx}
}

// CanUse evaluates whether a tool may run.
func (d *SimpleDecider) CanUse(toolName string, input json.RawMessage, mode claudetypes.PermissionMode) (Decision, error) {
	// Step 1: Check deny rules
	if dec := d.checkDenyRules(toolName, input); dec != nil {
		return *dec, nil
	}

	// Step 2: Bypass permissions mode — allow everything that survived deny
	if mode == claudetypes.ModeBypassPermissions {
		return Decision{
			Behavior: BehaviorAllow,
			Reason:   "bypass permissions mode",
		}, nil
	}

	// Step 3: Check allow rules (whole-tool match)
	if dec := d.checkAllowRules(toolName); dec != nil {
		return *dec, nil
	}

	// Step 4: Check content-specific allow rules
	if dec := d.checkAllowRulesContent(toolName, input); dec != nil {
		return *dec, nil
	}

	// Step 5: AcceptEdits mode — allow file writes in working directories
	if mode == claudetypes.ModeAcceptEdits {
		if dec := d.checkAcceptEdits(toolName, input); dec != nil {
			return *dec, nil
		}
	}

	// Step 6: Check ask rules
	if dec := d.checkAskRules(toolName, input); dec != nil {
		return *dec, nil
	}

	// Default: ask
	return Decision{
		Behavior: BehaviorAsk,
		Reason:   "default — no matching rule",
		AskUser: &AskPrompt{
			Message: fmt.Sprintf("Allow %s?", toolName),
		},
	}, nil
}

// checkDenyRules returns a deny decision if any deny rule matches.
func (d *SimpleDecider) checkDenyRules(toolName string, input json.RawMessage) *Decision {
	inputContent := extractInputContent(toolName, input)

	for _, rule := range d.ctx.DenyRules {
		parsed := ParseRule(rule)
		if parsed.Matches(toolName, inputContent) {
			return &Decision{
				Behavior: BehaviorDeny,
				Reason:   fmt.Sprintf("denied by rule: %s", formatRule(rule)),
				Rule: &MatchedRule{
					ToolName:    rule.ToolName,
					RuleContent: rule.RuleContent,
					Behavior:    BehaviorDeny,
				},
			}
		}
	}
	return nil
}

// checkAllowRules returns an allow decision if the entire tool is in the allow list.
func (d *SimpleDecider) checkAllowRules(toolName string) *Decision {
	for _, rule := range d.ctx.AllowRules {
		if rule.ToolName == toolName && rule.RuleContent == "" {
			return &Decision{
				Behavior: BehaviorAllow,
				Reason:   fmt.Sprintf("allowed by rule: %s", toolName),
				Rule: &MatchedRule{
					ToolName: rule.ToolName,
					Behavior: BehaviorAllow,
				},
			}
		}
	}
	return nil
}

// checkAllowRulesContent returns an allow decision if a content-specific allow rule matches.
func (d *SimpleDecider) checkAllowRulesContent(toolName string, input json.RawMessage) *Decision {
	inputContent := extractInputContent(toolName, input)

	for _, rule := range d.ctx.AllowRules {
		if rule.RuleContent == "" {
			continue
		}
		parsed := ParseRule(rule)
		if parsed.Matches(toolName, inputContent) {
			return &Decision{
				Behavior: BehaviorAllow,
				Reason:   fmt.Sprintf("allowed by rule: %s", formatRule(rule)),
				Rule: &MatchedRule{
					ToolName:    rule.ToolName,
					RuleContent: rule.RuleContent,
					Behavior:    BehaviorAllow,
				},
			}
		}
	}
	return nil
}

// checkAcceptEdits allows file edits within working directories.
func (d *SimpleDecider) checkAcceptEdits(toolName string, input json.RawMessage) *Decision {
	if toolName != "Edit" && toolName != "Write" {
		return nil
	}

	path := extractFilePath(input)
	if path == "" {
		return nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Check if path is within CWD or additional working directories
	allowedDirs := []string{d.ctx.CWD}
	allowedDirs = append(allowedDirs, d.ctx.AdditionalWorkingDirectories...)

	for _, dir := range allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if isPathUnder(absPath, absDir) {
			return &Decision{
				Behavior: BehaviorAllow,
				Reason:   fmt.Sprintf("acceptEdits mode — file within working directory %s", dir),
			}
		}
	}

	return nil
}

// checkAskRules returns an ask decision if any ask rule matches.
func (d *SimpleDecider) checkAskRules(toolName string, input json.RawMessage) *Decision {
	inputContent := extractInputContent(toolName, input)

	for _, rule := range d.ctx.AskRules {
		parsed := ParseRule(rule)
		if parsed.Matches(toolName, inputContent) {
			return &Decision{
				Behavior: BehaviorAsk,
				Reason:   fmt.Sprintf("ask by rule: %s", formatRule(rule)),
				Rule: &MatchedRule{
					ToolName:    rule.ToolName,
					RuleContent: rule.RuleContent,
					Behavior:    BehaviorAsk,
				},
				AskUser: &AskPrompt{
					Message: fmt.Sprintf("Allow %s (%s)?", toolName, formatRule(rule)),
				},
			}
		}
	}
	return nil
}

// ============================================================================
// Input extraction helpers
// ============================================================================

// extractInputContent extracts the relevant "content" string from a tool's input,
// used for rule matching. For Bash this is the command, for file tools it's the path.
func extractInputContent(toolName string, input json.RawMessage) string {
	var m map[string]any
	if err := json.Unmarshal(input, &m); err != nil {
		return ""
	}

	switch toolName {
	case "Bash":
		if cmd, ok := m["command"].(string); ok {
			return cmd
		}
	case "Edit", "Write":
		if path, ok := m["file_path"].(string); ok {
			return path
		}
	case "Read":
		if path, ok := m["file_path"].(string); ok {
			return path
		}
	}

	return ""
}

// extractFilePath extracts the file_path field from tool input.
func extractFilePath(input json.RawMessage) string {
	var m map[string]any
	if err := json.Unmarshal(input, &m); err != nil {
		return ""
	}
	if path, ok := m["file_path"].(string); ok {
		return path
	}
	return ""
}

// isPathUnder reports whether path is under dir (including dir itself).
func isPathUnder(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || !strings.HasPrefix(rel, "..")
}

func formatRule(r RuleEntry) string {
	if r.RuleContent == "" {
		return r.ToolName
	}
	return fmt.Sprintf("%s(%s)", r.ToolName, r.RuleContent)
}
