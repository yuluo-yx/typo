package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHistory_Record(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHistory(tmpDir)

	// Record a correction
	if err := h.Record("gut", "git"); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	// Verify it was recorded
	entry, ok := h.Lookup("gut")
	if !ok {
		t.Error("Expected to find recorded entry")
	}
	if entry.To != "git" {
		t.Errorf("Expected 'git', got %q", entry.To)
	}
	if entry.Count != 1 {
		t.Errorf("Expected count 1, got %d", entry.Count)
	}

	// Record again, should increment count
	h.Record("gut", "git")
	entry, _ = h.Lookup("gut")
	if entry.Count != 2 {
		t.Errorf("Expected count 2, got %d", entry.Count)
	}

	// Verify it was saved to file
	historyFile := filepath.Join(tmpDir, usageHistoryFileName)
	if _, err := os.Stat(historyFile); os.IsNotExist(err) {
		t.Error("Expected usage history file to be created")
	}
}

func TestHistory_Lookup(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHistory(tmpDir)

	// Lookup non-existent entry
	_, ok := h.Lookup("nonexistent")
	if ok {
		t.Error("Expected not to find non-existent entry")
	}

	// Record and lookup
	h.Record("test", "correct")
	entry, ok := h.Lookup("test")
	if !ok {
		t.Error("Expected to find entry")
	}
	if entry.To != "correct" {
		t.Errorf("Expected 'correct', got %q", entry.To)
	}
}

func TestHistory_Remove(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHistory(tmpDir)

	// Record then remove
	h.Record("test", "correct")
	h.Remove("test")

	_, ok := h.Lookup("test")
	if ok {
		t.Error("Expected entry to be removed")
	}
}

func TestHistory_RemoveEntriesForCommandWord(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHistory(tmpDir)

	if err := h.Record("gut status", "git status"); err != nil {
		t.Fatalf("Record failed: %v", err)
	}
	if err := h.Record("sudo gut status", "sudo git status"); err != nil {
		t.Fatalf("Record failed: %v", err)
	}
	if err := h.Record("docker ps", "docker ps"); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	if err := h.RemoveEntriesForCommandWord("gut"); err != nil {
		t.Fatalf("RemoveEntriesForCommandWord failed: %v", err)
	}

	if _, ok := h.Lookup("gut status"); ok {
		t.Fatal("Expected direct command history entry to be removed")
	}
	if _, ok := h.Lookup("sudo gut status"); ok {
		t.Fatal("Expected wrapped command history entry to be removed")
	}
	if _, ok := h.Lookup("docker ps"); !ok {
		t.Fatal("Expected unrelated history entry to remain")
	}
}

func TestHistory_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHistory(tmpDir)

	// Record multiple entries
	h.Record("test1", "correct1")
	h.Record("test2", "correct2")

	// Clear
	if err := h.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify all cleared
	if h.Count() != 0 {
		t.Errorf("Expected count 0 after clear, got %d", h.Count())
	}
}

func TestHistory_List(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHistory(tmpDir)

	// Record multiple entries
	h.Record("test1", "correct1")
	h.Record("test2", "correct2")

	entries := h.List()
	if len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}

	// Verify entries are present
	found1, found2 := false, false
	for _, e := range entries {
		if e.From == "test1" {
			found1 = true
		}
		if e.From == "test2" {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Error("Expected both entries to be present")
	}
}

func TestHistory_Count(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHistory(tmpDir)

	if h.Count() != 0 {
		t.Errorf("Expected count 0, got %d", h.Count())
	}

	h.Record("test", "correct")
	if h.Count() != 1 {
		t.Errorf("Expected count 1, got %d", h.Count())
	}
}

func TestHistory_UpdatePreference(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHistory(tmpDir)

	// Record initial correction
	h.Record("test", "correct1")

	// User changes preference
	h.Record("test", "correct2")

	entry, _ := h.Lookup("test")
	if entry.To != "correct2" {
		t.Errorf("Expected updated preference 'correct2', got %q", entry.To)
	}
}

func TestHistory_LoadExisting(t *testing.T) {
	tmpDir := t.TempDir()

	// Pre-create usage history file
	existing := []HistoryEntry{
		{From: "existing", To: "correct", Timestamp: 100, Count: 5},
	}
	data, _ := jsonMarshalHistory(existing)
	historyFile := filepath.Join(tmpDir, usageHistoryFileName)
	os.WriteFile(historyFile, data, 0644)

	// Load history
	h := NewHistory(tmpDir)

	// Verify existing entry was loaded
	entry, ok := h.Lookup("existing")
	if !ok {
		t.Error("Expected existing entry to be loaded")
	}
	if entry.Count != 5 {
		t.Errorf("Expected count 5, got %d", entry.Count)
	}
}

func TestHistory_EmptyConfigDir(t *testing.T) {
	h := NewHistory("")

	// Should still work in memory
	if err := h.Record("test", "correct"); err != nil {
		t.Errorf("Record should not error with empty config dir: %v", err)
	}

	entry, ok := h.Lookup("test")
	if !ok {
		t.Error("Expected to find entry")
	}
	if entry.To != "correct" {
		t.Errorf("Expected 'correct', got %q", entry.To)
	}
}

func jsonMarshalHistory(v interface{}) ([]byte, error) {
	return []byte(`[{"from":"existing","to":"correct","timestamp":100,"count":5}]`), nil
}

func TestHistory_LoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Write invalid JSON
	historyFile := filepath.Join(tmpDir, usageHistoryFileName)
	os.WriteFile(historyFile, []byte("invalid json {"), 0644)

	// Load history - should not panic
	h := NewHistory(tmpDir)

	// Should have empty entries
	if h.Count() != 0 {
		t.Errorf("Expected count 0 with invalid JSON, got %d", h.Count())
	}

	matches, err := filepath.Glob(historyFile + ".corrupt-*")
	if err != nil {
		t.Fatalf("Glob failed: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("Expected one quarantined history file, got %v", matches)
	}
}

func TestHistory_SaveMkdirError(t *testing.T) {
	// Create a file where a directory should be
	tmpFile, err := os.CreateTemp("", "typo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Use the file path as config dir (will fail MkdirAll)
	h := NewHistory(tmpFile.Name())

	// Record should fail because MkdirAll will fail
	err = h.Record("test", "correct")
	if err == nil {
		t.Error("Expected error when saving to invalid path")
	}
}

func TestHistory_ClearError(t *testing.T) {
	// Create a file where a directory should be
	tmpFile, err := os.CreateTemp("", "typo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	h := NewHistory(tmpFile.Name())
	err = h.Clear()
	if err == nil {
		t.Error("Expected error when clearing with invalid path")
	}
}

func TestHistory_ListSortedByTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHistory(tmpDir)

	h.entries["older"] = HistoryEntry{From: "older", To: "git", Timestamp: 10, Count: 1}
	h.entries["newer"] = HistoryEntry{From: "newer", To: "docker", Timestamp: 20, Count: 1}

	entries := h.List()
	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}
	if entries[0].From != "newer" {
		t.Fatalf("Expected newest entry first, got %q", entries[0].From)
	}
}

func TestHistory_RemoveConflictsForRule(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHistory(tmpDir)

	for from, to := range map[string]string{
		"dokcer":         "docker",
		"dokcer ps":      "docker ps",
		"sudo dokcer ps": "sudo docker ps",
		"gerp main":      "grep main",
	} {
		if err := h.Record(from, to); err != nil {
			t.Fatalf("Record(%q) failed: %v", from, err)
		}
	}

	if err := h.RemoveConflictsForRule("dokcer"); err != nil {
		t.Fatalf("RemoveConflictsForRule failed: %v", err)
	}

	for _, from := range []string{"dokcer", "dokcer ps", "sudo dokcer ps"} {
		if _, ok := h.Lookup(from); ok {
			t.Fatalf("Expected %q to be removed", from)
		}
	}

	if _, ok := h.Lookup("gerp main"); !ok {
		t.Fatal("Expected unrelated history entry to remain")
	}
}

func TestIsSingleCommandWord(t *testing.T) {
	tests := []struct {
		raw  string
		want bool
	}{
		{raw: "dokcer", want: true},
		{raw: " dokcer ", want: true},
		{raw: "dokcer ps", want: false},
		{raw: "dokcer&&ps", want: false},
		{raw: "", want: false},
	}

	for _, tt := range tests {
		if got := isSingleCommandWord(tt.raw); got != tt.want {
			t.Fatalf("isSingleCommandWord(%q) = %v, want %v", tt.raw, got, tt.want)
		}
	}
}

func TestHistoryEntryMatchesCommandWord(t *testing.T) {
	tests := []struct {
		raw    string
		target string
		want   bool
	}{
		{raw: "gut status", target: "gut", want: true},
		{raw: "sudo gut status", target: "gut", want: true},
		{raw: "gut '", target: "gut", want: true},
		{raw: "docker ps", target: "gut", want: false},
	}

	for _, tt := range tests {
		if got := historyEntryMatchesCommandWord(tt.raw, tt.target); got != tt.want {
			t.Fatalf("historyEntryMatchesCommandWord(%q, %q) = %v, want %v", tt.raw, tt.target, got, tt.want)
		}
	}
}

func TestHistory_RemoveEntriesForCommandWord_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHistory(tmpDir)
	if err := h.Record("gut status", "git status"); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	if err := h.RemoveEntriesForCommandWord(""); err != nil {
		t.Fatalf("Expected empty command word removal to be a no-op, got %v", err)
	}

	if _, ok := h.Lookup("gut status"); !ok {
		t.Fatal("Expected history entry to remain after empty removal")
	}
}
