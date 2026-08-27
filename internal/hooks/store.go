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
// missing or holds malformed JSON.
func (s *Store) Load() (hooksFile, error) {
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

// Save atomically writes hooks to the JSON file, creating the parent directory
// if it does not exist.
func (s *Store) Save(h hooksFile) error {
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hooks: %w", err)
	}

	return fileutil.AtomicWrite(s.path, data)
}

// SaveAudited persists h and emits the audit breadcrumb for it. Bulk rewrites
// have no single affected key, so the breadcrumb carries entries=N rather than a
// hook_key.
func (s *Store) SaveAudited(h hooksFile, op string, entries int, via string) error {
	if err := s.Save(h); err != nil {
		logger.Warn(op, "op", op, "entries", entries, "via", via,
			"error", err, "error_class", fileutil.ClassifyWriteError(err))
		return err
	}

	logger.Info(op, "op", op, "entries", entries, "via", via)
	return nil
}

// Set adds or overwrites the hook for key and event. Writing the same command
// again is a no-op: the file is left untouched. via records the mutation origin
// for the audit breadcrumb: cli, internal or migrate.
func (s *Store) Set(key, event, command, via string) error {
	h, err := s.Load()
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

	if err := s.Save(h); err != nil {
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
// event goes. It rewrites the file even when the key or event is absent, so the
// breadcrumb is emitted either way.
func (s *Store) Remove(key, event, via string) error {
	h, err := s.Load()
	if err != nil {
		return fmt.Errorf("failed to load hooks: %w", err)
	}

	if events, ok := h[key]; ok {
		delete(events, event)
		if len(events) == 0 {
			delete(h, key)
		}
	}

	if err := s.Save(h); err != nil {
		logger.Warn("rm", "op", "rm", "hook_key", key, "via", via,
			"error", err, "error_class", fileutil.ClassifyWriteError(err))
		return err
	}

	logger.Info("rm", "op", "rm", "hook_key", key, "via", via)
	return nil
}

// List returns the hooks sorted by key then event.
func (s *Store) List() ([]Hook, error) {
	h, err := s.Load()
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

	h, err := s.Load()
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

	if err := s.Save(kept); err != nil {
		storelog.EmitCleanStaleSummary(logger, len(removed), start, err)
		return nil, fmt.Errorf("failed to save after cleaning stale hooks: %w", err)
	}

	storelog.EmitCleanStaleSummary(logger, len(removed), start, nil)

	return removed, nil
}

// Get returns the event map for key, or an empty map when the key has no hooks.
func (s *Store) Get(key string) (map[string]string, error) {
	h, err := s.Load()
	if err != nil {
		return nil, err
	}

	events, ok := h[key]
	if !ok {
		return map[string]string{}, nil
	}

	return events, nil
}
