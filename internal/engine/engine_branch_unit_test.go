package engine

import (
	"context"
	"testing"

	"github.com/yuluo-yx/typo/internal/commands"
	itypes "github.com/yuluo-yx/typo/internal/types"
)

func TestDebugTraceNilEngine(t *testing.T) {
	var nilEngine *Engine
	if nilEngine.debugTrace() != nil {
		t.Fatalf("nil engine debugTrace should be nil")
	}
}

func TestDebugTraceMarksFeatures(t *testing.T) {
	eng := NewEngine()
	eng.markDebugFeature("rule")

	debug := &itypes.FixDebugInfo{}
	eng.currentDebug = debug
	for _, stage := range []string{
		"alias",
		fixSourceParser,
		fixSourceHistory,
		"rule",
		"tree",
		"subcommand",
		fixSourceDistance,
		"env",
		"option",
	} {
		eng.markDebugFeature(stage)
	}
	if !debug.UsedAlias || !debug.AliasContextUsed || !debug.UsedParser ||
		!debug.UsedHistory || !debug.UsedRule || !debug.UsedCommandTree ||
		!debug.UsedSubcommand || !debug.UsedDistance || !debug.UsedEnv || !debug.UsedOption {
		t.Fatalf("debug features were not all marked: %+v", debug)
	}
}

func TestDebugTraceRecordsAcceptedFix(t *testing.T) {
	eng := NewEngine()
	debug := &itypes.FixDebugInfo{}
	eng.currentDebug = debug

	eng.recordAcceptedFix(2, "before", itypes.FixResult{
		Fixed:      true,
		Command:    "after",
		Source:     fixSourceParser,
		Message:    "parsed",
		UsedParser: true,
	})
	if len(debug.Events) != 1 || debug.Events[0].Pass != 2 || debug.Events[0].Message != "parsed" {
		t.Fatalf("accepted fix event not recorded: %+v", debug.Events)
	}
}

func TestDebugTraceRejectedCandidateDedupeAndCap(t *testing.T) {
	eng := NewEngine()
	debug := &itypes.FixDebugInfo{}
	eng.currentDebug = debug

	eng.recordRejectedCandidate(fixSourceDistance, "", "git", 1, 0.5, "missing input")
	if len(debug.RejectedCandidates) != 0 {
		t.Fatalf("empty rejected candidate should be ignored")
	}
	eng.recordRejectedCandidate(fixSourceDistance, "gut", "git", 1, 0.5, "threshold")
	eng.recordRejectedCandidate(fixSourceDistance, "gut", "git", 1, 0.5, "duplicate")
	if len(debug.RejectedCandidates) != 1 {
		t.Fatalf("duplicate rejected candidate should be ignored: %+v", debug.RejectedCandidates)
	}
	for i := 0; i < 10; i++ {
		eng.recordRejectedCandidate("stage", "input", string(rune('a'+i)), i, 0.1, "reason")
	}
	if len(debug.RejectedCandidates) != 5 {
		t.Fatalf("rejected candidates should be capped at 5, got %d", len(debug.RejectedCandidates))
	}
}

func TestTryToolOptionFixFallbackBranches(t *testing.T) {
	eng := NewEngine(
		WithCommands([]string{"docker"}),
		WithKeyboard(NewQWERTYKeyboard()),
		WithToolTrees(commands.NewToolTreeRegistry("")),
	)

	if got := (&Engine{}).tryToolOptionFix("docker --contex prod '"); got.Fixed {
		t.Fatalf("nil tool tree option fix should fail, got %+v", got)
	}
	if got := eng.tryToolOptionFix("'"); got.Fixed {
		t.Fatalf("single token fallback should fail, got %+v", got)
	}
	if got := eng.tryToolOptionFix("zzzz --contex prod '"); got.Fixed {
		t.Fatalf("unresolvable command fallback should fail, got %+v", got)
	}
	if got := eng.tryToolOptionFix("docker --zzzzz prod '"); got.Fixed {
		t.Fatalf("unknown long option fallback should fail, got %+v", got)
	}

	got := eng.tryToolOptionFix("docker --contex prod run '")
	if !got.Fixed || got.Command != "docker --context prod run '" || got.Source != "option" {
		t.Fatalf("fallback option fix = %+v", got)
	}

	engWithDistance := NewEngine(
		WithCommands([]string{"docker"}),
		WithKeyboard(NewQWERTYKeyboard()),
		WithToolTrees(commands.NewToolTreeRegistry("")),
	)
	got = engWithDistance.tryToolOptionFix("dcoker --contex prod run '")
	if !got.Fixed || got.Command != "docker --context prod run '" {
		t.Fatalf("fallback option fix with resolved command = %+v", got)
	}
}

func TestLongOptionReplacementGuardBranches(t *testing.T) {
	eng := NewEngine(WithToolTrees(commands.NewToolTreeRegistry("")))
	cfg := eng.longOptionMatchConfig()

	if eng.canApplyLongOptionReplacement("docker", nil, nil, 0, "", "", cfg) {
		t.Fatalf("empty replacement should not apply")
	}
	if (&Engine{}).canApplyLongOptionReplacement("docker", nil, nil, 0, "--context", "", cfg) {
		t.Fatalf("nil tool tree should not apply")
	}
	if !eng.canApplyLongOptionReplacement("docker", nil, []string{"--contex=prod"}, 0, "--context", "=prod", cfg) {
		t.Fatalf("inline suffix should apply")
	}
	if eng.canApplyLongOptionReplacement("docker", nil, []string{"--contex"}, 0, "--context", "", cfg) {
		t.Fatalf("missing separate value should not apply")
	}
	if eng.canApplyLongOptionReplacement("docker", nil, []string{"--contex", "--"}, 0, "--context", "", cfg) {
		t.Fatalf("double dash should not be consumed as value")
	}
	if eng.canApplyLongOptionReplacement("docker", nil, []string{"--contex", "-x"}, 0, "--context", "", cfg) {
		t.Fatalf("next option should not be consumed as value")
	}
	if eng.canApplyLongOptionReplacement("docker", nil, []string{"--contex", "run"}, 0, "--context", "", cfg) {
		t.Fatalf("single following subcommand candidate should not be consumed as value")
	}
	if !eng.canApplyLongOptionReplacement("docker", nil, []string{"--contex", "prod", "run"}, 0, "--context", "", cfg) {
		t.Fatalf("plain value before subcommand should be accepted")
	}
}

func TestSubcommandOptionBehaviorBranches(t *testing.T) {
	cfg := (NewEngine()).distanceMatchConfig()
	subcommands := []string{"run", "build"}

	tests := []struct {
		name        string
		root        string
		option      string
		subcommands []string
		next        string
		afterNext   string
		handled     bool
		needsValue  bool
	}{
		{name: "double dash", root: "docker", option: "--", subcommands: subcommands},
		{name: "inline long option value", root: "docker", option: "--context=prod", subcommands: subcommands, handled: true},
		{name: "known long option value", root: "docker", option: "--context", subcommands: subcommands, next: "prod", afterNext: "run", handled: true, needsValue: true},
		{name: "bare dash", root: "docker", option: "-", subcommands: subcommands, next: "prod", afterNext: "run"},
		{name: "known short option value", root: "git", option: "-C", subcommands: []string{"status"}, next: "repo", afterNext: "status", handled: true, needsValue: true},
		{name: "unknown short option before subcommand", root: "docker", option: "-x", subcommands: subcommands, next: "prod", afterNext: "run", handled: true, needsValue: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handled, needsValue := subcommandOptionBehavior(tt.root, tt.option, tt.subcommands, tt.next, tt.afterNext, cfg)
			if handled != tt.handled || needsValue != tt.needsValue {
				t.Fatalf("subcommandOptionBehavior = %v, %v", handled, needsValue)
			}
		})
	}
}

func TestShouldTreatNextTokenAsOptionValueBranches(t *testing.T) {
	cfg := (NewEngine()).distanceMatchConfig()
	subcommands := []string{"run", "build"}

	tests := []struct {
		name      string
		next      string
		afterNext string
		want      bool
	}{
		{name: "empty next token", next: "", afterNext: "run"},
		{name: "double dash next token", next: "--", afterNext: "run"},
		{name: "subcommand candidate", next: "run", afterNext: "build"},
		{name: "plain token before subcommand", next: "prod", afterNext: "run", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldTreatNextTokenAsOptionValue(tt.next, tt.afterNext, subcommands, cfg); got != tt.want {
				t.Fatalf("shouldTreatNextTokenAsOptionValue = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAutoLearnNonPromotionReasons(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		eng := NewEngine(WithAutoLearnThreshold(0))
		got := eng.maybeAutoLearnFromHistory(context.Background(), "from", "to")
		if got.Reason != "auto-learn disabled" {
			t.Fatalf("disabled reason = %+v", got)
		}
		if autoLearnEnabled(nil) {
			t.Fatalf("nil engine should not enable auto-learn")
		}
	})

	t.Run("context fallback and empty pair", func(t *testing.T) {
		eng := NewEngine(WithAutoLearnThreshold(1))
		ctx := context.Background()
		if autoLearnContext(ctx) != ctx {
			t.Fatalf("existing context should be preserved")
		}
		got := eng.maybeAutoLearnFromHistory(ctx, " ", "to")
		if got.Reason != "empty correction pair" {
			t.Fatalf("empty pair reason = %+v", got)
		}
	})

	t.Run("history miss target mismatch and already applied", func(t *testing.T) {
		tmpDir := t.TempDir()
		history := NewHistory(tmpDir)
		eng := NewEngine(
			WithHistory(history),
			WithRules(NewRules(tmpDir)),
			WithAutoLearnThreshold(3),
		)

		if got := eng.maybeAutoLearnFromHistory(context.Background(), "from", "to"); got.Reason != "history pair not found" {
			t.Fatalf("missing history reason = %+v", got)
		}
		if err := history.Record("from", "other"); err != nil {
			t.Fatalf("record history: %v", err)
		}
		if got := eng.maybeAutoLearnFromHistory(context.Background(), "from", "to"); got.Reason != "history pair points to a different target" {
			t.Fatalf("target mismatch reason = %+v", got)
		}
		if _, err := history.MarkRuleApplied("from", "other"); err != nil {
			t.Fatalf("mark rule applied: %v", err)
		}
		if got := eng.maybeAutoLearnFromHistory(context.Background(), "from", "other"); got.Reason != "history pair already marked as rule-applied" {
			t.Fatalf("already applied reason = %+v", got)
		}
	})

	t.Run("existing rule elsewhere and threshold not reached", func(t *testing.T) {
		tmpDir := t.TempDir()
		history := NewHistory(tmpDir)
		rules := NewRules(tmpDir)
		eng := NewEngine(
			WithHistory(history),
			WithRules(rules),
			WithAutoLearnThreshold(3),
		)
		if err := history.Record("from", "to"); err != nil {
			t.Fatalf("record history: %v", err)
		}
		if err := rules.AddUserRule(itypes.Rule{From: "from", To: "elsewhere"}); err != nil {
			t.Fatalf("add rule: %v", err)
		}
		if got := eng.maybeAutoLearnFromHistory(context.Background(), "from", "to"); got.Reason != "existing user rule points elsewhere" {
			t.Fatalf("existing rule reason = %+v", got)
		}

		rules = NewRules(t.TempDir())
		eng = NewEngine(
			WithHistory(history),
			WithRules(rules),
			WithAutoLearnThreshold(3),
		)
		if got := eng.maybeAutoLearnFromHistory(context.Background(), "from", "to"); got.Reason != "threshold not reached" {
			t.Fatalf("threshold reason = %+v", got)
		}
	})
}

func TestEnvContextNamesBranches(t *testing.T) {
	entries := []itypes.AliasContextEntry{
		{Kind: "alias", Name: "g", Expansion: "git"},
		{Kind: "env", Name: " PATH "},
		{Kind: "env", Expansion: " HOME "},
		{Kind: "env", Name: "PATH"},
		{Kind: "env"},
	}
	if got := envContextNames(entries); len(got) != 2 || got[0] != "HOME" || got[1] != "PATH" {
		t.Fatalf("envContextNames = %#v", got)
	}
}

func TestCollectEnvVarWordReplacementsRejectsNilLine(t *testing.T) {
	if got := collectEnvVarWordReplacements(nil, "$PAHT", []string{"PATH"}, (NewEngine()).envDistanceMatchConfig()); got != nil {
		t.Fatalf("nil word replacements = %#v", got)
	}
}

func TestIsSimpleEnvNameBranches(t *testing.T) {
	if isSimpleEnvName("") || isSimpleEnvName("1PATH") || isSimpleEnvName("PA-TH") {
		t.Fatalf("invalid env names should be rejected")
	}
	for _, name := range []string{"PATH", "_PATH", "path_1"} {
		if !isSimpleEnvName(name) {
			t.Fatalf("valid env name %q was rejected", name)
		}
	}
}

func TestClosestEnvVarBranches(t *testing.T) {
	cfg := (NewEngine()).envDistanceMatchConfig()
	if got, _, unique := closestEnvVar("PAHT", []string{"PATH"}, cfg); !unique || got != "PATH" {
		t.Fatalf("closestEnvVar transposition = %q unique=%v", got, unique)
	}
	if got, _, unique := closestEnvVar("PAHT", []string{"PATH", "PHAT"}, cfg); unique || got != "" {
		t.Fatalf("conflicting closest env var = %q unique=%v", got, unique)
	}
	if got, distance, unique := closestEnvVar("PATH", []string{"zzzz"}, cfg); unique || got != "" || distance == 999 {
		t.Fatalf("bad closest env var = %q distance=%d unique=%v", got, distance, unique)
	}
	if got, distance, unique := closestEnvVar("PATH", nil, cfg); unique || got != "" || distance != 999 {
		t.Fatalf("empty closest env var = %q distance=%d unique=%v", got, distance, unique)
	}
}

func TestEnvCandidatesConflictBranches(t *testing.T) {
	if !envCandidatesConflict(
		envVarCandidate{name: "A", distance: 1, similarity: 0.5, lengthDelta: 0},
		envVarCandidate{name: "B", distance: 1, similarity: 0.5, lengthDelta: 0},
	) {
		t.Fatalf("matching env candidates should conflict")
	}
}

func testAliasContextEntries() []itypes.AliasContextEntry {
	return []itypes.AliasContextEntry{
		{Kind: "env", Name: "PATH", Expansion: "PATH"},
		{Kind: "alias", Name: " ", Expansion: "git"},
		{Kind: "alias", Name: "bad", Expansion: "git > out"},
		{Kind: "alias", Name: "g", Expansion: "git"},
		{Kind: "abbr", Name: "gst", Expansion: "g status"},
		{Kind: "function", Name: "loop", Expansion: "loop"},
		{Kind: "alias", Name: "self", Expansion: "self status"},
		{Kind: "alias", Name: "a", Expansion: "b"},
		{Kind: "alias", Name: "b", Expansion: "a"},
	}
}

func TestAliasResolverBranches(t *testing.T) {
	resolver := newAliasResolver(testAliasContextEntries())
	if resolver == nil {
		t.Fatalf("expected alias resolver")
	}
	if _, _, ok := resolver.resolve("missing"); ok {
		t.Fatalf("missing alias should not resolve")
	}
	if expansion, tokens, ok := resolver.resolve("gst"); !ok || expansion != "git status" || len(tokens) != 2 {
		t.Fatalf("chained alias resolution = %q %#v %v", expansion, tokens, ok)
	}
	if expansion, tokens, ok := resolver.resolve("self"); !ok || expansion != "self status" || len(tokens) != 2 {
		t.Fatalf("self alias with payload should resolve = %q %#v %v", expansion, tokens, ok)
	}
	if _, _, ok := resolver.resolve("loop"); ok {
		t.Fatalf("single-token alias loop should fail")
	}
	if _, _, ok := resolver.resolve("a"); ok {
		t.Fatalf("mutual alias loop should fail")
	}

	if newAliasResolver(nil) != nil {
		t.Fatalf("empty resolver should be nil")
	}
	if newAliasResolver([]itypes.AliasContextEntry{{Kind: "env", Name: "PATH", Expansion: "PATH"}}) != nil {
		t.Fatalf("resolver with no usable entries should be nil")
	}
}

func TestExpandCommandAliasesBranches(t *testing.T) {
	entries := testAliasContextEntries()
	if got, records, ok := expandCommandAliases("g status", nil); ok || got != "g status" || records != nil {
		t.Fatalf("expand without resolver = %q %#v %v", got, records, ok)
	}
	if got, _, ok := expandCommandAliases("'", entries); ok || got != "'" {
		t.Fatalf("parse error expansion should fail, got %q ok=%v", got, ok)
	}
	if got, _, ok := expandCommandAliases("git status", entries); ok || got != "git status" {
		t.Fatalf("no alias expansion should fail, got %q ok=%v", got, ok)
	}
	if got, records, ok := expandCommandAliases("gst", entries); !ok || got != "git status" || len(records) != 1 {
		t.Fatalf("alias expansion = %q %#v %v", got, records, ok)
	}
}

func TestSplitAliasExpansionBranches(t *testing.T) {
	if normalized, tokens := splitAliasExpansion("git status"); normalized != "git status" || len(tokens) != 2 {
		t.Fatalf("splitAliasExpansion = %q %#v", normalized, tokens)
	}
	if normalized, tokens := splitAliasExpansion("git > out"); normalized != "" || tokens != nil {
		t.Fatalf("redirection alias should be rejected: %q %#v", normalized, tokens)
	}
	if normalized, tokens := splitAliasExpansion(""); normalized != "" || tokens != nil {
		t.Fatalf("empty alias should be rejected: %q %#v", normalized, tokens)
	}
}

func TestRewriteCommandAliasesBranches(t *testing.T) {
	if got := rewriteCommandAliases("git status", nil); got != "git status" {
		t.Fatalf("rewrite without records = %q", got)
	}
	if got := rewriteCommandAliases("'", []aliasRewriteRecord{{alias: "g", tokens: []string{"git"}}}); got != "'" {
		t.Fatalf("rewrite parse error = %q", got)
	}
	records := []aliasRewriteRecord{
		{lineOrdinal: 9, alias: "late", tokens: []string{"git"}, canRealias: true},
		{lineOrdinal: 0, alias: "nope", tokens: []string{"git"}, canRealias: false},
		{lineOrdinal: 0, alias: "g", tokens: []string{"git", "status"}, canRealias: true},
	}
	if got := rewriteCommandAliases("git status --short", records); got != "g --short" {
		t.Fatalf("rewriteCommandAliases = %q", got)
	}
}

func TestLineHasAliasExpansionPrefixBranches(t *testing.T) {
	lines, err := parseShellCommandLines("git status")
	if err != nil {
		t.Fatalf("parse shell line: %v", err)
	}
	if lineHasAliasExpansionPrefix(nil, []string{"git"}) {
		t.Fatalf("nil line should not match alias prefix")
	}
	if lineHasAliasExpansionPrefix(lines[0], nil) {
		t.Fatalf("empty tokens should not match alias prefix")
	}
	if lineHasAliasExpansionPrefix(lines[0], []string{"git", "status", "--short"}) {
		t.Fatalf("too-long alias prefix should not match")
	}
	if lineHasAliasExpansionPrefix(lines[0], []string{"docker"}) {
		t.Fatalf("mismatched alias prefix should not match")
	}
}

func TestApplyRawRangeReplacementsIgnoresInvalidRanges(t *testing.T) {
	if got := applyRawRangeReplacements("abcdef", []rawRangeReplacement{
		{start: -1, end: 1, value: "x"},
		{start: 2, end: 99, value: "x"},
		{start: 4, end: 3, value: "x"},
	}); got != "abcdef" {
		t.Fatalf("invalid replacements should be ignored, got %q", got)
	}
}
