package fileread

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	toolpkg "github.com/example/claude-code-go/pkg/tool"
)

// Tool implements the "Read" tool — reads a file with optional offset/limit.
type Tool struct{}

func (Tool) Name() string { return "Read" }

func (Tool) Schema() toolpkg.JSONSchema {
	return toolpkg.JSONSchema{
		Type: "object",
		Properties: map[string]toolpkg.Property{
			"file_path": {Type: "string", Description: "Absolute path to the file to read"},
			"offset":    {Type: "integer", Description: "Starting line number (1-based)"},
			"limit":     {Type: "integer", Description: "Number of lines to read"},
		},
		Required: []string{"file_path"},
	}
}

type input struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

type output struct {
	FilePath    string `json:"filePath"`
	Content     string `json:"content"`
	NumLines    int    `json:"numLines"`
	StartLine   int    `json:"startLine"`
	TotalLines  int    `json:"totalLines"`
}

func (t Tool) Call(_ context.Context, raw json.RawMessage, tc toolpkg.ToolContext) (<-chan toolpkg.ToolEvent, error) {
	var in input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}

	ch := make(chan toolpkg.ToolEvent, 1)

	content, numLines, totalLines, startLine, err := readFile(in.FilePath, in.Offset, in.Limit)
	if err != nil {
		ch <- toolpkg.ToolEvent{
			Type:  toolpkg.EventResult,
			Result: &toolpkg.ToolResult{Output: err.Error(), IsError: true},
		}
		close(ch)
		return ch, nil
	}

	out := output{
		FilePath:   in.FilePath,
		Content:    content,
		NumLines:   numLines,
		StartLine:  startLine,
		TotalLines: totalLines,
	}

	data, _ := json.Marshal(out)
	ch <- toolpkg.ToolEvent{
		Type:   toolpkg.EventResult,
		Result: &toolpkg.ToolResult{Output: string(data)},
	}
	close(ch)
	return ch, nil
}

func readFile(path string, offset, limit int) (content string, numLines, totalLines, startLine int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("read %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line

	var allLines []string
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", 0, 0, 0, fmt.Errorf("scan %s: %w", path, err)
	}

	totalLines = len(allLines)

	// offset is 1-based
	if offset <= 0 {
		offset = 1
	}
	startIdx := offset - 1
	if startIdx >= totalLines {
		return "", 0, totalLines, offset, nil
	}

	endIdx := totalLines
	if limit > 0 && startIdx+limit < totalLines {
		endIdx = startIdx + limit
	}

	selected := allLines[startIdx:endIdx]

	// Format with line numbers
	var buf strings.Builder
	for i, line := range selected {
		lineNum := startIdx + i + 1
		fmt.Fprintf(&buf, "%d\t%s\n", lineNum, line)
	}

	return buf.String(), len(selected), totalLines, offset, nil
}

// Check that Tool implements the interface.
var _ toolpkg.Tool = Tool{}
