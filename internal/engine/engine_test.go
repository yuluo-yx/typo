package engine

import (
	"testing"

	"github.com/shown/typo/internal/parser"
)

func TestEngine_Fix(t *testing.T) {
	// Create engine with mock components
	tmpDir := t.TempDir()
	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithHistory(NewHistory(tmpDir)),
		WithParser(parser.NewRegistry()),
		WithCommands([]string{"git", "docker", "npm", "ls", "cat", "grep"}),
	)

	tests := []struct {
		name    string
		cmd     string
		stderr  string
		wantFix bool
		wantCmd string
	}{
		{
			name:    "rule match - gut to git",
			cmd:     "gut status",
			wantFix: true,
			wantCmd: "git status",
		},
		{
			name:    "rule match - dcoker to docker",
			cmd:     "dcoker ps",
			wantFix: true,
			wantCmd: "docker ps",
		},
		{
			name:    "no match",
			cmd:     "validcommand",
			wantFix: false,
		},
		{
			name:    "empty command",
			cmd:     "",
			wantFix: false,
		},
		{
			name:    "whitespace command",
			cmd:     "   ",
			wantFix: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eng.Fix(tt.cmd, tt.stderr)
			if result.Fixed != tt.wantFix {
				t.Errorf("Fix().Fixed = %v, want %v", result.Fixed, tt.wantFix)
			}
			if tt.wantFix && result.Command != tt.wantCmd {
				t.Errorf("Fix().Command = %q, want %q", result.Command, tt.wantCmd)
			}
		})
	}
}

func TestEngine_FixWithHistory(t *testing.T) {
	tmpDir := t.TempDir()
	history := NewHistory(tmpDir)

	// Record a correction
	history.Record("mytypo", "mycommand")

	eng := NewEngine(WithHistory(history))

	result := eng.Fix("mytypo", "")
	if !result.Fixed {
		t.Error("Expected to fix from history")
	}
	if result.Command != "mycommand" {
		t.Errorf("Expected 'mycommand', got %q", result.Command)
	}
	if result.Source != "history" {
		t.Errorf("Expected source 'history', got %q", result.Source)
	}
}

func TestEngine_FixWithParser(t *testing.T) {
	eng := NewEngine(
		WithParser(parser.NewRegistry()),
	)

	result := eng.Fix("git remove -v", "git: 'remove' is not a git command.\n\nThe most similar command is\n\tremote\n")
	if !result.Fixed {
		t.Error("Expected to fix from parser")
	}
	if result.Command != "git remote -v" {
		t.Errorf("Expected 'git remote -v', got %q", result.Command)
	}
	if result.Source != "parser" {
		t.Errorf("Expected source 'parser', got %q", result.Source)
	}
}

func TestEngine_FixWithParser_NoMatch(t *testing.T) {
	eng := NewEngine(
		WithParser(parser.NewRegistry()),
	)

	result := eng.Fix("git unknown", "some random error")
	if result.Fixed {
		t.Error("Expected not to fix from parser with unrecognized error")
	}
}

func TestEngine_FixWithDistance(t *testing.T) {
	eng := NewEngine(
		WithCommands([]string{"git", "docker", "npm", "myapp"}),
		WithKeyboard(NewQWERTYKeyboard()),
	)

	// Test distance-based fix (myap is close to myapp, no rule for this)
	result := eng.Fix("myap", "")
	if !result.Fixed {
		t.Error("Expected to fix from distance")
	}
	if result.Command != "myapp" {
		t.Errorf("Expected 'myapp', got %q", result.Command)
	}
	if result.Source != "distance" {
		t.Errorf("Expected source 'distance', got %q", result.Source)
	}
}

func TestEngine_FixCommand(t *testing.T) {
	tmpDir := t.TempDir()
	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"git"}),
		WithKeyboard(NewQWERTYKeyboard()),
	)

	// Test that args are preserved
	result := eng.FixCommand("gut status --verbose")
	if !result.Fixed {
		t.Error("Expected to fix command")
	}
	if result.Command != "git status --verbose" {
		t.Errorf("Expected 'git status --verbose', got %q", result.Command)
	}
}

func TestEngine_FixCommand_Empty(t *testing.T) {
	eng := NewEngine()

	result := eng.FixCommand("")
	if result.Fixed {
		t.Error("Expected not to fix empty command")
	}
}

func TestEngine_FixCommand_WithHistory(t *testing.T) {
	tmpDir := t.TempDir()
	history := NewHistory(tmpDir)
	history.Record("mycmd", "myrealcmd")

	eng := NewEngine(WithHistory(history))

	result := eng.FixCommand("mycmd --args")
	if !result.Fixed {
		t.Error("Expected to fix from history")
	}
	if result.Command != "myrealcmd --args" {
		t.Errorf("Expected 'myrealcmd --args', got %q", result.Command)
	}
}

func TestEngine_FixCommand_NoMatch(t *testing.T) {
	eng := NewEngine(WithCommands([]string{}))

	result := eng.FixCommand("unknowncommand")
	if result.Fixed {
		t.Error("Expected not to fix unknown command")
	}
}

func TestEngine_TryHistory_WithArgs(t *testing.T) {
	tmpDir := t.TempDir()
	history := NewHistory(tmpDir)
	history.Record("mycmd", "realcmd")

	eng := NewEngine(WithHistory(history))

	// Test history lookup with args preservation
	result := eng.Fix("mycmd arg1 arg2", "")
	if !result.Fixed {
		t.Error("Expected to fix from history")
	}
	if result.Command != "realcmd arg1 arg2" {
		t.Errorf("Expected 'realcmd arg1 arg2', got %q", result.Command)
	}
}

func TestEngine_TryDistance_NoCommands(t *testing.T) {
	eng := NewEngine(WithCommands([]string{}))

	result := eng.Fix("xyzqwerty", "")
	if result.Fixed {
		t.Error("Expected not to fix with no known commands")
	}
}

func TestEngine_TryDistance_SimilarityTooLow(t *testing.T) {
	eng := NewEngine(WithCommands([]string{"completelydifferent"}))

	result := eng.Fix("xyz", "")
	if result.Fixed {
		t.Error("Expected not to fix when similarity is too low")
	}
}

func TestEngine_TryDistance_WithArgs(t *testing.T) {
	eng := NewEngine(
		WithCommands([]string{"myapp"}),
		WithKeyboard(NewQWERTYKeyboard()),
	)

	result := eng.Fix("myap status", "")
	if !result.Fixed {
		t.Error("Expected to fix from distance")
	}
	if result.Command != "myapp status" {
		t.Errorf("Expected 'myapp status', got %q", result.Command)
	}
}

func TestEngine_Learn(t *testing.T) {
	tmpDir := t.TempDir()
	history := NewHistory(tmpDir)
	eng := NewEngine(WithHistory(history))

	// Learn a correction
	if err := eng.Learn("mytypo", "mycommand"); err != nil {
		t.Fatalf("Learn failed: %v", err)
	}

	// Verify it was learned
	result := eng.Fix("mytypo", "")
	if !result.Fixed {
		t.Error("Expected to fix learned correction")
	}
	if result.Command != "mycommand" {
		t.Errorf("Expected 'mycommand', got %q", result.Command)
	}
}

func TestEngine_AddRule(t *testing.T) {
	tmpDir := t.TempDir()
	eng := NewEngine(WithRules(NewRules(tmpDir)))

	// Add a rule
	if err := eng.AddRule("mytypo", "mycommand"); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	// Verify it was added
	result := eng.Fix("mytypo", "")
	if !result.Fixed {
		t.Error("Expected to fix with added rule")
	}
	if result.Command != "mycommand" {
		t.Errorf("Expected 'mycommand', got %q", result.Command)
	}
}

func TestEngine_ListRules(t *testing.T) {
	tmpDir := t.TempDir()
	eng := NewEngine(WithRules(NewRules(tmpDir)))

	rules := eng.ListRules()
	if len(rules) == 0 {
		t.Error("Expected some builtin rules")
	}
}

func TestEngine_ListHistory(t *testing.T) {
	tmpDir := t.TempDir()
	history := NewHistory(tmpDir)
	eng := NewEngine(WithHistory(history))

	// Add some history
	eng.Learn("typo1", "correct1")
	eng.Learn("typo2", "correct2")

	entries := eng.ListHistory()
	if len(entries) != 2 {
		t.Errorf("Expected 2 history entries, got %d", len(entries))
	}
}

func TestEngine_Priority(t *testing.T) {
	tmpDir := t.TempDir()
	history := NewHistory(tmpDir)
	rules := NewRules(tmpDir)

	// Set up history with a different correction than the rule
	history.Record("gut", "customgit")

	eng := NewEngine(
		WithRules(rules),
		WithHistory(history),
		WithCommands([]string{"git"}),
	)

	// History should take priority over rules
	result := eng.Fix("gut", "")
	if !result.Fixed {
		t.Error("Expected to fix")
	}
	if result.Source != "history" {
		t.Errorf("Expected source 'history', got %q", result.Source)
	}
	if result.Command != "customgit" {
		t.Errorf("Expected 'customgit' from history, got %q", result.Command)
	}
}

func TestEngine_EmptyCommand(t *testing.T) {
	eng := NewEngine()

	result := eng.Fix("", "")
	if result.Fixed {
		t.Error("Expected not to fix empty command")
	}
}

func TestNewEngine(t *testing.T) {
	// Test default engine creation
	eng := NewEngine()
	if eng == nil {
		t.Fatal("NewEngine returned nil")
	}

	// Test with options
	kb := NewQWERTYKeyboard()
	eng = NewEngine(WithKeyboard(kb))
	if eng.keyboard == nil {
		t.Error("Expected keyboard to be set")
	}
}
