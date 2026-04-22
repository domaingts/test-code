package glob

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	toolpkg "github.com/example/claude-code-go/pkg/tool"
)

// Tool implements the "Glob" tool — file pattern matching.
type Tool struct{}

func (Tool) Name() string { return "Glob" }

func (Tool) Schema() toolpkg.JSONSchema {
	return toolpkg.JSONSchema{
		Type: "object",
		Properties: map[string]toolpkg.Property{
			"pattern": {Type: "string", Description: "Glob pattern (e.g. '**/*.go', 'src/**/*.ts')"},
			"path":    {Type: "string", Description: "Directory to search in (defaults to CWD)"},
		},
		Required: []string{"pattern"},
	}
}

type input struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

type output struct {
	Filenames []string `json:"filenames"`
	NumFiles  int      `json:"numFiles"`
	Truncated bool     `json:"truncated"`
	DurationMs int64   `json:"durationMs"`
}

const maxResults = 100

func (t Tool) Call(_ context.Context, raw json.RawMessage, tc toolpkg.ToolContext) (<-chan toolpkg.ToolEvent, error) {
	var in input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}

	ch := make(chan toolpkg.ToolEvent, 1)
	start := time.Now()

	searchPath := in.Path
	if searchPath == "" {
		searchPath = tc.CWD
	}

	matches, truncated := findFiles(searchPath, in.Pattern)

	// Convert to relative paths
	var relPaths []string
	for _, m := range matches {
		rel, err := filepath.Rel(searchPath, m)
		if err != nil {
			rel = m
		}
		relPaths = append(relPaths, rel)
	}
	slices.Sort(relPaths)

	out := output{
		Filenames:  relPaths,
		NumFiles:   len(relPaths),
		Truncated:  truncated,
		DurationMs: time.Since(start).Milliseconds(),
	}

	data, _ := json.Marshal(out)
	ch <- toolpkg.ToolEvent{
		Type:   toolpkg.EventResult,
		Result: &toolpkg.ToolResult{Output: string(data)},
	}
	close(ch)
	return ch, nil
}

func findFiles(root, pattern string) ([]string, bool) {
	var results []string
	truncated := false

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			// Skip hidden dirs
			if name := info.Name(); len(name) > 1 && name[0] == '.' && name != "." {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		matched, err := filepath.Match(pattern, rel)
		if err != nil {
			return nil
		}
		if !matched {
			// Also try matching just the filename
			matched, _ = filepath.Match(pattern, info.Name())
		}
		if matched {
			if len(results) >= maxResults {
				truncated = true
				return filepath.SkipAll
			}
			results = append(results, path)
		}
		return nil
	})

	if err != nil && len(results) == 0 {
		// Try go:filepath.Glob for simple patterns
		if m, err2 := filepath.Glob(filepath.Join(root, pattern)); err2 == nil {
			return m, len(m) > maxResults
		}
	}

	return results, truncated
}

var _ toolpkg.Tool = Tool{}
