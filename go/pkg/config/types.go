package config

import "maps"

// ============================================================================
// Setting sources — where settings can be loaded from
// ============================================================================

// SettingSource identifies where a setting value was loaded from.
type SettingSource string

const (
	SourceUserSettings    SettingSource = "userSettings"
	SourceProjectSettings SettingSource = "projectSettings"
	SourceLocalSettings   SettingSource = "localSettings"
	SourceFlagSettings    SettingSource = "flagSettings"
	SourcePolicySettings  SettingSource = "policySettings"
)

// SourceDisplayName returns the human-readable name for a setting source.
func SourceDisplayName(s SettingSource) string {
	switch s {
	case SourceUserSettings:
		return "user"
	case SourceProjectSettings:
		return "project"
	case SourceLocalSettings:
		return "project, gitignored"
	case SourceFlagSettings:
		return "cli flag"
	case SourcePolicySettings:
		return "managed"
	default:
		return "unknown"
	}
}

// ============================================================================
// Settings — the merged configuration
// ============================================================================

// HookCommand defines a single hook command entry.
type HookCommand struct {
	Type    string `json:"type,omitempty"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// HookMatcher ties hook commands to event types and optional tool-name filters.
type HookMatcher struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []HookCommand `json:"hooks"`
}

// HooksSettings holds per-event hook matchers.
type HooksSettings struct {
	PreToolUse         []HookMatcher `json:"PreToolUse,omitempty"`
	PostToolUse        []HookMatcher `json:"PostToolUse,omitempty"`
	PostToolUseFailure []HookMatcher `json:"PostToolUseFailure,omitempty"`
	Notification       []HookMatcher `json:"Notification,omitempty"`
	UserPromptSubmit   []HookMatcher `json:"UserPromptSubmit,omitempty"`
	SessionStart       []HookMatcher `json:"SessionStart,omitempty"`
	SessionEnd         []HookMatcher `json:"SessionEnd,omitempty"`
	Stop               []HookMatcher `json:"Stop,omitempty"`
	StopFailure        []HookMatcher `json:"StopFailure,omitempty"`
	PreCompact         []HookMatcher `json:"PreCompact,omitempty"`
	PostCompact        []HookMatcher `json:"PostCompact,omitempty"`
}

// MCPStdioServerConfig is a stdio MCP server definition.
type MCPStdioServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// MCPSSEServerConfig is an SSE/HTTP MCP server definition.
type MCPSSEServerConfig struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// MCPServerConfig holds a single MCP server definition.
// Exactly one of Stdio or SSE will be populated.
type MCPServerConfig struct {
	Stdio *MCPStdioServerConfig
	SSE   *MCPSSEServerConfig
}

// CustomResponseEntry overrides the system prompt for a specific context.
type CustomResponseEntry struct {
	Placeholders []string `json:"placeholders"`
	Replacements []string `json:"replacements"`
}

// ThinkingMode controls extended thinking behavior.
type ThinkingMode string

const (
	ThinkingNone   ThinkingMode = "none"
	ThinkingNormal ThinkingMode = "normal"
	ThinkingHeavy  ThinkingMode = "heavy"
	ThinkingUltra  ThinkingMode = "ultra"
)

// StatusLineConfig controls the terminal status bar.
type StatusLineConfig struct {
	Enabled bool   `json:"enabled,omitempty"`
	Type    string `json:"type,omitempty"`
}

// NotifChannel selects how notifications are delivered.
type NotifChannel string

const (
	NotifTerminal NotifChannel = "terminal"
	NotifNone     NotifChannel = "none"
)

// Settings is the full configuration model for claude-code.
// Each field can be set in any source; the merged result respects
// the source precedence: user < project < local < flag < policy.
type Settings struct {
	// Permissions
	Permissions              PermissionSettings `json:"permissions"`
	DisableBypassPermissions string             `json:"disableBypassPermissionsMode,omitempty"`

	// Hooks
	Hooks HooksSettings `json:"hooks"`

	// Environment
	Env map[string]string `json:"env,omitempty"`

	// Appearance
	ThemePreference       string           `json:"themePreference,omitempty"`
	PreferredNotifChannel NotifChannel     `json:"preferredNotifChannel,omitempty"`
	StatusLine            StatusLineConfig `json:"statusLine"`

	// Model / thinking
	Model    string       `json:"model,omitempty"`
	Thinking ThinkingMode `json:"thinking,omitempty"`
	DiffTool string       `json:"diffTool,omitempty"`

	// MCP
	EnableAllProjectMcpServers bool                       `json:"enableAllProjectMcpServers,omitempty"`
	AllowedMcpServers          []string                   `json:"allowedMcpServers,omitempty"`
	MCPServers                 map[string]MCPServerConfig `json:"mcpServers,omitempty"`

	// Session / cleanup
	CleanupPeriodDays   int  `json:"cleanupPeriodDays,omitempty"`
	IncludeCoAuthoredBy bool `json:"includeCoAuthoredBy,omitempty"`
	EnableArchiving     bool `json:"enableArchiving,omitempty"`
	AutoCompactEnabled  bool `json:"autoCompactEnabled,omitempty"`

	// API key
	APIKeyHelperSuffix string `json:"apiKeyHelperSuffix,omitempty"`

	// Custom prompt responses
	CustomResponses []CustomResponseEntry `json:"customResponses,omitempty"`

	// Tracking
	InstallationID string `json:"installationId,omitempty"`
	AccountUUID    string `json:"accountUuid,omitempty"`

	// Analytics
	AnalyticsEnabled *bool `json:"analyticsEnabled,omitempty"`

	// Enterprise MDM
	ManagedSettingsURL string `json:"managedSettingsUrl,omitempty"`

	// Misc
	IsOpenSSHReleaseChannel bool `json:"isOpenSshReleaseChannel,omitempty"`
	IsSubnet                bool `json:"isSubnet,omitempty"`

	// Metadata — not serialized
	Source SettingSource `json:"-"`
}

// PermissionSettings holds the permissions section of settings.
type PermissionSettings struct {
	Allow                 []PermissionRuleEntry `json:"allow,omitempty"`
	Deny                  []PermissionRuleEntry `json:"deny,omitempty"`
	Ask                   []PermissionRuleEntry `json:"ask,omitempty"`
	DefaultMode           string                `json:"defaultMode,omitempty"`
	AdditionalDirectories []string              `json:"additionalDirectories,omitempty"`
}

// PermissionRuleEntry is a single permission rule from settings.
type PermissionRuleEntry struct {
	ToolName    string `json:"toolName"`
	RuleContent string `json:"ruleContent,omitempty"`
}

// ============================================================================
// Merge — layer settings with precedence
// ============================================================================

// mergeSettings overlays higher-precedence settings on top of base.
// Non-zero fields from `over` replace the corresponding field in `base`.
func mergeSettings(base, over Settings) Settings {
	out := base

	if len(over.Permissions.Allow) > 0 {
		out.Permissions.Allow = over.Permissions.Allow
	}
	if len(over.Permissions.Deny) > 0 {
		out.Permissions.Deny = over.Permissions.Deny
	}
	if len(over.Permissions.Ask) > 0 {
		out.Permissions.Ask = over.Permissions.Ask
	}
	if over.Permissions.DefaultMode != "" {
		out.Permissions.DefaultMode = over.Permissions.DefaultMode
	}
	if len(over.Permissions.AdditionalDirectories) > 0 {
		out.Permissions.AdditionalDirectories = over.Permissions.AdditionalDirectories
	}

	if over.DisableBypassPermissions != "" {
		out.DisableBypassPermissions = over.DisableBypassPermissions
	}

	if len(over.Hooks.PreToolUse) > 0 {
		out.Hooks.PreToolUse = over.Hooks.PreToolUse
	}
	if len(over.Hooks.PostToolUse) > 0 {
		out.Hooks.PostToolUse = over.Hooks.PostToolUse
	}
	if len(over.Hooks.Stop) > 0 {
		out.Hooks.Stop = over.Hooks.Stop
	}
	if len(over.Hooks.SessionStart) > 0 {
		out.Hooks.SessionStart = over.Hooks.SessionStart
	}
	if len(over.Hooks.SessionEnd) > 0 {
		out.Hooks.SessionEnd = over.Hooks.SessionEnd
	}
	if len(over.Hooks.UserPromptSubmit) > 0 {
		out.Hooks.UserPromptSubmit = over.Hooks.UserPromptSubmit
	}
	if len(over.Hooks.Notification) > 0 {
		out.Hooks.Notification = over.Hooks.Notification
	}
	if len(over.Hooks.PreCompact) > 0 {
		out.Hooks.PreCompact = over.Hooks.PreCompact
	}
	if len(over.Hooks.PostCompact) > 0 {
		out.Hooks.PostCompact = over.Hooks.PostCompact
	}

	if len(over.Env) > 0 {
		if out.Env == nil {
			out.Env = make(map[string]string)
		}
		maps.Copy(out.Env, over.Env)
	}

	if over.ThemePreference != "" {
		out.ThemePreference = over.ThemePreference
	}
	if over.PreferredNotifChannel != "" {
		out.PreferredNotifChannel = over.PreferredNotifChannel
	}
	if over.StatusLine.Enabled {
		out.StatusLine = over.StatusLine
	}

	if over.Model != "" {
		out.Model = over.Model
	}
	if over.Thinking != "" {
		out.Thinking = over.Thinking
	}
	if over.DiffTool != "" {
		out.DiffTool = over.DiffTool
	}

	if len(over.MCPServers) > 0 {
		out.MCPServers = over.MCPServers
	}
	if over.EnableAllProjectMcpServers {
		out.EnableAllProjectMcpServers = true
	}
	if len(over.AllowedMcpServers) > 0 {
		out.AllowedMcpServers = over.AllowedMcpServers
	}

	if over.CleanupPeriodDays > 0 {
		out.CleanupPeriodDays = over.CleanupPeriodDays
	}
	if over.IncludeCoAuthoredBy {
		out.IncludeCoAuthoredBy = true
	}
	if over.EnableArchiving {
		out.EnableArchiving = true
	}
	if over.AutoCompactEnabled {
		out.AutoCompactEnabled = true
	}

	if over.APIKeyHelperSuffix != "" {
		out.APIKeyHelperSuffix = over.APIKeyHelperSuffix
	}

	if len(over.CustomResponses) > 0 {
		out.CustomResponses = over.CustomResponses
	}

	if over.InstallationID != "" {
		out.InstallationID = over.InstallationID
	}
	if over.AccountUUID != "" {
		out.AccountUUID = over.AccountUUID
	}

	if over.AnalyticsEnabled != nil {
		out.AnalyticsEnabled = over.AnalyticsEnabled
	}

	if over.ManagedSettingsURL != "" {
		out.ManagedSettingsURL = over.ManagedSettingsURL
	}

	return out
}
