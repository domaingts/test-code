package permission

import (
	"encoding/json"

	"github.com/example/claude-code-go/pkg/claudetypes"
)

// ============================================================================
// Decision — the result of a permission check
// ============================================================================

// Behavior is the action taken on a permission check.
type Behavior string

const (
	BehaviorAllow Behavior = "allow"
	BehaviorDeny  Behavior = "deny"
	BehaviorAsk   Behavior = "ask"
)

// Decision is the result of a permission check.
type Decision struct {
	Behavior Behavior
	Reason   string
	// AskUser is non-nil when Behavior is "ask" — it carries the prompt
	// to show the user (handled by the TUI or SDK layer).
	AskUser *AskPrompt
	// Rule that caused this decision, if any.
	Rule *MatchedRule
}

// AskPrompt describes what to ask the user when permission is needed.
type AskPrompt struct {
	Message string
	// Suggestions are permission updates the user could apply.
	Suggestions []Suggestion
}

// Suggestion is a proposed permission update the user could accept.
type Suggestion struct {
	ToolName    string
	RuleContent string
	Behavior    Behavior
	// SaveTo where to persist the rule if accepted.
	SaveTo claudetypes.PermissionUpdateDestination
}

// MatchedRule identifies which rule produced this decision.
type MatchedRule struct {
	ToolName    string
	RuleContent string
	Behavior    Behavior
	Source      claudetypes.PermissionRuleSource
}

// ============================================================================
// Decider — the permission policy interface
// ============================================================================

// Decider evaluates whether a tool invocation is permitted.
type Decider interface {
	// CanUse checks whether the named tool may run with the given input.
	// Returns allow, deny, or ask.
	CanUse(toolName string, input json.RawMessage, mode claudetypes.PermissionMode) (Decision, error)
}

// ============================================================================
// Context — what the decider needs to know
// ============================================================================

// Context carries the ambient state for permission checks.
type Context struct {
	// Mode is the current permission mode.
	Mode claudetypes.PermissionMode
	// CWD is the current working directory.
	CWD string
	// AllowRules are the configured allow rules.
	AllowRules []RuleEntry
	// DenyRules are the configured deny rules.
	DenyRules []RuleEntry
	// AskRules are the configured ask rules.
	AskRules []RuleEntry
	// AdditionalWorkingDirectories are extra directories allowed for file access.
	AdditionalWorkingDirectories []string
}

// RuleEntry is a single permission rule.
type RuleEntry struct {
	ToolName    string `json:"toolName"`
	RuleContent string `json:"ruleContent,omitempty"`
}

// ============================================================================
// Rule parsing
// ============================================================================

// RuleType identifies how a rule matches commands.
type RuleType string

const (
	RuleExact    RuleType = "exact"
	RulePrefix   RuleType = "prefix"
	RuleWildcard RuleType = "wildcard"
)

// ParsedRule is a rule that has been parsed for matching.
type ParsedRule struct {
	Type     RuleType
	ToolName string
	Pattern  string // the content pattern (without prefix like "Bash(")
}

// ParseRule parses a rule entry into a ParsedRule.
// Supports:
//   - "Bash" — whole-tool match
//   - "Bash(command)" — exact command match
//   - "Bash(command:*...)" — prefix match (legacy)
//   - "Bash(command * ...)" — wildcard match
func ParseRule(entry RuleEntry) ParsedRule {
	return ParsedRule{
		ToolName: entry.ToolName,
		Type:     classifyContent(entry.RuleContent),
		Pattern:  entry.RuleContent,
	}
}

// classifyContent determines the rule type from its content string.
func classifyContent(content string) RuleType {
	if content == "" {
		return RuleExact // whole-tool match, no content filter
	}
	// Check for legacy prefix syntax: "command:*" (must end with :*)
	if len(content) >= 2 && content[len(content)-2:] == ":*" {
		return RulePrefix
	}
	// Check for wildcard: contains *
	for i := 0; i < len(content); i++ {
		if content[i] == '*' {
			return RuleWildcard
		}
	}
	return RuleExact
}

// Matches reports whether this rule matches the given tool name and input content.
func (r ParsedRule) Matches(toolName, inputContent string) bool {
	// Tool name must match
	if r.ToolName != toolName {
		return false
	}

	// No content filter — matches everything for this tool
	if r.Pattern == "" {
		return true
	}

	// Match content
	switch r.Type {
	case RuleExact:
		return inputContent == r.Pattern
	case RulePrefix:
		// "command:*" matches anything starting with "command "
		prefix := r.Pattern
		if len(prefix) >= 2 && prefix[len(prefix)-2:] == ":*" {
			prefix = prefix[:len(prefix)-2]
		}
		return inputContent == prefix || startsWith(inputContent, prefix+" ")
	case RuleWildcard:
		return MatchWildcard(r.Pattern, inputContent)
	default:
		return false
	}
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// ============================================================================
// Config — how to construct a decider
// ============================================================================

// Config holds the configuration for building a Decider.
type Config struct {
	Context Context
}
