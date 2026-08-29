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
	"github.com/leeovery/portal/internal/session"
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

// hooksFile is the on-disk shape: map[hook_key]map[event]command.
type hooksFile = map[string]map[string]string

type Store struct {
	path string
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

// Load reads hooks from the JSON file, returning an empty map when the file is
// missing or holds malformed JSON. via names the calling surface for the
// degradation breadcrumb.
func (s *Store) Load(via string) (hooksFile, error) {
	return s.loadShared(via)
}

// LoadSnapshot is the sweep's advisory pre-read. It exists as an exported read
// because that pre-read lives in cmd and cannot reach the unexported helper,
// and it is the only read taken at snapshotLockTimeout rather than lockTimeout:
// one sweep cycle takes the sidecar twice — shared here, exclusive inside
// CleanStale — so at the full bound a wedged writer would park the daemon's 1s
// tick for two full bounds every cycle, which is the outcome the bound exists
// to prevent.
func (s *Store) LoadSnapshot(via string) (hooksFile, error) {
	return s.loadSharedBounded(via, snapshotLockTimeout)
}

// loadShared reads under the shared hold every ordinary read takes.
func (s *Store) loadShared(via string) (hooksFile, error) {
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
func (s *Store) loadSharedBounded(via string, bound time.Duration) (hooksFile, error) {
	f, err := s.acquireSharedLock(bound)
	if err != nil {
		logger.Debug("load-unlocked", "op", "load-unlocked", "via", via, "error", err)
		return s.load()
	}
	defer func() { _ = f.Close() }()

	return s.load()
}

// load is the non-locking read the mutations use from inside their own hold: a
// second acquisition from the same process is not re-entrant and would block
// against that hold until the bound.
func (s *Store) load() (hooksFile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return hooksFile{}, nil
		}
		return nil, err
	}

	var h hooksFile
	if err := json.Unmarshal(data, &h); err != nil {
		return hooksFile{}, nil
	}

	return h, nil
}

// save atomically writes hooks to the JSON file, creating the parent directory
// if it does not exist. Non-locking, like load: it is called from inside a hold.
func (s *Store) save(h hooksFile) error {
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hooks: %w", err)
	}

	return fileutil.AtomicWrite(s.path, data)
}

// Set adds or overwrites the hook for key and event. Writing the same command
// again is a no-op: the file is left untouched. via records the mutation origin
// for the audit breadcrumb: cli, internal or migrate.
func (s *Store) Set(key, event, command, via string) error {
	lock, err := s.acquireMutationLock()
	if err != nil {
		// Under the method's own op, not classifySet's verdict: that verdict reads
		// the loaded file, and the acquire is what prevented the load. No
		// error_class — nothing reached the write phases it classifies — and no
		// value, which names what a write carried.
		logger.Warn("set", "op", "set", "hook_key", key, "via", via, "error", err)
		return err
	}
	defer func() { _ = lock.Close() }()

	h, err := s.load()
	if err != nil {
		return fmt.Errorf("failed to load hooks: %w", err)
	}

	op := classifySet(h, key, event, command)
	if op == "set-noop" {
		logger.Debug("set-noop", "op", "set-noop", "hook_key", key, "via", via)
		return nil
	}

	if h[key] == nil {
		h[key] = make(map[string]string)
	}
	h[key][event] = command

	if err := s.save(h); err != nil {
		logger.Warn(op, "op", op, "hook_key", key, "value", command, "via", via,
			"error", err, "error_class", fileutil.ClassifyWriteError(err))
		return err
	}

	logger.Info(op, "op", op, "hook_key", key, "value", command, "via", via)
	return nil
}

func classifySet(h hooksFile, key, event, command string) string {
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
func (s *Store) Remove(key, event, via string) (bool, error) {
	lock, err := s.acquireMutationLock()
	if err != nil {
		// A failed operation, not the silent no-removal below: that one changed
		// nothing because there was nothing to change, and this one could not look.
		logger.Warn("rm", "op", "rm", "hook_key", key, "via", via, "error", err)
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
		logger.Warn("rm", "op", "rm", "hook_key", key, "via", via,
			"error", err, "error_class", fileutil.ClassifyWriteError(err))
		return false, err
	}

	logger.Info("rm", "op", "rm", "hook_key", key, "via", via)
	return true, nil
}

// List returns the hooks sorted by key then event.
func (s *Store) List(via string) ([]Hook, error) {
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

// staleKeys is the single implementation of the staleness rule: a persisted key
// is stale iff it is absent from live and its shape is one the rule can judge —
// token-shaped, or empty. A key of any other shape cannot be told apart from an
// entry that has not been converted to a pane token yet, so it is retained.
func staleKeys(persisted hooksFile, live []string) []string {
	liveSet := make(map[string]struct{}, len(live))
	for _, k := range live {
		liveSet[k] = struct{}{}
	}
	var stale []string
	for key := range persisted {
		if _, ok := liveSet[key]; ok {
			continue
		}
		if key == "" || session.IsTokenShaped(key) {
			stale = append(stale, key)
		}
	}
	return stale
}

// StaleKeys returns the persisted hook keys absent from live whose shape the
// staleness rule can judge; a key it cannot judge is retained. It carries no
// mass-deletion guard: an empty live set makes every judgeable persisted key
// stale, and deferring on that is the caller's repair-safety policy.
func StaleKeys(persisted map[string]map[string]string, live []string) []string {
	return staleKeys(persisted, live)
}

// CleanStale removes and returns the hook entries whose key is absent from
// liveKeys and whose shape the staleness rule can judge; a key it cannot judge
// is retained untouched. A clean that removes nothing writes no file and emits
// no summary.
func (s *Store) CleanStale(liveKeys []string) ([]string, error) {
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

	removed := staleKeys(h, liveKeys)

	if len(removed) == 0 {
		return removed, nil
	}

	kept := maps.Clone(h)
	for _, key := range removed {
		delete(kept, key)
	}

	for _, key := range removed {
		logger.Info("clean-stale", "op", "clean-stale", "hook_key", key,
			"value", h[key]["on-resume"], "via", "internal")
	}

	if err := s.save(kept); err != nil {
		storelog.EmitCleanStaleSummary(logger, len(removed), start, err)
		return nil, fmt.Errorf("failed to save after cleaning stale hooks: %w", err)
	}

	storelog.EmitCleanStaleSummary(logger, len(removed), start, nil)

	return removed, nil
}

// Get returns the event map for key, or an empty map when the key has no hooks.
func (s *Store) Get(key, via string) (map[string]string, error) {
	h, err := s.loadShared(via)
	if err != nil {
		return nil, err
	}

	events, ok := h[key]
	if !ok {
		return map[string]string{}, nil
	}

	return events, nil
}
