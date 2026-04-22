package fileedit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	toolpkg "github.com/example/claude-code-go/pkg/tool"
)

// Tool implements the "Edit" tool — string replacement in files.
type Tool struct{}

func (Tool) Name() string { return "Edit" }

func (Tool) Schema() toolpkg.JSONSchema {
	return toolpkg.JSONSchema{
		Type: "object",
		Properties: map[string]toolpkg.Property{
			"file_path":   {Type: "string", Description: "Absolute path to the file to modify"},
			"old_string":  {Type: "string", Description: "The text to replace"},
			"new_string":  {Type: "string", Description: "The replacement text"},
			"replace_all": {Type: "boolean", Description: "Replace all occurrences"},
		},
		Required: []string{"file_path", "old_string", "new_string"},
	}
}

type input struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

type output struct {
	FilePath string `json:"filePath"`
	OldString string `json:"oldString"`
	NewString string `json:"newString"`
	OriginalFile string `json:"originalFile,omitempty"`
	ReplaceAll   bool   `json:"replaceAll"`
}

func (t Tool) Call(_ context.Context, raw json.RawMessage, tc toolpkg.ToolContext) (<-chan toolpkg.ToolEvent, error) {
	var in input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}

	ch := make(chan toolpkg.ToolEvent, 1)

	// old_string == new_string is invalid
	if in.OldString == in.NewString {
		ch <- toolpkg.ToolEvent{
			Type:   toolpkg.EventResult,
			Result: &toolpkg.ToolResult{Output: "old_string and new_string must differ", IsError: true},
		}
		close(ch)
		return ch, nil
	}

	// Read file
	data, err := os.ReadFile(in.FilePath)
	if err != nil {
		ch <- toolpkg.ToolEvent{
			Type:   toolpkg.EventResult,
			Result: &toolpkg.ToolResult{Output: fmt.Sprintf("read %s: %s", in.FilePath, err), IsError: true},
		}
		close(ch)
		return ch, nil
	}

	content := string(data)

	// Check old_string exists
	if !strings.Contains(content, in.OldString) {
		ch <- toolpkg.ToolEvent{
			Type:   toolpkg.EventResult,
			Result: &toolpkg.ToolResult{Output: "old_string not found in file", IsError: true},
		}
		close(ch)
		return ch, nil
	}

	// Check for multiple matches
	if !in.ReplaceAll {
		count := strings.Count(content, in.OldString)
		if count > 1 {
			ch <- toolpkg.ToolEvent{
				Type:   toolpkg.EventResult,
				Result: &toolpkg.ToolResult{
					Output:  fmt.Sprintf("Found %d occurrences of old_string. Use replace_all to replace all.", count),
					IsError: true,
				},
			}
			close(ch)
			return ch, nil
		}
	}

	// Perform replacement
	var newContent string
	if in.ReplaceAll {
		newContent = strings.ReplaceAll(content, in.OldString, in.NewString)
	} else {
		newContent = strings.Replace(content, in.OldString, in.NewString, 1)
	}

	// Write back
	if err := os.WriteFile(in.FilePath, []byte(newContent), 0644); err != nil {
		ch <- toolpkg.ToolEvent{
			Type:   toolpkg.EventResult,
			Result: &toolpkg.ToolResult{Output: fmt.Sprintf("write %s: %s", in.FilePath, err), IsError: true},
		}
		close(ch)
		return ch, nil
	}

	out := output{
		FilePath:     in.FilePath,
		OldString:    in.OldString,
		NewString:    in.NewString,
		OriginalFile: content,
		ReplaceAll:   in.ReplaceAll,
	}

	data, _ = json.Marshal(out)
	ch <- toolpkg.ToolEvent{
		Type:   toolpkg.EventResult,
		Result: &toolpkg.ToolResult{Output: string(data)},
	}
	close(ch)
	return ch, nil
}

var _ toolpkg.Tool = Tool{}
