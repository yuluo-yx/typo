package parser

import (
	"context"
	"errors"
	"slices"
	"testing"

	itypes "github.com/yuluo-yx/typo/internal/types"
	"github.com/yuluo-yx/typo/internal/utils"
)

type recordingParser struct {
	name   string
	result itypes.ParserResult
	calls  int
}

func (p *recordingParser) Name() string {
	return p.name
}

func (p *recordingParser) Parse(ctx itypes.ParserContext) itypes.ParserResult {
	p.calls++
	return p.result
}

func TestGitParser_Parse(t *testing.T) {
	p := NewGitParser()

	tests := []struct {
		name    string
		cmd     string
		stderr  string
		wantFix bool
		wantCmd string
	}{
		{
			name:    "did you mean - remove to remote",
			cmd:     "git remove -v",
			stderr:  "git: 'remove' is not a git command. See 'git --help'.\n\nThe most similar command is\n\tremote\n",
			wantFix: true,
			wantCmd: "git remote -v",
		},
		{
			name:    "did you mean - comit to commit",
			cmd:     "git comit -m 'test'",
			stderr:  "git: 'comit' is not a git command. See 'git --help'.\n\nThe most similar command is\n\tcommit\n",
			wantFix: true,
			wantCmd: "git commit -m 'test'",
		},
		{
			name:    "did you mean only replaces git subcommand token",
			cmd:     "git -c alias.remvoe=whatever remvoe -v",
			stderr:  "git: 'remvoe' is not a git command. See 'git --help'.\n\nThe most similar command is\n\tremote\n",
			wantFix: true,
			wantCmd: "git -c alias.remvoe=whatever remote -v",
		},
		{
			name:    "pull upstream hint is not inferred",
			cmd:     "git pull",
			stderr:  "There is no tracking information for the current branch.\nPlease specify which branch you want to merge with.\nSee git-pull(1) for details.\n\n    git pull <remote> <branch>\n\nIf you wish to set tracking information for this branch, you can do so with:\n\n    git branch --set-upstream-to=origin/main main\n",
			wantFix: false,
		},
		{
			name:    "pull upstream hint with options is not inferred",
			cmd:     "git pull --rebase --quiet",
			stderr:  "There is no tracking information for the current branch.\n\n    git branch --set-upstream-to=origin/main main\n",
			wantFix: false,
		},
		{
			name:    "no upstream branch rejects placeholder branch",
			cmd:     "git pull",
			stderr:  "There is no tracking information for the current branch.\nPlease specify which branch you want to rebase against.\nSee git-pull(1) for details.\n\n    git pull <remote> <branch>\n\nIf you wish to set tracking information for this branch you can do so with:\n\n    git branch --set-upstream-to=origin/<branch> 0322-yuluo/inprove-add-check\n",
			wantFix: false,
		},
		{
			name:    "no upstream branch rejects placeholder remote and branch",
			cmd:     "git pull",
			stderr:  "There is no tracking information for the current branch.\nPlease specify which branch you want to merge with.\nSee git-pull(1) for details.\n\n    git pull <remote> <branch>\n\nIf you wish to set tracking information for this branch you can do so with:\n\n    git branch --set-upstream-to=<remote>/<branch> 0426-yuluo/fix\n",
			wantFix: false,
		},
		{
			name:    "pull spaced set upstream hint is not inferred",
			cmd:     "git pull",
			stderr:  "There is no tracking information for the current branch.\nPlease specify which branch you want to merge with.\nSee git-pull(1) for details.\n\n    git pull <remote> <branch>\n\nIf you wish to set tracking information for this branch, you can do so with:\n\n    git branch --set-upstream-to origin/main main\n",
			wantFix: false,
		},
		{
			name:    "pull legacy set upstream hint is not inferred",
			cmd:     "git pull",
			stderr:  "There is no tracking information for the current branch.\nPlease specify which branch you want to merge with.\nSee git-pull(1) for details.\n\n    git pull <remote> <branch>\n\nIf you wish to set tracking information for this branch, you can do so with:\n\n    git branch --set-upstream main origin/main\n",
			wantFix: false,
		},
		{
			name:    "no upstream branch rejects cross shell metacharacters",
			cmd:     "git pull",
			stderr:  "There is no tracking information for the current branch.\n\n    git branch --set-upstream-to=origin/feature;touch${IFS}TYPO_PWNED feature;touch${IFS}TYPO_PWNED\n",
			wantFix: false,
		},
		{
			name:    "no upstream branch rejects unsafe remote characters",
			cmd:     "git pull",
			stderr:  "There is no tracking information for the current branch.\n\n    git branch --set-upstream-to=ori;gin/main main\n",
			wantFix: false,
		},
		{
			name:    "no upstream branch rejects ambiguous slash target with local branch",
			cmd:     "git pull",
			stderr:  "There is no tracking information for the current branch.\n\n    git branch --set-upstream-to=team/upstream/feature/topic feature/topic\n",
			wantFix: false,
		},
		{
			name:    "no upstream branch rejects ambiguous slash target",
			cmd:     "git pull",
			stderr:  "There is no tracking information for the current branch.\n\n    git branch --set-upstream-to=team/upstream/main\n",
			wantFix: false,
		},
		{
			name:    "no upstream branch is idempotent once fixed",
			cmd:     "git pull --set-upstream origin main",
			stderr:  "There is no tracking information for the current branch.\nPlease specify which branch you want to merge with.\nSee git-pull(1) for details.\n\n    git pull <remote> <branch>\n\nIf you wish to set tracking information for this branch, you can do so with:\n\n    git branch --set-upstream-to=origin/main main\n",
			wantFix: false,
		},
		{
			name:    "no upstream branch ignores non-pull command",
			cmd:     "git remove -v",
			stderr:  "There is no tracking information for the current branch.\nPlease specify which branch you want to merge with.\nSee git-pull(1) for details.\n\n    git pull <remote> <branch>\n\nIf you wish to set tracking information for this branch, you can do so with:\n\n    git branch --set-upstream-to=origin/main main\n",
			wantFix: false,
		},
		{
			name:    "non-git command",
			cmd:     "npm install",
			stderr:  "some error",
			wantFix: false,
		},
		{
			name:    "unrecognized git error",
			cmd:     "git push",
			stderr:  "fatal: some other error",
			wantFix: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Parse(itypes.ParserContext{Command: tt.cmd, Stderr: tt.stderr})
			if result.Fixed != tt.wantFix {
				t.Errorf("Parse().Fixed = %v, want %v", result.Fixed, tt.wantFix)
			}
			if tt.wantFix && result.Command != tt.wantCmd {
				t.Errorf("Parse().Command = %q, want %q", result.Command, tt.wantCmd)
			}
		})
	}
}

func TestGitParser_Name(t *testing.T) {
	p := NewGitParser()
	if p.Name() != "git" {
		t.Errorf("Name() = %q, want 'git'", p.Name())
	}
}

func TestDockerParser_Parse(t *testing.T) {
	p := NewDockerParser()

	tests := []struct {
		name    string
		cmd     string
		stderr  string
		wantFix bool
		wantCmd string
	}{
		{
			name:    "docker did you mean",
			cmd:     "docker psa",
			stderr:  "docker: 'psa' is not a docker command.\nSimilar command: ps\n\nRun 'docker --help' for more information",
			wantFix: true,
			wantCmd: "docker ps",
		},
		{
			name:    "docker unknown command pattern",
			cmd:     "docker imagesa",
			stderr:  "unknown command: imagesa\n\nDid you mean: images?",
			wantFix: true,
			wantCmd: "docker images",
		},
		{
			name:    "docker replacement does not rewrite option values",
			cmd:     "docker --context psa psa",
			stderr:  "docker: 'psa' is not a docker command.\nSimilar command: ps\n\nRun 'docker --help' for more information",
			wantFix: true,
			wantCmd: "docker --context psa ps",
		},
		{
			name:    "non-docker command",
			cmd:     "git status",
			stderr:  "some error",
			wantFix: false,
		},
		{
			name:    "docker empty stderr",
			cmd:     "docker ps",
			stderr:  "",
			wantFix: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Parse(itypes.ParserContext{Command: tt.cmd, Stderr: tt.stderr})
			if result.Fixed != tt.wantFix {
				t.Errorf("Parse().Fixed = %v, want %v", result.Fixed, tt.wantFix)
			}
			if tt.wantFix && result.Command != tt.wantCmd {
				t.Errorf("Parse().Command = %q, want %q", result.Command, tt.wantCmd)
			}
		})
	}
}

func TestDockerParser_Name(t *testing.T) {
	p := NewDockerParser()
	if p.Name() != "docker" {
		t.Errorf("Name() = %q, want 'docker'", p.Name())
	}
}

func TestNpmParser_Parse(t *testing.T) {
	p := NewNpmParser()

	tests := []struct {
		name    string
		cmd     string
		stderr  string
		wantFix bool
		wantCmd string
	}{
		{
			name:    "npm did you mean",
			cmd:     "npm isntall",
			stderr:  "npm ERR! code E404\nnpm ERR! 404 Command not found: isntall\nnpm ERR! Did you mean install?",
			wantFix: true,
			wantCmd: "npm install",
		},
		{
			name:    "npm with args",
			cmd:     "npm isntall package",
			stderr:  "npm ERR! code E404\nnpm ERR! 404 Command not found: isntall\nnpm ERR! Did you mean install?",
			wantFix: true,
			wantCmd: "npm install package",
		},
		{
			name:    "npm no suggestion",
			cmd:     "npm unknown",
			stderr:  "npm ERR! some error without suggestion",
			wantFix: false,
		},
		{
			name:    "non-npm command",
			cmd:     "git status",
			stderr:  "some error",
			wantFix: false,
		},
		{
			name:    "npm empty stderr",
			cmd:     "npm install",
			stderr:  "",
			wantFix: false,
		},
		{
			name:    "npm just suggestion",
			cmd:     "npm ist",
			stderr:  "npm ERR! Did you mean list?",
			wantFix: true,
			wantCmd: "npm list",
		},
		{
			name:    "npm command not found without suggestion",
			cmd:     "npm badcmd",
			stderr:  "npm ERR! code E404\nnpm ERR! 404 command badcmd not found",
			wantFix: false,
		},
		{
			name:    "npm just suggestion with args",
			cmd:     "npm ist --depth=0",
			stderr:  "npm ERR! Did you mean list?",
			wantFix: true,
			wantCmd: "npm list --depth=0",
		},
		{
			name:    "npm replacement skips prefix option value",
			cmd:     "npm --prefix web ist",
			stderr:  "npm ERR! Did you mean list?",
			wantFix: true,
			wantCmd: "npm --prefix web list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Parse(itypes.ParserContext{Command: tt.cmd, Stderr: tt.stderr})
			if result.Fixed != tt.wantFix {
				t.Errorf("Parse().Fixed = %v, want %v", result.Fixed, tt.wantFix)
			}
			if tt.wantFix && result.Command != tt.wantCmd {
				t.Errorf("Parse().Command = %q, want %q", result.Command, tt.wantCmd)
			}
		})
	}
}

func TestNpmParser_Name(t *testing.T) {
	p := NewNpmParser()
	if p.Name() != "npm" {
		t.Errorf("Name() = %q, want 'npm'", p.Name())
	}
}

func TestRegistry_Parse(t *testing.T) {
	r := NewRegistry()

	// Test git error
	result := r.Parse(itypes.ParserContext{Command: "git remove -v", Stderr: "git: 'remove' is not a git command.\n\nThe most similar command is\n\tremote\n"})
	if !result.Fixed {
		t.Error("Expected to fix git error")
	}
	if result.Command != "git remote -v" {
		t.Errorf("Expected 'git remote -v', got %q", result.Command)
	}
	if result.Parser != "git" {
		t.Errorf("Expected git parser, got %q", result.Parser)
	}

	// Test unknown error
	result = r.Parse(itypes.ParserContext{Command: "unknown command", Stderr: "unknown error"})
	if result.Fixed {
		t.Error("Expected not to fix unknown error")
	}
}

func TestRegistry_ParseFastPathKeepsPermissionFallback(t *testing.T) {
	r := NewRegistry()

	result := r.Parse(itypes.ParserContext{
		Command:  "docker ps",
		Stderr:   "docker: permission denied\n",
		ExitCode: 1,
	})
	if !result.Fixed {
		t.Fatal("Expected permission fallback to fix docker permission error")
	}
	if result.Command != "sudo docker ps" {
		t.Fatalf("Parse().Command = %q, want %q", result.Command, "sudo docker ps")
	}
	if result.Parser != "permission" {
		t.Fatalf("Parse().Parser = %q, want %q", result.Parser, "permission")
	}
}

func TestRegistry_ParseFastPathKeepsGenericFallback(t *testing.T) {
	r := NewRegistry()

	result := r.Parse(itypes.ParserContext{
		Command: "docker imags",
		Stderr:  "error: unknown command 'imags'. Did you mean 'images'?",
	})
	if !result.Fixed {
		t.Fatal("Expected generic fallback to fix docker command")
	}
	if result.Command != "docker images" {
		t.Fatalf("Parse().Command = %q, want %q", result.Command, "docker images")
	}
	if result.Parser != "generic" {
		t.Fatalf("Parse().Parser = %q, want %q", result.Parser, "generic")
	}
}

func TestRegistry_ParseFastPathDoesNotRepeatDedicatedParser(t *testing.T) {
	gitParser := &recordingParser{name: "git"}
	genericParser := &recordingParser{
		name: "generic",
		result: itypes.ParserResult{
			Fixed:   true,
			Command: "git status",
		},
	}

	r := &Registry{}
	r.Register(gitParser)
	r.Register(genericParser)

	result := r.Parse(itypes.ParserContext{Command: "git stauts", Stderr: "Did you mean 'status'?"})
	if !result.Fixed || result.Command != "git status" {
		t.Fatalf("Parse() = %+v, want fixed git status", result)
	}
	if result.Parser != "generic" {
		t.Fatalf("Parse().Parser = %q, want %q", result.Parser, "generic")
	}
	if gitParser.calls != 1 {
		t.Fatalf("git parser calls = %d, want 1", gitParser.calls)
	}
	if genericParser.calls != 1 {
		t.Fatalf("generic parser calls = %d, want 1", genericParser.calls)
	}
}

func TestRegistry_CanParseCommand(t *testing.T) {
	r := NewRegistry()

	tests := []struct {
		name        string
		parserName  string
		commandName string
		expected    bool
	}{
		{
			name:        "dedicated parser accepts its command",
			parserName:  "git",
			commandName: "git",
			expected:    true,
		},
		{
			name:        "dedicated parser accepts prefixed command",
			parserName:  "git",
			commandName: "git-lfs",
			expected:    true,
		},
		{
			name:        "dedicated parser rejects unrelated command",
			parserName:  "git",
			commandName: "docker",
			expected:    false,
		},
		{
			name:        "fallback parser accepts any command",
			parserName:  "generic",
			commandName: "tool",
			expected:    true,
		},
		{
			name:        "custom parser accepts any command",
			parserName:  "custom",
			commandName: "tool",
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.CanParseCommand(tt.parserName, tt.commandName); got != tt.expected {
				t.Fatalf(
					"CanParseCommand(%q, %q) = %v, want %v",
					tt.parserName,
					tt.commandName,
					got,
					tt.expected,
				)
			}
		})
	}
}

func TestRegistry_Register(t *testing.T) {
	r := &Registry{}

	// Initially no parsers
	result := r.Parse(itypes.ParserContext{Command: "test", Stderr: "error"})
	if result.Fixed {
		t.Error("Expected not to fix with no parsers")
	}

	// Register a parser
	r.Register(NewGitParser())

	// Now should parse git errors
	result = r.Parse(itypes.ParserContext{Command: "git remove -v", Stderr: "git: 'remove' is not a git command.\n\nThe most similar command is\n\tremote\n"})
	if !result.Fixed {
		t.Error("Expected to fix git error after registering parser")
	}
}

func TestPermissionParser_Parse(t *testing.T) {
	p := NewPermissionParser()

	tests := []struct {
		name string
		ctx  itypes.ParserContext
		want string
	}{
		{
			name: "macos mkdir permission denied",
			ctx: itypes.ParserContext{
				Command:  "mkdir 1",
				Stderr:   "mkdir: 1: Permission denied\n",
				ExitCode: 1,
			},
			want: "sudo mkdir 1",
		},
		{
			name: "linux mkdir permission denied",
			ctx: itypes.ParserContext{
				Command:  "mkdir 1",
				Stderr:   "mkdir: cannot create directory '1': Permission denied\n",
				ExitCode: 1,
			},
			want: "sudo mkdir 1",
		},
		{
			name: "already sudo prompt should be ignored",
			ctx: itypes.ParserContext{
				Command:             "mkdir 1",
				Stderr:              "Password:\n",
				ExitCode:            1,
				HasPrivilegeWrapper: true,
			},
		},
		{
			name: "shell builtin should be ignored",
			ctx: itypes.ParserContext{
				Command:  "cd /root",
				Stderr:   "cd: permission denied: /root\n",
				ExitCode: 1,
			},
		},
		{
			name: "multiple commands should be ignored",
			ctx: itypes.ParserContext{
				Command:             "mkdir 1",
				Stderr:              "mkdir: 1: Permission denied\n",
				ExitCode:            1,
				HasMultipleCommands: true,
			},
		},
		{
			name: "redirection should be ignored",
			ctx: itypes.ParserContext{
				Command:        "echo hi",
				Stderr:         "zsh: permission denied: /root/out\n",
				ExitCode:       1,
				HasRedirection: true,
			},
		},
		{
			name: "no stderr should be ignored",
			ctx: itypes.ParserContext{
				Command:  "mkdir 1",
				ExitCode: 1,
			},
		},
		{
			name: "git publickey denied should be ignored",
			ctx: itypes.ParserContext{
				Command:  "git push origin main",
				Stderr:   "git@github.com: Permission denied (publickey).\nfatal: Could not read from remote repository.\n",
				ExitCode: 128,
			},
		},
		{
			name: "docker socket permission denied should still fix",
			ctx: itypes.ParserContext{
				Command:  "docker ps",
				Stderr:   "permission denied while trying to connect to the Docker daemon socket at unix:///var/run/docker.sock\n",
				ExitCode: 1,
			},
			want: "sudo docker ps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Parse(tt.ctx)
			if tt.want == "" {
				if result.Fixed {
					t.Fatalf("Parse() = %+v, want no fix", result)
				}
				return
			}
			if !result.Fixed {
				t.Fatalf("Expected fix, got %+v", result)
			}
			if result.Command != tt.want {
				t.Fatalf("Parse().Command = %q, want %q", result.Command, tt.want)
			}
		})
	}
}

func TestPermissionParser_Helpers(t *testing.T) {
	p := NewPermissionParser()
	if p.Name() != "permission" {
		t.Fatalf("Name() = %q, want permission", p.Name())
	}

	tests := []struct {
		input string
		want  string
	}{
		{input: "git status", want: "git"},
		{input: "   docker ps", want: "docker"},
		{input: "", want: ""},
	}

	for _, tt := range tests {
		if got := firstCommandWord(tt.input); got != tt.want {
			t.Fatalf("firstCommandWord(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}

	if got := p.Parse(itypes.ParserContext{
		Command:          "chmod 644 /etc/hosts",
		Stderr:           "operation not permitted",
		ExitCode:         1,
		ShellParseFailed: true,
	}); got.Fixed {
		t.Fatalf("Expected shell parse failure to skip permission fix, got %+v", got)
	}

	if got := p.Parse(itypes.ParserContext{
		Command:  "chmod 644 /etc/hosts",
		Stderr:   "operation not permitted",
		ExitCode: 1,
	}); !got.Fixed || got.Command != "sudo chmod 644 /etc/hosts" {
		t.Fatalf("Expected strong permission pattern to trigger sudo, got %+v", got)
	}

	if !p.shouldSkipContext(itypes.ParserContext{ExitCode: 0}) {
		t.Fatal("Expected zero exit code to be skipped")
	}
	if !p.shouldSkipStderr("[sudo] password for user:") {
		t.Fatal("Expected password prompt stderr to be skipped")
	}
	if !p.matchesAny("git@github.com: Permission denied (publickey).\nfatal: Could not read from remote repository.\n", p.remoteAuthPatterns) {
		t.Fatal("Expected remote auth pattern to match")
	}
	if p.matchesAny("plain error", p.remoteAuthPatterns) {
		t.Fatal("Did not expect unrelated stderr to match remote auth patterns")
	}

	result := p.sudoResult("tar -xf archive.tar")
	if !result.Fixed || result.Command != "sudo tar -xf archive.tar" || result.Kind != itypes.FixKindPermissionSudo {
		t.Fatalf("Unexpected sudoResult: %+v", result)
	}
}

func TestGitCommandHelpers(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want string
	}{
		{name: "plain subcommand", cmd: "git status", want: "status"},
		{name: "prefixed executable", cmd: "git-status", want: "status"},
		{name: "global option with value", cmd: "git -C repo status", want: "status"},
		{name: "global option with inline value", cmd: "git --git-dir=repo status", want: "status"},
		{name: "double dash stops parsing", cmd: "git -- status", want: ""},
		{name: "unknown option aborts", cmd: "git --mystery status", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gitSubcommand(tt.cmd); got != tt.want {
				t.Fatalf("gitSubcommand(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}

	if got := gitPrefixedSubcommand("git-commit"); got != "commit" {
		t.Fatalf("gitPrefixedSubcommand() = %q, want commit", got)
	}
	if got := gitPrefixedSubcommand("commit"); got != "" {
		t.Fatalf("gitPrefixedSubcommand() = %q, want empty", got)
	}

	optionCases := []struct {
		arg  string
		want gitOptionParseState
	}{
		{arg: "--", want: gitOptionUnknown},
		{arg: "--git-dir", want: gitOptionConsumesNextValue},
		{arg: "--git-dir=repo", want: gitOptionHandled},
		{arg: "--help", want: gitOptionHandled},
		{arg: "-C", want: gitOptionConsumesNextValue},
		{arg: "-abc", want: gitOptionHandled},
		{arg: "status", want: gitOptionNotAnOption},
	}

	for _, tt := range optionCases {
		if got := gitOptionState(tt.arg); got != tt.want {
			t.Fatalf("gitOptionState(%q) = %v, want %v", tt.arg, got, tt.want)
		}
	}

	if got := gitShortOptionState("-x"); got != gitOptionUnknown {
		t.Fatalf("gitShortOptionState(-x) = %v, want gitOptionUnknown", got)
	}

	if name, _, hasInline := utils.SplitInlineValue("--git-dir=repo"); name != "--git-dir" || !hasInline {
		t.Fatalf("SplitInlineValue() = (%q, %v), want (--git-dir, true)", name, hasInline)
	}
	if name, _, hasInline := utils.SplitInlineValue("--help"); name != "--help" || hasInline {
		t.Fatalf("SplitInlineValue() = (%q, %v), want (--help, false)", name, hasInline)
	}

	if !gitCommandHasUpstreamFlag("git pull --set-upstream origin main") {
		t.Fatal("Expected --set-upstream to be detected")
	}
	if !gitCommandHasUpstreamFlag("git pull --set-upstream-to=origin/main") {
		t.Fatal("Expected inline --set-upstream-to to be detected")
	}
	if gitCommandHasUpstreamFlag("git pull origin main") {
		t.Fatal("Did not expect plain pull to report upstream flag")
	}
}

func mustParseShellCall(t *testing.T, raw string) *shellCall {
	t.Helper()

	call, err := parseShellCall(raw)
	if err != nil {
		t.Fatalf("parseShellCall(%q) failed: %v", raw, err)
	}

	return call
}

func TestParserShellHelpers_ReplaceSubcommand(t *testing.T) {
	call := mustParseShellCall(t, "git --git-dir repo status")

	replaced, ok := call.replaceSubcommand("git", "status", "switch", gitParserOptionsWithValues)
	if !ok || replaced != "git --git-dir repo switch" {
		t.Fatalf("replaceSubcommand() = (%q, %v), want (git --git-dir repo switch, true)", replaced, ok)
	}

	if replaced, ok := call.replaceSubcommand("git", "commit", "switch", gitParserOptionsWithValues); ok || replaced != "" {
		t.Fatalf("replaceSubcommand() mismatch = (%q, %v), want empty false", replaced, ok)
	}
}

func TestParserShellHelpers_FindShellSubcommandIndex(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		command string
		opts    map[string]bool
		wantIdx int
	}{
		{name: "git double dash", raw: "git -- status", command: "git", opts: gitParserOptionsWithValues, wantIdx: 2},
		{name: "command mismatch", raw: "npm --prefix web install", command: "docker", opts: dockerParserOptionsWithValues, wantIdx: -1},
		{name: "npm double dash", raw: "npm -- install", command: "npm", opts: npmParserOptionsWithValues, wantIdx: 2},
		{name: "npm long option with value", raw: "npm --prefix web install", command: "npm", opts: npmParserOptionsWithValues, wantIdx: 3},
		{name: "npm short option with value", raw: "npm -C web install", command: "npm", opts: npmParserOptionsWithValues, wantIdx: 3},
		{name: "npm inline value", raw: "npm --cache=/tmp install", command: "npm", opts: npmParserOptionsWithValues, wantIdx: 2},
		{name: "missing subcommand", raw: "npm --prefix web", command: "npm", opts: npmParserOptionsWithValues, wantIdx: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			call := mustParseShellCall(t, tt.raw)
			if got := findShellSubcommandIndex(call.args, tt.command, tt.opts); got != tt.wantIdx {
				t.Fatalf("findShellSubcommandIndex() = %d, want %d", got, tt.wantIdx)
			}
		})
	}
}

func TestParserShellHelpers_ParseFailures(t *testing.T) {
	if _, err := parseShellCall("git '"); err == nil {
		t.Fatal("Expected invalid shell input to fail parsing")
	}
	if _, err := parseShellCall("((1 + 1))"); err == nil {
		t.Fatal("Expected unsupported command shape to fail parsing")
	}
}

func TestCommandParsersRejectShellParseFailures(t *testing.T) {
	gitResult := NewGitParser().Parse(itypes.ParserContext{
		Command: "git remove '",
		Stderr:  "git: 'remove' is not a git command. See 'git --help'.\n\nThe most similar command is\n\tremote\n",
	})
	if gitResult.Fixed {
		t.Fatalf("Expected invalid git command to stay unchanged, got %+v", gitResult)
	}

	dockerResult := NewDockerParser().Parse(itypes.ParserContext{
		Command: "docker psa '",
		Stderr:  "docker: 'psa' is not a docker command.\nSimilar command: ps\n\nRun 'docker --help' for more information",
	})
	if dockerResult.Fixed {
		t.Fatalf("Expected invalid docker command to stay unchanged, got %+v", dockerResult)
	}

	npmResult := NewNpmParser().Parse(itypes.ParserContext{
		Command: "npm ist '",
		Stderr:  "npm ERR! Did you mean list?",
	})
	if npmResult.Fixed {
		t.Fatalf("Expected invalid npm command to stay unchanged, got %+v", npmResult)
	}

	npmNotFound := NewNpmParser().Parse(itypes.ParserContext{
		Command: "npm isntall '",
		Stderr:  "npm ERR! code E404\nnpm ERR! 404 command isntall not found\nnpm ERR! Did you mean install?",
	})
	if npmNotFound.Fixed {
		t.Fatalf("Expected invalid npm command to stay unchanged, got %+v", npmNotFound)
	}

	dockerUnknown := NewDockerParser().Parse(itypes.ParserContext{
		Command: "docker imagesa '",
		Stderr:  "unknown command: imagesa\n\nDid you mean: images?",
	})
	if dockerUnknown.Fixed {
		t.Fatalf("Expected invalid docker command to stay unchanged, got %+v", dockerUnknown)
	}

	genericResult := NewGenericParser().Parse(itypes.ParserContext{
		Command: "cargo bild 'foo   bar",
		Stderr:  "error: no such subcommand: `bild`\n\nDid you mean `build`?",
	})
	if genericResult.Fixed {
		t.Fatalf("Expected invalid generic command to stay unchanged, got %+v", genericResult)
	}
}

func TestGitParser_ParseNoUpstreamPlaceholderWithoutLocalBranch(t *testing.T) {
	result := NewGitParser().Parse(itypes.ParserContext{
		Command: "git pull",
		Stderr:  "There is no tracking information for the current branch.\nPlease specify which branch you want to merge with.\n\n    git branch --set-upstream-to=origin/<branch>\n",
	})
	if result.Fixed {
		t.Fatalf("Expected placeholder upstream without local branch to stay unchanged, got %+v", result)
	}
}

func TestGitParser_ParseBranchSetUpstreamMissing(t *testing.T) {
	stderr := "fatal: the requested upstream branch 'origin' does not exist\n" +
		"hint: If you are planning on basing your work on an upstream\n" +
		"hint: branch that already exists at the remote, you may need to run\n" +
		"hint: \"git fetch\" to retrieve it.\n"

	tests := []struct {
		name           string
		cmd            string
		stderr         string
		resolvedTarget string
		resolveOK      bool
		wantRepository []string
		wantRemote     string
		wantBranch     string
		wantFix        bool
		wantCmd        string
	}{
		{
			name:           "inline target uses current branch",
			cmd:            "git branch --set-upstream-to=origin",
			stderr:         stderr,
			resolvedTarget: "origin/main",
			resolveOK:      true,
			wantRemote:     "origin",
			wantFix:        true,
			wantCmd:        "git branch --set-upstream-to=origin/main",
		},
		{
			name:           "spaced target uses explicit local branch",
			cmd:            "git branch --set-upstream-to origin feature/topic",
			stderr:         stderr,
			resolvedTarget: "origin/feature/topic",
			resolveOK:      true,
			wantRemote:     "origin",
			wantBranch:     "feature/topic",
			wantFix:        true,
			wantCmd:        "git branch --set-upstream-to origin/feature/topic feature/topic",
		},
		{
			name:           "short option is supported",
			cmd:            "git branch -u origin main",
			stderr:         stderr,
			resolvedTarget: "origin/main",
			resolveOK:      true,
			wantRemote:     "origin",
			wantBranch:     "main",
			wantFix:        true,
			wantCmd:        "git branch -u origin/main main",
		},
		{
			name:           "repository selector is preserved for lookup",
			cmd:            "git -C 'repo path' branch --set-upstream-to=origin",
			stderr:         stderr,
			resolvedTarget: "origin/main",
			resolveOK:      true,
			wantRepository: []string{"-C", "repo path"},
			wantRemote:     "origin",
			wantFix:        true,
			wantCmd:        "git -C 'repo path' branch --set-upstream-to=origin/main",
		},
		{
			name:       "missing remote branch is not guessed",
			cmd:        "git branch --set-upstream-to=origin",
			stderr:     stderr,
			wantRemote: "origin",
		},
		{
			name:           "stderr target must match command target",
			cmd:            "git branch --set-upstream-to=upstream",
			stderr:         stderr,
			resolvedTarget: "upstream/main",
			resolveOK:      true,
			wantRemote:     "",
		},
		{
			name:           "concrete missing upstream is not rewritten",
			cmd:            "git branch --set-upstream-to=origin/missing",
			stderr:         "fatal: the requested upstream branch 'origin/missing' does not exist\n",
			resolvedTarget: "origin/main",
			resolveOK:      true,
		},
		{
			name:           "unsafe remote is rejected",
			cmd:            "git branch --set-upstream-to='ori;gin'",
			stderr:         "fatal: the requested upstream branch 'ori;gin' does not exist\n",
			resolvedTarget: "ori;gin/main",
			resolveOK:      true,
		},
		{
			name:           "extra positional arguments are rejected",
			cmd:            "git branch --set-upstream-to=origin main other",
			stderr:         stderr,
			resolvedTarget: "origin/main",
			resolveOK:      true,
		},
		{
			name:           "unrelated global options are rejected",
			cmd:            "git --bare branch --set-upstream-to=origin",
			stderr:         stderr,
			resolvedTarget: "origin/main",
			resolveOK:      true,
		},
		{
			name:           "dynamic repository selector is rejected",
			cmd:            "git -C \"$REPO\" branch --set-upstream-to=origin",
			stderr:         stderr,
			resolvedTarget: "origin/main",
			resolveOK:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewGitParser()
			resolveCalls := 0
			p.resolveBranchUpstream = func(repositoryArgs []string, remote, branch string) (string, bool) {
				resolveCalls++
				if !slices.Equal(repositoryArgs, tt.wantRepository) {
					t.Fatalf("repository args = %q, want %q", repositoryArgs, tt.wantRepository)
				}
				if remote != tt.wantRemote {
					t.Fatalf("remote = %q, want %q", remote, tt.wantRemote)
				}
				if branch != tt.wantBranch {
					t.Fatalf("branch = %q, want %q", branch, tt.wantBranch)
				}
				return tt.resolvedTarget, tt.resolveOK
			}

			result := p.Parse(itypes.ParserContext{Command: tt.cmd, Stderr: tt.stderr})
			if result.Fixed != tt.wantFix {
				t.Fatalf("Parse().Fixed = %v, want %v (%+v)", result.Fixed, tt.wantFix, result)
			}
			if tt.wantFix && result.Command != tt.wantCmd {
				t.Fatalf("Parse().Command = %q, want %q", result.Command, tt.wantCmd)
			}
			if tt.wantRemote == "" && resolveCalls != 0 {
				t.Fatalf("resolver calls = %d, want 0", resolveCalls)
			}
		})
	}
}

func TestResolveGitBranchUpstreamWithRunner_CurrentBranch(t *testing.T) {
	var calls [][]string
	run := func(_ context.Context, args []string) ([]byte, error) {
		calls = append(calls, slices.Clone(args))
		if slices.Contains(args, "symbolic-ref") {
			return []byte("feature/topic\n"), nil
		}
		return nil, nil
	}

	target, ok := resolveGitBranchUpstreamWithRunner(
		t.Context(),
		[]string{"-C", "repo"},
		"origin",
		"",
		run,
	)
	if !ok || target != "origin/feature/topic" {
		t.Fatalf("resolveGitBranchUpstreamWithRunner() = (%q, %v)", target, ok)
	}

	expectedCalls := [][]string{
		{"-C", "repo", "symbolic-ref", "--quiet", "--short", "HEAD"},
		{"-C", "repo", "show-ref", "--verify", "--quiet", "refs/remotes/origin/feature/topic"},
	}
	if len(calls) != len(expectedCalls) {
		t.Fatalf("Git calls = %q, want %q", calls, expectedCalls)
	}
	for i := range calls {
		if !slices.Equal(calls[i], expectedCalls[i]) {
			t.Fatalf("Git call %d = %q, want %q", i, calls[i], expectedCalls[i])
		}
	}
}

func TestResolveGitBranchUpstreamWithRunner_ExplicitBranch(t *testing.T) {
	var calls [][]string
	run := func(_ context.Context, args []string) ([]byte, error) {
		calls = append(calls, slices.Clone(args))
		return nil, nil
	}

	target, ok := resolveGitBranchUpstreamWithRunner(
		t.Context(),
		nil,
		"origin",
		"main",
		run,
	)
	if !ok || target != "origin/main" {
		t.Fatalf("resolveGitBranchUpstreamWithRunner() = (%q, %v)", target, ok)
	}
	expectedCalls := [][]string{
		{"show-ref", "--verify", "--quiet", "refs/heads/main"},
		{"show-ref", "--verify", "--quiet", "refs/remotes/origin/main"},
	}
	if len(calls) != len(expectedCalls) {
		t.Fatalf("Git calls = %q, want %q", calls, expectedCalls)
	}
	for i := range calls {
		if !slices.Equal(calls[i], expectedCalls[i]) {
			t.Fatalf("Git call %d = %q, want %q", i, calls[i], expectedCalls[i])
		}
	}
}

func TestResolveGitBranchUpstreamWithRunner_CurrentBranchFailure(t *testing.T) {
	run := func(_ context.Context, _ []string) ([]byte, error) {
		return nil, errors.New("not a repository")
	}

	if target, ok := resolveGitBranchUpstreamWithRunner(
		t.Context(),
		nil,
		"origin",
		"",
		run,
	); ok || target != "" {
		t.Fatalf("resolveGitBranchUpstreamWithRunner() = (%q, %v), want no target", target, ok)
	}
}

func TestResolveGitBranchUpstreamWithRunner_MissingLocalBranch(t *testing.T) {
	run := func(_ context.Context, _ []string) ([]byte, error) {
		return nil, errors.New("unknown ref")
	}

	if target, ok := resolveGitBranchUpstreamWithRunner(
		t.Context(),
		nil,
		"origin",
		"missing",
		run,
	); ok || target != "" {
		t.Fatalf("resolveGitBranchUpstreamWithRunner() = (%q, %v), want no target", target, ok)
	}
}

func TestResolveGitBranchUpstreamWithRunner_MissingRemoteRef(t *testing.T) {
	run := func(_ context.Context, args []string) ([]byte, error) {
		if slices.Contains(args, "refs/remotes/origin/main") {
			return nil, errors.New("unknown ref")
		}
		return nil, nil
	}

	if target, ok := resolveGitBranchUpstreamWithRunner(
		t.Context(),
		nil,
		"origin",
		"main",
		run,
	); ok || target != "" {
		t.Fatalf("resolveGitBranchUpstreamWithRunner() = (%q, %v), want no target", target, ok)
	}
}

func TestGitParser_ParsePushNoUpstream(t *testing.T) {
	p := NewGitParser()
	stderr := "fatal: The current branch feature/topic has no upstream branch.\n" +
		"To push the current branch and set the remote as upstream, use\n\n" +
		"    git push --set-upstream origin feature/topic\n"
	unsafeStderr := "fatal: The current branch feature;touch${IFS}TYPO_PWNED has no upstream branch.\n" +
		"To push the current branch and set the remote as upstream, use\n\n" +
		"    git push --set-upstream origin feature;touch${IFS}TYPO_PWNED\n"

	tests := []struct {
		name    string
		cmd     string
		stderr  string
		wantFix bool
		wantCmd string
	}{
		{
			name:    "plain push uses git hint",
			cmd:     "git push",
			stderr:  stderr,
			wantFix: true,
			wantCmd: "git push --set-upstream origin feature/topic",
		},
		{
			name:    "safe push option is preserved",
			cmd:     "git push --quiet",
			stderr:  stderr,
			wantFix: true,
			wantCmd: "git push --quiet --set-upstream origin feature/topic",
		},
		{
			name:    "windows CRLF hint is accepted",
			cmd:     "git push",
			stderr:  "fatal: The current branch feature/topic has no upstream branch.\r\nTo push the current branch and set the remote as upstream, use\r\n\r\n    git push --set-upstream origin feature/topic\r\n",
			wantFix: true,
			wantCmd: "git push --set-upstream origin feature/topic",
		},
		{
			name:    "quoted global option is preserved",
			cmd:     "git -C 'path with spaces' push",
			stderr:  stderr,
			wantFix: true,
			wantCmd: "git -C 'path with spaces' push --set-upstream origin feature/topic",
		},
		{
			name:    "git-push form is preserved",
			cmd:     "git-push",
			stderr:  stderr,
			wantFix: true,
			wantCmd: "git-push --set-upstream origin feature/topic",
		},
		{
			name:    "shell metacharacters are rejected across shells",
			cmd:     "git push",
			stderr:  unsafeStderr,
			wantFix: false,
		},
		{
			name:    "interactive history expansion is rejected",
			cmd:     "git push",
			stderr:  "git push --set-upstream origin feature's!urgent\n",
			wantFix: false,
		},
		{
			name:    "hint without no upstream fatal is rejected",
			cmd:     "git push",
			stderr:  "git push --set-upstream origin feature/topic\n",
			wantFix: false,
		},
		{
			name: "fatal and hint branches must match",
			cmd:  "git push",
			stderr: "fatal: The current branch feature/topic has no upstream branch.\n" +
				"git push --set-upstream origin other/topic\n",
			wantFix: false,
		},
		{
			name:    "existing positional argument is not duplicated",
			cmd:     "git push origin",
			stderr:  stderr,
			wantFix: false,
		},
		{
			name:    "existing upstream option is idempotent",
			cmd:     "git push --set-upstream origin feature/topic",
			stderr:  stderr,
			wantFix: false,
		},
		{
			name:    "placeholder hint is not guessed",
			cmd:     "git push",
			stderr:  "git push --set-upstream <remote> <branch>\n",
			wantFix: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Parse(itypes.ParserContext{Command: tt.cmd, Stderr: tt.stderr})
			if result.Fixed != tt.wantFix {
				t.Fatalf("Parse().Fixed = %v, want %v (%+v)", result.Fixed, tt.wantFix, result)
			}
			if tt.wantFix && result.Command != tt.wantCmd {
				t.Fatalf("Parse().Command = %q, want %q", result.Command, tt.wantCmd)
			}
		})
	}
}

func TestGitParser_ParseDivergentPullRebase(t *testing.T) {
	p := NewGitParser()
	stderr := gitDivergentPullStderr()

	tests := []struct {
		name    string
		cmd     string
		wantFix bool
		wantCmd string
	}{
		{
			name:    "plain pull",
			cmd:     "git pull",
			wantFix: true,
			wantCmd: "git pull --rebase",
		},
		{
			name:    "pull with remote and branch",
			cmd:     "git pull origin main",
			wantFix: true,
			wantCmd: "git pull --rebase origin main",
		},
		{
			name:    "pull with global option",
			cmd:     "git -C repo pull origin main",
			wantFix: true,
			wantCmd: "git -C repo pull --rebase origin main",
		},
		{
			name:    "git-pull form",
			cmd:     "git-pull origin main",
			wantFix: true,
			wantCmd: "git-pull --rebase origin main",
		},
		{
			name:    "unclosed quoted argument stays unchanged",
			cmd:     "git pull 'foo   bar",
			wantFix: false,
		},
		{
			name:    "already has rebase",
			cmd:     "git pull --rebase origin main",
			wantFix: false,
		},
		{
			name:    "already has rebase mode",
			cmd:     "git pull --rebase=merges origin main",
			wantFix: false,
		},
		{
			name:    "already has no rebase",
			cmd:     "git pull --no-rebase origin main",
			wantFix: false,
		},
		{
			name:    "already has ff only",
			cmd:     "git pull --ff-only origin main",
			wantFix: false,
		},
		{
			name:    "non pull command",
			cmd:     "git fetch origin main",
			wantFix: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Parse(itypes.ParserContext{Command: tt.cmd, Stderr: stderr})
			if result.Fixed != tt.wantFix {
				t.Fatalf("Parse().Fixed = %v, want %v (%+v)", result.Fixed, tt.wantFix, result)
			}
			if tt.wantFix && result.Command != tt.wantCmd {
				t.Fatalf("Parse().Command = %q, want %q", result.Command, tt.wantCmd)
			}
		})
	}
}

func TestGitParser_ParseDivergentPullRebaseRequiresGitSignals(t *testing.T) {
	p := NewGitParser()

	tests := []struct {
		name   string
		stderr string
	}{
		{
			name:   "missing fatal signal",
			stderr: "hint: You have divergent branches and need to specify how to reconcile them.\n",
		},
		{
			name:   "missing divergent signal",
			stderr: "fatal: Need to specify how to reconcile divergent branches.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Parse(itypes.ParserContext{Command: "git pull", Stderr: tt.stderr})
			if result.Fixed {
				t.Fatalf("Expected incomplete divergent pull stderr to stay unchanged, got %+v", result)
			}
		})
	}
}

func TestGitParser_PushRejectedDoesNotGuessPullTarget(t *testing.T) {
	p := NewGitParser()
	fetchFirstStderr := "To github.com:yuluo-yx/typo.git\n" +
		" ! [rejected]        main -> main (fetch first)\n" +
		"error: failed to push some refs to 'github.com:yuluo-yx/typo.git'\n" +
		"hint: Updates were rejected because the remote contains work that you do\n" +
		"hint: not have locally. This is usually caused by another repository pushing\n" +
		"hint: to the same ref. You may want to first integrate the remote changes\n" +
		"hint: (e.g., 'git pull ...') before pushing again.\n"

	for _, cmd := range []string{
		"git push",
		"git -C 'path with spaces' push",
		"git push origin main",
		"git push -u origin HEAD",
		"git push origin main other",
	} {
		t.Run(cmd, func(t *testing.T) {
			result := p.Parse(itypes.ParserContext{Command: cmd, Stderr: fetchFirstStderr})
			if result.Fixed {
				t.Fatalf("rejected push target is ambiguous and must stay unchanged: %+v", result)
			}
		})
	}
}

func TestGitParser_ParseDidYouMeanRejectsShellParseFailure(t *testing.T) {
	stderr := "git: 'statsu' is not a git command. See 'git --help'.\n\nThe most similar command is\n\tstatus\n"
	result := NewGitParser().Parse(itypes.ParserContext{
		Command: "git statsu '",
		Stderr:  stderr,
	})
	if result.Fixed {
		t.Fatalf("Expected invalid shell command to stay unchanged, got %+v", result)
	}
}

func TestGitPullRebaseHelpers(t *testing.T) {
	tests := []struct {
		name    string
		run     func() (string, bool)
		wantCmd string
		wantOK  bool
	}{
		{
			name: "plain pull malformed command",
			run: func() (string, bool) {
				return addGitPullRebaseFlag("git pull origin main '")
			},
			wantOK: false,
		},
		{
			name: "git-prefixed malformed command",
			run: func() (string, bool) {
				return addGitPullRebaseFlag("git-pull origin main '")
			},
			wantOK: false,
		},
		{
			name: "no pull token",
			run: func() (string, bool) {
				return addGitPullRebaseFlag("git fetch origin main '")
			},
			wantOK: false,
		},
		{
			name: "empty prefixed command",
			run: func() (string, bool) {
				return addGitPullRebaseFlag("")
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.run()
			if ok != tt.wantOK || got != tt.wantCmd {
				t.Fatalf("helper = %q, %v; want %q, %v", got, ok, tt.wantCmd, tt.wantOK)
			}
		})
	}
}

func gitDivergentPullStderr() string {
	return "$ git pull\n" +
		"hint: You have divergent branches and need to specify how to reconcile them.\n" +
		"hint: You can do so by running one of the following commands sometime before\n" +
		"hint: your next pull:\n" +
		"hint:\n" +
		"hint:   git config pull.rebase false  # merge\n" +
		"hint:   git config pull.rebase true   # rebase\n" +
		"hint:   git config pull.ff only       # fast-forward only\n" +
		"hint:\n" +
		"hint: You can replace \"git config\" with \"git config --global\" to set a default\n" +
		"hint: preference for all repositories. You can also pass --rebase, --no-rebase,\n" +
		"hint: or --ff-only on the command line to override the configured default per\n" +
		"hint: invocation.\n" +
		"fatal: Need to specify how to reconcile divergent branches.\n"
}

func TestIsGitCommand(t *testing.T) {
	tests := []struct {
		cmd      string
		expected bool
	}{
		{"git status", true},
		{"git", true},
		{"npm install", false},
		{"", false},
		{"git-commit", true},
	}

	for _, tt := range tests {
		result := isGitCommand(tt.cmd)
		if result != tt.expected {
			t.Errorf("isGitCommand(%q) = %v, want %v", tt.cmd, result, tt.expected)
		}
	}
}

func TestIsDockerCommand(t *testing.T) {
	tests := []struct {
		cmd      string
		expected bool
	}{
		{"docker ps", true},
		{"docker", true},
		{"git status", false},
		{"", false},
		{"docker-compose", true},
	}

	for _, tt := range tests {
		result := isDockerCommand(tt.cmd)
		if result != tt.expected {
			t.Errorf("isDockerCommand(%q) = %v, want %v", tt.cmd, result, tt.expected)
		}
	}
}

func TestIsNpmCommand(t *testing.T) {
	tests := []struct {
		cmd      string
		expected bool
	}{
		{"npm install", true},
		{"npm", true},
		{"git status", false},
		{"", false},
		{"npm-run", true},
	}

	for _, tt := range tests {
		result := isNpmCommand(tt.cmd)
		if result != tt.expected {
			t.Errorf("isNpmCommand(%q) = %v, want %v", tt.cmd, result, tt.expected)
		}
	}
}

func TestNpmParser_ParseCommandNotFoundWithSuggestion(t *testing.T) {
	p := NewNpmParser()

	// Test: command not found pattern with did you mean suggestion
	result := p.Parse(itypes.ParserContext{
		Command: "npm isntall",
		Stderr:  "npm ERR! code E404\nnpm ERR! 404 command isntall not found\nnpm ERR! Did you mean install?",
	})
	if !result.Fixed {
		t.Error("Expected to fix npm error with command not found and suggestion")
	}
	if result.Command != "npm install" {
		t.Errorf("Expected 'npm install', got %q", result.Command)
	}
}

func TestNpmParser_ParseJustSuggestionNoParts(t *testing.T) {
	p := NewNpmParser()

	// Test: just suggestion with npm command only (no subcommand)
	result := p.Parse(itypes.ParserContext{
		Command: "npm",
		Stderr:  "npm ERR! Did you mean install?",
	})
	if result.Fixed {
		t.Error("Expected not to fix when npm has no subcommand to replace")
	}
}

func TestGenericParser_Name(t *testing.T) {
	p := NewGenericParser()
	if p.Name() != "generic" {
		t.Errorf("Name() = %q, want 'generic'", p.Name())
	}
}

func TestGenericParser_Parse(t *testing.T) {
	p := NewGenericParser()

	tests := []struct {
		name    string
		cmd     string
		stderr  string
		wantFix bool
		wantCmd string
	}{
		{
			// rustup: single-quoted inline hint
			name:    "rustup did you mean",
			cmd:     "rustup taget list",
			stderr:  "error: Unknown command 'taget'. Did you mean 'target'?",
			wantFix: true,
			wantCmd: "rustup target list",
		},
		{
			// cargo: backtick-quoted inline hint with indentation
			name:    "cargo did you mean",
			cmd:     "cargo buid",
			stderr:  "error: no such subcommand: `buid`\n\n\tDid you mean `build`?\n",
			wantFix: true,
			wantCmd: "cargo build",
		},
		{
			// helm: next-line hint after "Did you mean this?"
			name:    "helm did you mean this",
			cmd:     "helm upgraed myrelease ./chart",
			stderr:  "Error: unknown command \"upgraed\" for \"helm\"\n\nDid you mean this?\n\tupgrade\n\nRun 'helm --help' for usage.",
			wantFix: true,
			wantCmd: "helm upgrade myrelease ./chart",
		},
		{
			// gh: next-line hint after "Did you mean this?" — wrong token at position 1
			name:    "gh did you mean this",
			cmd:     "gh pr-lst",
			stderr:  "unknown command \"pr-lst\" for \"gh\"\n\nDid you mean this?\n\tpr-list\n",
			wantFix: true,
			wantCmd: "gh pr-list",
		},
		{
			// kubectl: next-line hint after "Did you mean this?"
			name:    "kubectl did you mean this",
			cmd:     "kubectl appli -f pod.yaml",
			stderr:  "Error: unknown command \"appli\" for \"kubectl\"\n\nDid you mean this?\n\tapply\n",
			wantFix: true,
			wantCmd: "kubectl apply -f pod.yaml",
		},
		{
			// poetry: next-line hint after "Did you mean one of these?"
			name:    "poetry did you mean one of these",
			cmd:     "poetry addd requests",
			stderr:  "The command \"addd\" is not defined.\nDid you mean one of these?\n    add\n    addr\n",
			wantFix: true,
			wantCmd: "poetry add requests",
		},
		{
			name:    "generic replaces reported command after option value",
			cmd:     "poetry --directory project addd requests",
			stderr:  "The command \"addd\" is not defined.\nDid you mean one of these?\n    add\n    addr\n",
			wantFix: true,
			wantCmd: "poetry --directory project add requests",
		},
		{
			// pip: "maybe you meant" double-quoted inline hint
			name:    "pip maybe you meant",
			cmd:     "pip insatll requests",
			stderr:  "ERROR: unknown command \"insatll\" - maybe you meant \"install\"\n",
			wantFix: true,
			wantCmd: "pip install requests",
		},
		{
			// Flag suggestion should be ignored — not a subcommand fix
			name:    "pnpm flag suggestion ignored",
			cmd:     "pnpm install --savde",
			stderr:  "ERR_PNPM_UNKNOWN_OPTIONS  Unknown option: '--savde'\nDid you mean '--save'?\n",
			wantFix: false,
		},
		{
			// No stderr hint at all
			name:    "no hint in stderr",
			cmd:     "sometool badcmd",
			stderr:  "error: unrecognized command\n",
			wantFix: false,
		},
		{
			// Single-word command with no subcommand to replace
			name:    "command with no subcommand",
			cmd:     "rustup",
			stderr:  "Did you mean 'target'?",
			wantFix: false,
		},
		{
			// Git commands should not be double-handled — git parser runs first,
			// but even if it fell through, we verify generic produces a valid fix
			name:    "generic does not break on git-style stderr",
			cmd:     "git comit -m 'msg'",
			stderr:  "git: 'comit' is not a git command.\nDid you mean 'commit'?\n",
			wantFix: true,
			wantCmd: "git commit -m 'msg'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Parse(itypes.ParserContext{Command: tt.cmd, Stderr: tt.stderr})
			if result.Fixed != tt.wantFix {
				t.Errorf("Parse().Fixed = %v, want %v", result.Fixed, tt.wantFix)
			}
			if tt.wantFix && result.Command != tt.wantCmd {
				t.Errorf("Parse().Command = %q, want %q", result.Command, tt.wantCmd)
			}
		})
	}
}

func TestGenericParser_RegisteredLast(t *testing.T) {
	r := NewRegistry()

	// A git error must still be handled by the git parser (not the generic one)
	result := r.Parse(itypes.ParserContext{
		Command: "git comit -m 'msg'",
		Stderr:  "git: 'comit' is not a git command. See 'git --help'.\n\nThe most similar command is\n\tcommit\n",
	})
	if !result.Fixed {
		t.Fatal("Expected registry to fix git error")
	}
	if result.Parser != "git" {
		t.Errorf("Expected git parser to handle git error, got %q", result.Parser)
	}

	// An unknown CLI with a generic hint must be handled by the generic parser
	result = r.Parse(itypes.ParserContext{
		Command: "rustup taget list",
		Stderr:  "error: Unknown command 'taget'. Did you mean 'target'?",
	})
	if !result.Fixed {
		t.Fatal("Expected registry to fix rustup error via generic parser")
	}
	if result.Command != "rustup target list" {
		t.Errorf("Expected 'rustup target list', got %q", result.Command)
	}
	if result.Parser != "generic" {
		t.Errorf("Expected generic parser to handle rustup error, got %q", result.Parser)
	}
}
