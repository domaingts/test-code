package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/example/claude-code-go/pkg/claudetypes"
)

// --- Store tests ---

func TestFileStore_roundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir, "/home/user/project")
	if err != nil {
		t.Fatal(err)
	}

	msgs := []claudetypes.Message{
		claudetypes.UserMessage{
			MessageBase: claudetypes.MessageBase{
				UUID:      "u1",
				Timestamp: time.Now(),
			},
			Content: []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "hello"}},
		},
		claudetypes.AssistantMessage{
			MessageBase: claudetypes.MessageBase{
				UUID:      "a1",
				Timestamp: time.Now(),
			},
			Message: []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "hi there"}},
			Usage:   claudetypes.Usage{InputTokens: 10, OutputTokens: 5},
		},
		claudetypes.SystemMessage{
			MessageBase: claudetypes.MessageBase{
				UUID:      "s1",
				Timestamp: time.Now(),
			},
			Subtype: "informational",
			Level:   "info",
			Content: "system message",
		},
	}

	// Write
	if err := store.Append("sess1", msgs); err != nil {
		t.Fatal(err)
	}

	// Read back
	loaded, err := store.Load("sess1")
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded) != 3 {
		t.Fatalf("loaded %d messages, want 3", len(loaded))
	}

	if loaded[0].GetUUID() != "u1" {
		t.Errorf("msg[0].UUID = %q, want u1", loaded[0].GetUUID())
	}
	if loaded[1].GetUUID() != "a1" {
		t.Errorf("msg[1].UUID = %q, want a1", loaded[1].GetUUID())
	}
	if loaded[2].GetUUID() != "s1" {
		t.Errorf("msg[2].UUID = %q, want s1", loaded[2].GetUUID())
	}
}

func TestFileStore_contentPreserved(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileStore(dir, "/cwd")

	toolUseMsg := claudetypes.UserMessage{
		MessageBase: claudetypes.MessageBase{UUID: "u1", Timestamp: time.Now()},
		Content: []claudetypes.ContentBlock{
			claudetypes.TextBlock{Text: "run this"},
			claudetypes.ToolResultBlock{
				ToolUseID: "tool1",
				Content:   []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "output"}},
				IsError:   false,
			},
		},
	}

	store.Append("s1", []claudetypes.Message{toolUseMsg})
	loaded, _ := store.Load("s1")

	userMsg, ok := loaded[0].(claudetypes.UserMessage)
	if !ok {
		t.Fatal("expected UserMessage")
	}
	if len(userMsg.Content) != 2 {
		t.Fatalf("content blocks = %d, want 2", len(userMsg.Content))
	}
}

func TestFileStore_usagePreserved(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileStore(dir, "/cwd")

	msg := claudetypes.AssistantMessage{
		MessageBase: claudetypes.MessageBase{UUID: "a1", Timestamp: time.Now()},
		Message:     []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "response"}},
		Usage: claudetypes.Usage{
			InputTokens:              100,
			OutputTokens:             50,
			CacheReadInputTokens:     25,
			CacheCreationInputTokens: 10,
		},
	}

	store.Append("s1", []claudetypes.Message{msg})
	loaded, _ := store.Load("s1")

	assistant, ok := loaded[0].(claudetypes.AssistantMessage)
	if !ok {
		t.Fatal("expected AssistantMessage")
	}
	if assistant.Usage.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", assistant.Usage.InputTokens)
	}
	if assistant.Usage.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", assistant.Usage.OutputTokens)
	}
}

func TestFileStore_loadMissing(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileStore(dir, "/cwd")

	msgs, err := store.Load("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if msgs != nil {
		t.Errorf("expected nil for missing session, got %d messages", len(msgs))
	}
}

func TestFileStore_listAndDelete(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileStore(dir, "/cwd")

	store.Append("s1", []claudetypes.Message{
		claudetypes.UserMessage{MessageBase: claudetypes.MessageBase{UUID: "1", Timestamp: time.Now()}},
	})
	store.Append("s2", []claudetypes.Message{
		claudetypes.UserMessage{MessageBase: claudetypes.MessageBase{UUID: "2", Timestamp: time.Now()}},
	})

	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Errorf("List() returned %d, want 2", len(list))
	}

	store.Delete("s1")
	list, _ = store.List()
	if len(list) != 1 {
		t.Errorf("after delete, List() returned %d, want 1", len(list))
	}
}

func TestFileStore_appendMultiple(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileStore(dir, "/cwd")

	store.Append("s1", []claudetypes.Message{
		claudetypes.UserMessage{MessageBase: claudetypes.MessageBase{UUID: "1", Timestamp: time.Now()}},
	})
	store.Append("s1", []claudetypes.Message{
		claudetypes.AssistantMessage{MessageBase: claudetypes.MessageBase{UUID: "2", Timestamp: time.Now()}},
	})

	loaded, _ := store.Load("s1")
	if len(loaded) != 2 {
		t.Errorf("loaded %d messages after 2 appends, want 2", len(loaded))
	}
}

func TestFileStore_jsonlFormat(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileStore(dir, "/cwd")

	store.Append("s1", []claudetypes.Message{
		claudetypes.UserMessage{
			MessageBase: claudetypes.MessageBase{UUID: "u1", Timestamp: time.Now()},
			Content:     []claudetypes.ContentBlock{claudetypes.TextBlock{Text: "hello"}},
		},
	})

	// Verify the raw JSONL has the expected shape
	data, _ := os.ReadFile(filepath.Join(dir, "s1.jsonl"))
	var entry map[string]any
	json.Unmarshal(data, &entry)

	if entry["type"] != "user" {
		t.Errorf("type = %v, want user", entry["type"])
	}
	if entry["uuid"] != "u1" {
		t.Errorf("uuid = %v, want u1", entry["uuid"])
	}
	if entry["sessionId"] != "s1" {
		t.Errorf("sessionId = %v, want s1", entry["sessionId"])
	}
	if entry["cwd"] != "/cwd" {
		t.Errorf("cwd = %v, want /cwd", entry["cwd"])
	}
}

// --- FileStateCache tests ---

func TestFileStateCache_basic(t *testing.T) {
	c := NewFileStateCache()

	c.Set("/a", FileState{Content: "content a", Timestamp: time.Now()})
	c.Set("/b", FileState{Content: "content b", Timestamp: time.Now()})

	if c.Len() != 2 {
		t.Errorf("Len() = %d, want 2", c.Len())
	}

	state, ok := c.Get("/a")
	if !ok {
		t.Fatal("expected to find /a")
	}
	if state.Content != "content a" {
		t.Errorf("Content = %q, want content a", state.Content)
	}

	_, ok = c.Get("/missing")
	if ok {
		t.Error("expected not to find /missing")
	}
}

func TestFileStateCache_eviction(t *testing.T) {
	c := NewFileStateCache()
	c.maxEntries = 3

	c.Set("/a", FileState{Content: "aaa"})
	c.Set("/b", FileState{Content: "bbb"})
	c.Set("/c", FileState{Content: "ccc"})
	c.Set("/d", FileState{Content: "ddd"})

	if c.Len() != 3 {
		t.Errorf("Len() = %d, want 3", c.Len())
	}

	// /a should be evicted (oldest)
	if _, ok := c.Get("/a"); ok {
		t.Error("/a should have been evicted")
	}
	if _, ok := c.Get("/d"); !ok {
		t.Error("/d should still be present")
	}
}

func TestFileStateCache_delete(t *testing.T) {
	c := NewFileStateCache()
	c.Set("/a", FileState{Content: "aaa"})
	c.Delete("/a")

	if c.Len() != 0 {
		t.Errorf("Len() = %d, want 0", c.Len())
	}
}

func TestFileStateCache_clear(t *testing.T) {
	c := NewFileStateCache()
	c.Set("/a", FileState{Content: "aaa"})
	c.Set("/b", FileState{Content: "bbb"})
	c.Clear()

	if c.Len() != 0 {
		t.Errorf("Len() = %d, want 0 after Clear()", c.Len())
	}
}

// --- FileHistory tests ---

func TestFileHistory_trackAndRewind(t *testing.T) {
	dir := t.TempDir()
	historyDir := filepath.Join(dir, "history")
	fh := NewFileHistory(historyDir)

	// Create a file to edit
	filePath := filepath.Join(dir, "test.txt")
	os.WriteFile(filePath, []byte("original content"), 0644)

	// Track before edit
	if err := fh.TrackEdit(filePath); err != nil {
		t.Fatal(err)
	}

	// Make snapshot
	fh.MakeSnapshot("msg1")

	// Simulate an edit
	os.WriteFile(filePath, []byte("modified content"), 0644)

	// Rewind to snapshot 0
	if err := fh.Rewind(0); err != nil {
		t.Fatal(err)
	}

	// Verify file is restored
	data, _ := os.ReadFile(filePath)
	if string(data) != "original content" {
		t.Errorf("content = %q, want original content", string(data))
	}
}

func TestFileHistory_newFileRewind(t *testing.T) {
	dir := t.TempDir()
	historyDir := filepath.Join(dir, "history")
	fh := NewFileHistory(historyDir)

	// Track a file that doesn't exist yet
	filePath := filepath.Join(dir, "new.txt")
	fh.TrackEdit(filePath)
	fh.MakeSnapshot("msg1")

	// Create the file
	os.WriteFile(filePath, []byte("new content"), 0644)

	// Rewind should delete the file (it didn't exist at snapshot time)
	fh.Rewind(0)

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("expected file to be deleted after rewind")
	}
}

func TestFileHistory_multipleSnapshots(t *testing.T) {
	dir := t.TempDir()
	historyDir := filepath.Join(dir, "history")
	fh := NewFileHistory(historyDir)

	filePath := filepath.Join(dir, "test.txt")

	// v1
	os.WriteFile(filePath, []byte("v1"), 0644)
	fh.TrackEdit(filePath)
	fh.MakeSnapshot("msg1")

	// v2
	os.WriteFile(filePath, []byte("v2"), 0644)
	fh.TrackEdit(filePath)
	fh.MakeSnapshot("msg2")

	// v3
	os.WriteFile(filePath, []byte("v3"), 0644)

	// Rewind to snapshot 0 (v1)
	fh.Rewind(0)
	data, _ := os.ReadFile(filePath)
	if string(data) != "v1" {
		t.Errorf("content = %q, want v1", string(data))
	}

	// Snapshots after rewind point should be trimmed
	if len(fh.Snapshots()) != 1 {
		t.Errorf("snapshots = %d, want 1", len(fh.Snapshots()))
	}
}

func TestFileHistory_noOpIfEmpty(t *testing.T) {
	fh := NewFileHistory("/tmp/nosuchdir")
	if err := fh.MakeSnapshot("msg1"); err != nil {
		t.Fatal(err)
	}
	if len(fh.Snapshots()) != 0 {
		t.Error("should not create snapshot with no tracked files")
	}
}
