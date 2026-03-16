package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// HistoryEntry represents a single correction history entry.
type HistoryEntry struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Timestamp int64  `json:"timestamp,omitempty"`
	Count     int    `json:"count,omitempty"` // Times this correction was used
}

// History manages correction history.
type History struct {
	mu        sync.RWMutex
	entries   map[string]HistoryEntry // from -> entry
	configDir string
}

// NewHistory creates a new History instance.
func NewHistory(configDir string) *History {
	h := &History{
		entries:   make(map[string]HistoryEntry),
		configDir: configDir,
	}
	h.load()
	return h
}

// Record records a correction in history.
func (h *History) Record(from, to string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if entry, exists := h.entries[from]; exists {
		entry.Count++
		entry.To = to // Update in case user changed preference
		h.entries[from] = entry
	} else {
		h.entries[from] = HistoryEntry{
			From:  from,
			To:    to,
			Count: 1,
		}
	}

	return h.save()
}

// Lookup finds a historical correction for the given command.
func (h *History) Lookup(from string) (HistoryEntry, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	entry, ok := h.entries[from]
	return entry, ok
}

// Remove removes a history entry.
func (h *History) Remove(from string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.entries, from)
	return h.save()
}

// Clear clears all history.
func (h *History) Clear() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.entries = make(map[string]HistoryEntry)
	return h.save()
}

// List returns all history entries.
func (h *History) List() []HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	entries := make([]HistoryEntry, 0, len(h.entries))
	for _, entry := range h.entries {
		entries = append(entries, entry)
	}
	return entries
}

// Count returns the number of history entries.
func (h *History) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.entries)
}

func (h *History) load() {
	if h.configDir == "" {
		return
	}

	historyFile := filepath.Join(h.configDir, "history.json")
	data, err := os.ReadFile(historyFile)
	if err != nil {
		return // No history file yet
	}

	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return // Invalid JSON, ignore
	}

	for _, entry := range entries {
		h.entries[entry.From] = entry
	}
}

func (h *History) save() error {
	if h.configDir == "" {
		return nil
	}

	if err := os.MkdirAll(h.configDir, 0755); err != nil {
		return err
	}

	entries := make([]HistoryEntry, 0, len(h.entries))
	for _, entry := range h.entries {
		entries = append(entries, entry)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	historyFile := filepath.Join(h.configDir, "history.json")
	return os.WriteFile(historyFile, data, 0644)
}
