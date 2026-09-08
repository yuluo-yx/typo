package parser

import (
	"strings"
	"testing"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

func TestSuggestionsPreserveWholeCommandNames(t *testing.T) {
	tests := []struct {
		parser                itypes.Parser
		command, stderr, want string
	}{
		{NewGitParser(), "git cherry-pik abc123", "git: 'cherry-pik' is not a git command.\nThe most similar command is\n\tcherry-pick\n", "git cherry-pick abc123"},
		{NewGitParser(), "git -C 'my repo' checkuot-index -- 'a  b'", "git: 'checkuot-index' is not a git command.\nThe most similar command is\n\tcheckout-index\n", "git -C 'my repo' checkout-index -- 'a  b'"},
		{NewDockerParser(), "docker --context prod custom-plugn arg", "docker: 'custom-plugn' is not a docker command.\nSimilar command: custom-plugin\n", "docker --context prod custom-plugin arg"},
		{NewDockerParser(), "docker custom-plugn arg", "unknown command: custom-plugn\nDid you mean: custom-plugin\n", "docker custom-plugin arg"},
		{NewNpmParser(), "npm --prefix 'my app' run-scrip build", "npm ERR! command run-scrip not found\nnpm ERR! Did you mean run-script\n", "npm --prefix 'my app' run-script build"},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := tt.parser.Parse(itypes.ParserContext{Command: tt.command, Stderr: tt.stderr})
			if !got.Fixed || got.Command != tt.want {
				t.Fatalf("Parse() = %+v, want %q", got, tt.want)
			}
		})
	}
}

func TestSuggestionDoesNotAcceptPartialTokens(t *testing.T) {
	tests := []struct {
		parser          itypes.Parser
		command, prefix string
	}{
		{NewGitParser(), "git cherry-pik", "git: 'cherry-pik' is not a git command.\nThe most similar command is\n\t"},
		{NewDockerParser(), "docker custom-plugn", "docker: 'custom-plugn' is not a docker command.\nSimilar command: "},
		{NewDockerParser(), "docker custom-plugn", "unknown command: custom-plugn\nDid you mean: "},
		{NewNpmParser(), "npm run-scrip", "npm ERR! Did you mean "},
		{NewGenericParser(), "tool custom-plugn", "Unknown command 'custom-plugn'.\nDid you mean this?\n\t"},
	}
	for _, tt := range tests {
		for _, token := range []string{"custom-plugin/other", "custom-plugin;echo", "custom-plugin$VALUE", "custom-plugin.extra"} {
			t.Run(tt.command+token+tt.prefix, func(t *testing.T) {
				got := tt.parser.Parse(itypes.ParserContext{Command: tt.command, Stderr: tt.prefix + token + "\n"})
				if got.Fixed {
					t.Fatalf("partial suggestion was accepted: %+v", got)
				}
			})
		}
	}
}

func TestGenericSuggestionPreservesLongArgumentList(t *testing.T) {
	command := "tool custom-plugn" + strings.Repeat(" 'literal  argument'", 10000)
	got := NewGenericParser().Parse(itypes.ParserContext{Command: command, Stderr: "Unknown command 'custom-plugn'. Did you mean 'custom-plugin'?"})
	want := strings.Replace(command, "custom-plugn", "custom-plugin", 1)
	if !got.Fixed || got.Command != want {
		t.Fatal("long argument list was not preserved")
	}
}

func TestGenericSuggestionTargetIsUnambiguous(t *testing.T) {
	tests := []struct {
		command, stderr, want string
	}{
		{"poetry --directory addd addd requests", "Unknown command 'addd'. Did you mean 'add'?", ""},
		{"poetry --directory 'addd' addd requests", "Unknown command 'addd'. Did you mean 'add'?", ""},
		{"poetry --directory addd install", "Unknown command 'addd'. Did you mean 'add'?", ""},
		{"poetry --directory project addd requests", "Did you mean 'add'?", ""},
		{"poetry --directory project addd requests", "Unknown command 'addd'. Did you mean 'add'?", "poetry --directory project add requests"},
		{"poetry --directory=project addd requests", "Unknown command 'addd'. Did you mean 'add'?", "poetry --directory=project add requests"},
		{"cargo --color=always buid", "Unknown command 'buid'. Did you mean 'build'?", "cargo --color=always build"},
		{"tool --verbose buid", "Unknown command 'buid'. Did you mean 'build'?", ""},
		{"tool -o=data buid", "Unknown command 'buid'. Did you mean 'build'?", ""},
		{"poetry addd addd", "Unknown command 'addd'. Did you mean 'add'?", ""},
		{"cargo 'buid' --release", "Unknown command 'buid'. Did you mean 'build'?", "cargo build --release"},
		{"cargo buid --release", "Did you mean 'build'?", "cargo build --release"},
		{"tool -- data", "Did you mean 'date'?", ""},
	}
	for _, tt := range tests {
		t.Run(tt.command+tt.stderr, func(t *testing.T) {
			got := NewGenericParser().Parse(itypes.ParserContext{Command: tt.command, Stderr: tt.stderr})
			if got.Fixed != (tt.want != "") || got.Command != tt.want {
				t.Fatalf("Parse() = %+v, want %q", got, tt.want)
			}
		})
	}
}

func TestGenericReportedCommandUsesWholeToken(t *testing.T) {
	tests := []struct {
		command, stderr, want string
	}{
		{"tool remote remote.list", `Unknown command "remote.list". Did you mean "remote-list"?`, "tool remote remote-list"},
		{"tool remote remote.list", "Unknown command 'remote.list'. Did you mean 'remote-list'?", "tool remote remote-list"},
		{"tool remote remote.list", "no such subcommand: `remote.list`\nDid you mean `remote-list`?", "tool remote remote-list"},
		{"tool remote remote.list", "unknown command: remote.list\nDid you mean 'remote-list'?", "tool remote remote-list"},
		{"tool remote 'remote list'", `Unknown command "remote list". Did you mean "remote-list"?`, "tool remote remote-list"},
		{"tool remote remote.list", `The command "remote.list" is not defined. Did you mean "remote-list"?`, "tool remote remote-list"},
		{"tool remote remote.list", "command remote.list not found. Did you mean 'remote-list'?", "tool remote remote-list"},
		{"tool remote remote/list", `Unknown command "remote/list". Did you mean "remote-list"?`, "tool remote remote-list"},
		{"tool remote remote+list", `Unknown command "remote+list". Did you mean "remote-list"?`, "tool remote remote-list"},
		{"tool remote", `Unknown command "remote.list". Did you mean "remote-list"?`, ""},
		{"tool -- file remote.list", `Unknown command "remote.list". Did you mean "remote-list"?`, ""},
		{"tool -- file remote.list", "Unknown command 'remote.list'. Did you mean 'remote-list'?", ""},
		{"tool remote.list -- 'file name'", `Unknown command "remote.list". Did you mean "remote-list"?`, "tool remote-list -- 'file name'"},
	}
	for _, tt := range tests {
		t.Run(tt.command+tt.stderr, func(t *testing.T) {
			got := NewGenericParser().Parse(itypes.ParserContext{Command: tt.command, Stderr: tt.stderr})
			if got.Fixed != (tt.want != "") || got.Command != tt.want {
				t.Fatalf("Parse() = %+v, want %q", got, tt.want)
			}
		})
	}
}
