package engine

import (
	"strings"
	"testing"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

func TestFixCommandPreservesIncompleteArguments(t *testing.T) {
	dir := t.TempDir()
	rules := NewRules(dir)
	if err := rules.AddUserRule(itypes.Rule{From: "mytool", To: "realtool"}); err != nil {
		t.Fatal(err)
	}
	history := NewHistory(dir)
	if err := history.Record("oldtool", "realtool"); err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(WithRules(rules), WithHistory(history), WithCommands([]string{"docker", "grep", "realtool"}))
	for _, pair := range [][2]string{{"mytool", "realtool"}, {"oldtool", "realtool"}, {"dcoker", "docker"}, {"gerp", "grep"}} {
		for _, suffix := range []string{"  'a   b", "\t\"a\tb", "  --label 'hello\nworld", " 'caf\u00e9  menu"} {
			t.Run(pair[0]+suffix, func(t *testing.T) {
				input := pair[0] + suffix
				got := eng.FixCommand(input)
				want := pair[1] + suffix
				if !got.Fixed || got.Command != want {
					t.Fatalf("FixCommand(%q) = %+v, want %q", input, got, want)
				}
			})
		}
	}
}

func TestFixCommandDoesNotFallbackAfterSuccessfulShellParse(t *testing.T) {
	rules := NewRules(t.TempDir())
	if err := rules.AddUserRule(itypes.Rule{From: "env", To: "echo"}); err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(WithRules(rules), WithCommands([]string{"env", "git", "echo"}))
	for _, command := range []string{"env git status", "env  git 'a   b'", "env git status | echo ok"} {
		if got := eng.FixCommand(command); got.Fixed {
			t.Fatalf("wrapper was rewritten for %q: %+v", command, got)
		}
	}
	if got := eng.FixCommand("env gti status"); !got.Fixed || !strings.HasPrefix(got.Command, "env git ") {
		t.Fatalf("wrapped command typo was not corrected: %+v", got)
	}
}
