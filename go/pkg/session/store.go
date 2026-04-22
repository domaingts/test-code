package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/example/claude-code-go/pkg/claudetypes"
)

// ============================================================================
// Store interface
// ============================================================================

// SessionInfo describes a persisted session.
type SessionInfo struct {
	ID        string
	Path      string
	StartTime time.Time
}

// Store is the interface for session persistence.
type Store interface {
	// Load reads all messages from a session file.
	Load(sessionID string) ([]claudetypes.Message, error)

	// Append writes messages to a session file (JSONL append).
	Append(sessionID string, msgs []claudetypes.Message) error

	// List returns all sessions in a project directory.
	List() ([]SessionInfo, error)

	// Delete removes a session file.
	Delete(sessionID string) error
}

// ============================================================================
// FileStore — JSONL on-disk implementation
// ============================================================================

// FileStore stores sessions as JSONL files in a project directory.
type FileStore struct {
	// Dir is the project session directory (e.g. ~/.claude/projects/<sanitized-cwd>).
	Dir string
	// CWD is the current working directory, stamped into entries.
	CWD string
}

// NewFileStore creates a FileStore at the given directory.
// Creates the directory if it doesn't exist.
func NewFileStore(dir, cwd string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}
	return &FileStore{Dir: dir, CWD: cwd}, nil
}

// sessionPath returns the JSONL file path for a session.
func (s *FileStore) sessionPath(sessionID string) string {
	return filepath.Join(s.Dir, sessionID+".jsonl")
}

// Load reads all messages from a session file.
// Returns empty slice if the file doesn't exist.
func (s *FileStore) Load(sessionID string) ([]claudetypes.Message, error) {
	path := s.sessionPath(sessionID)

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var messages []claudetypes.Message
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry TranscriptEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Skip malformed lines rather than failing the whole load
			continue
		}

		// Skip non-transcript entries (summaries, metadata, etc.)
		if !isTranscriptEntry(entry.Type) {
			continue
		}

		msg, err := EntryToMessage(entry)
		if err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}

	return messages, nil
}

// Append writes messages to a session file (JSONL append).
func (s *FileStore) Append(sessionID string, msgs []claudetypes.Message) error {
	path := s.sessionPath(sessionID)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open %s for append: %w", path, err)
	}
	defer f.Close()

	var buf strings.Builder
	parentUUID := ""

	for _, msg := range msgs {
		entry, err := MessageToEntry(msg, sessionID, s.CWD, parentUUID)
		if err != nil {
			return fmt.Errorf("marshal entry: %w", err)
		}

		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("marshal JSON: %w", err)
		}

		buf.Write(data)
		buf.WriteByte('\n')
		parentUUID = msg.GetUUID()
	}

	if _, err := io.WriteString(f, buf.String()); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// List returns all sessions in the store directory, sorted by start time (newest first).
func (s *FileStore) List() ([]SessionInfo, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read dir %s: %w", s.Dir, err)
	}

	var sessions []SessionInfo
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}

		id := strings.TrimSuffix(e.Name(), ".jsonl")
		info, err := e.Info()
		if err != nil {
			continue
		}

		sessions = append(sessions, SessionInfo{
			ID:        id,
			Path:      filepath.Join(s.Dir, e.Name()),
			StartTime: info.ModTime(),
		})
	}

	// Sort newest first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})

	return sessions, nil
}

// Delete removes a session file.
func (s *FileStore) Delete(sessionID string) error {
	path := s.sessionPath(sessionID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete %s: %w", path, err)
	}
	return nil
}

func isTranscriptEntry(t EntryType) bool {
	switch t {
	case EntryUser, EntryAssistant, EntrySystem, EntryAttachment:
		return true
	default:
		return false
	}
}
