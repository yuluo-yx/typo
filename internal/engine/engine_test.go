package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yuluo-yx/typo/internal/commands"
	"github.com/yuluo-yx/typo/internal/parser"
	itypes "github.com/yuluo-yx/typo/internal/types"
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
	if err := history.Record("mytypo", "mycommand"); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

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

func TestEngine_FixWithHistory_DoesNotRewriteKnownSingleCommands(t *testing.T) {
	tmpDir := t.TempDir()
	history := NewHistory(tmpDir)
	for from, to := range map[string]string{
		"azd":   "az",
		"doctl": "local",
	} {
		if err := history.Record(from, to); err != nil {
			t.Fatalf("Record failed: %v", err)
		}
	}

	eng := NewEngine(
		WithHistory(history),
		WithCommands(commands.DiscoverCommon()),
		WithKeyboard(NewQWERTYKeyboard()),
	)

	for _, cmd := range []string{"azd", "doctl"} {
		t.Run(cmd, func(t *testing.T) {
			result := eng.Fix(cmd, "")
			if result.Fixed {
				t.Fatalf("Expected known command %q to ignore stale history, got %+v", cmd, result)
			}
		})
	}
}

func TestEngine_FixWithHistory_PrefersCommonTranspositionOverStaleHistory(t *testing.T) {
	tmpDir := t.TempDir()
	history := NewHistory(tmpDir)
	for from, to := range map[string]string{
		"za":  "eza",
		"oic": "tic",
	} {
		if err := history.Record(from, to); err != nil {
			t.Fatalf("Record failed: %v", err)
		}
	}

	commonCommands := append(commands.DiscoverCommon(), "eza", "tic")
	eng := NewEngine(
		WithHistory(history),
		WithCommands(commonCommands),
		WithKeyboard(NewQWERTYKeyboard()),
	)

	tests := []struct {
		cmd     string
		wantCmd string
	}{
		{cmd: "za", wantCmd: "az"},
		{cmd: "oic", wantCmd: "oci"},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			result := eng.Fix(tt.cmd, "")
			if !result.Fixed {
				t.Fatalf("Expected %q to be fixed", tt.cmd)
			}
			if result.Command != tt.wantCmd {
				t.Fatalf("Fix().Command = %q, want %q", result.Command, tt.wantCmd)
			}
		})
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
	subcommands := commands.NewToolTreeRegistry(tmpDir)
	eng := NewEngine(
		WithParser(parser.NewRegistry()),
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"git", "docker"}),
		WithToolTrees(subcommands),
	)

	result := eng.FixWithContext(itypes.ParserContext{
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

	result := eng.FixWithContext(itypes.ParserContext{
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

	result := eng.FixWithContext(itypes.ParserContext{
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

	result := eng.FixWithContext(itypes.ParserContext{
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

	result := eng.FixWithContext(itypes.ParserContext{
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

	result := eng.FixWithContext(itypes.ParserContext{
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
	subcmdRegistry := commands.NewToolTreeRegistry(tmpDir)
	subcmdRegistry.Get("git")

	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"git", "sudo", "env"}),
		WithKeyboard(NewQWERTYKeyboard()),
		WithToolTrees(subcmdRegistry),
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
	subcmdRegistry := commands.NewToolTreeRegistry(tmpDir)
	subcmdRegistry.Get("git")

	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"git", "sudo"}),
		WithKeyboard(NewQWERTYKeyboard()),
		WithToolTrees(subcmdRegistry),
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
	subcmdRegistry := commands.NewToolTreeRegistry(tmpDir)
	subcmdRegistry.Get("git")
	subcmdRegistry.Get("docker")

	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"git", "docker", "sudo"}),
		WithKeyboard(NewQWERTYKeyboard()),
		WithToolTrees(subcmdRegistry),
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
	if err := history.Record("mycmd", "myrealcmd"); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

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
	if err := history.Record("mycmd", "realcmd"); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

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

func TestEngine_MaybeAutoLearnFromHistory(t *testing.T) {
	tmpDir := t.TempDir()
	history := NewHistory(tmpDir)
	rules := NewRules(tmpDir)
	eng := NewEngine(
		WithHistory(history),
		WithRules(rules),
		WithAutoLearnThreshold(2),
	)

	if err := eng.RecordHistory("mytypo", "mytool"); err != nil {
		t.Fatalf("RecordHistory #1 failed: %v", err)
	}
	if err := eng.RecordHistory("mytypo", "mytool"); err != nil {
		t.Fatalf("RecordHistory #2 failed: %v", err)
	}

	result := eng.maybeAutoLearnFromHistory(context.Background(), "mytypo", "mytool")
	if !result.Triggered || !result.Persisted || result.Err != nil {
		t.Fatalf("maybeAutoLearnFromHistory() = %+v", result)
	}

	rule, ok := rules.MatchUser("mytypo")
	if !ok || rule.To != "mytool" {
		t.Fatalf("Expected auto-learned user rule, got ok=%v rule=%+v", ok, rule)
	}

	entry, ok := history.Lookup("mytypo")
	if !ok {
		t.Fatal("Expected history entry to remain after auto-learn")
	}
	if !entry.RuleApplied {
		t.Fatal("Expected history entry to be frozen after auto-learn")
	}
}

func TestEngine_MaybeAutoLearnFromHistory_MarksExistingUserRule(t *testing.T) {
	tmpDir := t.TempDir()
	history := NewHistory(tmpDir)
	rules := NewRules(tmpDir)
	if err := rules.AddUserRule(itypes.Rule{From: "mytypo", To: "mytool"}); err != nil {
		t.Fatalf("AddUserRule failed: %v", err)
	}

	eng := NewEngine(
		WithHistory(history),
		WithRules(rules),
		WithAutoLearnThreshold(2),
	)

	if err := eng.RecordHistory("mytypo", "mytool"); err != nil {
		t.Fatalf("RecordHistory failed: %v", err)
	}

	result := eng.maybeAutoLearnFromHistory(context.Background(), "mytypo", "mytool")
	if !result.Triggered || !result.Persisted || result.Err != nil {
		t.Fatalf("maybeAutoLearnFromHistory() = %+v", result)
	}

	entry, ok := history.Lookup("mytypo")
	if !ok || !entry.RuleApplied {
		t.Fatalf("Expected history entry to be marked as rule applied, got %+v ok=%v", entry, ok)
	}
}

func TestEngine_MaybeAutoLearnFromHistory_RespectsContextDeadline(t *testing.T) {
	tmpDir := t.TempDir()
	history := NewHistory(tmpDir)
	rules := NewRules(tmpDir)
	eng := NewEngine(
		WithHistory(history),
		WithRules(rules),
		WithAutoLearnThreshold(1),
	)

	if err := eng.RecordHistory("mytypo", "mytool"); err != nil {
		t.Fatalf("RecordHistory failed: %v", err)
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	result := eng.maybeAutoLearnFromHistory(ctx, "mytypo", "mytool")
	if !result.TimedOut {
		t.Fatalf("Expected timed out result, got %+v", result)
	}
	if _, ok := rules.MatchUser("mytypo"); ok {
		t.Fatal("Expected expired context to skip user rule creation")
	}
}

func TestEngine_Priority(t *testing.T) {
	tmpDir := t.TempDir()
	history := NewHistory(tmpDir)
	rules := NewRules(tmpDir)

	// Set up history with a different correction than the rule
	if err := history.Record("gut", "customgit"); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

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
	if err := rules.AddUserRule(itypes.Rule{From: "gut", To: "mygit"}); err != nil {
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

	subcmdRegistry := commands.NewToolTreeRegistry(tmpDir)
	subcmdRegistry.Get("git")

	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"git"}),
		WithKeyboard(NewQWERTYKeyboard()),
		WithToolTrees(subcmdRegistry),
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
	subcmdRegistry := commands.NewToolTreeRegistry(tmpDir)
	subcmdRegistry.Get("git")

	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"git"}),
		WithKeyboard(NewQWERTYKeyboard()),
		WithToolTrees(subcmdRegistry),
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
	cache := struct {
		SchemaVersion int                       `json:"schema_version"`
		Tools         []*commands.ToolTreeCache `json:"tools"`
	}{
		SchemaVersion: 2,
		Tools: []*commands.ToolTreeCache{
			toolTreeCache("git", map[string]*commands.TreeNode{
				"status": {}, "remote": {}, "rev-parse": {}, "ls-files": {},
			}),
			toolTreeCache("docker", map[string]*commands.TreeNode{
				"build": {}, "run": {}, "ps": {}, "images": {}, "logs": {}, "compose": {},
			}),
			toolTreeCache("npm", map[string]*commands.TreeNode{
				"install": {},
				"run":     {},
				"test":    {},
				"publish": {},
				"ci":      {},
				"config": {
					Children: map[string]*commands.TreeNode{
						"get":  {},
						"set":  {},
						"list": {},
					},
				},
			}),
			toolTreeCache("yarn", map[string]*commands.TreeNode{
				"add": {}, "build": {}, "cache": {}, "init": {}, "install": {}, "remove": {}, "run": {}, "test": {},
			}),
			toolTreeCache("cargo", map[string]*commands.TreeNode{
				"bench": {}, "build": {}, "check": {}, "fmt": {}, "run": {}, "test": {},
			}),
			toolTreeCache("go", map[string]*commands.TreeNode{
				"build": {}, "env": {}, "fmt": {}, "generate": {}, "get": {}, "install": {}, "run": {}, "test": {},
				"mod": {
					Children: map[string]*commands.TreeNode{
						"download": {},
						"edit":     {},
						"init":     {},
						"tidy":     {},
						"vendor":   {},
					},
				},
			}),
			toolTreeCache("pip", map[string]*commands.TreeNode{
				"install": {}, "uninstall": {}, "list": {}, "show": {}, "freeze": {},
			}),
			toolTreeCache("pip3", map[string]*commands.TreeNode{
				"install": {}, "uninstall": {}, "list": {}, "show": {}, "freeze": {},
			}),
			toolTreeCache("composer", map[string]*commands.TreeNode{
				"install": {}, "update": {}, "require": {}, "remove": {}, "dump-autoload": {},
			}),
			toolTreeCache("kubectl", map[string]*commands.TreeNode{
				"get": {}, "describe": {}, "apply": {}, "delete": {}, "logs": {},
			}),
			toolTreeCache("brew", map[string]*commands.TreeNode{
				"install": {}, "update": {}, "upgrade": {}, "list": {}, "search": {},
				"services": {
					Children: map[string]*commands.TreeNode{
						"start":   {},
						"stop":    {},
						"restart": {},
						"list":    {},
					},
				},
			}),
			toolTreeCache("terraform", map[string]*commands.TreeNode{
				"init": {}, "plan": {}, "apply": {}, "destroy": {}, "validate": {},
				"state": {
					Children: map[string]*commands.TreeNode{
						"list": {},
						"show": {},
						"mv":   {},
						"rm":   {},
					},
				},
			}),
			toolTreeCache("helm", map[string]*commands.TreeNode{
				"install": {}, "upgrade": {}, "template": {}, "lint": {}, "list": {},
				"repo": {
					Children: map[string]*commands.TreeNode{
						"add":    {},
						"list":   {},
						"remove": {},
						"update": {},
					},
				},
			}),
			toolTreeCache("aws", map[string]*commands.TreeNode{
				"s3": {
					Children: map[string]*commands.TreeNode{
						"cp": {}, "ls": {}, "mb": {}, "mv": {}, "rm": {}, "sync": {},
					},
				},
				"ec2": {
					Children: map[string]*commands.TreeNode{
						"describe-instances": {},
						"start-instances":    {},
						"stop-instances":     {},
					},
				},
				"cloudformation": {
					Children: map[string]*commands.TreeNode{
						"wait": {
							Children: map[string]*commands.TreeNode{
								"stack-create-complete": {},
								"stack-update-complete": {},
							},
						},
					},
				},
				"configure": {},
			}),
			toolTreeCache("gcloud", map[string]*commands.TreeNode{
				"auth": {},
				"compute": {
					Children: map[string]*commands.TreeNode{
						"instances": {
							Children: map[string]*commands.TreeNode{
								"describe": {},
								"list":     {},
							},
						},
					},
				},
				"container": {
					Children: map[string]*commands.TreeNode{
						"clusters": {
							Children: map[string]*commands.TreeNode{
								"get-credentials": {},
								"list":            {},
							},
						},
					},
				},
				"config": {
					Children: map[string]*commands.TreeNode{
						"get-value": {},
						"set":       {},
						"list":      {},
					},
				},
			}),
			toolTreeCache("az", map[string]*commands.TreeNode{
				"account": {
					Children: map[string]*commands.TreeNode{
						"list": {},
						"set":  {},
						"show": {},
					},
				},
				"network": {
					Children: map[string]*commands.TreeNode{
						"vnet": {
							Children: map[string]*commands.TreeNode{
								"list": {},
								"subnet": {
									Children: map[string]*commands.TreeNode{
										"create": {},
										"list":   {},
									},
								},
							},
						},
					},
				},
				"group": {
					Children: map[string]*commands.TreeNode{
						"create": {}, "delete": {}, "list": {}, "show": {},
					},
				},
				"storage": {
					Children: map[string]*commands.TreeNode{
						"account": {
							Children: map[string]*commands.TreeNode{
								"list": {},
								"show": {},
							},
						},
					},
				},
				"login": {},
			}),
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

	subcmdRegistry := commands.NewToolTreeRegistry(tmpDir)

	return NewEngine(
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"git", "docker", "npm", "yarn", "cargo", "go", "pip", "pip3", "composer", "kubectl", "brew", "terraform", "helm", "typo", "aws", "gcloud", "az"}),
		WithKeyboard(NewQWERTYKeyboard()),
		WithToolTrees(subcmdRegistry),
		WithCommandTrees(commands.NewCommandTreeRegistry()),
	)
}

func TestEngine_FixWithAliasContext(t *testing.T) {
	eng := newEngineWithCommonToolSubcommands(t)
	aliases := []itypes.AliasContextEntry{
		{Shell: "zsh", Kind: "alias", Name: "k", Expansion: "kubectl"},
		{Shell: "zsh", Kind: "alias", Name: "g", Expansion: "git"},
		{Shell: "zsh", Kind: "alias", Name: "tf", Expansion: "terraform"},
		{Shell: "zsh", Kind: "alias", Name: "kk", Expansion: "k"},
	}

	tests := []struct {
		name    string
		command string
		want    string
	}{
		{name: "kubectl alias subcommand", command: "k lgo", want: "k logs"},
		{name: "git alias subcommand", command: "g stauts", want: "g status"},
		{name: "terraform alias subcommand", command: "tf valdiate", want: "tf validate"},
		{name: "compound aliases", command: "k lgo && g stauts", want: "k logs && g status"},
		{name: "chained alias", command: "kk lgo", want: "kk logs"},
		{name: "wrapper emits canonical command", command: "sudo k lgo", want: "sudo kubectl logs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eng.FixWithContext(itypes.ParserContext{
				Command:      tt.command,
				AliasContext: aliases,
			})
			if !got.Fixed || got.Command != tt.want {
				t.Fatalf("FixWithContext(%q) = %+v, want %q", tt.command, got, tt.want)
			}
		})
	}
}

func TestEngine_FixWithAliasContextParser(t *testing.T) {
	eng := newEngineWithCommonToolSubcommands(t)
	got := eng.FixWithContext(itypes.ParserContext{
		Command:      "g remove -v",
		Stderr:       "git: 'remove' is not a git command.\n\nThe most similar command is\n\tremote\n",
		AliasContext: []itypes.AliasContextEntry{{Shell: "bash", Kind: "alias", Name: "g", Expansion: "git"}},
	})
	if !got.Fixed || got.Command != "g remote -v" || !got.UsedParser {
		t.Fatalf("Expected parser fix to be re-aliased, got %+v", got)
	}
}

func TestEngine_FixWithAliasContextNoVisibleChange(t *testing.T) {
	eng := newEngineWithCommonToolSubcommands(t)
	got := eng.FixWithContext(itypes.ParserContext{
		Command:      "k logs",
		AliasContext: []itypes.AliasContextEntry{{Shell: "fish", Kind: "abbr", Name: "k", Expansion: "kubectl"}},
	})
	if got.Fixed {
		t.Fatalf("Expected alias expansion alone to stay invisible, got %+v", got)
	}
}

func TestEngine_FixWithAliasContextLoopFallsBack(t *testing.T) {
	eng := newEngineWithCommonToolSubcommands(t)
	got := eng.FixWithContext(itypes.ParserContext{
		Command: "aa lgo",
		AliasContext: []itypes.AliasContextEntry{
			{Shell: "zsh", Kind: "alias", Name: "aa", Expansion: "bb"},
			{Shell: "zsh", Kind: "alias", Name: "bb", Expansion: "aa"},
		},
	})
	if got.Fixed {
		t.Fatalf("Expected cyclic alias context to be ignored, got %+v", got)
	}
}

func toolTreeCache(tool string, children map[string]*commands.TreeNode) *commands.ToolTreeCache {
	return &commands.ToolTreeCache{
		Tool:      tool,
		Tree:      &commands.TreeNode{Children: children},
		UpdatedAt: time.Now(),
	}
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
		{
			name: "gcloud nested subcommand typo after interleaved option with value",
			//nolint:misspell
			cmd:     "gcloud compute --zone us-east1 isntances listt",
			wantCmd: "gcloud compute --zone us-east1 instances list",
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

func TestEngine_CloudProviderCommonCommands_CanBeFixed(t *testing.T) {
	eng := NewEngine(
		WithRules(NewRules(t.TempDir())),
		WithCommands(commands.DiscoverCommon()),
		WithKeyboard(NewQWERTYKeyboard()),
	)

	tests := []struct {
		name    string
		cmd     string
		wantCmd string
	}{
		{name: "aws sam cli", cmd: "samm build", wantCmd: "sam build"},
		{name: "aws cdk cli", cmd: "cdkk deploy", wantCmd: "cdk deploy"},
		{name: "aws eksctl cli", cmd: "eksctll create cluster", wantCmd: "eksctl create cluster"},
		{name: "google cloud storage cli", cmd: "gsutill ls gs://bucket", wantCmd: "gsutil ls gs://bucket"},
		{name: "azure functions cli", cmd: "funcc start", wantCmd: "func start"},
		{name: "azure developer cli", cmd: "azdd up", wantCmd: "azd up"},
		{name: "digitalocean cli", cmd: "doclt compute droplet list", wantCmd: "doctl compute droplet list"},
		{name: "oracle cloud cli", cmd: "occi os ns get", wantCmd: "oci os ns get"},
		{name: "linode cli", cmd: "linode-clii list", wantCmd: "linode-cli list"},
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

func TestEngine_CloudProviderCommonCommands_DoNotRewriteValidShortCommands(t *testing.T) {
	eng := NewEngine(
		WithRules(NewRules(t.TempDir())),
		WithCommands(commands.DiscoverCommon()),
		WithKeyboard(NewQWERTYKeyboard()),
	)

	for _, cmd := range []string{
		"cd app",
		"az group list",
		"sam deploy",
		"oci os ns get",
	} {
		t.Run(cmd, func(t *testing.T) {
			result := eng.Fix(cmd, "")
			if result.Fixed {
				t.Fatalf("Expected valid command %q to stay unchanged, got %+v", cmd, result)
			}
		})
	}
}

func TestKeyboardByName(t *testing.T) {
	tests := []struct {
		name    string
		layout  string
		wantErr bool
	}{
		{name: "default qwerty", layout: "qwerty"},
		{name: "dvorak", layout: "dvorak"},
		{name: "colemak", layout: "colemak"},
		{name: "unknown", layout: "unknown", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kb, err := KeyboardByName(tt.layout)
			if tt.wantErr {
				if err == nil {
					t.Fatal("KeyboardByName should fail")
				}
				return
			}
			if err != nil {
				t.Fatalf("KeyboardByName failed: %v", err)
			}
			if kb == nil {
				t.Fatal("KeyboardByName returned nil keyboard")
			}
		})
	}
}

func TestAdditionalKeyboardLayouts_IsAdjacent(t *testing.T) {
	dvorak := NewDvorakKeyboard()
	if !dvorak.IsAdjacent('a', 'o') {
		t.Fatal("Dvorak keyboard should treat a and o as adjacent")
	}
	if dvorak.IsAdjacent('a', 'z') {
		t.Fatal("Dvorak keyboard should not treat a and z as adjacent")
	}

	colemak := NewColemakKeyboard()
	if !colemak.IsAdjacent('a', 'r') {
		t.Fatal("Colemak keyboard should treat a and r as adjacent")
	}
	if colemak.IsAdjacent('a', 'p') {
		t.Fatal("Colemak keyboard should not treat a and p as adjacent")
	}
}

func TestEngineConfigurableDistanceMatch(t *testing.T) {
	matchCfg := distanceMatchConfig{
		keyboard:            NewQWERTYKeyboard(),
		maxEditDistance:     2,
		similarityThreshold: 0.6,
	}

	if !isGoodDistanceMatch("cloen", "clone", 2, matchCfg) {
		t.Fatal("expected threshold 0.6 and distance 2 to accept clone")
	}
	matchCfg.similarityThreshold = 0.61
	if isGoodDistanceMatch("cloen", "clone", 2, matchCfg) {
		t.Fatal("expected threshold 0.61 to reject clone")
	}
	matchCfg.similarityThreshold = 0.6
	matchCfg.maxEditDistance = 1
	if isGoodDistanceMatch("cloen", "clone", 2, matchCfg) {
		t.Fatal("expected max edit distance 1 to reject clone")
	}
}

func TestEngineMaxFixPasses(t *testing.T) {
	tmpDir := t.TempDir()
	rules := NewRules(tmpDir)
	if err := rules.AddUserRule(itypes.Rule{From: "aaa", To: "bbb"}); err != nil {
		t.Fatalf("AddUserRule aaa failed: %v", err)
	}
	if err := rules.AddUserRule(itypes.Rule{From: "bbb", To: "ccc"}); err != nil {
		t.Fatalf("AddUserRule bbb failed: %v", err)
	}

	onePass := NewEngine(WithRules(rules), WithMaxFixPasses(1))
	if got := onePass.Fix("aaa", ""); !got.Fixed || got.Command != "bbb" {
		t.Fatalf("onePass.Fix() = %+v, want bbb", got)
	}

	twoPasses := NewEngine(WithRules(rules), WithMaxFixPasses(2))
	if got := twoPasses.Fix("aaa", ""); !got.Fixed || got.Command != "ccc" {
		t.Fatalf("twoPasses.Fix() = %+v, want ccc", got)
	}
}

func TestEngineOptionsAndDisabledCommands(t *testing.T) {
	eng := NewEngine(
		WithSimilarityThreshold(0.8),
		WithMaxEditDistance(4),
		WithMaxFixPasses(9),
		WithDisabledCommands([]string{"git", "docker"}),
		WithCommands([]string{"git", "docker", "grep"}),
	)

	if eng.similarityThreshold != 0.8 {
		t.Fatalf("similarityThreshold = %v, want 0.8", eng.similarityThreshold)
	}
	if eng.maxEditDistance != 4 {
		t.Fatalf("maxEditDistance = %d, want 4", eng.maxEditDistance)
	}
	if eng.maxFixPasses != 9 {
		t.Fatalf("maxFixPasses = %d, want 9", eng.maxFixPasses)
	}
	if !eng.disabledCommands["git"] || !eng.disabledCommands["docker"] {
		t.Fatalf("disabledCommands not applied: %v", eng.disabledCommands)
	}

	available := eng.availableCommands()
	if len(available) != 1 || available[0] != "grep" {
		t.Fatalf("availableCommands() = %v, want [grep]", available)
	}

	filtered := eng.filterDisabledCommands([]string{"git", "grep", "docker", "sed"})
	if len(filtered) != 2 || filtered[0] != "grep" || filtered[1] != "sed" {
		t.Fatalf("filterDisabledCommands() = %v, want [grep sed]", filtered)
	}
}

func TestEngineDisabledCommandsOptionOrderDoesNotMatter(t *testing.T) {
	first := NewEngine(
		WithDisabledCommands([]string{"git", "docker"}),
		WithCommands([]string{"git", "docker", "grep"}),
	)
	second := NewEngine(
		WithCommands([]string{"git", "docker", "grep"}),
		WithDisabledCommands([]string{"git", "docker"}),
	)

	firstAvailable := first.availableCommands()
	secondAvailable := second.availableCommands()

	if len(firstAvailable) != 1 || firstAvailable[0] != "grep" {
		t.Fatalf("first.availableCommands() = %v, want [grep]", firstAvailable)
	}
	if len(secondAvailable) != 1 || secondAvailable[0] != "grep" {
		t.Fatalf("second.availableCommands() = %v, want [grep]", secondAvailable)
	}
}

func TestEngineAvailableCommandsUsesCachedSlice(t *testing.T) {
	eng := NewEngine(
		WithDisabledCommands([]string{"git", "docker"}),
		WithCommands([]string{"git", "docker", "grep"}),
	)

	allocs := testing.AllocsPerRun(1000, func() {
		available := eng.availableCommands()
		if len(available) != 1 || available[0] != "grep" {
			t.Fatalf("availableCommands() = %v, want [grep]", available)
		}
	})

	if allocs != 0 {
		t.Fatalf("availableCommands() allocs = %v, want 0", allocs)
	}
}
