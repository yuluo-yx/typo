package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRules_Match(t *testing.T) {
	// Create temp dir for config
	tmpDir := t.TempDir()
	r := NewRules(tmpDir)

	tests := []struct {
		name      string
		cmd       string
		wantMatch bool
		wantTo    string
	}{
		{"match builtin git rule", "gut", true, "git"},
		{"match builtin docker rule", "dcoker", true, "docker"},
		{"no match", "validcommand", false, ""},
		{"match exact case", "gut", true, "git"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, ok := r.Match(tt.cmd)
			if ok != tt.wantMatch {
				t.Errorf("Match(%q) = %v, want %v", tt.cmd, ok, tt.wantMatch)
			}
			if ok && rule.To != tt.wantTo {
				t.Errorf("Match(%q).To = %q, want %q", tt.cmd, rule.To, tt.wantTo)
			}
		})
	}
}

func TestRules_UserRulesPriority(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRules(tmpDir)

	// Add a user rule that overrides builtin
	userRule := Rule{From: "gut", To: "customgit", Scope: "custom"}
	if err := r.AddUserRule(userRule); err != nil {
		t.Fatalf("AddUserRule failed: %v", err)
	}

	rule, ok := r.Match("gut")
	if !ok {
		t.Error("Expected to match 'gut'")
	}
	if rule.To != "customgit" {
		t.Errorf("Expected user rule 'customgit', got %q", rule.To)
	}
}

func TestRules_AddUserRule(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRules(tmpDir)

	rule := Rule{From: "testcmd", To: "correctcmd", Scope: "test"}

	// Add rule
	if err := r.AddUserRule(rule); err != nil {
		t.Fatalf("AddUserRule failed: %v", err)
	}

	// Verify it was added
	matched, ok := r.Match("testcmd")
	if !ok {
		t.Error("Expected to match added rule")
	}
	if matched.To != "correctcmd" {
		t.Errorf("Expected 'correctcmd', got %q", matched.To)
	}

	// Verify it was saved to file
	rulesFile := filepath.Join(tmpDir, "rules.json")
	if _, err := os.Stat(rulesFile); os.IsNotExist(err) {
		t.Error("Expected rules.json to be created")
	}
}

func TestRules_RemoveUserRule(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRules(tmpDir)

	// Add then remove
	rule := Rule{From: "testcmd", To: "correctcmd"}
	r.AddUserRule(rule)

	if err := r.RemoveUserRule("testcmd"); err != nil {
		t.Fatalf("RemoveUserRule failed: %v", err)
	}

	// Verify it was removed
	_, ok := r.Match("testcmd")
	if ok {
		t.Error("Expected rule to be removed")
	}
}

func TestRules_ListRules(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRules(tmpDir)

	rules := r.ListRules()

	if len(rules) == 0 {
		t.Error("Expected some builtin rules")
	}

	// Check that some known rules exist
	foundGit := false
	for _, rule := range rules {
		if rule.From == "gut" && rule.To == "git" {
			foundGit = true
			break
		}
	}
	if !foundGit {
		t.Error("Expected 'gut -> git' rule to be present")
	}
}

func TestRules_EnableRuleSet(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRules(tmpDir)

	// Verify git rules are enabled by default
	rule, ok := r.Match("gut")
	if !ok || !rule.Enable {
		t.Error("Expected 'gut' rule to be enabled by default")
	}

	// Disable git rules
	if err := r.EnableRuleSet("git", false); err != nil {
		t.Fatalf("EnableRuleSet failed: %v", err)
	}

	// Now gut should still match but be disabled
	rule, ok = r.Match("gut")
	if ok {
		t.Error("Expected 'gut' rule to be disabled")
	}

	// Re-enable
	r.EnableRuleSet("git", true)
	rule, ok = r.Match("gut")
	if !ok {
		t.Error("Expected 'gut' rule to be re-enabled")
	}
}

func TestRules_GetRuleSets(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRules(tmpDir)

	sets := r.GetRuleSets()

	expectedSets := []string{"git", "docker", "npm", "yarn", "kubectl", "cargo", "python", "pip", "go", "system"}
	for _, expected := range expectedSets {
		found := false
		for _, set := range sets {
			if set.Name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected rule set %q to be present", expected)
		}
	}
}

func TestRules_LoadExistingUserRules(t *testing.T) {
	tmpDir := t.TempDir()

	// Pre-create rules.json
	existingRules := []Rule{
		{From: "existingcmd", To: "correctcmd", Scope: "user"},
	}
	data, _ := jsonMarshal(existingRules)
	rulesFile := filepath.Join(tmpDir, "rules.json")
	os.WriteFile(rulesFile, data, 0644)

	// Load rules
	r := NewRules(tmpDir)

	// Verify existing rule was loaded
	rule, ok := r.Match("existingcmd")
	if !ok {
		t.Error("Expected existing user rule to be loaded")
	}
	if rule.To != "correctcmd" {
		t.Errorf("Expected 'correctcmd', got %q", rule.To)
	}
}

func TestRules_EmptyConfigDir(t *testing.T) {
	r := NewRules("")

	// Should still work with builtin rules
	rule, ok := r.Match("gut")
	if !ok {
		t.Error("Expected builtin rule to work with empty config dir")
	}
	if rule.To != "git" {
		t.Errorf("Expected 'git', got %q", rule.To)
	}

	// AddUserRule should not error with empty config dir
	err := r.AddUserRule(Rule{From: "test", To: "test"})
	if err != nil {
		t.Errorf("AddUserRule should not error with empty config dir: %v", err)
	}
}

func TestRules_BuiltinRulesCount(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRules(tmpDir)

	rules := r.ListRules()

	// Should have at least 30 builtin rules
	if len(rules) < 30 {
		t.Errorf("Expected at least 30 builtin rules, got %d", len(rules))
	}
}

func jsonMarshal(v interface{}) ([]byte, error) {
	// Simple JSON marshal for test
	return []byte(`[{"from":"existingcmd","to":"correctcmd","scope":"user"}]`), nil
}

func TestRules_RemoveUserRule_Nonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRules(tmpDir)

	err := r.RemoveUserRule("nonexistentrule12345")
	if err != ErrRuleNotFound {
		t.Errorf("Expected ErrRuleNotFound, got %v", err)
	}
}

func TestRules_LoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Write invalid JSON
	rulesFile := filepath.Join(tmpDir, "rules.json")
	os.WriteFile(rulesFile, []byte("invalid json {"), 0644)

	// Load rules - should not panic
	r := NewRules(tmpDir)

	// Should still have builtin rules
	rule, ok := r.Match("gut")
	if !ok {
		t.Error("Expected builtin rule to work with invalid JSON")
	}
	if rule.To != "git" {
		t.Errorf("Expected 'git', got %q", rule.To)
	}
}

func TestRules_SaveMkdirError(t *testing.T) {
	// Create a file where a directory should be
	tmpFile, err := os.CreateTemp("", "typo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Use the file path as config dir (will fail MkdirAll)
	r := NewRules(tmpFile.Name())

	// AddUserRule should fail because MkdirAll will fail
	err = r.AddUserRule(Rule{From: "test", To: "correct"})
	if err == nil {
		t.Error("Expected error when saving to invalid path")
	}
}

func TestRules_ListRules_WithUserRules(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRules(tmpDir)

	// Add user rule
	r.AddUserRule(Rule{From: "mycustom", To: "mycorrect"})

	rules := r.ListRules()

	// Check that user rule is included
	found := false
	for _, rule := range rules {
		if rule.From == "mycustom" && rule.To == "mycorrect" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected user rule to be included in list")
	}
}

func TestRules_Match_DisabledRule(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRules(tmpDir)

	// Add a disabled rule
	rule := Rule{From: "disabledcmd", To: "correctcmd", Enable: false}
	r.AddUserRule(rule)

	// Manually set the rule to disabled
	r.mu.Lock()
	r.user["disabledcmd"] = Rule{From: "disabledcmd", To: "correctcmd", Enable: false}
	r.mu.Unlock()

	// Should not match disabled rule
	_, ok := r.Match("disabledcmd")
	if ok {
		t.Error("Expected not to match disabled rule")
	}
}
