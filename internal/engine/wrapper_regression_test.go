package engine

import (
	"strings"
	"testing"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

func TestWrapperCorrectionPreservesArguments(t *testing.T) {
	eng := NewEngine(WithRules(NewRules(t.TempDir())), WithCommands([]string{"git", "env", "sudo", "echo"}))
	prefixes := []string{
		"command -- ",
		"env MODE='hello world' ",
		`env MODE="$USER" `,
		`env "MODE=hello world" `,
		"env -- MODE=prod ",
		"env -iu UNUSED MODE=prod ",
		"env --unset=UNUSED MODE=prod ",
		"sudo MODE=prod ",
		"sudo -nu root ",
		"sudo -nEu root MODE='hello world' ",
		"sudo -nuroot MODE=prod ",
		`sudo --user="$USER" MODE=prod `,
		"env MODE=prod sudo -nu root ",
		"sudo -nu root env MODE='hello world' ",
	}
	for _, prefix := range prefixes {
		t.Run(prefix, func(t *testing.T) {
			input := prefix + "gti status -- 'a  b'"
			want := prefix + "git status -- 'a  b'"
			if got := eng.Fix(input, ""); !got.Fixed || got.Command != want {
				t.Errorf("Fix(%q) = %+v, want %q", input, got, want)
			}
			if got := eng.FixCommand(input); !got.Fixed || got.Command != want {
				t.Errorf("FixCommand(%q) = %+v, want %q", input, got, want)
			}
			candidates := eng.FixCandidatesWithContext(itypes.ParserContext{Command: input}, 5)
			if len(candidates) == 0 || candidates[0].Command != want {
				t.Errorf("candidates = %+v, want first %q", candidates, want)
			}
			if got := eng.Fix(want, ""); got.Fixed {
				t.Errorf("correct command was rewritten: %+v", got)
			}
		})
	}
}

func TestWrapperExecutableBoundaries(t *testing.T) {
	tests := []struct {
		command, want string
	}{
		{"env MODE=prod --unset USER git status", "--unset"},
		{"env -- -u USER git status", "-u"},
		{"command -- -v git", "-v"},
		{"env -u MODE", ""},
		{"sudo -nu root", ""},
		{"env MODE='hello world'", ""},
		{"sudo MODE=prod", ""},
		{`env ${PREFIX}MODE=prod git status`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			args := parseCallArgs(t, tt.command)
			index := findExecutableArgIndex(args)
			got := ""
			if index >= 0 {
				got = args[index].Lit()
			}
			if got != tt.want {
				t.Fatalf("executable = %q at %d, want %q", got, index, tt.want)
			}
		})
	}
}

func TestWrapperQuotedValuesAreNotExecutable(t *testing.T) {
	eng := NewEngine(WithRules(NewRules(t.TempDir())), WithCommands([]string{"git", "env", "sudo", "echo"}))
	for _, command := range []string{
		"env MODE='gti' git status",
		"sudo -nu gti git status",
		"env -iu gti git status",
		"sudo --user='gti' git status",
		"env MODE='gti' echo 'gti'",
	} {
		if got := eng.Fix(command, ""); got.Fixed {
			t.Errorf("argument value was rewritten for %q: %+v", command, got)
		}
		if got := eng.FixCandidatesWithContext(itypes.ParserContext{Command: command}, 5); len(got) != 0 {
			t.Errorf("argument value became a candidate for %q: %+v", command, got)
		}
	}
	command := "env " + strings.Repeat("MODE='a b' ", 1000) + "gti status"
	if got := eng.Fix(command, ""); !got.Fixed || got.Command != strings.TrimSuffix(command, "gti status")+"git status" {
		t.Fatal("long assignment list was not preserved")
	}
}

func TestWrapperOptionAssignmentMatrix(t *testing.T) {
	eng := NewEngine(WithRules(NewRules(t.TempDir())), WithCommands([]string{"git", "env", "sudo", "printf"}))
	wrappers := []struct {
		name    string
		options []string
	}{
		{"env", []string{"-i", "-iu UNUSED", "--unset=UNUSED", "-u UNUSED"}},
		{"sudo", []string{"-n", "-nu root", "-nuroot", "--user=root"}},
	}
	assignments := []string{
		"MODE=prod",
		"MODE='two words'",
		`"MODE=two words"`,
		`MODE="two $LABEL"`,
		`MODE="$(printf value)"`,
		`MODE=$'line\nvalue'`,
		"MODE=prod DEBUG='1 2'",
	}
	for _, wrapper := range wrappers {
		for _, option := range wrapper.options {
			for _, separator := range []string{"", "-- "} {
				for _, assignment := range assignments {
					prefix := wrapper.name + " " + option + " " + separator + assignment + " "
					t.Run(prefix, func(t *testing.T) {
						input := prefix + "gti status -- 'a  b'"
						want := prefix + "git status -- 'a  b'"
						if got := eng.Fix(input, ""); !got.Fixed || got.Command != want {
							t.Fatalf("Fix(%q) = %+v, want %q", input, got, want)
						}
						if got := eng.Fix(want, ""); got.Fixed {
							t.Fatalf("correct command was rewritten: %+v", got)
						}
					})
				}
			}
		}
	}
}

func TestWrapperExpansionMustHaveStableArgumentBoundaries(t *testing.T) {
	eng := NewEngine(WithRules(NewRules(t.TempDir())), WithCommands([]string{"git", "env", "sudo", "printf"}))
	for _, prefix := range []string{
		`env MODE=$VALUES `,
		`env MODE=$(printf 'prod git') `,
		`env MODE="$@" `,
		`env MODE="${VALUES[@]}" `,
		`env MODE="${LABEL:-$@}" `,
		`sudo --user=$VALUES `,
		`sudo MODE="prefix"$VALUES `,
		`env MODE="${!LABEL}" `,
		`env $"MODE=prod" `,
	} {
		t.Run(prefix, func(t *testing.T) {
			command := prefix + "gti status"
			if got := eng.Fix(command, ""); got.Fixed {
				t.Errorf("uncertain executable was corrected: %+v", got)
			}
			if got := eng.FixCommand(command); got.Fixed {
				t.Errorf("uncertain executable was corrected by FixCommand: %+v", got)
			}
			if got := eng.FixCandidatesWithContext(itypes.ParserContext{Command: command}, 5); len(got) != 0 {
				t.Errorf("uncertain executable became a candidate: %+v", got)
			}
		})
	}
	command := `env MODE="${LABEL:-fallback}" gti status`
	if got := eng.Fix(command, ""); !got.Fixed || got.Command != `env MODE="${LABEL:-fallback}" git status` {
		t.Fatalf("quoted scalar fallback was not corrected: %+v", got)
	}
}
