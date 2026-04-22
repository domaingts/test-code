package filewrite

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	toolpkg "github.com/example/claude-code-go/pkg/tool"
)

// Tool implements the "Write" tool — writes full file content.
type Tool struct{}

func (Tool) Name() string { return "Write" }

func (Tool) Schema() toolpkg.JSONSchema {
	return toolpkg.JSONSchema{
		Type: "object",
		Properties: map[string]toolpkg.Property{
			"file_path": {Type: "string", Description: "Absolute path to the file to write"},
			"content":   {Type: "string", Description: "Full content to write"},
		},
		Required: []string{"file_path", "content"},
	}
}

type input struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type output struct {
	FilePath string `json:"filePath"`
	Type     string `json:"type"` // "create" or "update"
}

func (t Tool) Call(_ context.Context, raw json.RawMessage, tc toolpkg.ToolContext) (<-chan toolpkg.ToolEvent, error) {
	var in input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}

	ch := make(chan toolpkg.ToolEvent, 1)

	// Determine if creating or updating
	writeType := "update"
	if _, err := os.Stat(in.FilePath); os.IsNotExist(err) {
		writeType = "create"
	}

	// Ensure parent directory exists
	dir := filepath.Dir(in.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		ch <- toolpkg.ToolEvent{
			Type:   toolpkg.EventResult,
			Result: &toolpkg.ToolResult{Output: fmt.Sprintf("create directory: %s", err), IsError: true},
		}
		close(ch)
		return ch, nil
	}

	// Write file
	if err := os.WriteFile(in.FilePath, []byte(in.Content), 0644); err != nil {
		ch <- toolpkg.ToolEvent{
			Type:   toolpkg.EventResult,
			Result: &toolpkg.ToolResult{Output: fmt.Sprintf("write %s: %s", in.FilePath, err), IsError: true},
		}
		close(ch)
		return ch, nil
	}

	out := output{
		FilePath: in.FilePath,
		Type:     writeType,
	}

	data, _ := json.Marshal(out)
	ch <- toolpkg.ToolEvent{
		Type:   toolpkg.EventResult,
		Result: &toolpkg.ToolResult{Output: string(data)},
	}
	close(ch)
	return ch, nil
}

var _ toolpkg.Tool = Tool{}
