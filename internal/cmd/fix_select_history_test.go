package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuluo-yx/typo/internal/config"
	"github.com/yuluo-yx/typo/internal/engine"
	itypes "github.com/yuluo-yx/typo/internal/types"
)

func TestFixSelectPreservesHistoryPolicy(t *testing.T) {
	tests := []struct {
		name            string
		command         string
		stderr          string
		aliasContext    string
		noHistory       bool
		historyDisabled bool
		want            string
		wantHistory     bool
	}{
		{
			name:    "permission correction",
			command: "mkdir /root/test",
			stderr:  "mkdir: /root/test: Permission denied\n",
			want:    "sudo mkdir /root/test",
		},
		{
			name:    "parser correction",
			command: "git remove -v",
			stderr:  "git: 'remove' is not a git command.\n\nThe most similar command is\n\tremote\n",
			want:    "git remote -v",
		},
		{
			name:    "parser followed by spelling correction",
			command: "git remove -v && dcoker ps",
			stderr:  "git: 'remove' is not a git command.\n\nThe most similar command is\n\tremote\n",
			want:    "git remote -v && docker ps",
		},
		{
			name:        "spelling correction",
			command:     "gti status",
			want:        "git status",
			wantHistory: true,
		},
		{
			name:         "parser correction with alias",
			command:      "g remove -v",
			stderr:       "git: 'remove' is not a git command.\n\nThe most similar command is\n\tremote\n",
			aliasContext: "bash\talias\tg\tgit\n",
			want:         "g remote -v",
		},
		{
			name:      "spelling correction with no history flag",
			command:   "gti status",
			noHistory: true,
			want:      "git status",
		},
		{
			name:            "spelling correction with history disabled",
			command:         "gti status",
			historyDisabled: true,
			want:            "git status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useTempHome(t)
			cfg := config.Load()
			cfg.User.Candidates.Enabled = true
			cfg.User.Candidates.Limit = 1
			cfg.User.History.Enabled = !tt.historyDisabled
			if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			stderrFile := filepath.Join(t.TempDir(), "stderr.txt")
			if err := os.WriteFile(stderrFile, []byte(tt.stderr), 0600); err != nil {
				t.Fatal(err)
			}

			learned := make(chan struct{}, 1)
			oldRunner := autoLearnFromHistory
			t.Cleanup(func() { autoLearnFromHistory = oldRunner })
			autoLearnFromHistory = func(context.Context, *engine.Engine, string, string) itypes.AutoLearnDebugInfo {
				learned <- struct{}{}
				return itypes.AutoLearnDebugInfo{}
			}

			args := []string{"typo", "fix", "--select", "--exit-code", "1", "-s", stderrFile}
			if tt.noHistory {
				args = append(args, "--no-history")
			}
			if tt.aliasContext != "" {
				contextFile := filepath.Join(t.TempDir(), "context.tsv")
				if err := os.WriteFile(contextFile, []byte(tt.aliasContext), 0600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--alias-context", contextFile)
			}
			code, stdout, stderr := runCLI(t, append(args, tt.command))
			if code != 0 || strings.TrimSpace(stdout) != tt.want {
				t.Fatalf("fix: code=%d stdout=%q stderr=%q, want %q", code, stdout, stderr, tt.want)
			}
			entry, recorded := engine.NewHistory(cfg.ConfigDir).Lookup(tt.command)
			if recorded != tt.wantHistory || (recorded && entry.To != tt.want) {
				t.Errorf("history = %+v, recorded=%v, want recorded=%v", entry, recorded, tt.wantHistory)
			}
			if got := len(learned) > 0; got != tt.wantHistory {
				t.Errorf("auto-learn attempted=%v, want %v", got, tt.wantHistory)
			}
		})
	}
}

func TestSelectFixResultPreservesCorrectionContext(t *testing.T) {
	tests := []struct {
		name        string
		limit       int
		index       int
		terminalErr error
		want        string
		wantParser  bool
	}{
		{name: "single candidate", limit: 1, want: "sudo git status", wantParser: true},
		{name: "menu selection", limit: 2, want: "sudo git status", wantParser: true},
		{name: "terminal unavailable", limit: 2, terminalErr: errors.New("terminal unavailable"), want: "sudo git status", wantParser: true},
		{name: "alternative spelling candidate", limit: 2, index: 1, want: "git status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eng := engine.NewEngine(engine.WithCommands([]string{"git", "get"}))
			oldChoose := chooseFixCandidateFromTerminalFunc
			t.Cleanup(func() { chooseFixCandidateFromTerminalFunc = oldChoose })
			menuCalled := false
			chooseFixCandidateFromTerminalFunc = func(candidates []itypes.FixCandidate) (itypes.FixCandidate, bool, error) {
				menuCalled = true
				if len(candidates) != 2 {
					t.Fatalf("expected two candidates, got %+v", candidates)
				}
				return candidates[tt.index], true, tt.terminalErr
			}
			input := itypes.ParserContext{
				Command: "gti status", Stderr: "root privileges are required\n", ExitCode: 1,
			}
			result, ok := selectFixResult(eng, input, tt.limit)
			if !ok || !result.Fixed || result.Command != tt.want {
				t.Fatalf("selection = %+v, ok=%v, want %q", result, ok, tt.want)
			}
			if menuCalled != (tt.limit > 1) {
				t.Fatalf("menu called=%v, want %v", menuCalled, tt.limit > 1)
			}
			wantKind := ""
			if tt.wantParser {
				wantKind = itypes.FixKindPermissionSudo
			}
			if result.Kind != wantKind || result.UsedParser != tt.wantParser {
				t.Errorf("context = %+v, want Kind=%q UsedParser=%v", result, wantKind, tt.wantParser)
			}
			if got := shouldRecordHistory(input.Command, result); got == tt.wantParser {
				t.Errorf("record history=%v, want %v", got, !tt.wantParser)
			}
		})
	}
}
