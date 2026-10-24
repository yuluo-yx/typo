package engine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

// newPathFixture builds a home directory and a project tree, makes the project
// the working directory, and returns the home path.
func newPathFixture(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	for _, dir := range []string{".config/nvim", "proj/src", "proj/docs", "my docs", "it's here", "cost$dir"} {
		if err := os.MkdirAll(filepath.Join(home, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{
		".zshrc", ".bashrc",
		"proj/src/main.go", "proj/README.md", "proj/file1.txt", "proj/file2.txt",
		"proj/docs/guide.md", "my docs/notes.txt", "it's here/a.txt", "cost$dir/a.txt",
	} {
		if err := os.WriteFile(filepath.Join(home, filepath.FromSlash(file)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(home)
	return home
}

func newPathEngine(opts ...Option) *Engine {
	base := make([]Option, 0, 2+len(opts))
	base = append(base,
		WithPathCorrection(true),
		WithCommands([]string{"source", "cat", "cd", "cp", "mv", "ls", "head", "vim", "git", "sudo"}),
	)
	return NewEngine(append(base, opts...)...)
}

func TestPathFixCorrectsMissingArguments(t *testing.T) {
	newPathFixture(t)
	eng := newPathEngine()

	for _, tt := range []struct{ input, want string }{
		{"source ~/.zshcr", "source ~/.zshrc"},
		{"sourec ~/.zshcr", "source ~/.zshrc"},
		{". ~/.bashcr", ". ~/.bashrc"},
		{"cd ~/.confg/nvim", "cd ~/.config/nvim"},
		{"cd proj/scr", "cd proj/src"},
		{"ls proj/scr/", "ls proj/src/"},
		{"cat proj/src/mian.go", "cat proj/src/main.go"},
		{"cat prj/scr/main.go", "cat proj/src/main.go"},
		{"cat proj/fiel1.txt", "cat proj/file1.txt"},
		{`cat "my dcos/notes.txt"`, `cat "my docs/notes.txt"`},
		{`cat 'my dcos/notes.txt'`, `cat 'my docs/notes.txt'`},
		{`cat my\ dcos/notes.txt`, `cat 'my docs/notes.txt'`},
		{`cat ~/my\ dcos/notes.txt`, `cat ~/'my docs/notes.txt'`},
		{`cat 'its here/a.txt'`, `cat "it's here/a.txt"`},
		{`cat "cost-dir/a.txt"`, `cat 'cost$dir/a.txt'`},
		{"cp proj/READNE.md backup.md", "cp proj/README.md backup.md"},
		{"head -n 5 proj/READNE.md", "head -n 5 proj/README.md"},
		{"cat -- proj/READNE.md", "cat -- proj/README.md"},
		{"sudo cat proj/READNE.md", "sudo cat proj/README.md"},
		{"source proj/READNE.md extra-arg", "source proj/README.md extra-arg"},
		{"cat proj/READNE.md proj/docs/giude.md", "cat proj/README.md proj/docs/guide.md"},
	} {
		t.Run(tt.input, func(t *testing.T) {
			got := eng.FixWithContext(itypes.ParserContext{Command: tt.input, ExitCode: -1})
			if !got.Fixed || got.Command != tt.want {
				t.Fatalf("FixWithContext(%q) = %+v, want %q", tt.input, got, tt.want)
			}
			if got.Kind != itypes.FixKindPath {
				t.Fatalf("Kind = %q, want %q", got.Kind, itypes.FixKindPath)
			}
		})
	}
}

func TestPathFixLeavesUnsafeOrUnknownArgumentsAlone(t *testing.T) {
	newPathFixture(t)
	eng := newPathEngine()

	for _, input := range []string{
		"cat ~/.zshrc",                    // already exists
		"vim proj/READNE.md",              // editors may create files
		"cat proj/fil.txt",                // ambiguous: file1.txt and file2.txt
		"cat zshcr",                       // hidden entries only match hidden input
		"cat $HOME/.zshcr",                // dynamic word
		"cat proj/*.mdd",                  // glob
		"cat ~root/.zshcr",                // other users' homes
		`cat \~/.zshcr`,                   // escaped tilde is literal
		"cat proj/src/xy",                 // too short to match fuzzily
		"cat prj/scr/mian.go",             // more than two corrected segments
		"cd proj && cat scr/main.go",      // relative paths after cd are not resolvable
		"cp proj/READNE.md",               // single cp argument is the destination
		"cat -proj/READNE.md",             // option-looking words are skipped
		"cat proj/README.md/sub",          // file used as a directory
		"git checkout proj/READNE.md",     // not a path command
		"cat proj/totally-different.name", // nothing close
	} {
		t.Run(input, func(t *testing.T) {
			if got := eng.FixWithContext(itypes.ParserContext{Command: input, ExitCode: -1}); got.Fixed {
				t.Fatalf("FixWithContext(%q) = %+v, want no fix", input, got)
			}
		})
	}
}

func TestPathFixDisabledByDefault(t *testing.T) {
	newPathFixture(t)
	eng := NewEngine(WithCommands([]string{"source"}))
	if got := eng.Fix("source ~/.zshcr", ""); got.Fixed {
		t.Fatalf("path correction ran without WithPathCorrection: %+v", got)
	}
	if got := eng.pathFixCandidates("source ~/.zshcr", 3); got != nil {
		t.Fatalf("pathFixCandidates = %+v, want nil", got)
	}
}

func TestPathFixCandidatesOfferAmbiguousMatches(t *testing.T) {
	newPathFixture(t)
	eng := newPathEngine()

	candidates := eng.FixCandidatesWithContext(itypes.ParserContext{Command: "cat proj/fil.txt", ExitCode: -1}, 5)
	got := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		got = append(got, candidate.Command)
		if candidate.Kind != itypes.FixKindPath || candidate.Source != fixSourcePath {
			t.Fatalf("candidate %+v lacks path kind/source", candidate)
		}
	}
	want := "cat proj/file1.txt|cat proj/file2.txt"
	if strings.Join(got, "|") != want {
		t.Fatalf("candidates = %q, want %q", got, want)
	}

	if limited := eng.pathFixCandidates("cat proj/fil.txt", 1); len(limited) != 1 {
		t.Fatalf("limit 1 returned %d candidates", len(limited))
	}
	if none := eng.pathFixCandidates("cat proj/fil.txt", 0); none != nil {
		t.Fatalf("limit 0 returned %+v", none)
	}
	if unparsable := eng.pathFixCandidates("cat 'proj", 3); unparsable != nil {
		t.Fatalf("unparsable input returned %+v", unparsable)
	}
}

func TestPathFixRecordsDebugFeature(t *testing.T) {
	newPathFixture(t)
	eng := newPathEngine()
	eng.EnableDebug()

	got := eng.FixWithContext(itypes.ParserContext{Command: "cat proj/READNE.md", ExitCode: -1})
	if !got.Fixed || got.Debug == nil || !got.Debug.UsedPath {
		t.Fatalf("debug trace did not mark path usage: %+v", got.Debug)
	}
}

func TestPathFixAbsolutePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("absolute paths use drive letters on Windows")
	}
	home := newPathFixture(t)
	input := "cat " + filepath.Join(home, "proj", "READNE.md")
	want := "cat " + filepath.Join(home, "proj", "README.md")
	if strings.ContainsAny(home, " '\"$\\") {
		t.Skip("temporary directory needs quoting")
	}
	if got := newPathEngine().Fix(input, ""); got.Command != want {
		t.Fatalf("Fix(%q) = %+v, want %q", input, got, want)
	}
}

func TestPathFixWithoutHomeLeavesTildeAlone(t *testing.T) {
	newPathFixture(t)
	resolver := &pathResolver{cfg: newPathEngine().distanceMatchConfig(), entries: map[string][]os.DirEntry{}}
	if got := resolver.correct(pathWord{value: "~/.zshcr"}, 3); got != nil {
		t.Fatalf("correct without home = %+v, want nil", got)
	}
}

func TestReadBoundedDirSkipsLargeOrMissingDirectories(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a", "b", "c"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got := readBoundedDir(dir, 3); len(got) != 3 {
		t.Fatalf("readBoundedDir within bound = %d entries, want 3", len(got))
	}
	if got := readBoundedDir(dir, 2); got != nil {
		t.Fatalf("readBoundedDir over bound = %d entries, want nil", len(got))
	}
	if got := readBoundedDir(filepath.Join(dir, "missing"), 3); got != nil {
		t.Fatalf("readBoundedDir missing = %+v, want nil", got)
	}

	resolver := &pathResolver{entries: map[string][]os.DirEntry{}}
	if got := resolver.closestEntries(filepath.Join(dir, "missing"), "abc"); got != nil {
		t.Fatalf("closestEntries on missing dir = %+v, want nil", got)
	}
}

func TestQuotePathWord(t *testing.T) {
	for _, tt := range []struct {
		value string
		quote byte
		want  string
	}{
		{"plain/file.txt", 0, "plain/file.txt"},
		{"my docs/a.txt", 0, "'my docs/a.txt'"},
		{"~/my docs/a.txt", 0, "~/'my docs/a.txt'"},
		{"my docs/a.txt", '\'', "'my docs/a.txt'"},
		{"it's/a.txt", '\'', `"it's/a.txt"`},
		{"my docs/a.txt", '"', `"my docs/a.txt"`},
		{"cost$dir/a.txt", '"', "'cost$dir/a.txt'"},
		{"bad\x00name", 0, "'bad\x00name'"},
		{"bad\"\x00name", '"', "'bad\"\x00name'"},
	} {
		if got := quotePathWord(tt.value, tt.quote); got != tt.want {
			t.Fatalf("quotePathWord(%q, %q) = %q, want %q", tt.value, tt.quote, got, tt.want)
		}
	}
}

func TestUnescapeShellLiteral(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  string
		ok    bool
	}{
		{"plain", "plain", true},
		{`my\ docs`, "my docs", true},
		{`a\\b`, `a\b`, true},
		{`trailing\`, "", false},
		{"line\\\ncontinued", "", false},
	} {
		got, ok := unescapeShellLiteral(tt.input)
		if got != tt.want || ok != tt.ok {
			t.Fatalf("unescapeShellLiteral(%q) = %q, %v; want %q, %v", tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

func TestStaticPathWordValueRejectsDynamicWords(t *testing.T) {
	newPathFixture(t)
	eng := newPathEngine()
	for _, input := range []string{
		`cat $'proj/READNE.md'`,
		`cat "proj/$NAME.md"`,
		`cat "proj/"READNE.md`,
		`cat ""`,
		`cat "$(pwd)/READNE.md"`,
	} {
		if got := eng.pathFixCandidates(input, 3); got != nil {
			t.Fatalf("pathFixCandidates(%q) = %+v, want nil", input, got)
		}
	}
	if _, _, ok := staticPathWordValue(nil); ok {
		t.Fatal("nil word accepted")
	}
}
