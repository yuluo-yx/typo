package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/term"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

var chooseFixCandidateFromTerminalFunc = chooseFixCandidateFromTerminal

// escapeSequenceTimeout is how long a lone ESC waits for the rest of an arrow-key
// sequence. Terminals and SSH links may deliver ESC and "[B" in separate reads.
var escapeSequenceTimeout = 50 * time.Millisecond

// timedByteReader feeds input bytes through a channel so escape parsing can wait a
// bounded time for follow-up bytes instead of guessing from the current buffer.
type timedByteReader struct {
	ch <-chan byteRead
}

type byteRead struct {
	b   byte
	err error
}

func newTimedByteReader(input io.Reader) *timedByteReader {
	ch := make(chan byteRead, 64)
	go func() {
		defer close(ch)
		buf := make([]byte, 64)
		for {
			n, err := input.Read(buf)
			for _, b := range buf[:n] {
				ch <- byteRead{b: b}
			}
			if err != nil {
				ch <- byteRead{err: err}
				return
			}
		}
	}()
	return &timedByteReader{ch: ch}
}

// ReadByte blocks until the next byte or input error.
func (r *timedByteReader) ReadByte() (byte, error) {
	read, ok := <-r.ch
	if !ok {
		return 0, io.EOF
	}
	return read.b, read.err
}

// readByteWithin returns ok=false when no byte arrives before the timeout.
func (r *timedByteReader) readByteWithin(timeout time.Duration) (byte, bool) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case read, ok := <-r.ch:
		if !ok || read.err != nil {
			return 0, false
		}
		return read.b, true
	case <-timer.C:
		return 0, false
	}
}

func selectFixCandidate(candidates []itypes.FixCandidate, input io.Reader, output io.Writer) (itypes.FixCandidate, bool, error) {
	if len(candidates) == 0 {
		return itypes.FixCandidate{}, false, nil
	}

	reader := newTimedByteReader(input)
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

func readFixSelectionKey(reader *timedByteReader) (fixSelectionKey, error) {
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

func readEscapeSelectionKey(reader *timedByteReader) fixSelectionKey {
	next, ok := reader.readByteWithin(escapeSequenceTimeout)
	// CSI ("ESC [") and SS3 ("ESC O", application cursor mode) both carry arrow keys.
	if !ok || (next != '[' && next != 'O') {
		return fixSelectionKey{cancel: true}
	}
	code, ok := reader.readByteWithin(escapeSequenceTimeout)
	if !ok {
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
		if _, err := fmt.Fprintf(output, "%s%d) %s\r\n", prefix, idx+1, displayFixCandidate(candidate.Command)); err != nil {
			return err
		}
	}
	_, err := fmt.Fprint(output, "Use 1-9, Up/Down and Enter, or q/Esc to cancel. ")
	return err
}

func redrawFixCandidateMenu(output io.Writer, candidates []itypes.FixCandidate, selected int) error {
	// The prompt has no trailing newline, so it does not add another row to move up.
	menuLines := len(candidates) + 1
	if _, err := fmt.Fprintf(output, "\r\x1b[%dA\x1b[J", menuLines); err != nil {
		return err
	}
	return drawFixCandidateMenu(output, candidates, selected)
}

// Escape control characters for display without changing the selected command.
func displayFixCandidate(command string) string {
	var display strings.Builder
	for _, r := range command {
		if unicode.IsPrint(r) {
			display.WriteRune(r)
			continue
		}
		quoted := strconv.QuoteRune(r)
		display.WriteString(quoted[1 : len(quoted)-1])
	}
	return display.String()
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
