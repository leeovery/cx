//go:build integration

package spawn_test

import (
	"sort"
	"testing"

	"github.com/leeovery/portal/internal/spawn"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmuxtest"
)

func hasToken(set map[string]struct{}, token string) bool {
	_, ok := set[token]
	return ok
}

func sortedSet(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestServerOptionAckChannel_RealTmuxRoundTripAndIdempotentClean(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "spawnack-")
	client := ts.Client()

	// A session anchors the server so server options are queryable.
	ts.Run(t, "new-session", "-d", "-s", "anchor")

	ch := spawn.NewServerOptionAckChannel(client, client)

	if err := ch.Write("b1", "t1"); err != nil {
		t.Fatalf("Write(b1, t1): %v", err)
	}
	if err := state.SetSkeletonMarker(client, "foo"); err != nil {
		t.Fatalf("SetSkeletonMarker(foo): %v", err)
	}

	got, err := ch.Collect("b1")
	if err != nil {
		t.Fatalf("Collect(b1): %v", err)
	}
	if len(got) != 1 || !hasToken(got, "t1") {
		t.Fatalf("Collect(b1) = %v, want exactly {t1}", sortedSet(got))
	}

	skel, err := state.ListSkeletonMarkers(client)
	if err != nil {
		t.Fatalf("ListSkeletonMarkers: %v", err)
	}
	if len(skel) != 1 || !hasToken(skel, "foo") {
		t.Fatalf("ListSkeletonMarkers = %v, want exactly {foo}", sortedSet(skel))
	}

	if err := ch.Clean("b1"); err != nil {
		t.Fatalf("Clean(b1): %v", err)
	}
	afterClean, err := ch.Collect("b1")
	if err != nil {
		t.Fatalf("Collect(b1) after Clean: %v", err)
	}
	if len(afterClean) != 0 {
		t.Errorf("Collect(b1) after Clean = %v, want empty", sortedSet(afterClean))
	}

	if err := ch.Clean("b1"); err != nil {
		t.Errorf("second Clean(b1) = %v, want nil (idempotent no-op)", err)
	}

	skelAfter, err := state.ListSkeletonMarkers(client)
	if err != nil {
		t.Fatalf("ListSkeletonMarkers after Clean: %v", err)
	}
	if len(skelAfter) != 1 || !hasToken(skelAfter, "foo") {
		t.Errorf("ListSkeletonMarkers after Clean = %v, want {foo} untouched", sortedSet(skelAfter))
	}
}
