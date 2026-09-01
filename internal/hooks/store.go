package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"sort"
	"time"

	"github.com/leeovery/portal/internal/fileutil"
	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/nanoid"
	"github.com/leeovery/portal/internal/storelog"
)

// The op verb is both the slog message and a required "op" attr, so the
// `hooks: <verb>` grep idiom and `grep op=set` filtering both work.
var logger = log.For("hooks")

type Hook struct {
	Key     string
	Event   string
	Command string
}

// Snapshot is the on-disk shape: map[hook_key]map[event]command. The clean also
// names its pre-enumeration read of the file by this type — the older view a
// deletion may be narrowed by.
type Snapshot map[string]map[string]string

type Store struct {
	path string
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

// Load reads hooks from the JSON file, returning an empty map when the file is
// missing or holds malformed JSON. via names the calling surface for the
// degradation breadcrumb.
func (s *Store) Load(via Via) (Snapshot, error) {
	return s.loadShared(via)
}

// loadSnapshot is the clean's advisory pre-read. It reads at the derived
// pre-read bound, so a clean — which takes the sidecar twice, shared here and
// exclusive in deleteStale — never spends two full lockTimeouts waiting on a
// wedged writer.
func (s *Store) loadSnapshot() (Snapshot, error) {
	return s.loadSharedBounded(ViaInternal, snapshotLockBound())
}

// loadShared reads under the shared hold every ordinary read takes.
func (s *Store) loadShared(via Via) (Snapshot, error) {
	return s.loadSharedBounded(via, lockTimeout)
}

// loadSharedBounded reads under a shared hold acquired at bound, releasing it
// before returning so no read ever hands a lock to its caller. Any acquisition
// failure — an absent sidecar, an absent directory, an unreadable file, or the
// bound elapsing — falls through to the non-locking read after one DEBUG
// record: correctness never depends on the shared lock (AtomicWrite renames, so
// a reader sees the pre-state or the post-state, never a torn one), and failing
// a read would forfeit a hook for nothing. The bound is a parameter and is
// never derived from via, which is a log attr.
func (s *Store) loadSharedBounded(via Via, bound time.Duration) (Snapshot, error) {
	f, err := s.acquireSharedLock(bound)
	if err != nil {
		logger.Debug("load-unlocked", "op", "load-unlocked", "via", via.String(), "error", err)
		return s.load()
	}
	defer func() { _ = f.Close() }()

	return s.load()
}

// load is the non-locking read the mutations use from inside their own hold: a
// second acquisition from the same process is not re-entrant and would block
// against that hold until the bound.
func (s *Store) load() (Snapshot, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Snapshot{}, nil
		}
		return nil, err
	}

	var h Snapshot
	if err := json.Unmarshal(data, &h); err != nil {
		return Snapshot{}, nil
	}

	return h, nil
}

// save atomically writes hooks to the JSON file, creating the parent directory
// if it does not exist. Non-locking, like load: it is called from inside a hold.
func (s *Store) save(h Snapshot) error {
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hooks: %w", err)
	}

	return fileutil.AtomicWrite(s.path, data)
}

// Set adds or overwrites the hook for key and event. Writing the same command
// again is a no-op: the file is left untouched. via records the mutation origin
// for the audit breadcrumb.
func (s *Store) Set(key, event, command string, via Via) error {
	lock, err := s.acquireMutationLock()
	if err != nil {
		// Under the method's own op, not classifySet's verdict: that verdict reads
		// the loaded file, and the acquire is what prevented the load. No
		// error_class — nothing reached the write phases it classifies — and no
		// value, which names what a write carried.
		logger.Warn("set", "op", "set", "hook_key", key, "via", via.String(), "error", err)
		return err
	}
	defer func() { _ = lock.Close() }()

	h, err := s.load()
	if err != nil {
		return fmt.Errorf("failed to load hooks: %w", err)
	}

	op := classifySet(h, key, event, command)
	if op == "set-noop" {
		logger.Debug("set-noop", "op", "set-noop", "hook_key", key, "via", via.String())
		return nil
	}

	if h[key] == nil {
		h[key] = make(map[string]string)
	}
	h[key][event] = command

	if err := s.save(h); err != nil {
		logger.Warn(op, "op", op, "hook_key", key, "value", command, "via", via.String(),
			"error", err, "error_class", fileutil.ClassifyWriteError(err))
		return err
	}

	logger.Info(op, "op", op, "hook_key", key, "value", command, "via", via.String())
	return nil
}

func classifySet(h Snapshot, key, event, command string) string {
	events, ok := h[key]
	if !ok {
		return "set"
	}
	existing, ok := events[event]
	if !ok {
		return "set"
	}
	if existing == command {
		return "set-noop"
	}
	return "modify"
}

// Remove deletes the hook for key and event, dropping the outer key when its last
// event goes, and reports whether it removed anything. A call that removes
// nothing — an absent key, an absent event, an absent file — writes no file and
// emits no breadcrumb, and a failed save reports no removal. The answer comes
// from the map this call loaded and mutated, never from a separate read.
func (s *Store) Remove(key, event string, via Via) (bool, error) {
	lock, err := s.acquireMutationLock()
	if err != nil {
		// A failed operation, not the silent no-removal below: that one changed
		// nothing because there was nothing to change, and this one could not look.
		logger.Warn("rm", "op", "rm", "hook_key", key, "via", via.String(), "error", err)
		return false, err
	}
	defer func() { _ = lock.Close() }()

	h, err := s.load()
	if err != nil {
		return false, fmt.Errorf("failed to load hooks: %w", err)
	}

	events, ok := h[key]
	if !ok {
		return false, nil
	}
	if _, ok := events[event]; !ok {
		return false, nil
	}

	delete(events, event)
	if len(events) == 0 {
		delete(h, key)
	}

	if err := s.save(h); err != nil {
		logger.Warn("rm", "op", "rm", "hook_key", key, "via", via.String(),
			"error", err, "error_class", fileutil.ClassifyWriteError(err))
		return false, err
	}

	logger.Info("rm", "op", "rm", "hook_key", key, "via", via.String())
	return true, nil
}

// List returns the hooks sorted by key then event.
func (s *Store) List(via Via) ([]Hook, error) {
	h, err := s.loadShared(via)
	if err != nil {
		return nil, err
	}

	var list []Hook
	for key, events := range h {
		for event, command := range events {
			list = append(list, Hook{
				Key:     key,
				Event:   event,
				Command: command,
			})
		}
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].Key != list[j].Key {
			return list[i].Key < list[j].Key
		}
		return list[i].Event < list[j].Event
	})

	return list, nil
}

// staleKeys applies the staleness rule; every reader of staleness must route
// through here rather than restate it. A persisted key is stale iff it is
// absent from live and its shape is one the rule can judge — token-shaped, or
// empty. A key of any other shape cannot be told apart from an entry that has
// not been converted to a pane token yet, so it is retained.
func staleKeys(persisted Snapshot, live []string) []string {
	liveSet := make(map[string]struct{}, len(live))
	for _, k := range live {
		liveSet[k] = struct{}{}
	}
	var stale []string
	for key := range persisted {
		if _, ok := liveSet[key]; ok {
			continue
		}
		if key == "" || nanoid.IsTokenShaped(key) {
			stale = append(stale, key)
		}
	}
	return stale
}

// narrowToSnapshot drops every candidate the snapshot does not hold.
func narrowToSnapshot(candidates []string, snapshot Snapshot) []string {
	var narrowed []string
	for _, key := range candidates {
		if _, ok := snapshot[key]; ok {
			narrowed = append(narrowed, key)
		}
	}
	return narrowed
}

// StaleKeys returns the persisted hook keys absent from live whose shape the
// staleness rule can judge; a key it cannot judge is retained. It carries no
// mass-deletion guard: an empty live set makes every judgeable persisted key
// stale, and deferring on that is the caller's repair-safety policy.
func StaleKeys(persisted Snapshot, live []string) []string {
	return staleKeys(persisted, live)
}

// ErrSnapshotRead reports that the clean's own pre-read of the file failed, so
// the enumeration never ran and nothing was judged — as distinct from a failure
// of the deletion that would have followed it.
var ErrSnapshotRead = errors.New("failed to read hooks snapshot")

// CleanStale removes and returns the hook entries whose key enumerateLive's
// answer leaves stale: absent from that live set, judgeable by the staleness
// rule, and held by the snapshot this call read. A key it cannot judge is
// retained untouched, and a clean that removes nothing writes no file and emits
// no summary.
//
// The two reads a safe deletion needs are sequenced here rather than by the
// caller: the snapshot first, then enumerateLive with no lock held. Taking the
// snapshot first is what keeps a registration landing between the two out of
// the delete set: it is the older view, and deleteStale narrows to it. Running
// enumerateLive under the hold would also park every writer behind it.
//
// enumerateLive is handed that snapshot and returns the live keys; any error it
// returns aborts the clean with the file untouched and is returned unwrapped, so
// a caller can carry its own reasons through. A failed snapshot read returns
// ErrSnapshotRead and never calls it.
func (s *Store) CleanStale(enumerateLive func(Snapshot) ([]string, error)) ([]string, error) {
	snapshot, err := s.loadSnapshot()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSnapshotRead, err)
	}

	live, err := enumerateLive(snapshot)
	if err != nil {
		return nil, err
	}

	return s.deleteStale(live, snapshot)
}

// deleteStale is the clean's mutation: it derives the delete set from the file
// it loads under its own exclusive hold, so a deletion is decided on the file as
// it stands rather than on the older view the snapshot describes. The snapshot
// may only narrow that set, never widen it: a key it does not hold was written
// after the enumeration and so was never offered to it for protection, which
// makes it unjudgeable by this live set however stale its shape looks.
func (s *Store) deleteStale(live []string, snapshot Snapshot) ([]string, error) {
	start := time.Now()

	lock, err := s.acquireMutationLock()
	if err != nil {
		return nil, err
	}
	defer func() { _ = lock.Close() }()

	h, err := s.load()
	if err != nil {
		return nil, fmt.Errorf("failed to load hooks: %w", err)
	}

	removed := narrowToSnapshot(staleKeys(h, live), snapshot)

	if len(removed) == 0 {
		return removed, nil
	}

	kept := maps.Clone(h)
	for _, key := range removed {
		delete(kept, key)
	}

	if err := s.save(kept); err != nil {
		storelog.EmitCleanStaleSummary(logger, len(removed), start, err)
		return nil, fmt.Errorf("failed to save after cleaning stale hooks: %w", err)
	}

	// After the write, so a line exists only for a key the file no longer holds.
	// The commands come from h, the pre-delete map.
	for _, key := range removed {
		logger.Info("clean-stale", "op", "clean-stale", "hook_key", key,
			"value", h[key]["on-resume"], "via", ViaInternal.String())
	}

	storelog.EmitCleanStaleSummary(logger, len(removed), start, nil)

	return removed, nil
}
