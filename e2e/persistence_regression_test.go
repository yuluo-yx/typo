package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/yuluo-yx/typo/internal/engine"
)

func TestE2EConcurrentPersistentWriters(t *testing.T) {
	const writers = 24
	env := newE2EEnv(t)
	if result := env.run(t, "config", "set", "auto-learn-threshold", "1000"); result.code != 0 {
		t.Fatalf("configure auto-learn: %+v", result)
	}
	runConcurrentCLI(t, env, writers, func(i int) []string {
		return []string{"learn", fmt.Sprintf("auditword%d", i), "git status"}
	})
	rules := engine.NewRules(env.configDir())
	for i := range writers {
		if _, ok := rules.MatchUser(fmt.Sprintf("auditword%d", i)); !ok {
			t.Fatalf("successful writer %d lost its rule", i)
		}
	}
	runConcurrentCLI(t, env, writers, func(int) []string {
		return []string{"fix", "gti status"}
	})
	entry, ok := engine.NewHistory(env.configDir()).Lookup("gti status")
	if !ok || entry.Count != writers {
		t.Fatalf("concurrent history count = %+v, want %d", entry, writers)
	}
	if result := env.run(t, "config", "set", "auto-learn-threshold", "3"); result.code != 0 {
		t.Fatalf("configure auto-learn: %+v", result)
	}
	runConcurrentCLI(t, env, writers, func(int) []string {
		return []string{"fix", "gti status"}
	})
	entry, _ = engine.NewHistory(env.configDir()).Lookup("gti status")
	if !entry.RuleApplied {
		t.Fatalf("concurrent auto-learn did not freeze history: %+v", entry)
	}
	if rule, ok := engine.NewRules(env.configDir()).MatchUser("gti status"); !ok || rule.To != "git status" {
		t.Fatalf("concurrent auto-learn lost its rule: %+v", rule)
	}
}

func runConcurrentCLI(t *testing.T, env *e2eEnv, count int, args func(int) []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	start := make(chan struct{})
	failures := make(chan error, count)
	var workers sync.WaitGroup
	for i := range count {
		workers.Go(func() {
			<-start
			cmd := exec.CommandContext(ctx, env.bin, args(i)...)
			cmd.Dir = env.root
			cmd.Env = env.commandEnv()
			if output, err := cmd.CombinedOutput(); err != nil {
				failures <- fmt.Errorf("writer %d: %w: %s", i, err, output)
			}
		})
	}
	close(start)
	workers.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
}
