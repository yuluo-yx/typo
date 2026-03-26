package benchmarks

import (
	"encoding/json"
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
	subcommands := newBenchmarkSubcommandRegistry(b, map[string][]string{
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
			engine.WithSubcommands(subcommands),
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
			engine.WithSubcommands(subcommands),
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
			engine.WithSubcommands(subcommands),
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
			engine.WithSubcommands(subcommands),
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
			engine.WithSubcommands(subcommands),
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
			engine.WithSubcommands(subcommands),
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
			engine.WithSubcommands(subcommands),
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
}

func newBenchmarkSubcommandRegistry(b *testing.B, tools map[string][]string) *commands.SubcommandRegistry {
	b.Helper()

	cacheDir := b.TempDir()
	caches := make([]commands.SubcommandCache, 0, len(tools))
	now := time.Now()

	for tool, subcommands := range tools {
		caches = append(caches, commands.SubcommandCache{
			Tool:        tool,
			Subcommands: subcommands,
			UpdatedAt:   now,
		})
	}

	data, err := json.MarshalIndent(caches, "", "  ")
	if err != nil {
		b.Fatalf("failed to marshal benchmark subcommands: %v", err)
	}

	cacheFile := filepath.Join(cacheDir, "subcommands.json")
	if err := os.WriteFile(cacheFile, data, 0600); err != nil {
		b.Fatalf("failed to write benchmark subcommand cache: %v", err)
	}

	return commands.NewSubcommandRegistry(cacheDir)
}
