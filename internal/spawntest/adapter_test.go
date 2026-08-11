package spawntest_test

import (
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/spawn"
	"github.com/leeovery/portal/internal/spawntest"
)

var _ spawn.Adapter = (*spawntest.FakeAdapter)(nil)

func TestFakeAdapter_RecordsArgvPerCallInOrder(t *testing.T) {
	f := &spawntest.FakeAdapter{}
	first := []string{"/usr/bin/env", "-u", "TMUX", "/abs/portal", "attach", "s1"}
	second := []string{"/usr/bin/env", "-u", "TMUX", "/abs/portal", "attach", "s2"}

	f.OpenWindow(first)
	f.OpenWindow(second)

	if len(f.Calls) != 2 {
		t.Fatalf("len(Calls) = %d, want 2", len(f.Calls))
	}
	if !slices.Equal(f.Calls[0], first) {
		t.Errorf("Calls[0] = %v, want %v", f.Calls[0], first)
	}
	if !slices.Equal(f.Calls[1], second) {
		t.Errorf("Calls[1] = %v, want %v", f.Calls[1], second)
	}
}

func TestFakeAdapter_RecordsDefensiveCopy(t *testing.T) {
	f := &spawntest.FakeAdapter{}
	arg := []string{"/usr/bin/env", "/abs/portal", "attach", "s1"}

	f.OpenWindow(arg)
	arg[3] = "MUTATED"

	if got := f.Calls[0][3]; got != "s1" {
		t.Errorf("Calls[0][3] = %q, want %q — recording is not a defensive copy", got, "s1")
	}
}

func TestFakeAdapter_ReturnsScriptedResultsThenDefaultsToSuccess(t *testing.T) {
	f := &spawntest.FakeAdapter{
		Results: []spawn.Result{
			spawn.Success("first"),
			spawn.SpawnFailed("second boom"),
		},
	}

	got1 := f.OpenWindow([]string{"a"})
	if got1.Outcome != spawn.OutcomeSuccess || got1.Detail != "first" {
		t.Errorf("call 1 = %+v, want Success(\"first\")", got1)
	}

	got2 := f.OpenWindow([]string{"b"})
	if got2.Outcome != spawn.OutcomeSpawnFailed || got2.Detail != "second boom" {
		t.Errorf("call 2 = %+v, want SpawnFailed(\"second boom\")", got2)
	}

	got3 := f.OpenWindow([]string{"c"})
	if !got3.OK() || got3.Detail != "" {
		t.Errorf("call 3 (exhausted) = %+v, want default Success(\"\")", got3)
	}
}

func TestFakeAdapter_DefaultsToSuccessWhenResultsEmpty(t *testing.T) {
	f := &spawntest.FakeAdapter{}

	got := f.OpenWindow([]string{"a"})
	if !got.OK() || got.Detail != "" {
		t.Errorf("call = %+v, want default Success(\"\")", got)
	}
}
