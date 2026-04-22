package session

import (
	"sync"
	"time"
)

// ============================================================================
// FileStateCache — LRU cache of file read state
// ============================================================================

const (
	defaultMaxEntries = 100
	defaultMaxBytes   = 25 * 1024 * 1024 // 25 MB
)

// FileState records what the model has seen of a file.
type FileState struct {
	Content   string    // raw disk content at time of read
	Timestamp time.Time // when the file was read
	Offset    int       // line offset (0 = start)
	Limit     int       // line limit (0 = all)
}

// FileStateCache is an LRU cache keyed by file path.
// It tracks what content the model has read, enabling staleness
// checks before edits.
type FileStateCache struct {
	mu        sync.RWMutex
	states    map[string]*cacheEntry
	order     []string // LRU order: oldest first
	maxEntries int
	maxBytes  int
	curBytes  int
}

type cacheEntry struct {
	state    FileState
	lastUsed time.Time
}

// NewFileStateCache creates a cache with default limits.
func NewFileStateCache() *FileStateCache {
	return &FileStateCache{
		states:     make(map[string]*cacheEntry),
		maxEntries: defaultMaxEntries,
		maxBytes:   defaultMaxBytes,
	}
}

// Get returns the cached state for a file path.
func (c *FileStateCache) Get(path string) (FileState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.states[path]
	if !ok {
		return FileState{}, false
	}
	entry.lastUsed = time.Now()
	return entry.state, true
}

// Set stores or updates the cached state for a file path.
func (c *FileStateCache) Set(path string, state FileState) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entrySize := len(state.Content)

	// Evict if we're at capacity
	for c.curBytes+entrySize > c.maxBytes && len(c.states) > 0 {
		c.evictLRU()
	}
	for len(c.states) >= c.maxEntries && len(c.states) > 0 {
		c.evictLRU()
	}

	// If updating existing entry, subtract old size
	if old, exists := c.states[path]; exists {
		c.curBytes -= len(old.state.Content)
	}

	c.states[path] = &cacheEntry{
		state:    state,
		lastUsed: time.Now(),
	}
	c.curBytes += entrySize

	// Update order list
	c.removeFromOrder(path)
	c.order = append(c.order, path)
}

// Delete removes a cached entry.
func (c *FileStateCache) Delete(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.states[path]; ok {
		c.curBytes -= len(entry.state.Content)
		delete(c.states, path)
		c.removeFromOrder(path)
	}
}

// Len returns the number of cached entries.
func (c *FileStateCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.states)
}

// Clear removes all cached entries.
func (c *FileStateCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.states = make(map[string]*cacheEntry)
	c.order = nil
	c.curBytes = 0
}

// SizeBytes returns the total cached content size.
func (c *FileStateCache) SizeBytes() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.curBytes
}

func (c *FileStateCache) evictLRU() {
	if len(c.order) == 0 {
		return
	}
	// Evict the oldest (first in order)
	oldest := c.order[0]
	if entry, ok := c.states[oldest]; ok {
		c.curBytes -= len(entry.state.Content)
		delete(c.states, oldest)
	}
	c.order = c.order[1:]
}

func (c *FileStateCache) removeFromOrder(path string) {
	for i, p := range c.order {
		if p == path {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}
