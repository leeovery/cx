package spawn

import (
	"errors"
	"slices"
	"sort"
	"testing"

	"github.com/leeovery/portal/internal/state"
)

const optionDump = "@portal-spawn-b1-t1 1\n" +
	"@portal-spawn-b1-t2 1\n" +
	"@portal-spawn-b2-t9 1\n" +
	"@portal-skeleton-foo 1\n" +
	"@portal-skeleton-bar 1\n" +
	"default-terminal \"tmux-256color\"\n" +
	"\n" +
	"escape-time 10"

// Structurally satisfies both spawn.serverOptionLister and
// state.ServerOptionLister, so one fixture drives both enumerators.
type fakeOptionLister struct {
	out string
	err error
}

func (f fakeOptionLister) ShowAllServerOptions() (string, error) { return f.out, f.err }

type setCall struct {
	name  string
	value string
}

type fakeOptionWriter struct {
	sets     []setCall
	unsets   []string
	unsetErr func(name string) error
}

func (f *fakeOptionWriter) SetServerOption(name, value string) error {
	f.sets = append(f.sets, setCall{name: name, value: value})
	return nil
}

func (f *fakeOptionWriter) UnsetServerOption(name string) error {
	f.unsets = append(f.unsets, name)
	if f.unsetErr != nil {
		return f.unsetErr(name)
	}
	return nil
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestServerOptionAckChannel_CollectIgnoresForeignBatchesAndSkeletonMarkers(t *testing.T) {
	ch := NewServerOptionAckChannel(&fakeOptionWriter{}, fakeOptionLister{out: optionDump})

	got, err := ch.Collect("b1")
	if err != nil {
		t.Fatalf("Collect(b1) error = %v, want nil", err)
	}
	if want := []string{"t1", "t2"}; !slices.Equal(sortedKeys(got), want) {
		t.Errorf("Collect(b1) tokens = %v, want %v (foreign-batch and skeleton markers must be excluded)", sortedKeys(got), want)
	}

	none, err := ch.Collect("nope")
	if err != nil {
		t.Fatalf("Collect(nope) error = %v, want nil", err)
	}
	if none == nil {
		t.Errorf("Collect(nope) = nil map, want non-nil empty set")
	}
	if len(none) != 0 {
		t.Errorf("Collect(nope) = %v, want empty", sortedKeys(none))
	}
}

func TestListSkeletonMarkers_IgnoresSpawnMarkersOnSameDump(t *testing.T) {
	got, err := state.ListSkeletonMarkers(fakeOptionLister{out: optionDump})
	if err != nil {
		t.Fatalf("ListSkeletonMarkers error = %v, want nil", err)
	}
	if want := []string{"bar", "foo"}; !slices.Equal(sortedKeys(got), want) {
		t.Errorf("ListSkeletonMarkers paneKeys = %v, want %v (must be blind to @portal-spawn- markers)", sortedKeys(got), want)
	}
	for _, spawnLeak := range []string{"t1", "t2", "t9", "b1-t1", "b1-t2", "b2-t9"} {
		if _, ok := got[spawnLeak]; ok {
			t.Errorf("ListSkeletonMarkers leaked spawn-derived key %q", spawnLeak)
		}
	}
}

func TestServerOptionAckChannel_WriteSetsMarkerToOne(t *testing.T) {
	w := &fakeOptionWriter{}
	ch := NewServerOptionAckChannel(w, fakeOptionLister{})

	if err := ch.Write("b1", "t1"); err != nil {
		t.Fatalf("Write(b1, t1) error = %v, want nil", err)
	}
	want := []setCall{{name: "@portal-spawn-b1-t1", value: "1"}}
	if !slices.Equal(w.sets, want) {
		t.Errorf("Write recorded sets = %v, want %v", w.sets, want)
	}
}

func TestServerOptionAckChannel_CleanUnsetsOnlyBatchMarkersIdempotently(t *testing.T) {
	w := &fakeOptionWriter{}
	ch := NewServerOptionAckChannel(w, fakeOptionLister{out: optionDump})

	if err := ch.Clean("b1"); err != nil {
		t.Fatalf("Clean(b1) error = %v, want nil", err)
	}
	want := []string{"@portal-spawn-b1-t1", "@portal-spawn-b1-t2"}
	if !slices.Equal(w.unsets, want) {
		t.Errorf("Clean(b1) unset = %v, want %v (must not touch b2 or skeleton markers)", w.unsets, want)
	}

	w2 := &fakeOptionWriter{}
	ch2 := NewServerOptionAckChannel(w2, fakeOptionLister{out: optionDump})
	if err := ch2.Clean("absent"); err != nil {
		t.Fatalf("Clean(absent) error = %v, want nil", err)
	}
	if len(w2.unsets) != 0 {
		t.Errorf("Clean(absent) unset = %v, want none (zero-marker batch is a no-op)", w2.unsets)
	}
}

func TestServerOptionAckChannel_CleanContinuesAfterUnsetErrorReturnsFirst(t *testing.T) {
	boom := errors.New("unset boom")
	w := &fakeOptionWriter{
		unsetErr: func(name string) error {
			if name == "@portal-spawn-b1-t1" {
				return boom
			}
			return nil
		},
	}
	ch := NewServerOptionAckChannel(w, fakeOptionLister{out: optionDump})

	err := ch.Clean("b1")
	if !errors.Is(err, boom) {
		t.Fatalf("Clean(b1) error = %v, want it to be %v (first unset error)", err, boom)
	}
	want := []string{"@portal-spawn-b1-t1", "@portal-spawn-b1-t2"}
	if !slices.Equal(w.unsets, want) {
		t.Errorf("Clean(b1) unset = %v, want %v (must continue past an unset error)", w.unsets, want)
	}
}

func TestServerOptionAckChannel_CollectReturnsErrorNotFalseEmpty(t *testing.T) {
	boom := errors.New("show-options boom")
	ch := NewServerOptionAckChannel(&fakeOptionWriter{}, fakeOptionLister{err: boom})

	got, err := ch.Collect("b1")
	if !errors.Is(err, boom) {
		t.Fatalf("Collect error = %v, want it to be %v", err, boom)
	}
	if got != nil {
		t.Errorf("Collect on enumeration failure = %v, want nil (never a false-empty success set)", got)
	}
}
