package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

func sampleFixDebugResult() itypes.FixResult {
	return itypes.FixResult{
		Fixed:   true,
		Command: "git status",
		Source:  "rule",
		Kind:    "command",
		Message: "corrected",
		Debug: &itypes.FixDebugInfo{
			InputCommand:           "gut status",
			AliasContextProvided:   true,
			AliasContextUsed:       true,
			AliasContextEntries:    2,
			LoadedPATHCommands:     true,
			LoadedPATHCommandCount: 7,
			TotalDuration:          1500 * time.Microsecond,
			EngineDuration:         1 * time.Millisecond,
			UsedAlias:              true,
			UsedParser:             true,
			UsedHistory:            true,
			UsedRule:               true,
			UsedCommandTree:        true,
			UsedSubcommand:         true,
			UsedDistance:           true,
			UsedEnv:                true,
			UsedOption:             true,
			Events: []itypes.FixDebugEvent{
				{Pass: 1, Stage: "rule", Before: "gut status", After: "git status", Message: "builtin rule"},
			},
			RejectedCandidates: []itypes.FixDebugCandidate{
				{
					Stage:      "distance",
					Input:      "gut",
					Candidate:  "get",
					Distance:   1,
					Similarity: 0.75,
					Reason:     "not a known command",
				},
			},
			AutoLearn: itypes.AutoLearnDebugInfo{
				Attempted: true,
				Triggered: true,
				Persisted: true,
				TimedOut:  true,
				Duration:  2 * time.Millisecond,
				Error:     "store locked",
				Reason:    "threshold reached",
			},
		},
	}
}

func TestFixDebugModeStringAndSet(t *testing.T) {
	var mode fixDebugMode
	if got := mode.String(); got != "off" {
		t.Fatalf("empty String() = %q", got)
	}
	if got := (*fixDebugMode)(nil).String(); got != "off" {
		t.Fatalf("nil String() = %q", got)
	}

	tests := []struct {
		value string
		want  fixDebugMode
	}{
		{"", fixDebugModeText},
		{"true", fixDebugModeText},
		{" text ", fixDebugModeText},
		{"false", fixDebugModeOff},
		{"off", fixDebugModeOff},
		{"json", fixDebugModeJSON},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			var got fixDebugMode
			if err := got.Set(tt.value); err != nil {
				t.Fatalf("Set(%q) failed: %v", tt.value, err)
			}
			if got != tt.want {
				t.Fatalf("Set(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}

	if err := mode.Set("xml"); err == nil || !strings.Contains(err.Error(), "unsupported debug mode") {
		t.Fatalf("expected unsupported mode error, got %v", err)
	}
}

func TestEmitFixDebugWritesTraceAndModeOutput(t *testing.T) {
	result := sampleFixDebugResult()
	traceFile := filepath.Join(t.TempDir(), "trace.json")
	var buf bytes.Buffer

	if err := emitFixDebug(&buf, result, fixDebugModeText, traceFile); err != nil {
		t.Fatalf("emitFixDebug failed: %v", err)
	}

	text := buf.String()
	for _, want := range []string{
		"typo: debug",
		"input=\"gut status\" fixed=yes final=\"git status\" source=rule",
		"alias-context provided=yes used=yes entries=2",
		"rejected stage=distance",
		"auto-learn attempted=yes triggered=yes persisted=yes timed-out=yes",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("debug text missing %q: %q", want, text)
		}
	}

	data, err := os.ReadFile(traceFile)
	if err != nil {
		t.Fatalf("read trace file: %v", err)
	}
	if !strings.Contains(string(data), `"schema_version": "1"`) ||
		!strings.Contains(string(data), `"candidate": "get"`) {
		t.Fatalf("unexpected trace JSON: %s", data)
	}
}

func TestEmitFixDebugReportsUnavailableTrace(t *testing.T) {
	err := emitFixDebug(&bytes.Buffer{}, itypes.FixResult{}, fixDebugModeOff, filepath.Join(t.TempDir(), "trace.json"))
	if err == nil || !strings.Contains(err.Error(), "debug trace is unavailable") {
		t.Fatalf("expected unavailable trace error, got %v", err)
	}
}

func TestPrintFixDebugHandlesNilAndEmptyCollections(t *testing.T) {
	var buf bytes.Buffer
	printFixDebug(&buf, itypes.FixResult{})
	if buf.Len() != 0 {
		t.Fatalf("nil debug should not write output, got %q", buf.String())
	}

	result := itypes.FixResult{
		Debug: &itypes.FixDebugInfo{
			InputCommand: "unknown",
		},
	}
	printFixDebug(&buf, result)
	text := buf.String()
	for _, want := range []string{
		"matched-stages=none",
		"rejected-candidates=none",
		"fixed=no",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("debug text missing %q: %q", want, text)
		}
	}
}

func TestPrintFixExplanationIncludesFallbacksEventsAndRejections(t *testing.T) {
	result := sampleFixDebugResult()
	var buf bytes.Buffer

	printFixExplanation(&buf, result)

	text := buf.String()
	for _, want := range []string{
		"Input: gut status",
		`rules: "gut status" -> "git status" (builtin rule)`,
		"parser: used parser context",
		"alias context: no match from 2 alias entries",
		`distance: "gut" -> "get" rejected (not a known command, edit distance 1, similarity 75%)`,
		"Result:",
		"git status",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("explanation missing %q: %q", want, text)
		}
	}

	buf.Reset()
	printFixExplanation(&buf, itypes.FixResult{Debug: &itypes.FixDebugInfo{InputCommand: "x"}})
	if !strings.Contains(buf.String(), "no correction found") {
		t.Fatalf("missing no correction explanation: %q", buf.String())
	}
}

func TestPrintFixDebugJSONWritesStructuredPayload(t *testing.T) {
	result := sampleFixDebugResult()
	var buf bytes.Buffer

	if err := printFixDebugJSON(&buf, result); err != nil {
		t.Fatalf("printFixDebugJSON failed: %v", err)
	}

	var payload fixDebugJSON
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("debug JSON is invalid: %v\n%s", err, buf.String())
	}
	if payload.SchemaVersion != "1" ||
		payload.Input != "gut status" ||
		payload.Command != "git status" ||
		!payload.Features.Rule ||
		payload.PathCommands.Discovered != 7 ||
		len(payload.Events) != 1 ||
		len(payload.RejectedCandidates) != 1 ||
		!payload.AutoLearn.Persisted {
		t.Fatalf("unexpected debug JSON payload: %+v", payload)
	}
	if payload.Timing.Total == "0s" || payload.Timing.TotalMS <= 0 {
		t.Fatalf("expected positive timing payload: %+v", payload.Timing)
	}
}

func TestMarshalFixDebugJSONNormalizesNilSlices(t *testing.T) {
	payload, err := marshalFixDebugJSON(itypes.FixResult{
		Debug: &itypes.FixDebugInfo{InputCommand: "noop"},
	})
	if err != nil {
		t.Fatalf("marshalFixDebugJSON failed: %v", err)
	}

	var decoded fixDebugJSON
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("debug JSON is invalid: %v\n%s", err, string(payload))
	}
	if decoded.Events == nil || decoded.RejectedCandidates == nil {
		t.Fatalf("nil slices should be encoded as empty arrays: %+v", decoded)
	}
}

func TestFixDebugFormattingHelpers(t *testing.T) {
	if yesNo(true) != "yes" || yesNo(false) != "no" {
		t.Fatalf("yesNo returned unexpected values")
	}
	if debugValue("  ") != "-" || debugValue("rule") != "rule" {
		t.Fatalf("debugValue returned unexpected values")
	}
	if debugDuration(0*time.Second) != "0s" || debugDuration(3*time.Millisecond) != "3ms" {
		t.Fatalf("debugDuration returned unexpected values")
	}
	if durationJSON(-time.Second) != "0s" || durationJSON(2*time.Millisecond) != "2ms" {
		t.Fatalf("durationJSON returned unexpected values")
	}
	if durationMillis(-time.Second) != 0 || durationMillis(1500*time.Microsecond) != 1.5 {
		t.Fatalf("durationMillis returned unexpected values")
	}
}
