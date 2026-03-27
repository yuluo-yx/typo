package engine

import (
	"os"
	"testing"

	"github.com/yuluo-yx/typo/internal/commands"
	"github.com/yuluo-yx/typo/internal/parser"
)

func TestTryMatch(t *testing.T) {
	match := func(s string) (string, bool) {
		switch s {
		case "gut status":
			return "git status", true
		case "gut":
			return "git", true
		default:
			return "", false
		}
	}

	if got := tryMatch("gut status", "rule", match); !got.Fixed || got.Command != "git status" {
		t.Fatalf("Expected full command match, got %+v", got)
	}

	if got := tryMatch("gut branch", "rule", match); !got.Fixed || got.Command != "git branch" {
		t.Fatalf("Expected command word match, got %+v", got)
	}

	if got := tryMatch("valid command", "rule", match); got.Fixed {
		t.Fatalf("Expected no match, got %+v", got)
	}
}

func TestEngine_RebuildCommand(t *testing.T) {
	eng := NewEngine()

	if got := eng.rebuildCommand("git", nil, "rule"); got.Command != "git" {
		t.Fatalf("Expected bare command rebuild, got %+v", got)
	}

	if got := eng.rebuildCommand("git", []string{"status", "--short"}, "rule"); got.Command != "git status --short" {
		t.Fatalf("Expected args to be preserved, got %+v", got)
	}
}

func TestFindSubcommandIndex(t *testing.T) {
	tests := []struct {
		name    string
		mainCmd string
		parts   []string
		wantIdx int
	}{
		{name: "docker long option with value", mainCmd: "docker", parts: []string{"docker", "--context", "prod", "build"}, wantIdx: 3},
		{name: "docker long option with equals", mainCmd: "docker", parts: []string{"docker", "--context=prod", "build"}, wantIdx: 2},
		{name: "docker short flag without value", mainCmd: "docker", parts: []string{"docker", "-v", "build"}, wantIdx: 2},
		{name: "git short option with value", mainCmd: "git", parts: []string{"git", "-C", "repo", "status"}, wantIdx: 3},
		{name: "kubectl double dash", mainCmd: "kubectl", parts: []string{"kubectl", "--", "logs"}, wantIdx: 2},
		{name: "no subcommand", mainCmd: "git", parts: []string{"git"}, wantIdx: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findSubcommandIndex(tt.mainCmd, tt.parts); got != tt.wantIdx {
				t.Fatalf("findSubcommandIndex() = %d, want %d", got, tt.wantIdx)
			}
		})
	}
}

func TestFindSubcommandWordIndex(t *testing.T) {
	tests := []struct {
		name    string
		mainCmd string
		raw     string
		wantIdx int
	}{
		{name: "docker context option", mainCmd: "docker", raw: "docker --context prod build -t app .", wantIdx: 3},
		{name: "git C option", mainCmd: "git", raw: "git -C repo status", wantIdx: 3},
		{name: "kubectl double dash", mainCmd: "kubectl", raw: "kubectl -- logs pod/nginx", wantIdx: 2},
		{name: "docker option with equals", mainCmd: "docker", raw: "docker --context=prod build -t app .", wantIdx: 2},
		{name: "docker short flag without value", mainCmd: "docker", raw: "docker -v build", wantIdx: 2},
		{name: "missing subcommand", mainCmd: "git", raw: "git -C repo", wantIdx: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, err := parseShellCommandLines(tt.raw)
			if err != nil {
				t.Fatalf("parseShellCommandLines failed: %v", err)
			}
			if got := findSubcommandWordIndex(tt.mainCmd, lines[0]); got != tt.wantIdx {
				t.Fatalf("findSubcommandWordIndex() = %d, want %d", got, tt.wantIdx)
			}
		})
	}
}

func TestOptionTakesValue(t *testing.T) {
	if !optionTakesValue("docker", "--context") {
		t.Fatal("Expected docker --context to take a value")
	}
	if optionTakesValue("docker", "--rm") {
		t.Fatal("Expected docker --rm to not be tracked as taking a value")
	}
	if optionTakesValue("unknown", "--context") {
		t.Fatal("Expected unknown command option to return false")
	}
}

func TestEngine_CommandPriority(t *testing.T) {
	tmpDir := t.TempDir()
	rules := NewRules(tmpDir)
	if err := rules.AddUserRule(Rule{From: "dockre", To: "docker"}); err != nil {
		t.Fatalf("AddUserRule failed: %v", err)
	}

	subcommands := commands.NewSubcommandRegistry(tmpDir)
	eng := NewEngine(
		WithRules(rules),
		WithSubcommands(subcommands),
	)

	dockerPriority := eng.commandPriority("docker")
	echoPriority := eng.commandPriority("echo")
	unknownPriority := eng.commandPriority("totally-unknown-command")

	if dockerPriority <= echoPriority {
		t.Fatalf("Expected docker priority (%d) to exceed echo (%d)", dockerPriority, echoPriority)
	}
	if echoPriority <= unknownPriority {
		t.Fatalf("Expected echo priority (%d) to exceed unknown (%d)", echoPriority, unknownPriority)
	}
}

func TestEngine_SystemAndBuiltinCommands_CanBeFixed(t *testing.T) {
	tmpDir := t.TempDir()
	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"ls", "cat", "grep", "tail", "mkdir", "source", "echo", "sudo"}),
		WithKeyboard(NewQWERTYKeyboard()),
	)

	tests := []struct {
		name    string
		cmd     string
		wantCmd string
	}{
		//nolint:misspell // Intentional typo for correction coverage.
		{name: "source typo", cmd: "soruce ~/.zshrc", wantCmd: "source ~/.zshrc"},
		{name: "echo typo", cmd: "echp hello", wantCmd: "echo hello"},
		{name: "echo transposition typo", cmd: "ehco hello", wantCmd: "echo hello"},
		{name: "cat typo", cmd: "cta /tmp/file", wantCmd: "cat /tmp/file"},
		{name: "grep typo", cmd: "gerp main app.log", wantCmd: "grep main app.log"},
		{name: "tail typo", cmd: "taill -n 20 app.log", wantCmd: "tail -n 20 app.log"},
		{name: "mkdir typo", cmd: "mkidr tmpdir", wantCmd: "mkdir tmpdir"},
		{name: "sudo typo", cmd: "sduo cta /tmp/file", wantCmd: "sudo cat /tmp/file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eng.Fix(tt.cmd, "")
			if !result.Fixed {
				t.Fatalf("Expected to fix %q", tt.cmd)
			}
			if result.Command != tt.wantCmd {
				t.Fatalf("Fix().Command = %q, want %q", result.Command, tt.wantCmd)
			}
		})
	}
}

func TestEngine_FixCommand_WithShellWrappers(t *testing.T) {
	tmpDir := t.TempDir()
	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"echo", "grep", "source"}),
		WithKeyboard(NewQWERTYKeyboard()),
	)

	tests := []struct {
		name    string
		cmd     string
		wantCmd string
	}{
		{name: "command wrapper", cmd: "command -p gerp main app.log", wantCmd: "command -p grep main app.log"},
		{name: "builtin wrapper", cmd: "builtin echp hello", wantCmd: "builtin echo hello"},
		//nolint:misspell // Intentional typo for correction coverage.
		{name: "nocorrect wrapper", cmd: "nocorrect soruce ~/.zshrc", wantCmd: "nocorrect source ~/.zshrc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eng.FixCommand(tt.cmd)
			if !result.Fixed {
				t.Fatalf("Expected FixCommand to fix %q", tt.cmd)
			}
			if result.Command != tt.wantCmd {
				t.Fatalf("FixCommand().Command = %q, want %q", result.Command, tt.wantCmd)
			}
		})
	}
}

func TestEngine_CommonTools_WithWrappersAndPipelines(t *testing.T) {
	eng := newEngineWithCommonToolSubcommands(t)
	eng.commands = append(eng.commands, "echo", "grep")

	tests := []struct {
		name    string
		cmd     string
		wantCmd string
	}{
		{name: "docker pipeline", cmd: "dcoker ps | gerp web", wantCmd: "docker ps | grep web"},
		{name: "npm and docker compound", cmd: "nmp isntall lodash && dcoker ps", wantCmd: "npm install lodash && docker ps"},
		{name: "kubectl pipe and echo", cmd: "kubctl get pods | gerp api && echp done", wantCmd: "kubectl get pods | grep api && echo done"},
		//nolint:misspell // Intentional typo for correction coverage.
		{name: "brew or docker", cmd: "brew upgarde || dcoker ps", wantCmd: "brew upgrade || docker ps"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eng.Fix(tt.cmd, "")
			if !result.Fixed {
				t.Fatalf("Expected to fix %q", tt.cmd)
			}
			if result.Command != tt.wantCmd {
				t.Fatalf("Fix().Command = %q, want %q", result.Command, tt.wantCmd)
			}
		})
	}
}

func TestEngine_TryDistance_FallbackBranches(t *testing.T) {
	tmpDir := t.TempDir()
	eng := NewEngine(
		WithRules(NewRules(tmpDir)),
		WithCommands([]string{"git", "myapp"}),
		WithKeyboard(NewQWERTYKeyboard()),
	)

	if got := eng.tryDistance("myap '"); !got.Fixed || got.Command != "myapp '" || got.Source != "distance" {
		t.Fatalf("Expected fallback distance fix, got %+v", got)
	}

	if got := eng.tryDistance("git '"); got.Fixed {
		t.Fatalf("Expected protected command word to skip distance, got %+v", got)
	}

	if got := eng.tryDistance("'"); got.Fixed {
		t.Fatalf("Expected no fix for invalid empty-ish command, got %+v", got)
	}

	if got := eng.tryDistance("myapp '"); got.Fixed {
		t.Fatalf("Expected identical best match to not be changed, got %+v", got)
	}

	if got := eng.tryDistance("zzzz '"); got.Fixed {
		t.Fatalf("Expected low-similarity command to stay unchanged, got %+v", got)
	}
}

func TestEngine_TrySubcommandFix_FallbackBranches(t *testing.T) {
	eng := newEngineWithCommonToolSubcommands(t)

	if got := eng.trySubcommandFix("dcoker biuld '"); !got.Fixed || got.Command != "docker build '" || got.Source != "subcommand" {
		t.Fatalf("Expected fallback subcommand fix, got %+v", got)
	}

	if got := eng.trySubcommandFix("docker build '"); got.Fixed {
		t.Fatalf("Expected valid subcommand to not be changed, got %+v", got)
	}

	if got := eng.trySubcommandFix("ls anything '"); got.Fixed {
		t.Fatalf("Expected command without subcommands to remain unchanged, got %+v", got)
	}

	if got := eng.trySubcommandFix("unknown biuld '"); got.Fixed {
		t.Fatalf("Expected unresolved main command to not be changed, got %+v", got)
	}

	if got := eng.trySubcommandFix("docker zzzz '"); got.Fixed {
		t.Fatalf("Expected low-similarity subcommand to stay unchanged, got %+v", got)
	}

	if got := NewEngine().trySubcommandFix("docker build"); got.Fixed {
		t.Fatalf("Expected nil subcommand registry to return no fix, got %+v", got)
	}
}

func TestEngine_TryParser_FallbackAndShell(t *testing.T) {
	eng := NewEngine(WithParser(parser.NewRegistry()))

	stderr := "git: 'remove' is not a git command.\n\nThe most similar command is\n\tremote\n"
	if got := eng.tryParser(parser.Context{Command: "sudo git remove -v", Stderr: stderr}); !got.Fixed || got.Command != "sudo git remote -v" {
		t.Fatalf("Expected shell parser fix, got %+v", got)
	}

	if got := eng.tryParser(parser.Context{Command: "git remove '", Stderr: stderr}); !got.Fixed || got.Command != "git remote '" {
		t.Fatalf("Expected fallback parser fix, got %+v", got)
	}

	if got := eng.tryParser(parser.Context{Command: "git status", Stderr: "unrecognized error"}); got.Fixed {
		t.Fatalf("Expected parser miss for unrelated stderr, got %+v", got)
	}
}

func TestEngine_TryHistoryAndTryUserRules_SubcommandAware(t *testing.T) {
	eng := newEngineWithCommonToolSubcommands(t)

	history := NewHistory(t.TempDir())
	if err := history.Record("dcoker", "docker"); err != nil {
		t.Fatalf("Record failed: %v", err)
	}
	eng.history = history

	if got := eng.tryHistory("dcoker biuld"); !got.Fixed || got.Command != "docker build" || got.Source != "history" {
		t.Fatalf("Expected history + subcommand fix, got %+v", got)
	}
	if got := eng.tryHistory("unknown tool"); got.Fixed {
		t.Fatalf("Expected unknown history lookup to fail, got %+v", got)
	}

	if err := eng.rules.AddUserRule(Rule{From: "nmp", To: "npm"}); err != nil {
		t.Fatalf("AddUserRule failed: %v", err)
	}
	if got := eng.tryUserRules("nmp rn test"); !got.Fixed || got.Command != "npm run test" || got.Source != "rule" {
		t.Fatalf("Expected user rule + subcommand fix, got %+v", got)
	}
	if got := eng.tryUserRules("unknown tool"); got.Fixed {
		t.Fatalf("Expected unknown user rule lookup to fail, got %+v", got)
	}
}

func TestTryMatchOnCommand_Fallback(t *testing.T) {
	match := func(s string) (string, bool) {
		if s == "gut" {
			return "git", true
		}
		return "", false
	}

	if got := NewEngine().tryMatchOnCommand("gut '", "rule", match); !got.Fixed || got.Command != "git '" {
		t.Fatalf("Expected fallback tryMatchOnCommand to rebuild command, got %+v", got)
	}
}

func TestEngine_FixCommand_PriorityPaths(t *testing.T) {
	tmpDir := t.TempDir()
	history := NewHistory(tmpDir)
	if err := history.Record("gut", "historygit"); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	rules := NewRules(tmpDir)
	if err := rules.AddUserRule(Rule{From: "gut", To: "rulegit"}); err != nil {
		t.Fatalf("AddUserRule failed: %v", err)
	}

	eng := NewEngine(
		WithRules(rules),
		WithHistory(history),
		WithCommands([]string{"source"}),
		WithKeyboard(NewQWERTYKeyboard()),
	)

	if got := eng.FixCommand("gut status"); !got.Fixed || got.Command != "rulegit status" || got.Source != "rule" {
		t.Fatalf("Expected user rule to win, got %+v", got)
	}

	//nolint:misspell // Intentional typo for correction coverage.
	if got := eng.FixCommand("soruce ~/.zshrc"); !got.Fixed || got.Command != "source ~/.zshrc" || got.Source != "distance" {
		t.Fatalf("Expected distance-based FixCommand result, got %+v", got)
	}
}

func TestEngine_FixCommand_FallbackPriorityPaths(t *testing.T) {
	tmpDir := t.TempDir()

	history := NewHistory(tmpDir)
	if err := history.Record("mycmd", "historycmd"); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	rules := NewRules(tmpDir)
	if err := rules.AddUserRule(Rule{From: "myrule", To: "rulecmd"}); err != nil {
		t.Fatalf("AddUserRule failed: %v", err)
	}

	eng := NewEngine(
		WithRules(rules),
		WithHistory(history),
		WithCommands([]string{"docker", "source"}),
		WithKeyboard(NewQWERTYKeyboard()),
	)

	tests := []struct {
		name       string
		cmd        string
		wantFixed  bool
		wantCmd    string
		wantSource string
	}{
		{name: "user rule fallback", cmd: "myrule '", wantFixed: true, wantCmd: "rulecmd '", wantSource: "rule"},
		{name: "history fallback", cmd: "mycmd '", wantFixed: true, wantCmd: "historycmd '", wantSource: "history"},
		{name: "builtin rule fallback", cmd: "dcoker '", wantFixed: true, wantCmd: "docker '", wantSource: "rule"},
		//nolint:misspell // Intentional typo for correction coverage.
		{name: "distance fallback", cmd: "soruce '", wantFixed: true, wantCmd: "source '", wantSource: "distance"},
		{name: "no match fallback", cmd: "totallyunknown '", wantFixed: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eng.FixCommand(tt.cmd)
			if got.Fixed != tt.wantFixed {
				t.Fatalf("FixCommand().Fixed = %v, want %v (%+v)", got.Fixed, tt.wantFixed, got)
			}
			if !tt.wantFixed {
				return
			}
			if got.Command != tt.wantCmd || got.Source != tt.wantSource {
				t.Fatalf("FixCommand() = %+v, want command=%q source=%q", got, tt.wantCmd, tt.wantSource)
			}
		})
	}
}

func TestEngine_LearnAndAddRule_WithoutHistory(t *testing.T) {
	tmpDir := t.TempDir()
	rules := NewRules(tmpDir)
	eng := NewEngine(
		WithRules(rules),
		WithHistory(nil),
	)

	if err := eng.Learn("mytypo", "mycmd"); err != nil {
		t.Fatalf("Learn without history failed: %v", err)
	}
	if err := eng.AddRule("othertyppo", "othercmd"); err != nil {
		t.Fatalf("AddRule without history failed: %v", err)
	}

	if rule, ok := rules.MatchUser("mytypo"); !ok || rule.To != "mycmd" {
		t.Fatalf("Expected learned user rule to be stored, got ok=%v rule=%+v", ok, rule)
	}
	if rule, ok := rules.MatchUser("othertyppo"); !ok || rule.To != "othercmd" {
		t.Fatalf("Expected added user rule to be stored, got ok=%v rule=%+v", ok, rule)
	}
}

func TestEngine_LearnAndAddRule_ErrorPaths(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "typo-rules-file-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	rules := NewRules(tmpFile.Name())
	eng := NewEngine(
		WithRules(rules),
		WithHistory(NewHistory("")),
	)

	if err := eng.Learn("broken", "command"); err == nil {
		t.Fatal("Expected Learn to fail when rules cannot be saved")
	}
	if err := eng.AddRule("broken2", "command2"); err == nil {
		t.Fatal("Expected AddRule to fail when rules cannot be saved")
	}
}

func TestSimilarityAndCommandsEquivalent(t *testing.T) {
	if got := Similarity("", "", DefaultKeyboard); got != 1.0 {
		t.Fatalf("Similarity(empty, empty) = %v, want 1", got)
	}
	if got := Similarity("git", "git", DefaultKeyboard); got != 1.0 {
		t.Fatalf("Similarity(git, git) = %v, want 1", got)
	}
	if got := Similarity("", "git", DefaultKeyboard); got != 0.0 {
		t.Fatalf("Similarity(empty, git) = %v, want 0", got)
	}

	if !commandsEquivalent("git status", "git status") {
		t.Fatal("Expected equal commands to be equivalent")
	}
	if commandsEquivalent("git status", "git branch") {
		t.Fatal("Expected different commands to not be equivalent")
	}
	if commandsEquivalent("git status", "git status --short") {
		t.Fatal("Expected commands with different arity to not be equivalent")
	}
}
