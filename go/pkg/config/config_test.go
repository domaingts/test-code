package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSettingsFile_missing(t *testing.T) {
	s, err := loadSettingsFile("/tmp/no-such-file-12345.json", SourceUserSettings)
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if s.Source != "" {
		t.Errorf("expected zero source, got %s", s.Source)
	}
}

func TestLoadSettingsFile_valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	cfg := Settings{
		Model: "claude-sonnet-4-20250514",
		Permissions: PermissionSettings{
			Allow: []PermissionRuleEntry{
				{ToolName: "Bash"},
			},
		},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	s, err := loadSettingsFile(path, SourceUserSettings)
	if err != nil {
		t.Fatal(err)
	}
	if s.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want claude-sonnet-4-20250514", s.Model)
	}
	if len(s.Permissions.Allow) != 1 {
		t.Errorf("Allow rules = %d, want 1", len(s.Permissions.Allow))
	}
	if s.Source != SourceUserSettings {
		t.Errorf("Source = %q, want %q", s.Source, SourceUserSettings)
	}
}

func TestLoadSettingsFile_invalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := loadSettingsFile(path, SourceUserSettings)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMergeSettings_precedence(t *testing.T) {
	base := Settings{
		Model:         "base-model",
		ThemePreference: "dark",
		CleanupPeriodDays: 7,
	}
	over := Settings{
		Model: "override-model",
		Permissions: PermissionSettings{
			DefaultMode: "acceptEdits",
		},
	}

	merged := mergeSettings(base, over)

	if merged.Model != "override-model" {
		t.Errorf("Model = %q, want override-model", merged.Model)
	}
	if merged.ThemePreference != "dark" {
		t.Errorf("ThemePreference = %q, want dark (preserved from base)", merged.ThemePreference)
	}
	if merged.Permissions.DefaultMode != "acceptEdits" {
		t.Errorf("DefaultMode = %q, want acceptEdits", merged.Permissions.DefaultMode)
	}
	if merged.CleanupPeriodDays != 7 {
		t.Errorf("CleanupPeriodDays = %d, want 7", merged.CleanupPeriodDays)
	}
}

func TestMergeSettings_env(t *testing.T) {
	base := Settings{
		Env: map[string]string{"FOO": "base"},
	}
	over := Settings{
		Env: map[string]string{"FOO": "override", "BAR": "new"},
	}

	merged := mergeSettings(base, over)
	if merged.Env["FOO"] != "override" {
		t.Errorf("FOO = %q, want override", merged.Env["FOO"])
	}
	if merged.Env["BAR"] != "new" {
		t.Errorf("BAR = %q, want new", merged.Env["BAR"])
	}
}

func TestMergeSettings_permissions(t *testing.T) {
	base := Settings{
		Permissions: PermissionSettings{
			Allow: []PermissionRuleEntry{{ToolName: "Bash"}},
			Deny:  []PermissionRuleEntry{{ToolName: "Delete"}},
		},
	}
	over := Settings{
		Permissions: PermissionSettings{
			Allow: []PermissionRuleEntry{{ToolName: "Read"}},
		},
	}

	merged := mergeSettings(base, over)
	if len(merged.Permissions.Allow) != 1 {
		t.Errorf("Allow rules = %d, want 1 (over replaces)", len(merged.Permissions.Allow))
	}
	if merged.Permissions.Allow[0].ToolName != "Read" {
		t.Errorf("Allow[0].ToolName = %q, want Read", merged.Permissions.Allow[0].ToolName)
	}
	if len(merged.Permissions.Deny) != 1 {
		t.Errorf("Deny rules = %d, want 1 (preserved from base)", len(merged.Permissions.Deny))
	}
}

func TestLoader_fullSources(t *testing.T) {
	globalDir := t.TempDir()
	projectDir := t.TempDir()

	// User settings
	writeJSON(t, filepath.Join(globalDir, "settings.json"), Settings{
		Model: "user-model",
		ThemePreference: "dark",
	})

	// Project settings
	os.MkdirAll(filepath.Join(projectDir, ".claude"), 0755)
	writeJSON(t, filepath.Join(projectDir, ".claude", "settings.json"), Settings{
		Model: "project-model",
		Permissions: PermissionSettings{
			DefaultMode: "acceptEdits",
		},
	})

	// Local settings
	writeJSON(t, filepath.Join(projectDir, ".claude", "settings.local.json"), Settings{
		ThemePreference: "light",
	})

	l := &Loader{
		GlobalDir:  globalDir,
		ProjectDir: projectDir,
	}

	s, err := l.Global()
	if err != nil {
		t.Fatal(err)
	}

	// Local overrides project, project overrides user
	if s.Model != "project-model" {
		t.Errorf("Model = %q, want project-model", s.Model)
	}
	if s.ThemePreference != "light" {
		t.Errorf("ThemePreference = %q, want light (local overrides user)", s.ThemePreference)
	}
	if s.Permissions.DefaultMode != "acceptEdits" {
		t.Errorf("DefaultMode = %q, want acceptEdits", s.Permissions.DefaultMode)
	}
}

func TestLoader_allowedSources(t *testing.T) {
	globalDir := t.TempDir()

	writeJSON(t, filepath.Join(globalDir, "settings.json"), Settings{
		Model: "user-model",
	})

	l := &Loader{
		GlobalDir:      globalDir,
		ProjectDir:     t.TempDir(),
		AllowedSources: []SettingSource{SourceUserSettings},
	}

	s, err := l.Global()
	if err != nil {
		t.Fatal(err)
	}
	if s.Model != "user-model" {
		t.Errorf("Model = %q, want user-model", s.Model)
	}
}

func TestSanitizeProjectPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/home/user/project", "home-user-project"},
		{"/tmp/test", "tmp-test"},
	}
	for _, tc := range tests {
		got := sanitizeProjectPath(tc.input)
		if got != tc.want {
			t.Errorf("sanitizeProjectPath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSourceDisplayName(t *testing.T) {
	tests := []struct {
		source SettingSource
		want   string
	}{
		{SourceUserSettings, "user"},
		{SourceProjectSettings, "project"},
		{SourceLocalSettings, "project, gitignored"},
		{SourceFlagSettings, "cli flag"},
		{SourcePolicySettings, "managed"},
	}
	for _, tc := range tests {
		if got := SourceDisplayName(tc.source); got != tc.want {
			t.Errorf("SourceDisplayName(%q) = %q, want %q", tc.source, got, tc.want)
		}
	}
}

func TestFeatureFlag(t *testing.T) {
	SetFeatureOverrides(map[string]bool{"TEST_FLAG": true})
	if !Feature("TEST_FLAG") {
		t.Error("TEST_FLAG should be true")
	}
	if Feature("NONEXISTENT") {
		t.Error("NONEXISTENT should be false")
	}
	SetFeatureOverrides(nil) // cleanup
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
