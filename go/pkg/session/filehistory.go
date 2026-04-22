package session

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// ============================================================================
// FileHistory — backup and rewind for file edits
// ============================================================================

const (
	maxSnapshots = 100
)

// Snapshot records the state of tracked files at a point in time.
type Snapshot struct {
	MessageID string
	Backups   map[string]BackupEntry // filePath → backup info
	Timestamp time.Time
}

// BackupEntry describes a single file backup within a snapshot.
type BackupEntry struct {
	FileName string // backup file name, or "" if file didn't exist
}

// FileHistory manages file edit backups for undo/rewind.
type FileHistory struct {
	historyDir string
	snapshots  []Snapshot
	// trackedFiles maps original file path to the backup file name ("" if file didn't exist).
	trackedFiles map[string]string
}

// NewFileHistory creates a FileHistory for the given session.
// The historyDir is typically ~/.claude/file-history/{sessionID}/.
func NewFileHistory(historyDir string) *FileHistory {
	return &FileHistory{
		historyDir:   historyDir,
		trackedFiles: make(map[string]string),
	}
}

// TrackEdit backs up a file's current content before an edit.
// Call this before modifying the file. If the file doesn't exist,
// it records that fact (backup with empty FileName).
func (fh *FileHistory) TrackEdit(filePath string) error {
	if err := os.MkdirAll(fh.historyDir, 0755); err != nil {
		return fmt.Errorf("create history dir: %w", err)
	}

	// If already tracked in this round, skip
	if _, exists := fh.trackedFiles[filePath]; exists {
		return nil
	}

	// Check if source file exists
	src, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			fh.trackedFiles[filePath] = "" // file doesn't exist
			return nil
		}
		return fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	// Generate backup name
	version := len(fh.snapshots) + 1
	backupName := backupFileName(filePath, version)

	// Copy to backup
	dstPath := filepath.Join(fh.historyDir, backupName)
	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy to backup: %w", err)
	}

	fh.trackedFiles[filePath] = backupName
	return nil
}

// MakeSnapshot records the current state of all tracked files.
// Call this after a message completes to create a restore point.
func (fh *FileHistory) MakeSnapshot(messageID string) error {
	if len(fh.trackedFiles) == 0 {
		return nil
	}

	backups := make(map[string]BackupEntry, len(fh.trackedFiles))
	for filePath, backupName := range fh.trackedFiles {
		backups[filePath] = BackupEntry{
			FileName: backupName,
		}
	}

	snapshot := Snapshot{
		MessageID: messageID,
		Backups:   backups,
		Timestamp: time.Now(),
	}

	fh.snapshots = append(fh.snapshots, snapshot)

	// Trim old snapshots
	if len(fh.snapshots) > maxSnapshots {
		fh.snapshots = fh.snapshots[len(fh.snapshots)-maxSnapshots:]
	}

	// Reset tracked files for next message
	fh.trackedFiles = make(map[string]string)

	return nil
}

// Rewind restores the filesystem to the state at the given snapshot index.
// If a file didn't exist at that snapshot, it is deleted.
func (fh *FileHistory) Rewind(snapshotIndex int) error {
	if snapshotIndex < 0 || snapshotIndex >= len(fh.snapshots) {
		return fmt.Errorf("snapshot index %d out of range [0, %d)", snapshotIndex, len(fh.snapshots))
	}

	snapshot := fh.snapshots[snapshotIndex]

	for filePath, entry := range snapshot.Backups {
		if entry.FileName == "" {
			// File didn't exist at this point — delete it
			os.Remove(filePath)
			continue
		}

		// Restore from backup
		srcPath := filepath.Join(fh.historyDir, entry.FileName)
		src, err := os.Open(srcPath)
		if err != nil {
			return fmt.Errorf("open backup %s: %w", srcPath, err)
		}

		// Ensure parent dir exists
		os.MkdirAll(filepath.Dir(filePath), 0755)

		dst, err := os.Create(filePath)
		if err != nil {
			src.Close()
			return fmt.Errorf("create %s: %w", filePath, err)
		}

		if _, err := io.Copy(dst, src); err != nil {
			src.Close()
			dst.Close()
			return fmt.Errorf("restore %s: %w", filePath, err)
		}
		src.Close()
		dst.Close()
	}

	// Trim snapshots after the rewind point
	fh.snapshots = fh.snapshots[:snapshotIndex+1]

	return nil
}

// Snapshots returns all recorded snapshots.
func (fh *FileHistory) Snapshots() []Snapshot {
	return fh.snapshots
}

// backupFileName generates a deterministic backup filename for a file path.
// Format: {sha256(filePath)[:16]}@v{version}
func backupFileName(filePath string, version int) string {
	h := sha256.Sum256([]byte(filePath))
	return fmt.Sprintf("%x@v%d", h[:8], version)
}
