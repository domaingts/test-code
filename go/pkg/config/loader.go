package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ============================================================================
// Loader — loads and merges settings from all sources
// ============================================================================

// Loader loads settings from multiple sources with proper precedence.
type Loader struct {
	// GlobalDir is the user-level Claude directory (typically ~/.claude).
	GlobalDir string

	// ProjectDir is the project-level directory (typically the CWD or CLAUDE_PROJECT_DIR).
	ProjectDir string

	// FlagOverrides are settings passed via CLI --settings flag.
	FlagOverrides Settings

	// AllowedSources controls which sources are loaded.
	// If empty, all sources are loaded.
	AllowedSources []SettingSource
}

// NewLoader creates a Loader with sensible defaults.
// Resolves global dir from $HOME or $CLAUDE_CONFIG_DIR.
// Project dir defaults to cwd, overridable with $CLAUDE_PROJECT_DIR.
func NewLoader() (*Loader, error) {
	globalDir, err := globalDir()
	if err != nil {
		return nil, fmt.Errorf("resolve global dir: %w", err)
	}

	projectDir, err := projectDir()
	if err != nil {
		return nil, fmt.Errorf("resolve project dir: %w", err)
	}

	return &Loader{
		GlobalDir:  globalDir,
		ProjectDir: projectDir,
	}, nil
}

// Global returns the merged settings from all enabled sources.
// Precedence (lowest to highest): user < project < local < flag < policy.
func (l *Loader) Global() (Settings, error) {
	var merged Settings

	if l.isSourceEnabled(SourceUserSettings) {
		s, err := loadSettingsFile(filepath.Join(l.GlobalDir, "settings.json"), SourceUserSettings)
		if err != nil {
			return Settings{}, fmt.Errorf("load user settings: %w", err)
		}
		merged = mergeSettings(merged, s)
	}

	if l.isSourceEnabled(SourceProjectSettings) {
		s, err := loadSettingsFile(filepath.Join(l.ProjectDir, ".claude", "settings.json"), SourceProjectSettings)
		if err != nil {
			return Settings{}, fmt.Errorf("load project settings: %w", err)
		}
		merged = mergeSettings(merged, s)
	}

	if l.isSourceEnabled(SourceLocalSettings) {
		s, err := loadSettingsFile(filepath.Join(l.ProjectDir, ".claude", "settings.local.json"), SourceLocalSettings)
		if err != nil {
			return Settings{}, fmt.Errorf("load local settings: %w", err)
		}
		merged = mergeSettings(merged, s)
	}

	if l.isSourceEnabled(SourceFlagSettings) {
		merged = mergeSettings(merged, l.FlagOverrides)
	}

	if l.isSourceEnabled(SourcePolicySettings) {
		s, err := loadSettingsFile(filepath.Join(l.GlobalDir, "managed-settings.json"), SourcePolicySettings)
		if err != nil {
			return Settings{}, fmt.Errorf("load policy settings: %w", err)
		}
		merged = mergeSettings(merged, s)
	}

	// Apply environment variable overrides
	applyEnvOverrides(&merged)

	return merged, nil
}

// Project returns settings loaded from project-specific sources only.
func (l *Loader) Project(cwd string) (Settings, error) {
	var merged Settings

	s, err := loadSettingsFile(filepath.Join(cwd, ".claude", "settings.json"), SourceProjectSettings)
	if err != nil {
		return Settings{}, fmt.Errorf("load project settings: %w", err)
	}
	merged = mergeSettings(merged, s)

	s, err = loadSettingsFile(filepath.Join(cwd, ".claude", "settings.local.json"), SourceLocalSettings)
	if err != nil {
		return Settings{}, fmt.Errorf("load local settings: %w", err)
	}
	merged = mergeSettings(merged, s)

	return merged, nil
}

func (l *Loader) isSourceEnabled(s SettingSource) bool {
	if len(l.AllowedSources) == 0 {
		return true
	}
	return slices.Contains(l.AllowedSources, s)
}

// ============================================================================
// File loading
// ============================================================================

// loadSettingsFile reads and parses a JSON settings file.
// Returns zero-value Settings if the file does not exist (no error).
func loadSettingsFile(path string, source SettingSource) (Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Settings{}, nil
		}
		return Settings{}, fmt.Errorf("read %s: %w", path, err)
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, fmt.Errorf("parse %s: %w", path, err)
	}
	s.Source = source
	return s, nil
}

// ============================================================================
// Path resolution
// ============================================================================

// globalDir returns the user-level Claude config directory.
// Priority: $CLAUDE_CONFIG_DIR > ~/.claude
func globalDir() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".claude"), nil
}

// projectDir returns the project root directory.
// Priority: $CLAUDE_PROJECT_DIR > $GITHUB_WORKSPACE > os.Getwd()
func projectDir() (string, error) {
	if dir := os.Getenv("CLAUDE_PROJECT_DIR"); dir != "" {
		return dir, nil
	}
	if dir := os.Getenv("GITHUB_WORKSPACE"); dir != "" {
		return dir, nil
	}
	return os.Getwd()
}

// SessionDir returns the default session storage directory.
func SessionDir() (string, error) {
	g, err := globalDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(g, "projects", sanitizeProjectPath(projectDirMust())), nil
}

func projectDirMust() string {
	d, _ := projectDir()
	return d
}

// sanitizeProjectPath converts a filesystem path to a directory name
// suitable for storing project-specific data.
func sanitizeProjectPath(path string) string {
	// Replace path separators and strip leading separators/dots
	s := strings.TrimLeft(path, string(filepath.Separator)+".")
	s = strings.ReplaceAll(s, string(filepath.Separator), "-")
	return s
}

// ============================================================================
// Env overrides
// ============================================================================

// applyEnvOverrides patches settings with environment variables.
func applyEnvOverrides(s *Settings) {
	// API key
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		// API key is not stored in Settings; it's consumed directly by the LLM client.
	}

	// Model override
	if v := os.Getenv("ANTHROPIC_MODEL"); v != "" {
		s.Model = v
	}

	// Auth token
	if v := os.Getenv("CLAUDE_CODE_USE_BEDROCK"); v != "" {
		// Bedrock mode is handled by the LLM client, not config.
	}

	if v := os.Getenv("CLAUDE_CODE_USE_VERTEX"); v != "" {
		// Vertex mode is handled by the LLM client, not config.
	}
}

// ============================================================================
// Feature flags
// ============================================================================

// FeatureFlagMap holds runtime feature flag overrides.
// Build tags handle compile-time flags; this map handles runtime overrides
// from settings.json.
type FeatureFlagMap map[string]bool

// Feature checks whether a named feature is enabled.
// Checks runtime override map first, then falls back to build-tag defaults.
func Feature(name string) bool {
	return featureRuntimeOverrides[name]
}

var featureRuntimeOverrides = FeatureFlagMap{}

// SetFeatureOverrides replaces the runtime feature flag map.
func SetFeatureOverrides(overrides map[string]bool) {
	featureRuntimeOverrides = overrides
}

// AllEnv returns the merged environment from settings + process env.
// Settings env vars are lower priority than process env.
func AllEnv(settingsEnv map[string]string) map[string]string {
	result := make(map[string]string, len(settingsEnv)+len(os.Environ()))
	// Start with settings env (lowest priority)
	maps.Copy(result, settingsEnv)
	// Process env overrides settings env
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		result[k] = v
	}
	return result
}
