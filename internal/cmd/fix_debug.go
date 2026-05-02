package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

type fixDebugMode string

const (
	fixDebugModeOff  fixDebugMode = "off"
	fixDebugModeText fixDebugMode = "text"
	fixDebugModeJSON fixDebugMode = "json"
)

func (m *fixDebugMode) String() string {
	if m == nil || *m == "" {
		return string(fixDebugModeOff)
	}
	return string(*m)
}

// Set parses the --debug flag value.
func (m *fixDebugMode) Set(value string) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "true", "text":
		*m = fixDebugModeText
	case "false", "off":
		*m = fixDebugModeOff
	case "json":
		*m = fixDebugModeJSON
	default:
		return fmt.Errorf("unsupported debug mode %q; use --debug or --debug=json", value)
	}
	return nil
}

// IsBoolFlag allows --debug to keep working without an explicit value.
func (m *fixDebugMode) IsBoolFlag() bool {
	return true
}

func (m fixDebugMode) enabled() bool {
	return m == fixDebugModeText || m == fixDebugModeJSON
}

func emitFixDebug(w io.Writer, result itypes.FixResult, mode fixDebugMode, traceFile string) error {
	if traceFile != "" {
		payload, err := marshalFixDebugJSON(result)
		if err != nil {
			return err
		}
		if err := os.WriteFile(traceFile, append(payload, '\n'), 0o600); err != nil {
			return fmt.Errorf("write debug trace: %w", err)
		}
	}

	switch mode {
	case fixDebugModeText:
		printFixDebug(w, result)
	case fixDebugModeJSON:
		return printFixDebugJSON(w, result)
	}
	return nil
}

func printFixDebug(w io.Writer, result itypes.FixResult) {
	if w == nil || result.Debug == nil {
		return
	}

	debug := result.Debug
	var builder strings.Builder
	writef := func(format string, args ...any) {
		_, _ = fmt.Fprintf(&builder, format, args...)
	}

	builder.WriteString("typo: debug\n")
	writef("typo:   input=%q fixed=%s", debug.InputCommand, yesNo(result.Fixed))
	if result.Fixed {
		writef(" final=%q source=%s", result.Command, debugValue(result.Source))
	}
	builder.WriteString("\n")
	writef(
		"typo:   alias-context provided=%s used=%s entries=%d\n",
		yesNo(debug.AliasContextProvided),
		yesNo(debug.AliasContextUsed),
		debug.AliasContextEntries,
	)
	writef(
		"typo:   features alias=%s parser=%s history=%s rule=%s tree=%s subcommand=%s distance=%s env=%s option=%s\n",
		yesNo(debug.UsedAlias),
		yesNo(debug.UsedParser),
		yesNo(debug.UsedHistory),
		yesNo(debug.UsedRule),
		yesNo(debug.UsedCommandTree),
		yesNo(debug.UsedSubcommand),
		yesNo(debug.UsedDistance),
		yesNo(debug.UsedEnv),
		yesNo(debug.UsedOption),
	)
	writef(
		"typo:   path-commands-loaded=%s discovered=%d\n",
		yesNo(debug.LoadedPATHCommands),
		debug.LoadedPATHCommandCount,
	)
	writef(
		"typo:   timing total=%s engine=%s auto-learn=%s\n",
		debugDuration(debug.TotalDuration),
		debugDuration(debug.EngineDuration),
		debugDuration(debug.AutoLearn.Duration),
	)

	if len(debug.Events) == 0 {
		builder.WriteString("typo:   matched-stages=none\n")
	} else {
		for _, event := range debug.Events {
			writef(
				"typo:   stage pass=%d stage=%s before=%q after=%q",
				event.Pass,
				debugValue(event.Stage),
				event.Before,
				event.After,
			)
			if event.Message != "" {
				writef(" message=%q", event.Message)
			}
			builder.WriteString("\n")
		}
	}

	if len(debug.RejectedCandidates) == 0 {
		builder.WriteString("typo:   rejected-candidates=none\n")
	} else {
		for _, candidate := range debug.RejectedCandidates {
			writef(
				"typo:   rejected stage=%s input=%q candidate=%q distance=%d similarity=%.2f reason=%q\n",
				debugValue(candidate.Stage),
				candidate.Input,
				candidate.Candidate,
				candidate.Distance,
				candidate.Similarity,
				candidate.Reason,
			)
		}
	}

	writef(
		"typo:   auto-learn attempted=%s triggered=%s persisted=%s timed-out=%s",
		yesNo(debug.AutoLearn.Attempted),
		yesNo(debug.AutoLearn.Triggered),
		yesNo(debug.AutoLearn.Persisted),
		yesNo(debug.AutoLearn.TimedOut),
	)
	if debug.AutoLearn.Reason != "" {
		writef(" reason=%q", debug.AutoLearn.Reason)
	}
	if debug.AutoLearn.Error != "" {
		writef(" error=%q", debug.AutoLearn.Error)
	}
	builder.WriteString("\n")

	_, _ = io.WriteString(w, builder.String())
}

func printFixExplanation(w io.Writer, result itypes.FixResult) {
	if w == nil || result.Debug == nil {
		return
	}

	debug := result.Debug
	var builder strings.Builder
	writef := func(format string, args ...any) {
		_, _ = fmt.Fprintf(&builder, format, args...)
	}

	writef("Input: %s\n\n", debug.InputCommand)

	eventsByStage := map[string][]itypes.FixDebugEvent{}
	for _, event := range debug.Events {
		eventsByStage[event.Stage] = append(eventsByStage[event.Stage], event)
	}

	step := 1
	for _, stage := range explainStages(debug) {
		events := eventsByStage[stage.key]
		if len(events) == 0 {
			writef("%d. %s: %s\n", step, stage.label, stage.fallback)
			step++
			continue
		}
		for _, event := range events {
			writef("%d. %s: %q -> %q", step, stage.label, event.Before, event.After)
			if event.Message != "" {
				writef(" (%s)", event.Message)
			}
			builder.WriteString("\n")
			step++
		}
	}

	if len(debug.RejectedCandidates) > 0 {
		builder.WriteString("\nRejected candidates:\n")
		for _, candidate := range debug.RejectedCandidates {
			writef(
				"  - %s: %q -> %q rejected (%s",
				debugValue(candidate.Stage),
				candidate.Input,
				candidate.Candidate,
				candidate.Reason,
			)
			if candidate.Distance > 0 {
				writef(", edit distance %d", candidate.Distance)
			}
			if candidate.Similarity > 0 {
				writef(", similarity %.0f%%", candidate.Similarity*100)
			}
			builder.WriteString(")\n")
		}
	}

	builder.WriteString("\nResult:\n")
	if result.Fixed {
		writef("  %s\n", result.Command)
	} else {
		builder.WriteString("  no correction found\n")
	}

	_, _ = io.WriteString(w, builder.String())
}

type explainStage struct {
	key      string
	label    string
	fallback string
}

func explainStages(debug *itypes.FixDebugInfo) []explainStage {
	parserFallback := "skipped, no stderr provided"
	if debug != nil && debug.UsedParser {
		parserFallback = "used parser context"
	}
	aliasFallback := "skipped, no alias context provided"
	if debug != nil && debug.AliasContextProvided {
		aliasFallback = fmt.Sprintf("no match from %d alias entries", debug.AliasContextEntries)
	}

	return []explainStage{
		{key: "parser", label: "parser", fallback: parserFallback},
		{key: "alias", label: "alias context", fallback: aliasFallback},
		{key: "history", label: "history", fallback: "no match"},
		{key: "tree", label: "command tree", fallback: "no match"},
		{key: "rule", label: "rules", fallback: "no match"},
		{key: "subcommand", label: "subcommand", fallback: "no match"},
		{key: "distance", label: "distance", fallback: "no match"},
		{key: "env", label: "environment", fallback: "no match"},
		{key: "option", label: "option", fallback: "no match"},
	}
}

func printFixDebugJSON(w io.Writer, result itypes.FixResult) error {
	if w == nil || result.Debug == nil {
		return nil
	}

	payload, err := marshalFixDebugJSON(result)
	if err != nil {
		return err
	}
	_, err = w.Write(append(payload, '\n'))
	return err
}

func marshalFixDebugJSON(result itypes.FixResult) ([]byte, error) {
	if result.Debug == nil {
		return nil, fmt.Errorf("debug trace is unavailable")
	}

	debug := result.Debug
	payload := fixDebugJSON{
		SchemaVersion: "1",
		Input:         debug.InputCommand,
		Fixed:         result.Fixed,
		Command:       result.Command,
		Source:        result.Source,
		Kind:          result.Kind,
		Message:       result.Message,
		AliasContext: fixDebugAliasContextJSON{
			Provided: debug.AliasContextProvided,
			Used:     debug.AliasContextUsed,
			Entries:  debug.AliasContextEntries,
		},
		Features: fixDebugFeaturesJSON{
			Alias:       debug.UsedAlias,
			Parser:      debug.UsedParser,
			History:     debug.UsedHistory,
			Rule:        debug.UsedRule,
			CommandTree: debug.UsedCommandTree,
			Subcommand:  debug.UsedSubcommand,
			Distance:    debug.UsedDistance,
			Env:         debug.UsedEnv,
			Option:      debug.UsedOption,
		},
		PathCommands: fixDebugPathCommandsJSON{
			Loaded:     debug.LoadedPATHCommands,
			Discovered: debug.LoadedPATHCommandCount,
		},
		Timing: fixDebugTimingJSON{
			Total:       durationJSON(debug.TotalDuration),
			Engine:      durationJSON(debug.EngineDuration),
			AutoLearn:   durationJSON(debug.AutoLearn.Duration),
			TotalMS:     durationMillis(debug.TotalDuration),
			EngineMS:    durationMillis(debug.EngineDuration),
			AutoLearnMS: durationMillis(debug.AutoLearn.Duration),
		},
		Events:             fixDebugEventsJSON(debug.Events),
		RejectedCandidates: fixDebugCandidatesJSON(debug.RejectedCandidates),
		AutoLearn:          fixAutoLearnJSON(debug.AutoLearn),
	}

	if payload.Events == nil {
		payload.Events = []fixDebugEventJSON{}
	}
	if payload.RejectedCandidates == nil {
		payload.RejectedCandidates = []fixDebugCandidateJSON{}
	}

	return json.MarshalIndent(payload, "", "  ")
}

type fixDebugJSON struct {
	SchemaVersion      string                   `json:"schema_version"`
	Input              string                   `json:"input"`
	Fixed              bool                     `json:"fixed"`
	Command            string                   `json:"command,omitempty"`
	Source             string                   `json:"source,omitempty"`
	Kind               string                   `json:"kind,omitempty"`
	Message            string                   `json:"message,omitempty"`
	AliasContext       fixDebugAliasContextJSON `json:"alias_context"`
	Features           fixDebugFeaturesJSON     `json:"features"`
	PathCommands       fixDebugPathCommandsJSON `json:"path_commands"`
	Timing             fixDebugTimingJSON       `json:"timing"`
	Events             []fixDebugEventJSON      `json:"events"`
	RejectedCandidates []fixDebugCandidateJSON  `json:"rejected_candidates"`
	AutoLearn          fixAutoLearnJSONPayload  `json:"auto_learn"`
}

type fixDebugAliasContextJSON struct {
	Provided bool `json:"provided"`
	Used     bool `json:"used"`
	Entries  int  `json:"entries"`
}

type fixDebugFeaturesJSON struct {
	Alias       bool `json:"alias"`
	Parser      bool `json:"parser"`
	History     bool `json:"history"`
	Rule        bool `json:"rule"`
	CommandTree bool `json:"command_tree"`
	Subcommand  bool `json:"subcommand"`
	Distance    bool `json:"distance"`
	Env         bool `json:"env"`
	Option      bool `json:"option"`
}

type fixDebugPathCommandsJSON struct {
	Loaded     bool `json:"loaded"`
	Discovered int  `json:"discovered"`
}

type fixDebugTimingJSON struct {
	Total       string  `json:"total"`
	Engine      string  `json:"engine"`
	AutoLearn   string  `json:"auto_learn"`
	TotalMS     float64 `json:"total_ms"`
	EngineMS    float64 `json:"engine_ms"`
	AutoLearnMS float64 `json:"auto_learn_ms"`
}

type fixDebugEventJSON struct {
	Pass    int    `json:"pass"`
	Stage   string `json:"stage"`
	Before  string `json:"before"`
	After   string `json:"after"`
	Message string `json:"message,omitempty"`
}

type fixDebugCandidateJSON struct {
	Stage      string  `json:"stage"`
	Input      string  `json:"input"`
	Candidate  string  `json:"candidate"`
	Distance   int     `json:"distance"`
	Similarity float64 `json:"similarity"`
	Reason     string  `json:"reason"`
}

type fixAutoLearnJSONPayload struct {
	Attempted  bool    `json:"attempted"`
	Triggered  bool    `json:"triggered"`
	Persisted  bool    `json:"persisted"`
	TimedOut   bool    `json:"timed_out"`
	Duration   string  `json:"duration"`
	DurationMS float64 `json:"duration_ms"`
	Error      string  `json:"error,omitempty"`
	Reason     string  `json:"reason,omitempty"`
}

func fixDebugEventsJSON(events []itypes.FixDebugEvent) []fixDebugEventJSON {
	if len(events) == 0 {
		return nil
	}
	out := make([]fixDebugEventJSON, 0, len(events))
	for _, event := range events {
		out = append(out, fixDebugEventJSON{
			Pass:    event.Pass,
			Stage:   event.Stage,
			Before:  event.Before,
			After:   event.After,
			Message: event.Message,
		})
	}
	return out
}

func fixDebugCandidatesJSON(candidates []itypes.FixDebugCandidate) []fixDebugCandidateJSON {
	if len(candidates) == 0 {
		return nil
	}
	out := make([]fixDebugCandidateJSON, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, fixDebugCandidateJSON{
			Stage:      candidate.Stage,
			Input:      candidate.Input,
			Candidate:  candidate.Candidate,
			Distance:   candidate.Distance,
			Similarity: candidate.Similarity,
			Reason:     candidate.Reason,
		})
	}
	return out
}

func fixAutoLearnJSON(info itypes.AutoLearnDebugInfo) fixAutoLearnJSONPayload {
	return fixAutoLearnJSONPayload{
		Attempted:  info.Attempted,
		Triggered:  info.Triggered,
		Persisted:  info.Persisted,
		TimedOut:   info.TimedOut,
		Duration:   durationJSON(info.Duration),
		DurationMS: durationMillis(info.Duration),
		Error:      info.Error,
		Reason:     info.Reason,
	}
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func debugValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func debugDuration(value interface{ String() string }) string {
	text := value.String()
	if text == "" || text == "0s" {
		return "0s"
	}
	return text
}

func durationJSON(value time.Duration) string {
	if value <= 0 {
		return "0s"
	}
	return value.String()
}

func durationMillis(value time.Duration) float64 {
	if value <= 0 {
		return 0
	}
	return float64(value) / float64(time.Millisecond)
}
