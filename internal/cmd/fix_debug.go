package cmd

import (
	"fmt"
	"io"
	"strings"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

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
