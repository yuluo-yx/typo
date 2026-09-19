package engine

import (
	"fmt"
	"sync"
	"testing"
)

func persistentTestEngine(dir string) *Engine {
	return NewEngine(WithRules(NewRules(dir)), WithHistory(NewHistory(dir)))
}

func TestPersistenceReloadsStaleSnapshots(t *testing.T) {
	dir := t.TempDir()
	first, stale := persistentTestEngine(dir), persistentTestEngine(dir)
	if err := first.Learn("first", "git status"); err != nil {
		t.Fatal(err)
	}
	if err := stale.Learn("second", "git branch"); err != nil {
		t.Fatal(err)
	}
	if _, ok := stale.rules.MatchUser("first"); !ok {
		t.Fatal("stale writer lost the first rule")
	}
	if err := first.RemoveRule("second"); err != nil {
		t.Fatalf("remove must reload a rule written by another instance: %v", err)
	}
	if err := stale.Learn("third", "git diff"); err != nil {
		t.Fatal(err)
	}
	if _, ok := NewRules(dir).MatchUser("second"); ok {
		t.Fatal("stale writer resurrected a removed rule")
	}
	if err := first.RecordHistory("gut", "git"); err != nil {
		t.Fatal(err)
	}
	if err := stale.ClearHistory(); err != nil {
		t.Fatal(err)
	}
	if err := first.RecordHistory("gti", "git"); err != nil {
		t.Fatal(err)
	}
	if got := NewHistory(dir).List(); len(got) != 1 || got[0].From != "gti" {
		t.Fatalf("cleared history was resurrected: %+v", got)
	}
}

func TestPersistenceConcurrentWritersAndAutoLearn(t *testing.T) {
	const writers = 24
	dir := t.TempDir()
	engines := make([]*Engine, writers)
	for i := range engines {
		// All snapshots predate every write, making lost updates reproducible.
		engines[i] = persistentTestEngine(dir)
	}
	start := make(chan struct{})
	errors := make(chan error, writers)
	var workers sync.WaitGroup
	for i, eng := range engines {
		workers.Go(func() {
			<-start
			if err := eng.Learn(fmt.Sprintf("audit%d", i), "git status"); err != nil {
				errors <- err
				return
			}
			errors <- eng.RecordHistory("gut status", "git status")
		})
	}
	close(start)
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	rules := NewRules(dir)
	for i := range writers {
		if _, ok := rules.MatchUser(fmt.Sprintf("audit%d", i)); !ok {
			t.Fatalf("missing rule %d", i)
		}
	}
	entry, ok := NewHistory(dir).Lookup("gut status")
	if !ok || entry.Count != writers {
		t.Fatalf("concurrent count = %+v, want %d", entry, writers)
	}
	assertAutoLearnReloadsState(t, dir, engines, writers)
}

func assertAutoLearnReloadsState(t *testing.T, dir string, engines []*Engine, count int) {
	t.Helper()
	// Promotion and subsequent recording must reload both stores and keep the count frozen.
	info := engines[0].MaybeAutoLearnFromHistory(t.Context(), "gut status", "git status")
	if !info.Persisted || info.Error != "" {
		t.Fatalf("auto-learn failed: %+v", info)
	}
	if err := engines[1].RecordHistory("gut status", "git status"); err != nil {
		t.Fatal(err)
	}
	entry, _ := NewHistory(dir).Lookup("gut status")
	if !entry.RuleApplied || entry.Count != count {
		t.Fatalf("promotion was lost by a stale writer: %+v", entry)
	}
	if err := engines[2].Learn("gut status", "git status --short"); err != nil {
		t.Fatal(err)
	}
	info = engines[3].MaybeAutoLearnFromHistory(t.Context(), "gut status", "git status")
	if info.Persisted {
		t.Fatalf("stale history must not override an explicit rule: %+v", info)
	}
	if rule, _ := NewRules(dir).MatchUser("gut status"); rule.To != "git status --short" {
		t.Fatalf("explicit rule was overwritten: %+v", rule)
	}
}
