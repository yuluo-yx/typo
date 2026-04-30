package benchmarks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yuluo-yx/typo/internal/commands"
	"github.com/yuluo-yx/typo/internal/engine"
	"github.com/yuluo-yx/typo/internal/parser"
)

func BenchmarkDistance(b *testing.B) {
	keyboard := engine.DefaultKeyboard

	tests := []struct {
		name string
		a, b string
	}{
		{"short-same", "git", "git"},
		{"short-close", "gut", "git"},
		{"short-far", "abc", "xyz"},
		{"medium-same", "docker", "docker"},
		{"medium-close", "dcoker", "docker"},
		{"long-same", "kubernetes", "kubernetes"},
		{"long-close", "kubernete", "kubernetes"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				engine.Distance(tt.a, tt.b, keyboard)
			}
		})
	}
}

func BenchmarkSimilarity(b *testing.B) {
	keyboard := engine.DefaultKeyboard

	tests := []struct {
		name string
		a, b string
	}{
		{"short", "gut", "git"},
		{"medium", "dcoker", "docker"},
		{"long", "kubernete", "kubernetes"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				engine.Similarity(tt.a, tt.b, keyboard)
			}
		})
	}
}

func BenchmarkEngine_Fix(b *testing.B) {
	keyboard := engine.DefaultKeyboard
	rules := engine.NewRules("")
	history := engine.NewHistory("")
	commandList := []string{"git", "docker", "npm", "go", "python", "kubectl", "make", "ls", "cd", "grep", "sudo", "env", "echo", "true"}
	parserRegistry := parser.NewRegistry()
	subcommands := newBenchmarkToolTreeRegistry(b, map[string][]string{
		"git":     {"status", "remote", "commit", "checkout", "branch", "pull", "push"},
		"docker":  {"build", "ps", "images", "run", "logs", "compose"},
		"npm":     {"install", "list", "run", "test", "ci"},
		"kubectl": {"apply", "describe", "get", "logs"},
	})
	gitStderr := "git: 'remove' is not a git command.\n\nThe most similar command is\n\tremote\n"

	b.Run("exact-match-rule", func(b *testing.B) {
		eng := engine.NewEngine(
			engine.WithKeyboard(keyboard),
			engine.WithRules(rules),
			engine.WithHistory(history),
			engine.WithCommands(commandList),
			engine.WithParser(parserRegistry),
			engine.WithToolTrees(subcommands),
		)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			eng.Fix("gut status", "")
		}
	})

	b.Run("distance-match", func(b *testing.B) {
		eng := engine.NewEngine(
			engine.WithKeyboard(keyboard),
			engine.WithRules(rules),
			engine.WithHistory(history),
			engine.WithCommands(commandList),
			engine.WithParser(parserRegistry),
			engine.WithToolTrees(subcommands),
		)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			eng.Fix("gti status", "")
		}
	})

	b.Run("compound-multi-match", func(b *testing.B) {
		eng := engine.NewEngine(
			engine.WithKeyboard(keyboard),
			engine.WithRules(rules),
			engine.WithHistory(history),
			engine.WithCommands(commandList),
			engine.WithParser(parserRegistry),
			engine.WithToolTrees(subcommands),
		)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			eng.Fix("gti stauts && dcoker ps", "")
		}
	})

	b.Run("pipeline-compound-match", func(b *testing.B) {
		eng := engine.NewEngine(
			engine.WithKeyboard(keyboard),
			engine.WithRules(rules),
			engine.WithHistory(history),
			engine.WithCommands(commandList),
			engine.WithParser(parserRegistry),
			engine.WithToolTrees(subcommands),
		)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			eng.Fix("gut status | grep main && dcoker ps", "")
		}
	})

	b.Run("wrapper-subcommand-match", func(b *testing.B) {
		eng := engine.NewEngine(
			engine.WithKeyboard(keyboard),
			engine.WithRules(rules),
			engine.WithHistory(history),
			engine.WithCommands(commandList),
			engine.WithParser(parserRegistry),
			engine.WithToolTrees(subcommands),
		)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			eng.Fix("sudo git -C repo stauts || dcoker ps", "")
		}
	})

	b.Run("parser-compound-match", func(b *testing.B) {
		eng := engine.NewEngine(
			engine.WithKeyboard(keyboard),
			engine.WithRules(rules),
			engine.WithHistory(history),
			engine.WithCommands(commandList),
			engine.WithParser(parserRegistry),
			engine.WithToolTrees(subcommands),
		)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			eng.Fix("sudo git remove -v && dcoker ps", gitStderr)
		}
	})

	b.Run("no-match", func(b *testing.B) {
		eng := engine.NewEngine(
			engine.WithKeyboard(keyboard),
			engine.WithRules(rules),
			engine.WithHistory(history),
			engine.WithCommands(commandList),
			engine.WithParser(parserRegistry),
			engine.WithToolTrees(subcommands),
		)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			eng.Fix("xyzabc", "")
		}
	})
}

func BenchmarkKeyboard_IsAdjacent(b *testing.B) {
	kb := engine.DefaultKeyboard

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kb.IsAdjacent('a', 's')
		kb.IsAdjacent('g', 'h')
		kb.IsAdjacent('a', 'z')
	}
}

func BenchmarkRules_Match(b *testing.B) {
	rules := engine.NewRules("")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rules.Match("gut")
		rules.Match("dcoker")
		rules.Match("git")
	}
}

func BenchmarkHistory_Lookup(b *testing.B) {
	history := engine.NewHistory("")
	_ = history.Record("gut", "git")
	_ = history.Record("dcoker", "docker")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		history.Lookup("gut")
		history.Lookup("dcoker")
		history.Lookup("unknown")
	}
}

func BenchmarkEngine_Fix_WithCommands(b *testing.B) {
	commands := []string{
		"git", "docker", "npm", "node", "go", "python", "pip", "cargo", "rustc",
		"make", "cmake", "gcc", "clang", "java", "javac", "mvn", "gradle",
		"kubectl", "helm", "terraform", "ansible", "vagrant",
		"ls", "cd", "grep", "find", "sed", "awk", "sort", "uniq", "wc", "head", "tail",
		"cat", "echo", "printf", "tee", "xargs", "parallel",
		"ssh", "scp", "rsync", "curl", "wget", "tar", "zip", "unzip",
		"vim", "nvim", "emacs", "code", "subl",
		"ps", "kill", "top", "htop", "lsof", "netstat", "ss",
	}

	eng := engine.NewEngine(
		engine.WithRules(engine.NewRules("")),
		engine.WithHistory(engine.NewHistory("")),
		engine.WithCommands(commands),
	)

	b.ResetTimer()

	b.Run("distance-short", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			eng.Fix("gt status", "")
		}
	})

	b.Run("distance-medium", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			eng.Fix("dkcer ps", "")
		}
	})

	largeCommands := syntheticCommandList(10000)
	largeCommands = append(largeCommands, "git", "docker", "kubectl")
	largeEng := engine.NewEngine(
		engine.WithRules(engine.NewRules("")),
		engine.WithHistory(engine.NewHistory("")),
		engine.WithCommands(largeCommands),
	)

	b.Run("distance-large-known-10k", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			largeEng.Fix("gti status", "")
		}
	})
}

func syntheticCommandList(count int) []string {
	commands := make([]string, 0, count)
	for i := 0; i < count; i++ {
		commands = append(commands, fmt.Sprintf("synthetic-command-%05d", i))
	}
	return commands
}

func newBenchmarkToolTreeRegistry(b *testing.B, tools map[string][]string) *commands.ToolTreeRegistry {
	b.Helper()

	cacheDir := b.TempDir()
	data, err := marshalBenchmarkToolTreeCache(tools, time.Now())
	if err != nil {
		b.Fatalf("failed to marshal benchmark subcommands: %v", err)
	}

	cacheFile := filepath.Join(cacheDir, "subcommands.json")
	if err := os.WriteFile(cacheFile, data, 0600); err != nil {
		b.Fatalf("failed to write benchmark subcommand cache: %v", err)
	}

	return commands.NewToolTreeRegistry(cacheDir)
}

func marshalBenchmarkToolTreeCache(tools map[string][]string, updatedAt time.Time) ([]byte, error) {
	wrapper := struct {
		SchemaVersion int                       `json:"schema_version"`
		Tools         []*commands.ToolTreeCache `json:"tools"`
	}{
		SchemaVersion: 2,
		Tools:         make([]*commands.ToolTreeCache, 0, len(tools)),
	}

	for tool, subcommands := range tools {
		children := make(map[string]*commands.TreeNode, len(subcommands))
		for _, subcommand := range subcommands {
			children[subcommand] = &commands.TreeNode{}
		}
		wrapper.Tools = append(wrapper.Tools, &commands.ToolTreeCache{
			Tool:      tool,
			Tree:      &commands.TreeNode{Children: children},
			UpdatedAt: updatedAt,
		})
	}

	return json.MarshalIndent(wrapper, "", "  ")
}
