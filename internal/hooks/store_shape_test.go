package hooks_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/hookstest"
	"github.com/leeovery/portal/internal/logtest"
)

func TestCleanStaleShapeAwareness(t *testing.T) {
	t.Run("it retains a non-token-shaped key absent from the live set", func(t *testing.T) {
		store, path := hookstest.StageStore(t, hookstest.Staging{Seed: fmt.Sprintf(`{%q:{"on-resume":"old"},%q:{"on-resume":"live"}}`, hookstest.UnjudgeableSeedA, hookstest.LiveSeedA)})

		before := readFileBytes(t, path)

		removed, err := store.CleanStale(enumerating(hookstest.LiveSeedA))
		if err != nil {
			t.Fatalf("CleanStale: %v", err)
		}
		if len(removed) != 0 {
			t.Fatalf("CleanStale removed %v, want nothing", removed)
		}

		persisted, err := store.Load(hooks.ViaInternal)
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		predicted := hooks.StaleKeys(persisted, []string{hookstest.LiveSeedA})
		if len(predicted) != 0 {
			t.Errorf("StaleKeys reported %v, want nothing", predicted)
		}

		hookstest.AssertHooksFileUnchanged(t, path, before, "changed by a clean that removed nothing")
	})

	t.Run("it deletes a token-shaped key absent from the live set", func(t *testing.T) {
		store, _ := hookstest.StageStore(t, hookstest.Staging{Seed: fmt.Sprintf(`{%q:{"on-resume":"live"},%q:{"on-resume":"gone"}}`, hookstest.LiveSeedA, hookstest.ReapableSeedA)})

		removed, err := store.CleanStale(enumerating(hookstest.LiveSeedA))
		if err != nil {
			t.Fatalf("CleanStale: %v", err)
		}
		if !slices.Equal(removed, []string{hookstest.ReapableSeedA}) {
			t.Fatalf("CleanStale removed %v, want [%s]", removed, hookstest.ReapableSeedA)
		}

		h, err := store.Load(hooks.ViaInternal)
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if _, ok := h[hookstest.ReapableSeedA]; ok {
			t.Error("token-shaped absent key should have been removed")
		}
		if _, ok := h[hookstest.LiveSeedA]; !ok {
			t.Error("token-shaped live key should have been kept")
		}
	})

	t.Run("it deletes an empty key", func(t *testing.T) {
		store, _ := hookstest.StageStore(t, hookstest.Staging{Seed: fmt.Sprintf(`{"":{"on-resume":"malformed"},%q:{"on-resume":"live"}}`, hookstest.LiveSeedA)})

		removed, err := store.CleanStale(enumerating(hookstest.LiveSeedA))
		if err != nil {
			t.Fatalf("CleanStale: %v", err)
		}
		if !slices.Equal(removed, []string{""}) {
			t.Fatalf("CleanStale removed %v, want the empty key", removed)
		}

		h, err := store.Load(hooks.ViaInternal)
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if _, ok := h[""]; ok {
			t.Error("empty key should have been removed")
		}
	})

	t.Run("it writes no file and emits no summary when every candidate is retained", func(t *testing.T) {
		store, path := hookstest.StageStore(t, hookstest.Staging{Seed: fmt.Sprintf(`{%q:{"on-resume":"a"},%q:{"on-resume":"b"}}`, hookstest.UnjudgeableSeedB, hookstest.UnjudgeableSeedC)})

		before := readFileBytes(t, path)

		sink := logtest.Install(t)
		removed, err := store.CleanStale(enumerating())
		if err != nil {
			t.Fatalf("CleanStale: %v", err)
		}
		if len(removed) != 0 {
			t.Fatalf("CleanStale removed %v, want nothing", removed)
		}

		for _, r := range sink.Records() {
			if r.Msg == "clean-stale" {
				t.Errorf("unexpected clean-stale record: %+v", r)
			}
		}

		hookstest.AssertHooksFileUnchanged(t, path, before, "changed when every candidate was retained")
	})
}
