package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yuluo-yx/typo/internal/engine"
	itypes "github.com/yuluo-yx/typo/internal/types"
)

var fzfTestCandidates = []itypes.FixCandidate{
	{Command: "git status", Source: "distance"},
	{Command: "get status", Source: "distance"},
}

// installFakeFZF makes lookPath resolve fzf to a shell script with the given body.
func installFakeFZF(t *testing.T, body string) {
	t.Helper()
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("fake fzf needs a POSIX shell")
	}
	path := filepath.Join(t.TempDir(), "fzf")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	stubFZFLookPath(t, path, nil)
}

func stubFZFLookPath(t *testing.T, path string, err error) {
	t.Helper()
	oldLookPath := lookPath
	t.Cleanup(func() { lookPath = oldLookPath })
	lookPath = func(file string) (string, error) {
		if file != "fzf" {
			return oldLookPath(file)
		}
		return path, err
	}
}

func TestChooseFixCandidateWithFZFSelectsByIndex(t *testing.T) {
	// Echo the second input line back, as fzf does when the user picks it.
	installFakeFZF(t, `sed -n 2p`)

	selected, ok, err := chooseFixCandidateWithFZF(fzfTestCandidates)
	if err != nil || !ok {
		t.Fatalf("chooseFixCandidateWithFZF() ok=%v err=%v", ok, err)
	}
	if selected.Command != "get status" {
		t.Fatalf("selected = %q, want get status", selected.Command)
	}
}

func TestChooseFixCandidateWithFZFPassesArguments(t *testing.T) {
	installFakeFZF(t, `for arg in "$@"; do [ "$arg" = "--with-nth=2.." ] && { sed -n 1p; exit 0; }; done; exit 2`)

	selected, ok, err := chooseFixCandidateWithFZF(fzfTestCandidates)
	if err != nil || !ok || selected.Command != "git status" {
		t.Fatalf("chooseFixCandidateWithFZF() = %+v, %v, %v", selected, ok, err)
	}
}

func TestChooseFixCandidateWithFZFCancel(t *testing.T) {
	for _, status := range []string{"1", "130"} {
		t.Run(status, func(t *testing.T) {
			installFakeFZF(t, "cat >/dev/null; exit "+status)
			if _, ok, err := chooseFixCandidateWithFZF(fzfTestCandidates); ok || err != nil {
				t.Fatalf("cancel status %s: ok=%v err=%v, want cancel", status, ok, err)
			}
		})
	}
}

func TestChooseFixCandidateWithFZFReportsUnavailable(t *testing.T) {
	t.Run("missing binary", func(t *testing.T) {
		stubFZFLookPath(t, "", errors.New("not found"))
		if _, _, err := chooseFixCandidateWithFZF(fzfTestCandidates); !errors.Is(err, errFZFUnavailable) {
			t.Fatalf("err = %v, want errFZFUnavailable", err)
		}
	})
	t.Run("error status", func(t *testing.T) {
		installFakeFZF(t, "cat >/dev/null; exit 2")
		if _, _, err := chooseFixCandidateWithFZF(fzfTestCandidates); !errors.Is(err, errFZFUnavailable) {
			t.Fatalf("err = %v, want errFZFUnavailable", err)
		}
	})
	t.Run("unexpected output", func(t *testing.T) {
		installFakeFZF(t, "cat >/dev/null; echo garbage")
		if _, _, err := chooseFixCandidateWithFZF(fzfTestCandidates); !errors.Is(err, errFZFUnavailable) {
			t.Fatalf("err = %v, want errFZFUnavailable", err)
		}
	})
}

func TestParseFZFSelection(t *testing.T) {
	for _, tt := range []struct {
		output string
		want   string
		ok     bool
	}{
		{"1\tgit status\n", "git status", true},
		{"2\tget status\n1\tgit status\n", "get status", true},
		{"3\tother\n", "", false},
		{"0\tother\n", "", false},
		{"x\tother\n", "", false},
		{"no tab\n", "", false},
		{"get status\n", "get status", true},
		{"git status\r\n", "git status", true},
		{"", "", false},
	} {
		got, ok := parseFZFSelection(tt.output, fzfTestCandidates)
		if ok != tt.ok || got.Command != tt.want {
			t.Fatalf("parseFZFSelection(%q) = %q, %v; want %q, %v", tt.output, got.Command, ok, tt.want, tt.ok)
		}
	}
}

func TestChooseFixCandidateFallsBackToBuiltinMenu(t *testing.T) {
	oldFZF := chooseFixCandidateWithFZFFunc
	oldTerminal := chooseFixCandidateFromTerminalFunc
	t.Cleanup(func() {
		chooseFixCandidateWithFZFFunc = oldFZF
		chooseFixCandidateFromTerminalFunc = oldTerminal
	})

	fzfCalls, terminalCalls := 0, 0
	chooseFixCandidateFromTerminalFunc = func(candidates []itypes.FixCandidate) (itypes.FixCandidate, bool, error) {
		terminalCalls++
		return candidates[1], true, nil
	}

	chooseFixCandidateWithFZFFunc = func(candidates []itypes.FixCandidate) (itypes.FixCandidate, bool, error) {
		fzfCalls++
		return itypes.FixCandidate{}, false, errFZFUnavailable
	}
	if selected, ok, err := chooseFixCandidate(fzfTestCandidates, true); err != nil || !ok || selected.Command != "get status" {
		t.Fatalf("fallback = %+v, %v, %v", selected, ok, err)
	}
	if fzfCalls != 1 || terminalCalls != 1 {
		t.Fatalf("fzf calls = %d, terminal calls = %d; want 1 and 1", fzfCalls, terminalCalls)
	}

	chooseFixCandidateWithFZFFunc = func(candidates []itypes.FixCandidate) (itypes.FixCandidate, bool, error) {
		fzfCalls++
		return itypes.FixCandidate{}, false, nil
	}
	if _, ok, err := chooseFixCandidate(fzfTestCandidates, true); ok || err != nil {
		t.Fatalf("fzf cancel should not fall back: ok=%v err=%v", ok, err)
	}
	if terminalCalls != 1 {
		t.Fatalf("terminal menu ran after fzf cancel")
	}

	if _, _, err := chooseFixCandidate(fzfTestCandidates, false); err != nil || fzfCalls != 2 || terminalCalls != 2 {
		t.Fatalf("fzf disabled: err=%v fzf calls=%d terminal calls=%d", err, fzfCalls, terminalCalls)
	}
}

func TestSelectFixResultUsesFZFWhenEnabled(t *testing.T) {
	eng := engine.NewEngine(
		engine.WithCommands([]string{"git", "get"}),
		engine.WithMaxEditDistance(3),
		engine.WithSimilarityThreshold(0.2),
	)
	oldFZF := chooseFixCandidateWithFZFFunc
	t.Cleanup(func() { chooseFixCandidateWithFZFFunc = oldFZF })
	chooseFixCandidateWithFZFFunc = func(candidates []itypes.FixCandidate) (itypes.FixCandidate, bool, error) {
		return candidates[1], true, nil
	}

	result, ok := selectFixResult(eng, itypes.ParserContext{Command: "gti status"}, 2, true)
	if !ok || result.Command != "get status" {
		t.Fatalf("selectFixResult with fzf = %+v, %v", result, ok)
	}
}

func TestSkippedAutoLearnReasonForPathFix(t *testing.T) {
	info := skippedAutoLearnDebugInfo(true, false, "cat READNE.md", itypes.FixResult{
		Fixed: true, Command: "cat README.md", Kind: itypes.FixKindPath,
	})
	if info.Reason != "path fixes depend on the filesystem and are not auto-learned" {
		t.Fatalf("reason = %q", info.Reason)
	}
}
