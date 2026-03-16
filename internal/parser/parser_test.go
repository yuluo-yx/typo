package parser

import (
	"testing"
)

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
			name:    "no upstream branch",
			cmd:     "git pull",
			stderr:  "There is no tracking information for the current branch.\nPlease specify which branch you want to merge with.\nSee git-pull(1) for details.\n\n    git pull <remote> <branch>\n\nIf you wish to set tracking information for this branch, you can do so with:\n\n    git branch --set-upstream-to=origin/main main\n",
			wantFix: true,
			wantCmd: "git pull --set-upstream origin main",
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
			result := p.Parse(tt.cmd, tt.stderr)
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
			result := p.Parse(tt.cmd, tt.stderr)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Parse(tt.cmd, tt.stderr)
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
	result := r.Parse("git remove -v", "git: 'remove' is not a git command.\n\nThe most similar command is\n\tremote\n")
	if !result.Fixed {
		t.Error("Expected to fix git error")
	}
	if result.Command != "git remote -v" {
		t.Errorf("Expected 'git remote -v', got %q", result.Command)
	}

	// Test unknown error
	result = r.Parse("unknown command", "unknown error")
	if result.Fixed {
		t.Error("Expected not to fix unknown error")
	}
}

func TestRegistry_Register(t *testing.T) {
	r := &Registry{}

	// Initially no parsers
	result := r.Parse("test", "error")
	if result.Fixed {
		t.Error("Expected not to fix with no parsers")
	}

	// Register a parser
	r.Register(NewGitParser())

	// Now should parse git errors
	result = r.Parse("git remove -v", "git: 'remove' is not a git command.\n\nThe most similar command is\n\tremote\n")
	if !result.Fixed {
		t.Error("Expected to fix git error after registering parser")
	}
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
	result := p.Parse("npm isntall", "npm ERR! code E404\nnpm ERR! 404 command isntall not found\nnpm ERR! Did you mean install?")
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
	result := p.Parse("npm", "npm ERR! Did you mean install?")
	if result.Fixed {
		t.Error("Expected not to fix when npm has no subcommand to replace")
	}
}
