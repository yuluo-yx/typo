package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/yuluo-yx/typo/internal/config"
	"github.com/yuluo-yx/typo/internal/engine"
	itypes "github.com/yuluo-yx/typo/internal/types"
)

func TestCandidateRedrawStaysWithinMenu(t *testing.T) {
	candidates := []itypes.FixCandidate{{Command: "git status"}, {Command: "get status"}}
	var initial, redraw bytes.Buffer
	if err := drawFixCandidateMenu(&initial, candidates, 0); err != nil {
		t.Fatal(err)
	}
	if err := redrawFixCandidateMenu(&redraw, candidates, 1); err != nil {
		t.Fatal(err)
	}
	rows := strings.Count(initial.String(), "\n")
	if !strings.HasPrefix(redraw.String(), "\r\x1b["+strconv.Itoa(rows)+"A") {
		t.Fatalf("redraw leaves the %d-row menu: %q", rows, redraw.String())
	}
}

func TestCandidateDisplayEscapesControlCharacters(t *testing.T) {
	command := "git commit -m 'first\nsecond\t\x1b[2J'"
	var out bytes.Buffer
	selected, ok, err := selectFixCandidate([]itypes.FixCandidate{{Command: command}}, strings.NewReader("1"), &out)
	if err != nil || !ok || selected.Command != command {
		t.Fatalf("selection must preserve the original command: %+v, %v, %v", selected, ok, err)
	}
	if strings.Contains(out.String(), "\x1b") || strings.Count(out.String(), "\n") != 2 || !strings.Contains(out.String(), `first\nsecond\t\x1b[2J`) {
		t.Fatalf("control characters disrupt the menu: %q", out.String())
	}
}

func TestFixNormalizesHistoryKeyForAutoLearn(t *testing.T) {
	useTempHome(t)
	cfg := config.Load()
	cfg.User.AutoLearnThreshold = 2
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"  gti status  ", "gti status"} {
		code, stdout, stderr := runCLI(t, []string{"typo", "fix", command})
		if code != 0 || strings.TrimSpace(stdout) != "git status" {
			t.Fatalf("fix failed: %d %q %q", code, stdout, stderr)
		}
	}
	h := engine.NewHistory(cfg.ConfigDir)
	entry, ok := h.Lookup("gti status")
	if !ok || entry.Count != 2 || !entry.RuleApplied || h.Count() != 1 {
		t.Fatalf("equivalent inputs did not share learning history: %+v, entries=%d", entry, h.Count())
	}
}

func TestConfigGenHelpSucceeds(t *testing.T) {
	useTempHome(t)
	code, _, stderr := runCLI(t, []string{"typo", "config", "gen", "--help"})
	if code != 0 || !strings.Contains(stderr, "Usage of config gen") {
		t.Fatalf("help failed: code=%d stderr=%q", code, stderr)
	}
}

func TestFixPreservesExecutableDiscoveredFromPATH(t *testing.T) {
	useTempHome(t)
	dir := t.TempDir()
	name := "gti"
	if os.PathSeparator == '\\' {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	code, stdout, stderr := runCLI(t, []string{"typo", "fix", "gti status"})
	if code != 1 || stdout != "" || !strings.Contains(stderr, "no correction found") {
		t.Fatalf("existing PATH executable was rewritten: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
