package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"golang.org/x/term"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

var chooseFixCandidateFromTerminalFunc = chooseFixCandidateFromTerminal

func selectFixCandidate(candidates []itypes.FixCandidate, input io.Reader, output io.Writer) (itypes.FixCandidate, bool, error) {
	if len(candidates) == 0 {
		return itypes.FixCandidate{}, false, nil
	}

	reader := bufio.NewReader(input)
	selected := 0
	if err := drawFixCandidateMenu(output, candidates, selected); err != nil {
		return itypes.FixCandidate{}, false, err
	}

	for {
		key, err := readFixSelectionKey(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return itypes.FixCandidate{}, false, nil
			}
			return itypes.FixCandidate{}, false, err
		}
		next, done, ok, err := applyFixSelectionKey(output, candidates, selected, key)
		if err != nil || done {
			return next, ok, err
		}
		selected = selectedIndex(selected, len(candidates), key.direction)
	}
}

type fixSelectionKey struct {
	number    int
	direction int
	confirm   bool
	cancel    bool
}

func applyFixSelectionKey(
	output io.Writer,
	candidates []itypes.FixCandidate,
	selected int,
	key fixSelectionKey,
) (itypes.FixCandidate, bool, bool, error) {
	switch {
	case key.cancel:
		return itypes.FixCandidate{}, true, false, nil
	case key.confirm:
		return candidates[selected], true, true, nil
	case key.number > 0:
		idx := key.number - 1
		if idx >= 0 && idx < len(candidates) {
			return candidates[idx], true, true, nil
		}
	case key.direction != 0:
		nextSelected := selectedIndex(selected, len(candidates), key.direction)
		return itypes.FixCandidate{}, false, false, redrawFixCandidateMenu(output, candidates, nextSelected)
	}

	return itypes.FixCandidate{}, false, false, nil
}

func selectedIndex(selected int, count int, direction int) int {
	if count == 0 || direction == 0 {
		return selected
	}
	if direction < 0 {
		if selected == 0 {
			return count - 1
		}
		return selected - 1
	}
	return (selected + 1) % count
}

func readFixSelectionKey(reader *bufio.Reader) (fixSelectionKey, error) {
	b, err := reader.ReadByte()
	if err != nil {
		return fixSelectionKey{}, err
	}

	switch {
	case b >= '1' && b <= '9':
		return fixSelectionKey{number: int(b - '0')}, nil
	case b == '\r' || b == '\n':
		return fixSelectionKey{confirm: true}, nil
	case b == 'q' || b == 'Q' || b == 3:
		return fixSelectionKey{cancel: true}, nil
	case b == 27:
		return readEscapeSelectionKey(reader), nil
	default:
		return fixSelectionKey{}, nil
	}
}

func readEscapeSelectionKey(reader *bufio.Reader) fixSelectionKey {
	if reader.Buffered() == 0 {
		return fixSelectionKey{cancel: true}
	}
	next, err := reader.ReadByte()
	if err != nil || next != '[' {
		return fixSelectionKey{cancel: true}
	}
	code, err := reader.ReadByte()
	if err != nil {
		return fixSelectionKey{cancel: true}
	}
	switch code {
	case 'A':
		return fixSelectionKey{direction: -1}
	case 'B':
		return fixSelectionKey{direction: 1}
	default:
		return fixSelectionKey{}
	}
}

func drawFixCandidateMenu(output io.Writer, candidates []itypes.FixCandidate, selected int) error {
	if _, err := fmt.Fprint(output, "typo: choose a correction\r\n"); err != nil {
		return err
	}
	for idx, candidate := range candidates {
		prefix := "  "
		if idx == selected {
			prefix = "> "
		}
		if _, err := fmt.Fprintf(output, "%s%d) %s\r\n", prefix, idx+1, candidate.Command); err != nil {
			return err
		}
	}
	_, err := fmt.Fprint(output, "Use 1-9, Up/Down and Enter, or q/Esc to cancel. ")
	return err
}

func redrawFixCandidateMenu(output io.Writer, candidates []itypes.FixCandidate, selected int) error {
	menuLines := len(candidates) + 2
	if _, err := fmt.Fprintf(output, "\r\x1b[%dA\x1b[J", menuLines); err != nil {
		return err
	}
	return drawFixCandidateMenu(output, candidates, selected)
}

func chooseFixCandidateFromTerminal(candidates []itypes.FixCandidate) (itypes.FixCandidate, bool, error) {
	in, out, cleanup, err := openFixSelectionTerminal()
	if err != nil {
		return itypes.FixCandidate{}, false, err
	}
	defer cleanup()

	fd, err := terminalFileDescriptor(in)
	if err != nil {
		return itypes.FixCandidate{}, false, err
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return itypes.FixCandidate{}, false, err
	}
	defer func() {
		_ = term.Restore(fd, oldState)
	}()

	selected, ok, err := selectFixCandidate(candidates, in, out)
	if _, writeErr := fmt.Fprintln(out); err == nil {
		err = writeErr
	}
	return selected, ok, err
}

func terminalFileDescriptor(file *os.File) (int, error) {
	fd := file.Fd()
	maxInt := uintptr(^uint(0) >> 1)
	if fd > maxInt {
		return 0, fmt.Errorf("terminal file descriptor is too large: %d", fd)
	}
	return int(fd), nil
}

func openFixSelectionTerminal() (*os.File, *os.File, func(), error) {
	if runtime.GOOS == "windows" {
		in, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
		if err != nil {
			return nil, nil, nil, err
		}
		out, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
		if err != nil {
			_ = in.Close()
			return nil, nil, nil, err
		}
		return in, out, func() {
			_ = in.Close()
			_ = out.Close()
		}, nil
	}

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, nil, err
	}
	return tty, tty, func() { _ = tty.Close() }, nil
}
