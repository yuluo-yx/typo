package engine

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"mvdan.cc/sh/v3/syntax"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

const (
	fixSourcePath = "path"

	// maxPathDirEntries bounds how many entries one directory scan may read.
	// Larger directories are skipped instead of paying for a full listing.
	maxPathDirEntries = 4096
	// maxPathSegmentMatches bounds how many near matches one missing segment may expand to.
	maxPathSegmentMatches = 3
	// maxPathCorrectedSegments bounds how many segments of one path may be rewritten.
	maxPathCorrectedSegments = 2
	// minPathSegmentRunes keeps very short names, which match too many entries, out of fuzzy matching.
	minPathSegmentRunes = 3
)

// pathArgPolicy selects which positional arguments of a command must name an existing path.
type pathArgPolicy int

const (
	pathArgsFirst      pathArgPolicy = iota // Only the first positional argument, such as `source FILE ARGS...`.
	pathArgsAll                             // Every positional argument.
	pathArgsAllButLast                      // Every positional argument except the destination.
)

// pathCommandPolicies lists commands whose path arguments are read, never created.
// Commands that may create their arguments (editors, touch, mkdir, rm targets) are
// deliberately absent so a correction never redirects a write to another file.
var pathCommandPolicies = map[string]pathArgPolicy{
	"source": pathArgsFirst,
	".":      pathArgsFirst,
	"cd":     pathArgsFirst,
	"pushd":  pathArgsFirst,
	"cat":    pathArgsAll,
	"bat":    pathArgsAll,
	"less":   pathArgsAll,
	"more":   pathArgsAll,
	"head":   pathArgsAll,
	"tail":   pathArgsAll,
	"wc":     pathArgsAll,
	"stat":   pathArgsAll,
	"file":   pathArgsAll,
	"ls":     pathArgsAll,
	"du":     pathArgsAll,
	"diff":   pathArgsAll,
	"cp":     pathArgsAllButLast,
	"mv":     pathArgsAllButLast,
}

// pathWord is one static shell word that names a path.
type pathWord struct {
	index int    // Index of the word in the command line.
	value string // Unquoted word value.
	quote byte   // Original quote style: 0, '\'' or '"'.
}

// pathSegmentMatch is one directory entry close to a missing path segment.
type pathSegmentMatch struct {
	name string
	cost int // Ranking cost in half edits: case-only = 1, transposition = 2, otherwise 2*distance.
}

// pathCorrection is one corrected path value with its accumulated ranking cost.
type pathCorrection struct {
	value string
	cost  int
}

// pathResolver resolves path corrections while memoizing directory listings for one fix attempt.
type pathResolver struct {
	cfg     distanceMatchConfig
	home    string
	entries map[string][]os.DirEntry
}

func (e *Engine) newPathResolver() *pathResolver {
	home, _ := os.UserHomeDir()
	return &pathResolver{
		cfg:     e.distanceMatchConfig(),
		home:    home,
		entries: make(map[string][]os.DirEntry),
	}
}

// tryPathFix rewrites path arguments that do not exist into the unique closest
// existing path. Ambiguous paths are left for the candidate menu.
func (e *Engine) tryPathFix(cmd string) itypes.FixResult {
	if !e.pathFixEnabled {
		return itypes.FixResult{Fixed: false}
	}
	results := e.pathFixCandidates(cmd, 2)
	if len(results) == 0 || len(results) > 1 && results[0].cost == results[1].cost {
		return itypes.FixResult{Fixed: false}
	}
	return results[0].result
}

type pathFixCandidate struct {
	result itypes.FixResult
	cost   int
}

// pathFixCandidates returns up to limit corrected commands ranked by cost.
// Only the first correctable path argument is rewritten; the fix chain runs
// again for any later argument.
func (e *Engine) pathFixCandidates(cmd string, limit int) []pathFixCandidate {
	if !e.pathFixEnabled || limit < 1 {
		return nil
	}
	lines, err := parseShellCommandLines(cmd)
	if err != nil {
		return nil
	}

	resolver := e.newPathResolver()
	for _, line := range lines {
		name := line.commandWord()
		policy, ok := pathCommandPolicies[name]
		if !ok {
			continue
		}
		for _, word := range pathArgumentWords(line, policy) {
			corrections := resolver.correct(word, limit)
			if len(corrections) == 0 {
				continue
			}
			candidates := make([]pathFixCandidate, 0, len(corrections))
			for _, correction := range corrections {
				candidates = append(candidates, pathFixCandidate{
					result: itypes.FixResult{
						Fixed:   true,
						Command: line.replaceWord(word.index, quotePathWord(correction.value, word.quote)),
						Source:  fixSourcePath,
						Kind:    itypes.FixKindPath,
					},
					cost: correction.cost,
				})
			}
			return candidates
		}
		// A directory change makes later relative paths unresolvable from here.
		if name == "cd" || name == "pushd" {
			break
		}
	}
	return nil
}

// pathArgumentWords returns the static positional words selected by policy.
func pathArgumentWords(line *shellCommandLine, policy pathArgPolicy) []pathWord {
	positionals := make([]pathWord, 0, len(line.args)-line.commandIdx-1)
	optionsDone := false
	for i := line.commandIdx + 1; i < len(line.args); i++ {
		value, quote, ok := staticPathWordValue(line.args[i])
		if !ok {
			// Dynamic words still occupy a positional slot.
			positionals = append(positionals, pathWord{index: -1})
			continue
		}
		if !optionsDone && quote == 0 {
			if value == "--" {
				optionsDone = true
				continue
			}
			if strings.HasPrefix(value, "-") || strings.HasPrefix(value, "+") {
				continue
			}
		}
		positionals = append(positionals, pathWord{index: i, value: value, quote: quote})
	}

	switch policy {
	case pathArgsFirst:
		if len(positionals) > 1 {
			positionals = positionals[:1]
		}
	case pathArgsAllButLast:
		if len(positionals) < 2 {
			return nil
		}
		positionals = positionals[:len(positionals)-1]
	}

	words := positionals[:0]
	for _, word := range positionals {
		if word.index >= 0 {
			words = append(words, word)
		}
	}
	return words
}

// staticPathWordValue returns the literal value of a word made of one plain,
// single-quoted, or double-quoted literal without expansions or escapes.
func staticPathWordValue(word *syntax.Word) (string, byte, bool) {
	if word == nil || len(word.Parts) != 1 {
		return "", 0, false
	}
	switch part := word.Parts[0].(type) {
	case *syntax.Lit:
		// An escaped leading tilde is literal, which the tilde handling cannot express.
		if strings.ContainsAny(part.Value, "*?[]{}$`") || strings.HasPrefix(part.Value, "\\~") {
			return "", 0, false
		}
		value, ok := unescapeShellLiteral(part.Value)
		return value, 0, ok && value != ""
	case *syntax.SglQuoted:
		if part.Dollar {
			return "", 0, false
		}
		return part.Value, '\'', part.Value != ""
	case *syntax.DblQuoted:
		if len(part.Parts) != 1 {
			return "", 0, false
		}
		lit, ok := part.Parts[0].(*syntax.Lit)
		if !ok || strings.ContainsAny(lit.Value, "$`\\") {
			return "", 0, false
		}
		return lit.Value, '"', lit.Value != ""
	default:
		return "", 0, false
	}
}

// unescapeShellLiteral removes backslash escapes from an unquoted literal such as `my\ docs`.
func unescapeShellLiteral(value string) (string, bool) {
	if !strings.Contains(value, "\\") {
		return value, true
	}
	var unescaped strings.Builder
	unescaped.Grow(len(value))
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' {
			unescaped.WriteByte(value[i])
			continue
		}
		if i+1 == len(value) || value[i+1] == '\n' {
			return "", false
		}
		i++
		unescaped.WriteByte(value[i])
	}
	return unescaped.String(), true
}

// quotePathWord renders value in the original quote style, falling back to
// shell quoting when that style cannot hold the corrected value.
func quotePathWord(value string, quote byte) string {
	switch quote {
	case '\'':
		if !strings.Contains(value, "'") {
			return "'" + value + "'"
		}
	case '"':
		if !strings.ContainsAny(value, "\"$`\\") {
			return `"` + value + `"`
		}
	default:
		// A leading tilde only expands when it stays unquoted.
		prefix := ""
		rest := value
		if strings.HasPrefix(value, "~/") {
			prefix, rest = "~/", value[2:]
		}
		quoted, err := syntax.Quote(rest, syntax.LangBash)
		if err == nil && quoted != "" {
			return prefix + quoted
		}
		return prefix + shellQuoteFallback(rest)
	}
	quoted, err := syntax.Quote(value, syntax.LangBash)
	if err != nil {
		return shellQuoteFallback(value)
	}
	return quoted
}

func shellQuoteFallback(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// correct returns up to limit existing paths close to word, best first, or nil
// when word already exists or nothing is close enough.
func (r *pathResolver) correct(word pathWord, limit int) []pathCorrection {
	root, prefix, rest, ok := r.splitPathRoot(word)
	if !ok || rest == "" {
		return nil
	}
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rest))); err == nil {
		return nil
	}

	segments := strings.Split(rest, "/")
	results := make([]pathCorrection, 0, limit)
	r.walk(root, segments, 0, make([]string, 0, len(segments)), 0, 0, &results)
	if len(results) == 0 {
		return nil
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].cost < results[j].cost })
	if len(results) > limit {
		results = results[:limit]
	}
	for i := range results {
		results[i].value = prefix + results[i].value
	}
	return results
}

// splitPathRoot returns the directory a word resolves from, the literal prefix
// kept in the output, and the remaining slash-separated path.
func (r *pathResolver) splitPathRoot(word pathWord) (string, string, string, bool) {
	value := word.value
	switch {
	case word.quote == 0 && strings.HasPrefix(value, "~/"):
		if r.home == "" {
			return "", "", "", false
		}
		return r.home, "~/", value[2:], true
	case word.quote == 0 && strings.HasPrefix(value, "~"):
		// `~` alone and `~user` forms are left untouched.
		return "", "", "", false
	case strings.HasPrefix(value, "/"):
		return string(filepath.Separator), "/", strings.TrimLeft(value, "/"), true
	default:
		return ".", "", value, true
	}
}

// walk resolves segments depth-first, expanding missing segments into their
// closest directory entries and recording every resulting path that exists.
func (r *pathResolver) walk(dir string, segments []string, index int, built []string, cost, changed int, results *[]pathCorrection) {
	if index == len(segments) {
		*results = append(*results, pathCorrection{value: strings.Join(built, "/"), cost: cost})
		return
	}

	segment := segments[index]
	last := index == len(segments)-1
	if segment == "" || segment == "." || segment == ".." {
		r.walk(filepath.Join(dir, segment), segments, index+1, append(built, segment), cost, changed, results)
		return
	}

	next := filepath.Join(dir, segment)
	if pathUsable(next, last) {
		r.walk(next, segments, index+1, append(built, segment), cost, changed, results)
		return
	}
	if changed >= maxPathCorrectedSegments {
		return
	}
	for _, match := range r.closestEntries(dir, segment) {
		next := filepath.Join(dir, match.name)
		if !pathUsable(next, last) {
			continue
		}
		r.walk(next, segments, index+1, append(built, match.name), cost+match.cost, changed+1, results)
	}
}

// pathUsable reports whether path exists and, when more segments follow, is a directory.
func pathUsable(path string, last bool) bool {
	if last {
		_, err := os.Lstat(path)
		return err == nil
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// closestEntries returns the entries of dir closest to segment, best first.
func (r *pathResolver) closestEntries(dir, segment string) []pathSegmentMatch {
	segmentRunes := utf8.RuneCountInString(segment)
	entries := r.readDir(dir)
	if len(entries) == 0 {
		return nil
	}

	maxDistance := 2
	if segmentRunes <= 4 {
		maxDistance = 1
	}
	cfg := r.cfg
	if cfg.maxEditDistance > 0 && cfg.maxEditDistance < maxDistance {
		maxDistance = cfg.maxEditDistance
	}
	cfg.maxEditDistance = maxDistance
	hidden := strings.HasPrefix(segment, ".")
	lowerSegment := strings.ToLower(segment)

	matches := make([]pathSegmentMatch, 0, maxPathSegmentMatches)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") != hidden {
			continue
		}
		if strings.ToLower(name) == lowerSegment {
			matches = append(matches, pathSegmentMatch{name: name, cost: 1})
			continue
		}
		if segmentRunes < minPathSegmentRunes {
			continue
		}
		// One swapped pair of neighbors is a single typing slip, even in short names.
		if isSingleAdjacentTransposition(segment, name) {
			matches = append(matches, pathSegmentMatch{name: name, cost: 2})
			continue
		}
		nameRunes := utf8.RuneCountInString(name)
		if absInt(nameRunes-segmentRunes) > maxDistance {
			continue
		}
		distance, ok := distanceRunesWithin([]rune(segment), []rune(name), cfg.keyboard, maxDistance)
		if !ok || !isGoodDistanceMatch(segment, name, distance, cfg) {
			continue
		}
		matches = append(matches, pathSegmentMatch{name: name, cost: 2 * distance})
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].cost != matches[j].cost {
			return matches[i].cost < matches[j].cost
		}
		return matches[i].name < matches[j].name
	})
	if len(matches) > maxPathSegmentMatches {
		matches = matches[:maxPathSegmentMatches]
	}
	return matches
}

// readDir returns at most maxPathDirEntries entries of dir, or nil when the
// directory cannot be read or holds more entries than that bound.
func (r *pathResolver) readDir(dir string) []os.DirEntry {
	if entries, ok := r.entries[dir]; ok {
		return entries
	}
	entries := readBoundedDir(dir, maxPathDirEntries)
	r.entries[dir] = entries
	return entries
}

func readBoundedDir(dir string, limit int) []os.DirEntry {
	file, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()

	entries, err := file.ReadDir(limit + 1)
	if err != nil && !errors.Is(err, io.EOF) || len(entries) > limit {
		return nil
	}
	return entries
}
