package engine

import (
	"strings"
	"testing"

	"mvdan.cc/sh/v3/syntax"
)

func TestParseShellCommandLines_CompoundWrappers(t *testing.T) {
	raw := "command -p gti status && env --unset HOME FOO=1 gerp main app.log | time -p taill -n 1 app.log"

	lines, err := parseShellCommandLines(raw)
	if err != nil {
		t.Fatalf("parseShellCommandLines failed: %v", err)
	}

	wantWords := []string{"gti", "gerp", "taill"}
	if len(lines) != len(wantWords) {
		t.Fatalf("Expected %d command lines, got %d", len(wantWords), len(lines))
	}

	for i, want := range wantWords {
		if got := lines[i].commandWord(); got != want {
			t.Fatalf("lines[%d].commandWord() = %q, want %q", i, got, want)
		}
	}

	if got := lines[0].replaceCommandWord("git"); got != "command -p git status && env --unset HOME FOO=1 gerp main app.log | time -p taill -n 1 app.log" {
		t.Fatalf("replaceCommandWord() got %q", got)
	}

	if got := lines[1].replaceCommandSuffix("grep main app.log"); got != "command -p gti status && env --unset HOME FOO=1 grep main app.log | time -p taill -n 1 app.log" {
		t.Fatalf("replaceCommandSuffix() got %q", got)
	}

	if !lines[0].hasWrapper("command") {
		t.Fatal("Expected first line to report command wrapper")
	}
}

func TestParseShellCommandLines_RedirectionMetadata(t *testing.T) {
	lines, err := parseShellCommandLines("echo ok > /root/out")
	if err != nil {
		t.Fatalf("parseShellCommandLines failed: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("Expected 1 command line, got %d", len(lines))
	}
	if !lines[0].hasRedirection {
		t.Fatal("Expected redirection metadata to be set")
	}
}

func TestParseShellCommandLines_UnsupportedShape(t *testing.T) {
	if _, err := parseShellCommandLines("((1 + 1))"); err == nil {
		t.Fatal("Expected unsupported command shape error")
	}
}

func TestFindExecutableArgIndex_Wrappers(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantIdx  int
		wantWord string
	}{
		{name: "builtin wrapper", raw: "builtin echo ok", wantIdx: 1, wantWord: "echo"},
		{name: "nocorrect wrapper", raw: "nocorrect gut status", wantIdx: 1, wantWord: "gut"},
		{name: "noglob wrapper", raw: "noglob gerp main file", wantIdx: 1, wantWord: "gerp"},
		{name: "command wrapper option", raw: "command -p gti status", wantIdx: 2, wantWord: "gti"},
		{name: "command wrapper multiple options", raw: "command -p -v git", wantIdx: 3, wantWord: "git"},
		{name: "env wrapper with assignments", raw: "env --unset HOME FOO=1 gti status", wantIdx: 4, wantWord: "gti"},
		{name: "sudo wrapper with value", raw: "sudo -u root gti status", wantIdx: 3, wantWord: "gti"},
		{name: "time wrapper", raw: "time -p gti status", wantIdx: 0, wantWord: "gti"},
		{name: "env wrapper missing command", raw: "env --", wantIdx: -1},
		{name: "sudo wrapper missing command", raw: "sudo -u root", wantIdx: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := parseCallArgs(t, tt.raw)
			got := findExecutableArgIndex(args)
			if got != tt.wantIdx {
				t.Fatalf("findExecutableArgIndex() = %d, want %d", got, tt.wantIdx)
			}
			if got >= 0 && args[got].Lit() != tt.wantWord {
				t.Fatalf("args[%d] = %q, want %q", got, args[got].Lit(), tt.wantWord)
			}
		})
	}
}

func TestFindExecutableArgIndex_SyntheticArgs(t *testing.T) {
	args := []*syntax.Word{
		litWord("time"),
		litWord("-p"),
		litWord("git"),
		litWord("status"),
	}
	if got := findExecutableArgIndex(args); got != 2 {
		t.Fatalf("Expected synthetic time wrapper index 2, got %d", got)
	}

	args = []*syntax.Word{
		litWord("builtin"),
		litWord("nocorrect"),
		litWord("noglob"),
		litWord("grep"),
	}
	if got := findExecutableArgIndex(args); got != 3 {
		t.Fatalf("Expected stacked wrapper index 3, got %d", got)
	}
}

func TestHandleLongWrapperOption(t *testing.T) {
	tests := []struct {
		name          string
		arg           string
		wantHandled   bool
		wantNeedValue bool
	}{
		{name: "non long option", arg: "-u", wantHandled: false, wantNeedValue: false},
		{name: "long option with value inline", arg: "--user=root", wantHandled: true, wantNeedValue: false},
		{name: "long option with separate value", arg: "--user", wantHandled: true, wantNeedValue: true},
		{name: "long flag without value", arg: "--stdin", wantHandled: true, wantNeedValue: false},
		{name: "unknown long option", arg: "--unknown", wantHandled: true, wantNeedValue: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handled, needValue := handleLongWrapperOption(tt.arg, sudoWrapperOptionsWithValues, sudoWrapperOptions)
			if handled != tt.wantHandled || needValue != tt.wantNeedValue {
				t.Fatalf("handleLongWrapperOption(%q) = (%v, %v), want (%v, %v)", tt.arg, handled, needValue, tt.wantHandled, tt.wantNeedValue)
			}
		})
	}
}

func TestHandleShortWrapperOption(t *testing.T) {
	tests := []struct {
		name          string
		arg           string
		wantHandled   bool
		wantNeedValue bool
	}{
		{name: "non option", arg: "git", wantHandled: false, wantNeedValue: false},
		{name: "short option with value", arg: "-u", wantHandled: true, wantNeedValue: true},
		{name: "short flag without value", arg: "-S", wantHandled: true, wantNeedValue: false},
		{name: "combined short flags", arg: "-abc", wantHandled: true, wantNeedValue: false},
		{name: "double dash", arg: "--user", wantHandled: false, wantNeedValue: false},
		{name: "single dash only", arg: "-", wantHandled: false, wantNeedValue: false},
		{name: "unknown short option", arg: "-x", wantHandled: true, wantNeedValue: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handled, needValue := handleShortWrapperOption(tt.arg, sudoWrapperOptionsWithValues, sudoWrapperOptions)
			if handled != tt.wantHandled || needValue != tt.wantNeedValue {
				t.Fatalf("handleShortWrapperOption(%q) = (%v, %v), want (%v, %v)", tt.arg, handled, needValue, tt.wantHandled, tt.wantNeedValue)
			}
		})
	}
}

func TestSkipWrapperOptions(t *testing.T) {
	args := parseCallArgs(t, "sudo --user root --stdin git status")
	if got := skipWrapperOptions(args, 1, sudoWrapperOptionsWithValues, sudoWrapperOptions); got != 4 {
		t.Fatalf("skipWrapperOptions() = %d, want 4", got)
	}

	args = parseCallArgs(t, "sudo -- git status")
	if got := skipWrapperOptions(args, 1, sudoWrapperOptionsWithValues, sudoWrapperOptions); got != 2 {
		t.Fatalf("skipWrapperOptions() with -- = %d, want 2", got)
	}

	args = parseCallArgs(t, "sudo --user root")
	if got := skipWrapperOptions(args, 1, sudoWrapperOptionsWithValues, sudoWrapperOptions); got != -1 {
		t.Fatalf("skipWrapperOptions() missing command = %d, want -1", got)
	}
}

func TestSkipEnvWrapperArgs(t *testing.T) {
	args := parseCallArgs(t, "env --chdir /tmp FOO=1 grep main file")
	if got := skipEnvWrapperArgs(args, 1); got != 4 {
		t.Fatalf("skipEnvWrapperArgs() = %d, want 4", got)
	}

	args = parseCallArgs(t, "env -C /tmp -u HOME grep main file")
	if got := skipEnvWrapperArgs(args, 1); got != 5 {
		t.Fatalf("skipEnvWrapperArgs() short options = %d, want 5", got)
	}

	args = parseCallArgs(t, "env -- grep main file")
	if got := skipEnvWrapperArgs(args, 1); got != 2 {
		t.Fatalf("skipEnvWrapperArgs() with -- = %d, want 2", got)
	}

	args = parseCallArgs(t, "env --unset HOME")
	if got := skipEnvWrapperArgs(args, 1); got != -1 {
		t.Fatalf("skipEnvWrapperArgs() missing command = %d, want -1", got)
	}
}

func TestOffsetToIndex(t *testing.T) {
	if got := offsetToIndex(3, 10); got != 3 {
		t.Fatalf("offsetToIndex(3, 10) = %d, want 3", got)
	}
	if got := offsetToIndex(99, 10); got != 10 {
		t.Fatalf("offsetToIndex(99, 10) = %d, want 10", got)
	}
	if got := offsetToIndex(^uint(0), 10); got != 10 {
		t.Fatalf("offsetToIndex(maxUint, 10) = %d, want 10", got)
	}
}

func TestShellCommandLineReplacementHelpers(t *testing.T) {
	lines, err := parseShellCommandLines("sudo git status")
	if err != nil {
		t.Fatalf("parseShellCommandLines failed: %v", err)
	}
	line := lines[0]

	if got := line.commandSuffixRaw(); got != "git status" {
		t.Fatalf("commandSuffixRaw() = %q, want git status", got)
	}
	if got := line.replaceWords(); got != "sudo git status" {
		t.Fatalf("replaceWords() with no replacements = %q", got)
	}
	if got := line.replaceCommandSuffixDedup("sudo git remote -v"); got != "sudo git remote -v" {
		t.Fatalf("replaceCommandSuffixDedup() = %q, want sudo git remote -v", got)
	}
	if got := line.replaceCommandSuffixDedup("sudo"); got != "sudo sudo" {
		t.Fatalf("replaceCommandSuffixDedup() empty normalized branch = %q, want sudo sudo", got)
	}

	lines, err = parseShellCommandLines("docker --context prod ps")
	if err != nil {
		t.Fatalf("parseShellCommandLines docker failed: %v", err)
	}
	line = lines[0]
	got := line.replaceWords(
		shellWordReplacement{index: 1, value: "--host"},
		shellWordReplacement{index: 2, value: "unix:///var/run/docker.sock"},
		shellWordReplacement{index: 3, value: "run"},
	)
	if got != "docker --host unix:///var/run/docker.sock run" {
		t.Fatalf("replaceWords() = %q", got)
	}
}

func TestTrimShellWordPrefix(t *testing.T) {
	if got := trimShellWordPrefix("git remote -v", nil); got != "git remote -v" {
		t.Fatalf("trimShellWordPrefix() nil prefix = %q, want original", got)
	}
	if got := trimShellWordPrefix("sudo git remote -v", []string{"sudo"}); got != "git remote -v" {
		t.Fatalf("trimShellWordPrefix() = %q, want git remote -v", got)
	}
	if got := trimShellWordPrefix("git remote -v", []string{"sudo"}); got != "git remote -v" {
		t.Fatalf("trimShellWordPrefix() mismatch = %q, want original", got)
	}
	if got := trimShellWordPrefix("broken '", []string{"sudo"}); got != "broken '" {
		t.Fatalf("trimShellWordPrefix() parse failure = %q, want original", got)
	}
}

func parseCallArgs(t *testing.T, raw string) []*syntax.Word {
	t.Helper()

	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(raw+"\n"), "")
	if err != nil {
		t.Fatalf("Failed to parse shell input %q: %v", raw, err)
	}

	var args []*syntax.Word
	syntax.Walk(file, func(node syntax.Node) bool {
		call, ok := node.(*syntax.CallExpr)
		if !ok || len(call.Args) == 0 || args != nil {
			return true
		}
		args = call.Args
		return false
	})

	if len(args) == 0 {
		t.Fatalf("No call args found for %q", raw)
	}

	return args
}

func litWord(value string) *syntax.Word {
	return &syntax.Word{
		Parts: []syntax.WordPart{
			&syntax.Lit{Value: value},
		},
	}
}
