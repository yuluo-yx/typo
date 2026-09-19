package engine

import (
	"testing"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

func TestEnvCorrectionPreservesExplicitBindings(t *testing.T) {
	for _, command := range []string{
		`HOEM=/tmp; echo "$HOEM"`,
		`HOEM=/tmp echo "$HOEM"`,
		`for HOEM in /tmp; do echo "$HOEM"; done`,
		`for HOEM; do echo "$HOEM"; done`,
		`for ((HOEM=0; HOEM<3; HOEM++)); do echo "$HOEM"; done`,
		`((HOEM+=1)); echo "$HOEM"`,
		`((++HOEM)); echo "$HOEM"`,
		`demo() { local HOEM=/tmp; echo "$HOEM"; }; demo`,
		`demo() { local HOEM; echo "$HOEM"; }; demo`,
		`declare HOEM=/tmp; echo "$HOEM"`,
		`export HOEM=/tmp; echo "$HOEM"`,
		`HOEM=(one two); echo "$HOEM"`,
		`if true; then HOEM=/tmp; fi; echo "$HOEM"`,
	} {
		t.Run(command, func(t *testing.T) {
			eng := NewEngine(WithCommands([]string{"demo", "echo", "true"}))
			got := eng.FixWithContext(itypes.ParserContext{Command: command, AliasContext: envContextEntries("HOME")})
			if got.Fixed {
				t.Fatalf("explicit binding changed: %+v", got)
			}
		})
	}
}

func TestEnvCorrectionStillFixesUnboundReferences(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{`echo "$HOEM"`, `echo "$HOME"`},
		{`HOEM=/tmp; echo "$HOEM" "$PAHT"`, `HOEM=/tmp; echo "$HOEM" "$PATH"`},
		{`echo 'HOEM=/tmp' "$HOEM"`, `echo 'HOEM=/tmp' "$HOME"`},
		{`((1+2)); echo "$HOEM"`, `((1+2)); echo "$HOME"`},
	} {
		t.Run(tt.input, func(t *testing.T) {
			eng := NewEngine(WithCommands([]string{"echo"}))
			got := eng.FixWithContext(itypes.ParserContext{Command: tt.input, AliasContext: envContextEntries("HOME", "PATH")})
			if !got.Fixed || got.Command != tt.want {
				t.Fatalf("got %+v, want %q", got, tt.want)
			}
		})
	}
}
