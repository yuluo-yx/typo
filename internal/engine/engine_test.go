package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yuluo-yx/typo/internal/commands"
	"github.com/yuluo-yx/typo/internal/parser"
)

func TestEngine_Fix(t *testing.T) {
	// Create engine with mock components
	tmpDir := t.TempDir()
	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
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

func TestEngine_FixWithParser_ClearsStderrAfterFirstParserFix(t *testing.T) {
	tmpDir := t.TempDir()
	subcommands := commands.NewSubcommandRegistry(tmpDir)
	eng := NewEngine(
		WithParser(parser.NewRegistry()),
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"git", "docker"}),
		WithSubcommands(subcommands),
	)

	result := eng.FixWithContext(parser.Context{
		Command: "git remove -v && dcoker ps",
		Stderr:  "git: 'remove' is not a git command. See 'git --help'.\n\nThe most similar command is\n\tremote\n",
	})
	if !result.Fixed {
		t.Fatal("Expected parser fix to be followed by normal typo fixes")
	}
	if result.Command != "git remote -v && docker ps" {
		t.Fatalf("Expected parser fix plus docker rule fix, got %q", result.Command)
	}
	if !result.UsedParser {
		t.Fatal("Expected parser-assisted fix chain to retain parser usage marker")
	}
}

func TestEngine_FixWithParser_NoUpstreamTargetsPullOnly(t *testing.T) {
	eng := NewEngine(
		WithParser(parser.NewRegistry()),
	)

	result := eng.FixWithContext(parser.Context{
		Command: "git remove -v && git pull",
		Stderr:  "There is no tracking information for the current branch.\nPlease specify which branch you want to merge with.\nSee git-pull(1) for details.\n\n    git pull <remote> <branch>\n\nIf you wish to set tracking information for this branch, you can do so with:\n\n    git branch --set-upstream-to=origin/main main\n",
	})
	if !result.Fixed {
		t.Fatal("Expected pull command to be fixed")
	}
	if result.Command != "git remove -v && git pull --set-upstream origin main" {
		t.Fatalf("Expected upstream fix to apply only to git pull, got %q", result.Command)
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

func TestEngine_FixWithPermissionParser(t *testing.T) {
	eng := NewEngine(
		WithParser(parser.NewRegistry()),
	)

	result := eng.FixWithContext(parser.Context{
		Command:  "mkdir 1",
		Stderr:   "mkdir: 1: Permission denied\n",
		ExitCode: 1,
	})
	if !result.Fixed {
		t.Fatal("Expected to fix from permission parser")
	}
	if result.Command != "sudo mkdir 1" {
		t.Fatalf("Expected 'sudo mkdir 1', got %q", result.Command)
	}
	if result.Source != "parser" {
		t.Fatalf("Expected source 'parser', got %q", result.Source)
	}
}

func TestEngine_FixWithPermissionHistory_DoesNotDuplicateSudo(t *testing.T) {
	tmpDir := t.TempDir()
	eng := NewEngine(
		WithParser(parser.NewRegistry()),
		WithHistory(NewHistory(tmpDir)),
	)

	if err := eng.RecordHistory("mkdir 1", "sudo mkdir 1"); err != nil {
		t.Fatalf("RecordHistory failed: %v", err)
	}

	result := eng.Fix("sudo mkdir 1", "")
	if result.Fixed {
		t.Fatalf("Expected wrapped permission command to stay unchanged, got %+v", result)
	}
}

func TestEngine_FixWithPermissionParserAndHistory_DoesNotEscalateRepeatedly(t *testing.T) {
	tmpDir := t.TempDir()
	eng := NewEngine(
		WithParser(parser.NewRegistry()),
		WithHistory(NewHistory(tmpDir)),
	)

	if err := eng.RecordHistory("mkdir 1", "sudo mkdir 1"); err != nil {
		t.Fatalf("RecordHistory failed: %v", err)
	}

	result := eng.FixWithContext(parser.Context{
		Command:  "mkdir 1",
		Stderr:   "mkdir: 1: Permission denied\n",
		ExitCode: 1,
	})
	if !result.Fixed {
		t.Fatal("Expected permission parser to keep working")
	}
	if result.Command != "sudo mkdir 1" {
		t.Fatalf("Expected single sudo wrapper, got %q", result.Command)
	}
}

func TestEngine_FixWithPermissionParser_SkipsRedirection(t *testing.T) {
	eng := NewEngine(
		WithParser(parser.NewRegistry()),
	)

	result := eng.FixWithContext(parser.Context{
		Command:  "echo ok > /root/out",
		Stderr:   "zsh: permission denied: /root/out\n",
		ExitCode: 1,
	})
	if result.Fixed {
		t.Fatalf("Expected redirection command to stay unchanged, got %+v", result)
	}
}

func TestEngine_FixWithPermissionParser_SkipsRemoteAuthFailure(t *testing.T) {
	eng := NewEngine(
		WithParser(parser.NewRegistry()),
	)

	result := eng.FixWithContext(parser.Context{
		Command:  "git push origin main",
		Stderr:   "git@github.com: Permission denied (publickey).\nfatal: Could not read from remote repository.\n",
		ExitCode: 128,
	})
	if result.Fixed {
		t.Fatalf("Expected remote auth failure to stay unchanged, got %+v", result)
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

func TestEngine_FixWithDistance_PrefersSupportedCommandOnTie(t *testing.T) {
	tmpDir := t.TempDir()
	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"colcrt", "docker"}),
		WithKeyboard(NewQWERTYKeyboard()),
	)

	result := eng.Fix("dokcer ps", "")
	if !result.Fixed {
		t.Fatal("Expected to fix from distance")
	}
	if result.Command != "docker ps" {
		t.Fatalf("Expected docker to win tie-break, got %q", result.Command)
	}
	if result.Source != "distance" && result.Source != "rule" {
		t.Fatalf("Expected distance or rule source, got %q", result.Source)
	}
}

func TestEngine_Fix_DoesNotReportNoopAsFix(t *testing.T) {
	eng := NewEngine(
		WithCommands([]string{"git", "docker", "npm"}),
		WithKeyboard(NewQWERTYKeyboard()),
	)

	tests := []string{
		"git status",
		"docker ps",
		"npm test",
	}

	for _, cmd := range tests {
		t.Run(cmd, func(t *testing.T) {
			result := eng.Fix(cmd, "")
			if result.Fixed {
				t.Fatalf("Expected no fix for valid command %q, got %+v", cmd, result)
			}
		})
	}
}

func TestEngine_Fix_WithWrapperPrefixes(t *testing.T) {
	tmpDir := t.TempDir()
	subcmdRegistry := commands.NewSubcommandRegistry(tmpDir)
	subcmdRegistry.Get("git")

	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"git", "sudo", "env"}),
		WithKeyboard(NewQWERTYKeyboard()),
		WithSubcommands(subcmdRegistry),
	)

	tests := []struct {
		name    string
		cmd     string
		wantCmd string
	}{
		{
			name:    "sudo wrapped command",
			cmd:     "sudo gti status",
			wantCmd: "sudo git status",
		},
		{
			name:    "shell assignment before command",
			cmd:     "FOO=1 gut status",
			wantCmd: "FOO=1 git status",
		},
		{
			name:    "env wrapper with assignment",
			cmd:     "env FOO=1 gut status",
			wantCmd: "env FOO=1 git status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eng.Fix(tt.cmd, "")
			if !result.Fixed {
				t.Fatalf("Expected to fix %q, got no fix", tt.cmd)
			}
			if result.Command != tt.wantCmd {
				t.Fatalf("Fix().Command = %q, want %q", result.Command, tt.wantCmd)
			}
		})
	}
}

func TestEngine_Fix_PreservesQuotedArguments(t *testing.T) {
	tmpDir := t.TempDir()
	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"git"}),
		WithKeyboard(NewQWERTYKeyboard()),
	)

	result := eng.Fix("gut commit -m 'a   b'", "")
	if !result.Fixed {
		t.Fatal("Expected quoted command to be fixed")
	}
	if result.Command != "git commit -m 'a   b'" {
		t.Fatalf("Expected quoted argument spacing to be preserved, got %q", result.Command)
	}
}

func TestEngine_Fix_PreservesCompoundCommands(t *testing.T) {
	tmpDir := t.TempDir()
	subcmdRegistry := commands.NewSubcommandRegistry(tmpDir)
	subcmdRegistry.Get("git")

	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"git", "sudo"}),
		WithKeyboard(NewQWERTYKeyboard()),
		WithSubcommands(subcmdRegistry),
	)

	tests := []struct {
		name    string
		cmd     string
		wantCmd string
	}{
		{
			name:    "semicolon command preserves separator",
			cmd:     "gut status; echo ok",
			wantCmd: "git status; echo ok",
		},
		{
			name:    "wrapped command in and-list",
			cmd:     "sudo gti status && echo ok",
			wantCmd: "sudo git status && echo ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eng.Fix(tt.cmd, "")
			if !result.Fixed {
				t.Fatalf("Expected to fix %q, got no fix", tt.cmd)
			}
			if result.Command != tt.wantCmd {
				t.Fatalf("Fix().Command = %q, want %q", result.Command, tt.wantCmd)
			}
		})
	}
}

func TestEngine_Fix_CanCorrectMultipleTyposInOneCommand(t *testing.T) {
	tmpDir := t.TempDir()
	subcmdRegistry := commands.NewSubcommandRegistry(tmpDir)
	subcmdRegistry.Get("git")
	subcmdRegistry.Get("docker")

	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"git", "docker", "sudo"}),
		WithKeyboard(NewQWERTYKeyboard()),
		WithSubcommands(subcmdRegistry),
	)

	tests := []struct {
		name    string
		cmd     string
		wantCmd string
	}{
		{
			name:    "two command typos in and-list",
			cmd:     "gti status && dcoker ps",
			wantCmd: "git status && docker ps",
		},
		{
			name:    "mixed wrapper and subcommand typo",
			cmd:     "sudo git -C repo stauts || dcoker ps",
			wantCmd: "sudo git -C repo status || docker ps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eng.Fix(tt.cmd, "")
			if !result.Fixed {
				t.Fatalf("Expected to fix %q, got no fix", tt.cmd)
			}
			if result.Command != tt.wantCmd {
				t.Fatalf("Fix().Command = %q, want %q", result.Command, tt.wantCmd)
			}
		})
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
	rules := NewRules(tmpDir)
	eng := NewEngine(WithRules(rules))

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

func TestEngine_LearnClearsConflictingHistory(t *testing.T) {
	tmpDir := t.TempDir()
	history := NewHistory(tmpDir)
	if err := history.Record("dokcer ps", "colcrt ps"); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithHistory(history),
		WithCommands([]string{"colcrt", "docker"}),
		WithKeyboard(NewQWERTYKeyboard()),
	)

	if err := eng.Learn("dokcer", "docker"); err != nil {
		t.Fatalf("Learn failed: %v", err)
	}

	if _, ok := history.Lookup("dokcer ps"); ok {
		t.Fatal("Expected conflicting history to be cleared after learn")
	}

	result := eng.Fix("dokcer ps", "")
	if !result.Fixed {
		t.Fatal("Expected learned rule to fix command")
	}
	if result.Command != "docker ps" {
		t.Fatalf("Expected learned rule to override stale history, got %q", result.Command)
	}
	if result.Source != "rule" {
		t.Fatalf("Expected source rule, got %q", result.Source)
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

func TestEngine_AddRuleClearsConflictingHistory(t *testing.T) {
	tmpDir := t.TempDir()
	history := NewHistory(tmpDir)
	if err := history.Record("dokcer ps", "colcrt ps"); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithHistory(history),
		WithCommands([]string{"colcrt", "docker"}),
		WithKeyboard(NewQWERTYKeyboard()),
	)

	if err := eng.AddRule("dokcer", "docker"); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	if _, ok := history.Lookup("dokcer ps"); ok {
		t.Fatal("Expected conflicting history to be cleared after rules add")
	}

	result := eng.Fix("dokcer ps", "")
	if !result.Fixed {
		t.Fatal("Expected added rule to fix command")
	}
	if result.Command != "docker ps" {
		t.Fatalf("Expected added rule to override stale history, got %q", result.Command)
	}
	if result.Source != "rule" {
		t.Fatalf("Expected source rule, got %q", result.Source)
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
	if err := eng.RecordHistory("typo1", "correct1"); err != nil {
		t.Fatalf("RecordHistory failed: %v", err)
	}
	if err := eng.RecordHistory("typo2", "correct2"); err != nil {
		t.Fatalf("RecordHistory failed: %v", err)
	}

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

	// History should take priority over builtin rules
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

func TestEngine_UserRuleOverridesHistory(t *testing.T) {
	tmpDir := t.TempDir()
	history := NewHistory(tmpDir)
	if err := history.Record("gut", "customgit"); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	rules := NewRules(tmpDir)
	if err := rules.AddUserRule(Rule{From: "gut", To: "mygit"}); err != nil {
		t.Fatalf("AddUserRule failed: %v", err)
	}

	eng := NewEngine(
		WithRules(rules),
		WithHistory(history),
		WithCommands([]string{"git"}),
	)

	result := eng.Fix("gut", "")
	if !result.Fixed {
		t.Fatal("Expected to fix")
	}
	if result.Source != "rule" {
		t.Fatalf("Expected source rule, got %q", result.Source)
	}
	if result.Command != "mygit" {
		t.Fatalf("Expected user rule to override history, got %q", result.Command)
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

func TestEngine_FixSubcommand_SimilarityBoundary(t *testing.T) {
	// Test case: "gti cloen" should be fixed to "git clone" or "git clean"
	// Both have distance=2 and similarity=0.6, so either is acceptable

	tmpDir := t.TempDir()

	subcmdRegistry := commands.NewSubcommandRegistry(tmpDir)
	subcmdRegistry.Get("git")

	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"git"}),
		WithKeyboard(NewQWERTYKeyboard()),
		WithSubcommands(subcmdRegistry),
	)

	tests := []struct {
		name     string
		cmd      string
		wantFix  bool
		wantCmds []string // acceptable results
	}{
		{
			name:     "gti cloen - similarity exactly 0.6 (ambiguous: clean/clone)",
			cmd:      "gti cloen",
			wantFix:  true,
			wantCmds: []string{"git clone", "git clean"},
		},
		{
			name:     "gti clnoe - distance 2, similarity 0.6",
			cmd:      "gti clnoe",
			wantFix:  true,
			wantCmds: []string{"git clone"},
		},
		{
			name:     "gti colne - distance 2, similarity 0.6",
			cmd:      "gti colne",
			wantFix:  true,
			wantCmds: []string{"git clone"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eng.Fix(tt.cmd, "")
			if result.Fixed != tt.wantFix {
				t.Errorf("Fix().Fixed = %v, want %v", result.Fixed, tt.wantFix)
			}
			if tt.wantFix {
				found := false
				for _, wantCmd := range tt.wantCmds {
					if result.Command == wantCmd {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Fix().Command = %q, want one of %v", result.Command, tt.wantCmds)
				}
			}
		})
	}
}

func TestEngine_GitCommands_CommonTypos(t *testing.T) {
	// Comprehensive test for common git command typos

	tmpDir := t.TempDir()
	subcmdRegistry := commands.NewSubcommandRegistry(tmpDir)
	subcmdRegistry.Get("git")

	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"git"}),
		WithKeyboard(NewQWERTYKeyboard()),
		WithSubcommands(subcmdRegistry),
	)

	tests := []struct {
		name     string
		cmd      string
		wantFix  bool
		wantCmds []string
	}{
		// Main command typos (handled by rules)
		{"gti -> git", "gti status", true, []string{"git status"}},
		{"gut -> git", "gut status", true, []string{"git status"}},
		{"got -> git", "got status", true, []string{"git status"}},

		// push/pull typos
		{"gti pus -> git push", "gti pus", true, []string{"git push"}},
		{"gti pll -> git pull", "gti pll", true, []string{"git pull"}},
		{"gti pul -> git pull", "gti pul", true, []string{"git pull"}},
		// psuh -> push has distance 3, similarity 0.25, too low to fix

		// commit typos
		{"gti comit -> git commit", "gti comit", true, []string{"git commit"}},
		{"gti commt -> git commit", "gti commt", true, []string{"git commit"}},
		{"gti commiti -> git commit", "gti commiti", true, []string{"git commit"}},

		// clone typos
		{"gti clon -> git clone", "gti clon", true, []string{"git clone"}},
		{"gti clnoe -> git clone", "gti clnoe", true, []string{"git clone"}},

		// checkout typos
		{"gti chckout -> git checkout", "gti chckout", true, []string{"git checkout"}},
		{"gti chekcout -> git checkout", "gti chekcout", true, []string{"git checkout"}},

		// branch typos
		{"gti brnach -> git branch", "gti brnach", true, []string{"git branch"}},
		{"gti brnch -> git branch", "gti brnch", true, []string{"git branch"}},

		// merge typos
		{"gti mrge -> git merge", "gti mrge", true, []string{"git merge"}},
		{"gti merg -> git merge", "gti merg", true, []string{"git merge"}},

		// rebase typos
		{"gti reabse -> git rebase", "gti reabse", true, []string{"git rebase"}},
		{"gti rbase -> git rebase", "gti rbase", true, []string{"git rebase"}},

		// stash typos
		{"gti stahs -> git stash", "gti stahs", true, []string{"git stash"}},
		{"gti stsh -> git stash", "gti stsh", true, []string{"git stash"}},

		// fetch typos
		{"gti fethc -> git fetch", "gti fethc", true, []string{"git fetch"}},
		{"gti fetc -> git fetch", "gti fetc", true, []string{"git fetch"}},

		// remote typos
		{"gti remtoe -> git restore", "gti remtoe", true, []string{"git remote", "git restore"}}, // restore has higher similarity
		{"gti remoe -> git remote", "gti remoe", true, []string{"git remote"}},

		// add typos
		{"gti ad -> git add", "gti ad", true, []string{"git add"}},

		// status typos
		{"gti stauts -> git status", "gti stauts", true, []string{"git status"}},
		{"gti statu -> git status", "gti statu", true, []string{"git status"}},

		// log typos
		{"gti lo -> git log", "gti lo", true, []string{"git log"}},

		// diff typos
		{"gti dif -> git diff", "gti dif", true, []string{"git diff"}},

		// reset typos
		{"gti rset -> git reset", "gti rset", true, []string{"git reset"}},

		// With arguments
		{"gti pus origin main", "gti pus origin main", true, []string{"git push origin main"}},
		{"gti comit -m 'test'", "gti comit -m 'test'", true, []string{"git commit -m 'test'"}},
		{"gti chckout main", "gti chckout main", true, []string{"git checkout main"}},
		{"gti ad .", "gti ad .", true, []string{"git add ."}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eng.Fix(tt.cmd, "")
			if result.Fixed != tt.wantFix {
				t.Errorf("Fix().Fixed = %v, want %v", result.Fixed, tt.wantFix)
			}
			if tt.wantFix {
				found := false
				for _, wantCmd := range tt.wantCmds {
					if result.Command == wantCmd {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Fix().Command = %q, want one of %v", result.Command, tt.wantCmds)
				}
			}
		})
	}
}

func newEngineWithCommonToolSubcommands(t *testing.T) *Engine {
	t.Helper()

	tmpDir := t.TempDir()
	cache := []commands.SubcommandCache{
		{
			Tool:        "git",
			Subcommands: []string{"status", "remote", "rev-parse", "ls-files"},
			UpdatedAt:   time.Now(),
		},
		{
			Tool:        "docker",
			Subcommands: []string{"build", "run", "ps", "images", "logs", "compose"},
			UpdatedAt:   time.Now(),
		},
		{
			Tool:        "npm",
			Subcommands: []string{"install", "run", "test", "publish", "ci"},
			UpdatedAt:   time.Now(),
		},
		{
			Tool:        "cargo",
			Subcommands: []string{"bench", "build", "check", "fmt", "run", "test"},
			UpdatedAt:   time.Now(),
		},
		{
			Tool:        "kubectl",
			Subcommands: []string{"get", "describe", "apply", "delete", "logs"},
			UpdatedAt:   time.Now(),
		},
		{
			Tool:        "brew",
			Subcommands: []string{"install", "update", "upgrade", "list", "search"},
			UpdatedAt:   time.Now(),
		},
		{
			Tool:        "terraform",
			Subcommands: []string{"init", "plan", "apply", "destroy", "validate"},
			UpdatedAt:   time.Now(),
		},
		{
			Tool:        "helm",
			Subcommands: []string{"install", "upgrade", "template", "repo", "lint", "list"},
			UpdatedAt:   time.Now(),
		},
	}

	data, err := json.Marshal(cache)
	if err != nil {
		t.Fatalf("Failed to marshal subcommand cache: %v", err)
	}

	cacheFile := filepath.Join(tmpDir, "subcommands.json")
	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		t.Fatalf("Failed to write subcommand cache: %v", err)
	}

	subcmdRegistry := commands.NewSubcommandRegistry(tmpDir)

	return NewEngine(
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"git", "docker", "npm", "cargo", "kubectl", "brew", "terraform", "helm"}),
		WithKeyboard(NewQWERTYKeyboard()),
		WithSubcommands(subcmdRegistry),
	)
}

func TestEngine_CommonCommands_CanBeFixed(t *testing.T) {
	eng := newEngineWithCommonToolSubcommands(t)

	tests := []struct {
		name    string
		cmd     string
		wantCmd string
	}{
		{
			name:    "docker main command and subcommand typo",
			cmd:     "dcoker biuld -t app .",
			wantCmd: "docker build -t app .",
		},
		{
			name:    "docker subcommand typo",
			cmd:     "docker imgaes",
			wantCmd: "docker images",
		},
		{
			name:    "docker subcommand typo after global option with value",
			cmd:     "docker --context prod biuld -t app .",
			wantCmd: "docker --context prod build -t app .",
		},
		{
			name:    "npm main command and subcommand typo",
			cmd:     "nmp isntall react",
			wantCmd: "npm install react",
		},
		{
			name:    "npm subcommand typo",
			cmd:     "npm rn test",
			wantCmd: "npm run test",
		},
		{
			name:    "cargo subcommand typo",
			cmd:     "cargo helpd",
			wantCmd: "cargo help",
		},
		{
			name:    "cargo global option typo",
			cmd:     "cargo --versino",
			wantCmd: "cargo --version",
		},
		{
			name:    "kubectl main command and subcommand typo",
			cmd:     "kubctl desribe pod/nginx",
			wantCmd: "kubectl describe pod/nginx",
		},
		{
			name:    "kubectl subcommand typo",
			cmd:     "kubectl aplly -f deployment.yaml",
			wantCmd: "kubectl apply -f deployment.yaml",
		},
		{
			name:    "git subcommand typo after global option with value",
			cmd:     "git -C repo stauts",
			wantCmd: "git -C repo status",
		},
		{
			name:    "git hyphenated subcommand typo",
			cmd:     "git rev-prase HEAD",
			wantCmd: "git rev-parse HEAD",
		},
		{
			name:    "brew main command and subcommand typo",
			cmd:     "bre instlal wget",
			wantCmd: "brew install wget",
		},
		{
			name:    "brew subcommand typo",
			cmd:     "brew upgarde", //nolint:misspell // intentional typo for correction test
			wantCmd: "brew upgrade",
		},
		{
			name:    "terraform subcommand typo after global option with value",
			cmd:     "terraform -chdir infra valdiate",
			wantCmd: "terraform -chdir infra validate",
		},
		{
			name:    "helm subcommand typo after global option with value",
			cmd:     "helm --kube-context prod temlpate chart",
			wantCmd: "helm --kube-context prod template chart",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eng.Fix(tt.cmd, "")
			if !result.Fixed {
				t.Fatalf("Expected to fix %q, but got no fix", tt.cmd)
			}
			if result.Command != tt.wantCmd {
				t.Errorf("Fix().Command = %q, want %q", result.Command, tt.wantCmd)
			}
		})
	}
}
