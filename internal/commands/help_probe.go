package commands

import (
	"bytes"
	"os"
	"strings"
	"time"
)

const (
	// defaultNegativeProbeTTL bounds how long an empty help probe suppresses re-probing.
	defaultNegativeProbeTTL = 24 * time.Hour
	// maxHelpOutputBytes caps captured help output so a runaway tool cannot exhaust memory.
	maxHelpOutputBytes = 1 << 20
)

// helpSubcommandTools lists tools whose positional "help" argument is a real help
// subcommand. Any other tool may treat "help" as data (for example "rm help"
// deletes a file named help), so the fallback must never run for them.
var helpSubcommandTools = map[string]bool{
	"git": true, "docker": true, "npm": true, "yarn": true, "kubectl": true,
	"cargo": true, "go": true, "brew": true, "terraform": true, "helm": true,
	"aws": true, "gcloud": true, "az": true, "pip": true, "pip3": true, "composer": true,
}

// NegativeProbe records a help probe that produced no subcommands, keyed by the
// executable identity so upgrading or replacing the tool invalidates the entry.
type NegativeProbe struct {
	Tool      string    `json:"tool"`
	Prefix    []string  `json:"prefix,omitempty"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"mod_time"`
	CheckedAt time.Time `json:"checked_at"`
}

// SetDiscoverUnknownTools allows help probing for tools outside the builtin allowlist.
// It is disabled by default because probing executes the tool.
func (r *ToolTreeRegistry) SetDiscoverUnknownTools(enabled bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.discoverUnknown = enabled
	r.mu.Unlock()
}

// canProbeTool reports whether typo may execute the tool to discover subcommands.
func (r *ToolTreeRegistry) canProbeTool(tool string) bool {
	if isKnownSubcommandTool(tool) {
		return true
	}
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.discoverUnknown
}

func probeKey(tool string, prefix []string) string {
	return tool + "\x00" + strings.Join(prefix, "\x00")
}

// claimProbe marks a tool path as probed in this process and reports whether the
// caller should run the probe. Each path is probed at most once per process, and a
// fresh negative entry from an earlier process also suppresses the probe.
func (r *ToolTreeRegistry) claimProbe(tool string, prefix []string) bool {
	key := probeKey(tool, prefix)

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.probed == nil {
		r.probed = make(map[string]bool)
	}
	if r.probed[key] {
		return false
	}
	r.probed[key] = true

	entry, ok := r.negative[key]
	if !ok {
		return true
	}
	if time.Since(entry.CheckedAt) >= r.negativeExpiry() || !sameExecutable(entry, tool) {
		delete(r.negative, key)
		return true
	}
	return false
}

// recordNegativeProbe remembers that probing produced nothing usable.
func (r *ToolTreeRegistry) recordNegativeProbe(tool string, prefix []string) {
	path := GetPath(tool)
	if path == "" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		return
	}

	r.mu.Lock()
	if r.negative == nil {
		r.negative = make(map[string]NegativeProbe)
	}
	r.negative[probeKey(tool, prefix)] = NegativeProbe{
		Tool:      tool,
		Prefix:    append([]string(nil), prefix...),
		Path:      path,
		Size:      info.Size(),
		ModTime:   info.ModTime().UTC(),
		CheckedAt: time.Now().UTC(),
	}
	r.mu.Unlock()

	r.saveCache()
}

func (r *ToolTreeRegistry) negativeExpiry() time.Duration {
	if r.negativeTTL <= 0 {
		return defaultNegativeProbeTTL
	}
	return r.negativeTTL
}

func sameExecutable(entry NegativeProbe, tool string) bool {
	path := GetPath(tool)
	if path == "" || path != entry.Path {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() == entry.Size && info.ModTime().UTC().Equal(entry.ModTime.UTC())
}

// cappedBuffer keeps at most limit bytes and silently discards the rest, so the
// child process never blocks on a full pipe.
type cappedBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if remaining := b.limit - b.buf.Len(); remaining > 0 {
		if len(p) > remaining {
			b.buf.Write(p[:remaining])
		} else {
			b.buf.Write(p)
		}
	}
	return len(p), nil
}

func (b *cappedBuffer) String() string {
	return b.buf.String()
}
