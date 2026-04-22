package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	toolpkg "github.com/example/claude-code-go/pkg/tool"
	"github.com/example/claude-code-go/pkg/tools/bash"
	"github.com/example/claude-code-go/pkg/tools/fileedit"
	"github.com/example/claude-code-go/pkg/tools/fileread"
	"github.com/example/claude-code-go/pkg/tools/filewrite"
	"github.com/example/claude-code-go/pkg/tools/glob"
	"github.com/example/claude-code-go/pkg/tools/grep"
	"github.com/example/claude-code-go/pkg/tools/todowrite"
)

// --- Schema tests ---

func TestToolNames(t *testing.T) {
	tools := map[string]toolpkg.Tool{
		"Bash":      bash.Tool{},
		"Read":      fileread.Tool{},
		"Edit":      fileedit.Tool{},
		"Write":     filewrite.Tool{},
		"Glob":      glob.Tool{},
		"Grep":      grep.Tool{},
		"TodoWrite": &todowrite.Tool{},
	}
	for name, tool := range tools {
		if tool.Name() != name {
			t.Errorf("tool.Name() = %q, want %q", tool.Name(), name)
		}
	}
}

func TestSchemas_valid(t *testing.T) {
	tools := []toolpkg.Tool{
		bash.Tool{},
		fileread.Tool{},
		fileedit.Tool{},
		filewrite.Tool{},
		glob.Tool{},
		grep.Tool{},
		&todowrite.Tool{},
	}
	for _, tool := range tools {
		schema := tool.Schema()
		if schema.Type != "object" {
			t.Errorf("%s: schema.Type = %q, want object", tool.Name(), schema.Type)
		}
		if len(schema.Properties) == 0 {
			t.Errorf("%s: schema has no properties", tool.Name())
		}
		if len(schema.Required) == 0 {
			t.Errorf("%s: schema has no required fields", tool.Name())
		}
		// Verify it marshals to valid JSON
		_, err := json.Marshal(schema)
		if err != nil {
			t.Errorf("%s: schema marshal error: %v", tool.Name(), err)
		}
	}
}

// --- FileReadTool tests ---

func TestFileReadTool_full(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	writeFile(t, path, "line 1\nline 2\nline 3\nline 4\nline 5\n")

	tool := fileread.Tool{}
	ch, err := tool.Call(context.Background(), toJSON(map[string]any{"file_path": path}), tc(dir))
	if err != nil {
		t.Fatal(err)
	}
	evt := <-ch
	if evt.Type != toolpkg.EventResult {
		t.Fatalf("event type = %q, want result", evt.Type)
	}
	if evt.Result.IsError {
		t.Fatalf("unexpected error: %s", evt.Result.Output)
	}

	var out map[string]any
	json.Unmarshal([]byte(evt.Result.Output), &out)
	if out["totalLines"].(float64) != 5 {
		t.Errorf("totalLines = %v, want 5", out["totalLines"])
	}
}

func TestFileReadTool_offsetLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	writeFile(t, path, "line 1\nline 2\nline 3\nline 4\nline 5\n")

	tool := fileread.Tool{}
	ch, err := tool.Call(context.Background(), toJSON(map[string]any{
		"file_path": path,
		"offset":    2,
		"limit":     2,
	}), tc(dir))
	if err != nil {
		t.Fatal(err)
	}
	evt := <-ch

	var out map[string]any
	json.Unmarshal([]byte(evt.Result.Output), &out)
	if out["numLines"].(float64) != 2 {
		t.Errorf("numLines = %v, want 2", out["numLines"])
	}
	if out["startLine"].(float64) != 2 {
		t.Errorf("startLine = %v, want 2", out["startLine"])
	}
}

func TestFileReadTool_missing(t *testing.T) {
	tool := fileread.Tool{}
	ch, err := tool.Call(context.Background(), toJSON(map[string]any{
		"file_path": "/tmp/no-such-file-99999.txt",
	}), tc("/tmp"))
	if err != nil {
		t.Fatal(err)
	}
	evt := <-ch
	if !evt.Result.IsError {
		t.Error("expected error for missing file")
	}
}

// --- FileWriteTool tests ---

func TestFileWriteTool_create(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "new.txt")

	tool := filewrite.Tool{}
	ch, err := tool.Call(context.Background(), toJSON(map[string]any{
		"file_path": path,
		"content":   "hello world",
	}), tc(dir))
	if err != nil {
		t.Fatal(err)
	}
	evt := <-ch
	if evt.Result.IsError {
		t.Fatalf("unexpected error: %s", evt.Result.Output)
	}

	var out map[string]any
	json.Unmarshal([]byte(evt.Result.Output), &out)
	if out["type"] != "create" {
		t.Errorf("type = %v, want create", out["type"])
	}

	// Verify file contents
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("content = %q, want hello world", string(data))
	}
}

// --- FileEditTool tests ---

func TestFileEditTool_success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	writeFile(t, path, "hello world\n")

	tool := fileedit.Tool{}
	ch, err := tool.Call(context.Background(), toJSON(map[string]any{
		"file_path":  path,
		"old_string": "world",
		"new_string": "golang",
	}), tc(dir))
	if err != nil {
		t.Fatal(err)
	}
	evt := <-ch
	if evt.Result.IsError {
		t.Fatalf("unexpected error: %s", evt.Result.Output)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "hello golang\n" {
		t.Errorf("content = %q, want hello golang\\n", string(data))
	}
}

func TestFileEditTool_notFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	writeFile(t, path, "hello world\n")

	tool := fileedit.Tool{}
	ch, err := tool.Call(context.Background(), toJSON(map[string]any{
		"file_path":  path,
		"old_string": "notexist",
		"new_string": "replacement",
	}), tc(dir))
	if err != nil {
		t.Fatal(err)
	}
	evt := <-ch
	if !evt.Result.IsError {
		t.Error("expected error for missing old_string")
	}
}

func TestFileEditTool_sameString(t *testing.T) {
	tool := fileedit.Tool{}
	ch, err := tool.Call(context.Background(), toJSON(map[string]any{
		"file_path":  "/dev/null",
		"old_string": "same",
		"new_string": "same",
	}), tc("/tmp"))
	if err != nil {
		t.Fatal(err)
	}
	evt := <-ch
	if !evt.Result.IsError {
		t.Error("expected error when old_string == new_string")
	}
}

// --- GlobTool tests ---

func TestGlobTool(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.go"), "")
	writeFile(t, filepath.Join(dir, "b.go"), "")
	writeFile(t, filepath.Join(dir, "c.txt"), "")

	tool := glob.Tool{}
	ch, err := tool.Call(context.Background(), toJSON(map[string]any{
		"pattern": "*.go",
		"path":    dir,
	}), tc(dir))
	if err != nil {
		t.Fatal(err)
	}
	evt := <-ch

	var out map[string]any
	json.Unmarshal([]byte(evt.Result.Output), &out)
	if out["numFiles"].(float64) != 2 {
		t.Errorf("numFiles = %v, want 2", out["numFiles"])
	}
}

// --- GrepTool tests ---

func TestGrepTool_filesWithMatches(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "hello world\n")
	writeFile(t, filepath.Join(dir, "b.txt"), "goodbye world\n")
	writeFile(t, filepath.Join(dir, "c.txt"), "hello golang\n")

	tool := grep.Tool{}
	ch, err := tool.Call(context.Background(), toJSON(map[string]any{
		"pattern": "hello",
		"path":    dir,
	}), tc(dir))
	if err != nil {
		t.Fatal(err)
	}
	evt := <-ch

	var out map[string]any
	json.Unmarshal([]byte(evt.Result.Output), &out)
	if out["numFiles"].(float64) != 2 {
		t.Errorf("numFiles = %v, want 2", out["numFiles"])
	}
}

func TestGrepTool_content(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "hello world\ngoodbye\nhello again\n")

	tool := grep.Tool{}
	ch, err := tool.Call(context.Background(), toJSON(map[string]any{
		"pattern":     "hello",
		"path":        dir,
		"output_mode": "content",
	}), tc(dir))
	if err != nil {
		t.Fatal(err)
	}
	evt := <-ch

	var out map[string]any
	json.Unmarshal([]byte(evt.Result.Output), &out)
	if out["numLines"].(float64) != 2 {
		t.Errorf("numLines = %v, want 2", out["numLines"])
	}
}

func TestGrepTool_invalidRegex(t *testing.T) {
	tool := grep.Tool{}
	ch, err := tool.Call(context.Background(), toJSON(map[string]any{
		"pattern": "[invalid",
		"path":    "/tmp",
	}), tc("/tmp"))
	if err != nil {
		t.Fatal(err)
	}
	evt := <-ch
	if !evt.Result.IsError {
		t.Error("expected error for invalid regex")
	}
}

// --- TodoWriteTool tests ---

func TestTodoWriteTool(t *testing.T) {
	tool := &todowrite.Tool{}
	ch, err := tool.Call(context.Background(), toJSON(map[string]any{
		"todos": []map[string]any{
			{"content": "Write code", "status": "in_progress", "activeForm": "Writing code"},
			{"content": "Run tests", "status": "pending", "activeForm": "Running tests"},
		},
	}), tc("/tmp"))
	if err != nil {
		t.Fatal(err)
	}
	evt := <-ch
	if evt.Result.IsError {
		t.Fatalf("unexpected error: %s", evt.Result.Output)
	}

	var out map[string]any
	json.Unmarshal([]byte(evt.Result.Output), &out)
	newTodos := out["newTodos"].([]any)
	if len(newTodos) != 2 {
		t.Errorf("newTodos length = %d, want 2", len(newTodos))
	}
}

func TestTodoWriteTool_clearOnAllDone(t *testing.T) {
	tool := &todowrite.Tool{}
	// First write some todos
	ch, _ := tool.Call(context.Background(), toJSON(map[string]any{
		"todos": []map[string]any{
			{"content": "Task 1", "status": "in_progress", "activeForm": "Doing task 1"},
		},
	}), tc("/tmp"))
	<-ch

	// Now complete all
	ch, _ = tool.Call(context.Background(), toJSON(map[string]any{
		"todos": []map[string]any{
			{"content": "Task 1", "status": "completed", "activeForm": "Done"},
		},
	}), tc("/tmp"))
	evt := <-ch

	var out map[string]any
	json.Unmarshal([]byte(evt.Result.Output), &out)
	// After all-completed, the tool should have stored nil internally
	// but the newTodos in the response still shows what was passed in
	newTodos := out["newTodos"].([]any)
	if len(newTodos) != 1 {
		t.Errorf("newTodos length = %d, want 1", len(newTodos))
	}
}

// --- BashTool test ---

func TestBashTool_simple(t *testing.T) {
	tool := bash.Tool{}
	ch, err := tool.Call(context.Background(), toJSON(map[string]any{
		"command": "echo hello",
	}), tc("/tmp"))
	if err != nil {
		t.Fatal(err)
	}
	evt := <-ch

	var out map[string]any
	json.Unmarshal([]byte(evt.Result.Output), &out)
	if out["stdout"] != "hello\n" {
		t.Errorf("stdout = %q, want hello\\n", out["stdout"])
	}
	if out["exitCode"].(float64) != 0 {
		t.Errorf("exitCode = %v, want 0", out["exitCode"])
	}
}

func TestBashTool_failure(t *testing.T) {
	tool := bash.Tool{}
	ch, err := tool.Call(context.Background(), toJSON(map[string]any{
		"command": "exit 42",
	}), tc("/tmp"))
	if err != nil {
		t.Fatal(err)
	}
	evt := <-ch

	var out map[string]any
	json.Unmarshal([]byte(evt.Result.Output), &out)
	if out["exitCode"].(float64) != 42 {
		t.Errorf("exitCode = %v, want 42", out["exitCode"])
	}
}

// --- Registry integration test ---

func TestRegistry_allTools(t *testing.T) {
	reg := toolpkg.NewRegistry()
	reg.Register(bash.Tool{})
	reg.Register(fileread.Tool{})
	reg.Register(fileedit.Tool{})
	reg.Register(filewrite.Tool{})
	reg.Register(glob.Tool{})
	reg.Register(grep.Tool{})
	reg.Register(&todowrite.Tool{})

	if reg.Len() != 7 {
		t.Errorf("registry has %d tools, want 7", reg.Len())
	}

	names := reg.Names()
	expected := []string{"Bash", "Edit", "Glob", "Grep", "Read", "TodoWrite", "Write"}
	for i, n := range names {
		if n != expected[i] {
			t.Errorf("names[%d] = %q, want %q", i, n, expected[i])
		}
	}
}

// --- Helpers ---

func tc(cwd string) toolpkg.ToolContext {
	return toolpkg.ToolContext{
		ToolUseID: "test",
		CWD:       cwd,
	}
}

func toJSON(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
