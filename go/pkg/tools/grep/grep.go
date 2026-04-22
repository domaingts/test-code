package grep

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	toolpkg "github.com/example/claude-code-go/pkg/tool"
)

// Tool implements the "Grep" tool — regex search across files.
type Tool struct{}

func (Tool) Name() string { return "Grep" }

func (Tool) Schema() toolpkg.JSONSchema {
	return toolpkg.JSONSchema{
		Type: "object",
		Properties: map[string]toolpkg.Property{
			"pattern":     {Type: "string", Description: "Regex pattern to search for"},
			"path":        {Type: "string", Description: "File or directory to search in"},
			"glob":        {Type: "string", Description: "Glob filter (e.g. '*.go')"},
			"output_mode": {Type: "string", Description: "Output format: files_with_matches, content, or count", Enum: []string{"files_with_matches", "content", "count"}},
			"-i":          {Type: "boolean", Description: "Case insensitive search"},
			"context":     {Type: "integer", Description: "Lines of context around matches (content mode)"},
			"-C":          {Type: "integer", Description: "Alias for context"},
			"-B":          {Type: "integer", Description: "Lines before each match"},
			"-A":          {Type: "integer", Description: "Lines after each match"},
			"-n":          {Type: "boolean", Description: "Show line numbers (content mode)"},
			"head_limit":  {Type: "integer", Description: "Max results to return (0 = unlimited)"},
			"offset":      {Type: "integer", Description: "Skip first N results"},
			"multiline":   {Type: "boolean", Description: "Enable multiline matching"},
		},
		Required: []string{"pattern"},
	}
}

type input struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path,omitempty"`
	Glob       string `json:"glob,omitempty"`
	OutputMode string `json:"output_mode,omitempty"`
	CaseInsensitive bool `json:"-i,omitempty"`
	Context    int    `json:"context,omitempty"`
	C          int    `json:"-C,omitempty"`
	B          int    `json:"-B,omitempty"`
	A          int    `json:"-A,omitempty"`
	N          bool   `json:"-n,omitempty"`
	HeadLimit  int    `json:"head_limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	Multiline  bool   `json:"multiline,omitempty"`
}

type output struct {
	Mode        string   `json:"mode,omitempty"`
	Filenames   []string `json:"filenames"`
	NumFiles    int      `json:"numFiles"`
	Content     string   `json:"content,omitempty"`
	NumLines    int      `json:"numLines,omitempty"`
	NumMatches  int      `json:"numMatches,omitempty"`
}

const defaultHeadLimit = 250

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

	if in.OutputMode == "" {
		in.OutputMode = "files_with_matches"
	}
	headLimit := in.HeadLimit
	if headLimit == 0 {
		headLimit = defaultHeadLimit
	}

	// Build regex
	pattern := in.Pattern
	if in.Multiline {
		pattern = "(?s)" + pattern
	}
	if in.CaseInsensitive {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		ch <- toolpkg.ToolEvent{
			Type:   toolpkg.EventResult,
			Result: &toolpkg.ToolResult{Output: fmt.Sprintf("invalid regex: %s", err), IsError: true},
		}
		close(ch)
		return ch, nil
	}

	// Find matching files
	files := findMatchingFiles(searchPath, in.Glob)

	var result output
	result.Mode = in.OutputMode

	switch in.OutputMode {
	case "files_with_matches":
		var matched []string
		for _, f := range files {
			if fileMatches(f, re) {
				rel, _ := filepath.Rel(searchPath, f)
				matched = append(matched, rel)
			}
		}
		matched = applyOffsetLimit(matched, in.Offset, headLimit)
		result.Filenames = matched
		result.NumFiles = len(matched)

	case "content":
			ctxLines := in.Context
			if in.C > 0 {
				ctxLines = in.C
			}
			content, numLines, _ := searchContent(files, re, in.Offset, headLimit, in.B, in.A, ctxLines, in.N, searchPath)
		result.Content = content
		result.NumLines = numLines
		result.NumFiles = countMatchingFiles(files, re)

	case "count":
		content, numLines, numMatches := searchCount(files, re, in.Offset, headLimit, searchPath)
		result.Content = content
		result.NumLines = numLines
		result.NumMatches = numMatches
		result.NumFiles = countMatchingFiles(files, re)
	}

	_ = time.Since(start) // could add DurationMs

	data, _ := json.Marshal(result)
	ch <- toolpkg.ToolEvent{
		Type:   toolpkg.EventResult,
		Result: &toolpkg.ToolResult{Output: string(data)},
	}
	close(ch)
	return ch, nil
}

func findMatchingFiles(root, globPattern string) []string {
	var files []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if err == nil && info.IsDir() {
				name := info.Name()
				if len(name) > 1 && name[0] == '.' {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if globPattern != "" {
			matched, _ := filepath.Match(globPattern, info.Name())
			if !matched {
				return nil
			}
		}
		files = append(files, path)
		return nil
	})
	slices.Sort(files)
	return files
}

func fileMatches(path string, re *regexp.Regexp) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		if re.Match(scanner.Bytes()) {
			return true
		}
	}
	return false
}

func countMatchingFiles(files []string, re *regexp.Regexp) int {
	count := 0
	for _, f := range files {
		if fileMatches(f, re) {
			count++
		}
	}
	return count
}

type matchLine struct {
	File    string
	LineNum int
	Text    string
}

func searchContent(files []string, re *regexp.Regexp, offset, limit, before, after, contextLines int, showLineNum bool, basePath string) (string, int, int) {
	var allMatches []matchLine
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
		lines := []string{}
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		f.Close()

		if contextLines > 0 && before == 0 {
			before = contextLines
		}
		if contextLines > 0 && after == 0 {
			after = contextLines
		}

		rel, _ := filepath.Rel(basePath, file)
		matchedLines := map[int]bool{}
		for i, line := range lines {
			if re.MatchString(line) {
				matchedLines[i] = true
			}
		}

		// Expand context
		expandedLines := map[int]bool{}
		for lineIdx := range matchedLines {
			expandedLines[lineIdx] = true
			for j := 1; j <= before; j++ {
				if lineIdx-j >= 0 {
					expandedLines[lineIdx-j] = true
				}
			}
			for j := 1; j <= after; j++ {
				if lineIdx+j < len(lines) {
					expandedLines[lineIdx+j] = true
				}
			}
		}

		indices := make([]int, 0, len(expandedLines))
		for idx := range expandedLines {
			indices = append(indices, idx)
		}
		slices.Sort(indices)

		for _, idx := range indices {
			line := rel
			if showLineNum {
				line += fmt.Sprintf(":%d", idx+1)
			}
			line += ":" + lines[idx]
			allMatches = append(allMatches, matchLine{
				File:    rel,
				LineNum: idx + 1,
				Text:    line,
			})
		}
	}

	if offset > 0 && offset < len(allMatches) {
		allMatches = allMatches[offset:]
	}
	if limit > 0 && limit < len(allMatches) {
		allMatches = allMatches[:limit]
	}

	var buf strings.Builder
	for _, m := range allMatches {
		buf.WriteString(m.Text)
		buf.WriteString("\n")
	}

	return buf.String(), len(allMatches), len(allMatches)
}

func searchCount(files []string, re *regexp.Regexp, offset, limit int, basePath string) (string, int, int) {
	type countResult struct {
		file  string
		count int
	}

	var results []countResult
	totalMatches := 0

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			continue
		}
		count := 0
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
		for scanner.Scan() {
			count += len(re.FindAllString(scanner.Text(), -1))
		}
		f.Close()

		if count > 0 {
			rel, _ := filepath.Rel(basePath, file)
			results = append(results, countResult{rel, count})
			totalMatches += count
		}
	}

	if offset > 0 && offset < len(results) {
		results = results[offset:]
	}
	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}

	var buf strings.Builder
	for _, r := range results {
		fmt.Fprintf(&buf, "%s:%d\n", r.file, r.count)
	}

	return buf.String(), len(results), totalMatches
}

func applyOffsetLimit(items []string, offset, limit int) []string {
	if offset > 0 && offset < len(items) {
		items = items[offset:]
	}
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items
}

var _ toolpkg.Tool = Tool{}
