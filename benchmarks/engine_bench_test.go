package benchmarks

import (
	"testing"

	"github.com/shown/typo/internal/engine"
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
	commands := []string{"git", "docker", "npm", "go", "python", "kubectl", "make", "ls", "cd", "grep"}

	b.ResetTimer()

	b.Run("exact-match-rule", func(b *testing.B) {
		eng := engine.NewEngine(
			engine.WithKeyboard(keyboard),
			engine.WithRules(rules),
			engine.WithHistory(history),
			engine.WithCommands(commands),
		)
		for i := 0; i < b.N; i++ {
			eng.Fix("gut status", "")
		}
	})

	b.Run("distance-match", func(b *testing.B) {
		eng := engine.NewEngine(
			engine.WithKeyboard(keyboard),
			engine.WithRules(rules),
			engine.WithHistory(history),
			engine.WithCommands(commands),
		)
		for i := 0; i < b.N; i++ {
			eng.Fix("gti status", "")
		}
	})

	b.Run("no-match", func(b *testing.B) {
		eng := engine.NewEngine(
			engine.WithKeyboard(keyboard),
			engine.WithRules(rules),
			engine.WithHistory(history),
			engine.WithCommands(commands),
		)
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
