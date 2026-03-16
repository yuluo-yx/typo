package benchmarks

import (
	"os/exec"
	"testing"
)

// BenchmarkTypoCLI 测试完整 CLI 的性能
func BenchmarkTypoCLI(b *testing.B) {
	b.Run("fix-rule-match", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cmd := exec.Command("typo", "fix", "gut status")
			_ = cmd.Run()
		}
	})

	b.Run("fix-distance-match", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cmd := exec.Command("typo", "fix", "dkcer ps")
			_ = cmd.Run()
		}
	})

	b.Run("fix-no-match", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cmd := exec.Command("typo", "fix", "xyzabc")
			_ = cmd.Run()
		}
	})

	b.Run("rules-list", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cmd := exec.Command("typo", "rules", "list")
			_ = cmd.Run()
		}
	})

	b.Run("version", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cmd := exec.Command("typo", "version")
			_ = cmd.Run()
		}
	})
}
