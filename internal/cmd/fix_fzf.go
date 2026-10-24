package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

// fzfSelectionArgs renders a compact inline picker that keeps the engine ranking.
// Each input line is "<index>\t<command>"; only the command is shown and searched.
var fzfSelectionArgs = []string{
	"--height=40%",
	"--layout=reverse",
	"--no-multi",
	"--no-sort",
	"--delimiter=\t",
	"--with-nth=2..",
	"--prompt=typo> ",
}

var chooseFixCandidateWithFZFFunc = chooseFixCandidateWithFZF

// errFZFUnavailable reports that fzf could not be used, so the builtin menu should run instead.
var errFZFUnavailable = errors.New("fzf is unavailable")

// chooseFixCandidate selects one candidate, preferring fzf when enabled. Any fzf
// failure other than an explicit cancel falls back to the builtin terminal menu,
// so a missing or broken fzf never blocks a correction.
func chooseFixCandidate(candidates []itypes.FixCandidate, useFZF bool) (itypes.FixCandidate, bool, error) {
	if useFZF {
		selected, ok, err := chooseFixCandidateWithFZFFunc(candidates)
		if err == nil {
			return selected, ok, nil
		}
	}
	return chooseFixCandidateFromTerminalFunc(candidates)
}

// chooseFixCandidateWithFZF runs fzf over the candidates. It returns ok=false with
// a nil error when the user cancels, and errFZFUnavailable-wrapped errors when fzf
// is missing, fails to start, or exits with an error status.
func chooseFixCandidateWithFZF(candidates []itypes.FixCandidate) (itypes.FixCandidate, bool, error) {
	path, err := lookPath("fzf")
	if err != nil {
		return itypes.FixCandidate{}, false, fmt.Errorf("%w: %w", errFZFUnavailable, err)
	}

	var input strings.Builder
	for i, candidate := range candidates {
		fmt.Fprintf(&input, "%d\t%s\n", i+1, displayFixCandidate(candidate.Command))
	}

	var output bytes.Buffer
	cmd := exec.Command(path, fzfSelectionArgs...)
	cmd.Stdin = strings.NewReader(input.String())
	cmd.Stdout = &output
	// fzf draws its interface on stderr when it is not attached to the terminal.
	// Shell integrations discard typo's stderr, so point it at the terminal directly.
	var stderr io.Writer = os.Stderr
	if _, out, cleanup, ttyErr := openFixSelectionTerminal(); ttyErr == nil {
		defer cleanup()
		stderr = out
	}
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && isFZFCancelStatus(exitErr.ExitCode()) {
			return itypes.FixCandidate{}, false, nil
		}
		return itypes.FixCandidate{}, false, fmt.Errorf("%w: %w", errFZFUnavailable, err)
	}

	selected, ok := parseFZFSelection(output.String(), candidates)
	if !ok {
		return itypes.FixCandidate{}, false, fmt.Errorf("%w: unexpected output %q", errFZFUnavailable, output.String())
	}
	return selected, true, nil
}

// isFZFCancelStatus reports fzf exit statuses that mean "nothing chosen":
// 1 when no line matched the query and 130 when the user pressed ESC or CTRL-C.
func isFZFCancelStatus(code int) bool {
	return code == 1 || code == 130
}

// parseFZFSelection maps the first selected line back to its candidate. fzf
// normally prints the original "<index>\t<command>" line, but some modes print
// only the displayed command, so an exact display match is accepted as well.
func parseFZFSelection(output string, candidates []itypes.FixCandidate) (itypes.FixCandidate, bool) {
	line, _, _ := strings.Cut(output, "\n")
	line = strings.TrimSuffix(line, "\r")
	if indexText, _, found := strings.Cut(line, "\t"); found {
		index, err := strconv.Atoi(indexText)
		if err == nil && index >= 1 && index <= len(candidates) {
			return candidates[index-1], true
		}
	}
	for _, candidate := range candidates {
		if line != "" && displayFixCandidate(candidate.Command) == line {
			return candidate, true
		}
	}
	return itypes.FixCandidate{}, false
}
