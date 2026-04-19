package claudetypes

// ============================================================================
// Permission Modes
// ============================================================================

// PermissionMode controls how tool permissions are handled.
type PermissionMode string

const (
	ModeDefault            PermissionMode = "default"
	ModeAcceptEdits        PermissionMode = "acceptEdits"
	ModeBypassPermissions  PermissionMode = "bypassPermissions"
	ModeDontAsk            PermissionMode = "dontAsk"
	ModePlan               PermissionMode = "plan"
	ModeAuto               PermissionMode = "auto"
	ModeBubble             PermissionMode = "bubble"
)

// ExternalPermissionModes are user-addressable permission modes.
var ExternalPermissionModes = []PermissionMode{
	ModeAcceptEdits,
	ModeBypassPermissions,
	ModeDefault,
	ModeDontAsk,
	ModePlan,
}

// ============================================================================
// Permission Behaviors
// ============================================================================

// PermissionBehavior is the action taken on a permission check.
type PermissionBehavior string

const (
	BehaviorAllow PermissionBehavior = "allow"
	BehaviorDeny  PermissionBehavior = "deny"
	BehaviorAsk   PermissionBehavior = "ask"
)

// ============================================================================
// Permission Rules
// ============================================================================

// PermissionRuleSource identifies where a permission rule was defined.
type PermissionRuleSource string

const (
	SourceUserSettings    PermissionRuleSource = "userSettings"
	SourceProjectSettings PermissionRuleSource = "projectSettings"
	SourceLocalSettings   PermissionRuleSource = "localSettings"
	SourceFlagSettings    PermissionRuleSource = "flagSettings"
	SourcePolicySettings  PermissionRuleSource = "policySettings"
	SourceCLIArg          PermissionRuleSource = "cliArg"
	SourceCommand         PermissionRuleSource = "command"
	SourceSession         PermissionRuleSource = "session"
)

// PermissionRuleValue specifies which tool and optional content a rule applies to.
type PermissionRuleValue struct {
	ToolName    string `json:"toolName"`
	RuleContent string `json:"ruleContent,omitempty"`
}

// PermissionRule ties a rule value to its source and behavior.
type PermissionRule struct {
	Source       PermissionRuleSource `json:"source"`
	RuleBehavior PermissionBehavior   `json:"ruleBehavior"`
	RuleValue    PermissionRuleValue  `json:"ruleValue"`
}

// ============================================================================
// Permission Updates
// ============================================================================

// PermissionUpdateDestination specifies where a permission change is persisted.
type PermissionUpdateDestination string

const (
	DestUserSettings    PermissionUpdateDestination = "userSettings"
	DestProjectSettings PermissionUpdateDestination = "projectSettings"
	DestLocalSettings   PermissionUpdateDestination = "localSettings"
	DestSession         PermissionUpdateDestination = "session"
	DestCLIArg          PermissionUpdateDestination = "cliArg"
)

// PermissionUpdateType identifies the kind of permission update operation.
type PermissionUpdateType string

const (
	UpdateAddRules     PermissionUpdateType = "addRules"
	UpdateReplaceRules PermissionUpdateType = "replaceRules"
	UpdateRemoveRules  PermissionUpdateType = "removeRules"
	UpdateSetMode      PermissionUpdateType = "setMode"
	UpdateAddDirs      PermissionUpdateType = "addDirectories"
	UpdateRemoveDirs   PermissionUpdateType = "removeDirectories"
)

// PermissionUpdate describes a single change to the permission configuration.
type PermissionUpdate struct {
	Type        PermissionUpdateType        `json:"type"`
	Destination PermissionUpdateDestination `json:"destination"`
	Rules       []PermissionRuleValue       `json:"rules,omitempty"`
	Behavior    PermissionBehavior          `json:"behavior,omitempty"`
	Mode        PermissionMode              `json:"mode,omitempty"`
	Directories []string                    `json:"directories,omitempty"`
}

// ============================================================================
// Permission Decisions
// ============================================================================

// PermissionDecisionReason explains why a permission decision was made.
type PermissionDecisionReason struct {
	Type        string // "rule", "mode", "hook", "classifier", "safetyCheck", etc.
	Rule        *PermissionRule
	Mode        PermissionMode
	HookName    string
	HookSource  string
	Classifier  string
	Reason      string
	ClassifierApprovable bool
}

// PermissionAllowDecision grants permission to use a tool.
type PermissionAllowDecision struct {
	UpdatedInput    map[string]any
	UserModified    bool
	DecisionReason  *PermissionDecisionReason
	ToolUseID       string
	AcceptFeedback  string
}

// PermissionAskDecision asks the user before proceeding.
type PermissionAskDecision struct {
	Message             string
	UpdatedInput        map[string]any
	DecisionReason      *PermissionDecisionReason
	Suggestions         []PermissionUpdate
	BlockedPath         string
	PendingClassifier   *PendingClassifierCheck
}

// PermissionDenyDecision denies permission to use a tool.
type PermissionDenyDecision struct {
	Message        string
	DecisionReason PermissionDecisionReason
	ToolUseID      string
}

// PendingClassifierCheck describes an async classifier evaluation.
type PendingClassifierCheck struct {
	Command      string
	CWD          string
	Descriptions []string
}

// ============================================================================
// Tool Permission Context
// ============================================================================

// AdditionalWorkingDirectory is a directory added to the tool's working scope.
type AdditionalWorkingDirectory struct {
	Path   string              `json:"path"`
	Source PermissionRuleSource `json:"source"`
}

// ToolPermissionRulesBySource maps rule sources to their rule content lists.
type ToolPermissionRulesBySource map[PermissionRuleSource][]string

// ToolPermissionContext carries the full permission state for a tool check.
type ToolPermissionContext struct {
	Mode                          PermissionMode
	AdditionalWorkingDirectories  map[string]AdditionalWorkingDirectory
	AlwaysAllowRules              ToolPermissionRulesBySource
	AlwaysDenyRules               ToolPermissionRulesBySource
	AlwaysAskRules                ToolPermissionRulesBySource
	IsBypassPermissionsModeAvailable bool
	StrippedDangerousRules        ToolPermissionRulesBySource
	ShouldAvoidPermissionPrompts  bool
	AwaitAutomatedChecksBeforeDialog bool
	PrePlanMode                   PermissionMode
}

// ============================================================================
// Risk classification
// ============================================================================

// RiskLevel classifies the danger of a tool invocation.
type RiskLevel string

const (
	RiskLow    RiskLevel = "LOW"
	RiskMedium RiskLevel = "MEDIUM"
	RiskHigh   RiskLevel = "HIGH"
)
