package commands

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTreeNodeCloneAndCachesHandleNilAndUncachedNodes(t *testing.T) {
	var nilNode *TreeNode
	if nilNode.clone() != nil {
		t.Fatalf("nil clone should return nil")
	}
	if nilNode.childTokens() != nil {
		t.Fatalf("nil childTokens should return nil")
	}
	nilNode.refreshChildTokens()
	if nilNode.longOptions() != nil {
		t.Fatalf("nil longOptions should return nil")
	}
	if nilNode.hasLongOption("--help") {
		t.Fatalf("nil hasLongOption should be false")
	}
	if nilNode.longOptionTakesValue("--file") {
		t.Fatalf("nil longOptionTakesValue should be false")
	}

	node := &TreeNode{
		Children: map[string]*TreeNode{
			"z": {},
			"a": {},
		},
		LongOptions:           []string{" --verbose ", "bad", "--", "--verbose", "--all"},
		LongOptionsWithValues: []string{"--file", "", "file"},
	}

	if got := node.childTokens(); !reflect.DeepEqual(got, []string{"a", "z"}) {
		t.Fatalf("uncached childTokens = %#v", got)
	}
	if got := node.longOptions(); !reflect.DeepEqual(got, []string{"--all", "--verbose"}) {
		t.Fatalf("normalized long options = %#v", got)
	}
	if !node.hasLongOption("--verbose") || node.hasLongOption("") || node.hasLongOption("--missing") {
		t.Fatalf("unexpected long option lookup result")
	}
	if !node.longOptionTakesValue("--file") || node.longOptionTakesValue("") || node.longOptionTakesValue("--missing") {
		t.Fatalf("unexpected long option value lookup result")
	}

	cloned := node.clone()
	cloned.Children["a"].Alias = "changed"
	if node.Children["a"].Alias == "changed" {
		t.Fatalf("clone should deep-copy children")
	}
}

func TestToolTreeRegistrySmallHelpers(t *testing.T) {
	var nilRegistry *ToolTreeRegistry
	if nilRegistry.expiry() != 7*24*time.Hour {
		t.Fatalf("nil registry should use default expiry")
	}
	if nilRegistry.cachedChildren("git", nil) != nil {
		t.Fatalf("nil cachedChildren should return nil")
	}
	if nilRegistry.cachedPathNodesLocked("git", nil) != nil {
		t.Fatalf("nil cachedPathNodesLocked should return nil")
	}
	if nilRegistry.cachedNodeLocked("git", nil) != nil {
		t.Fatalf("nil cachedNodeLocked should return nil")
	}

	r := &ToolTreeRegistry{cacheExpiry: time.Hour}
	r.ensureTrees()
	if r.trees == nil {
		t.Fatalf("ensureTrees should initialize the map")
	}
	if r.expiry() != time.Hour {
		t.Fatalf("custom expiry not returned")
	}

	if got := mergeChildTokens(nil, treeBranch(map[string]*TreeNode{"b": {}, "a": {}})); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("mergeChildTokens fallback only = %#v", got)
	}
	if got := mergeChildTokens(treeBranch(map[string]*TreeNode{"a": {}}), nil); !reflect.DeepEqual(got, []string{"a"}) {
		t.Fatalf("mergeChildTokens primary only = %#v", got)
	}
}

func TestResolveChildUsesCachedAliasThenBuiltinFallback(t *testing.T) {
	r := &ToolTreeRegistry{
		trees: map[string]*ToolTreeCache{
			"custom": {
				Tool: "custom",
				Tree: treeBranch(map[string]*TreeNode{
					"short": {Alias: "canonical"},
				}),
				UpdatedAt: time.Now(),
			},
		},
		cacheExpiry: time.Hour,
	}

	if got, ok := r.ResolveChild("", nil, "x"); ok || got != "" {
		t.Fatalf("empty tool should not resolve")
	}
	if got, ok := r.ResolveChild("custom", nil, "short"); !ok || got != "canonical" {
		t.Fatalf("cached alias resolution = %q, %v", got, ok)
	}
	if got, ok := r.ResolveChild("git", nil, "status"); !ok || got != "status" {
		t.Fatalf("builtin fallback resolution = %q, %v", got, ok)
	}
	if got, ok := resolveTreeChild(nil, "status"); ok || got != "" {
		t.Fatalf("nil resolveTreeChild should fail")
	}
	if got, ok := resolveTreeChild(treeBranch(map[string]*TreeNode{"plain": nil}), "plain"); !ok || got != "plain" {
		t.Fatalf("nil child should resolve to token, got %q, %v", got, ok)
	}
}

func TestCachedNodesRespectExpiryAndMissingPaths(t *testing.T) {
	r := &ToolTreeRegistry{
		trees: map[string]*ToolTreeCache{
			"git": {
				Tool: "git",
				Tree: treeBranch(map[string]*TreeNode{
					"commit": treeBranch(map[string]*TreeNode{"amend": {}}),
				}),
				UpdatedAt: time.Now().Add(-2 * time.Hour),
			},
		},
		cacheExpiry: time.Hour,
	}
	if r.cachedNodeLocked("git", []string{"commit"}) != nil {
		t.Fatalf("expired cache should not return nodes")
	}
	if r.cachedPathNodesLocked("git", []string{"commit"}) != nil {
		t.Fatalf("expired cache should not return path nodes")
	}

	r.trees["git"].UpdatedAt = time.Now()
	if r.cachedNodeLocked("git", []string{"missing"}) != nil {
		t.Fatalf("missing path should not return a node")
	}
	if got := collectTreePathNodes(r.trees["git"].Tree, []string{"missing", "child"}); len(got) != 1 {
		t.Fatalf("missing path should stop at root, got %d nodes", len(got))
	}
	if got := collectTreePathNodes(nil, nil); got != nil {
		t.Fatalf("nil root should return nil")
	}
}

func TestStoreChildrenInitializesCachesAndSkipsDuplicates(t *testing.T) {
	r := &ToolTreeRegistry{cacheDir: t.TempDir()}
	r.storeChildren("", nil, []string{"x"})
	if len(r.trees) != 0 {
		t.Fatalf("empty tool should not store children")
	}
	r.storeChildren("custom", []string{"parent"}, []string{"child", "", "child"})

	node := r.cachedNodeLocked("custom", []string{"parent"})
	if node == nil || !reflect.DeepEqual(node.childTokens(), []string{"child"}) {
		t.Fatalf("stored node children = %#v", node)
	}

	cacheFile := filepath.Join(r.cacheDir, "subcommands.json")
	if _, err := os.Stat(cacheFile); err != nil {
		t.Fatalf("storeChildren should save cache: %v", err)
	}

	root := &TreeNode{}
	ensured := ensureTreePath(root, "git", []string{"commit"})
	if ensured == nil || root.Children["commit"] == nil {
		t.Fatalf("ensureTreePath should create builtin-backed child")
	}
	ensured = ensureTreePath(root, "custom", []string{"unknown"})
	if ensured == nil || root.Children["unknown"] == nil {
		t.Fatalf("ensureTreePath should create empty child")
	}
}

func TestLongOptionScopeCoversEmptyToolCachedAndBuiltinMatches(t *testing.T) {
	r := &ToolTreeRegistry{
		trees: map[string]*ToolTreeCache{
			"custom": {
				Tool: "custom",
				Tree: withLongOptions(treeBranch(map[string]*TreeNode{
					"run": withLongOptions(&TreeNode{}, []string{"--child"}, []string{"--child"}),
				}), []string{"--root"}, []string{"--root"}),
				UpdatedAt: time.Now(),
			},
		},
		cacheExpiry: time.Hour,
	}
	r.trees["custom"].Tree.refreshChildTokens()

	if r.LongOptionsInScope("", nil) != nil {
		t.Fatalf("empty tool should have no long options")
	}
	if got := r.LongOptionsInScope("custom", []string{"run"}); !reflect.DeepEqual(got, []string{"--root", "--child"}) {
		t.Fatalf("cached long options = %#v", got)
	}
	if !r.HasLongOptionInScope("custom", []string{"run"}, "--child") {
		t.Fatalf("expected cached child option")
	}
	if !r.LongOptionTakesValue("custom", nil, "--root") {
		t.Fatalf("expected cached root option with value")
	}
	if r.anyLongOptionInScope("", nil, func(*TreeNode) bool { return true }) {
		t.Fatalf("empty tool should not match")
	}
	if r.anyLongOptionInScope("custom", nil, nil) {
		t.Fatalf("nil matcher should not match")
	}
}

func TestPathStopsAtTerminalAndGetChildrenEdges(t *testing.T) {
	r := &ToolTreeRegistry{
		trees: map[string]*ToolTreeCache{
			"custom": {
				Tool: "custom",
				Tree: treeBranch(map[string]*TreeNode{
					"done": {Terminal: true},
				}),
				UpdatedAt: time.Now(),
			},
		},
		cacheExpiry: time.Hour,
	}

	if got := r.GetChildren("", nil); got != nil {
		t.Fatalf("empty tool children = %#v", got)
	}
	if !r.pathStopsAtTerminal("custom", []string{"done"}) {
		t.Fatalf("cached terminal path should stop")
	}
	if got := r.GetChildren("custom", []string{"done"}); got != nil {
		t.Fatalf("terminal children should be nil, got %#v", got)
	}
	if !((&ToolTreeRegistry{}).pathStopsAtTerminal("docker", []string{"run"})) {
		t.Fatalf("builtin terminal path should stop")
	}
}

func TestParseToolHelpDispatchesRemainingFormats(t *testing.T) {
	tests := []struct {
		name   string
		tool   string
		prefix []string
		help   string
		want   []string
	}{
		{
			name:   "git nested grouped usage",
			tool:   "git",
			prefix: []string{"remote"},
			help:   "usage: git remote (add|remove)\nusage: git remote rename old new\n",
			want:   []string{"add", "remove", "rename"},
		},
		{
			name: "yarn",
			tool: "yarn",
			help: "Commands:\n  add       Installs a package\n  init      Creates package.json\n",
			want: []string{"add", "init"},
		},
		{
			name: "brew",
			tool: "brew",
			help: "==> Built-in commands\n--cache\ninstall\nlist\nbad_command\n",
			want: []string{"install", "list"},
		},
		{
			name: "aws",
			tool: "aws",
			help: "SERVICES\n  ec2       Compute\n  s3        Storage\nOTHER\n  ignored   nope\n",
			want: []string{"ec2", "s3"},
		},
		{
			name: "gcloud",
			tool: "gcloud",
			help: "GROUPS\n  compute   Compute Engine\nCOMMANDS\n  auth      Authenticate\n",
			want: []string{"compute", "auth"},
		},
		{
			name: "az",
			tool: "az",
			help: "SUBGROUPS\n  account   Manage accounts\nCOMMANDS\n  login     Log in\n",
			want: []string{"account", "login"},
		},
		{
			name: "generic",
			tool: "unknown",
			help: "Commands:\n  start  Start service\n  stop   Stop service\n",
			want: []string{"start", "stop"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseToolHelp(tt.tool, tt.prefix, tt.help); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseToolHelp = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseGoHelpStopsAtNonCommandAfterCommands(t *testing.T) {
	help := "The commands are:\n    build       compile\n    test        test packages\nnot a command section\n    ignored     ignored\n"
	if got := parseGoHelp(help); !reflect.DeepEqual(got, []string{"build", "test"}) {
		t.Fatalf("parseGoHelp = %#v", got)
	}
}

func TestParseSectionedHelpSkipsDuplicatesAndStopsAtNewHeader(t *testing.T) {
	help := "COMMANDS\n  start    Start\n  start    Duplicate\nNEXT\n  ignored  Ignored\nCOMMANDS:\n  stop     Stop\n"
	if got := parseSectionedHelp(help, "COMMANDS"); !reflect.DeepEqual(got, []string{"start", "stop"}) {
		t.Fatalf("parseSectionedHelp = %#v", got)
	}
	if normalizeHelpSection("commands:") != "COMMANDS" {
		t.Fatalf("normalizeHelpSection did not normalize suffix and case")
	}
}

func TestNestedHelpOutputRejectsUnsupportedDepths(t *testing.T) {
	r := &ToolTreeRegistry{}
	tests := []struct {
		tool   string
		prefix []string
	}{
		{tool: "git", prefix: nil},
		{tool: "git", prefix: []string{"a", "b", "c", "d"}},
		{tool: "npm", prefix: []string{"run"}},
		{tool: "git", prefix: []string{"remote", "add"}},
		{tool: "docker", prefix: []string{"image", "ls"}},
	}
	for _, tt := range tests {
		t.Run(tt.tool+"/"+strings.Join(tt.prefix, "/"), func(t *testing.T) {
			out, err := r.getNestedHelpOutput(tt.tool, tt.prefix)
			if err != nil || out != "" {
				t.Fatalf("getNestedHelpOutput = %q, %v", out, err)
			}
		})
	}
	if !supportsHierarchicalDiscovery("az") || supportsHierarchicalDiscovery("npm") {
		t.Fatalf("unexpected hierarchical discovery support")
	}
}

func TestRunHelpCommandReportsStartError(t *testing.T) {
	r := &ToolTreeRegistry{helpTimeout: time.Millisecond}
	if _, err := r.runHelpCommand(filepath.Join(t.TempDir(), "missing-tool"), "--help"); err == nil {
		t.Fatalf("expected missing command start error")
	}
}
