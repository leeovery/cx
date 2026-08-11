package alias

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/leeovery/portal/internal/fileutil"
	"github.com/leeovery/portal/internal/log"
)

// The op verb is both the slog message and a required "op" attr, so the
// `aliases: <verb>` grep idiom and `grep op=set` filtering both work.
var logger = log.For("aliases")

type Alias struct {
	Name string
	Path string
}

type Store struct {
	path    string
	aliases map[string]string
}

func NewStore(path string) *Store {
	return &Store{
		path:    path,
		aliases: make(map[string]string),
	}
}

// Load reads aliases from the flat key=value file, returning an empty map when
// the file is missing or empty. Duplicate keys are last-wins.
func (s *Store) Load() (map[string]string, error) {
	f, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.aliases = make(map[string]string)
			return s.aliases, nil
		}
		return nil, fmt.Errorf("failed to open aliases file: %w", err)
	}
	defer func() { _ = f.Close() }()

	aliases := make(map[string]string)
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		name, path, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		aliases[name] = path
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read aliases file: %w", err)
	}

	s.aliases = aliases
	return s.aliases, nil
}

// Save writes all aliases in sorted key=value format, creating the parent
// directory if needed. The write is in place, not atomic. Failures are wrapped in
// fileutil's write-phase sentinels — a missing directory as ErrWriteTempCreate —
// so the audited methods can classify them without their own token table.
func (s *Store) Save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%w: failed to create config directory: %w", fileutil.ErrWriteTempCreate, err)
	}

	sorted := s.List()

	var b strings.Builder
	for _, a := range sorted {
		fmt.Fprintf(&b, "%s=%s\n", a.Name, a.Path)
	}

	if err := os.WriteFile(s.path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("%w: failed to write aliases file: %w", fileutil.ErrWriteWrite, err)
	}

	return nil
}

func (s *Store) Get(name string) (string, bool) {
	path, ok := s.aliases[name]
	return path, ok
}

// Set adds or overwrites an in-memory alias; it does not persist.
func (s *Store) Set(name, path string) {
	s.aliases[name] = path
}

// Delete removes the in-memory alias without persisting, reporting whether it
// existed.
func (s *Store) Delete(name string) bool {
	_, ok := s.aliases[name]
	if ok {
		delete(s.aliases, name)
	}
	return ok
}

// SetAndSave adds or updates an alias, persists it and emits the audit
// breadcrumb. Re-setting an alias to the path it already has is a no-op: the file
// is left untouched. via records the mutation origin: cli, internal or migrate.
func (s *Store) SetAndSave(name, path, via string) error {
	existing, present := s.aliases[name]
	if present && existing == path {
		logger.Debug("set-noop", "op", "set-noop", "alias", name, "via", via)
		return nil
	}

	op := "set"
	if present {
		op = "modify"
	}

	s.Set(name, path)

	if err := s.Save(); err != nil {
		logger.Warn(op, "op", op, "alias", name, "value", path, "via", via,
			"error", err, "error_class", fileutil.ClassifyWriteError(err))
		return err
	}

	logger.Info(op, "op", op, "alias", name, "value", path, "via", via)
	return nil
}

// DeleteAndSave removes an alias, persists the result and emits the audit
// breadcrumb. Deleting an absent name returns (false, nil) without writing the
// file or emitting: there is no mutation to audit.
func (s *Store) DeleteAndSave(name, via string) (existed bool, err error) {
	existed = s.Delete(name)
	if !existed {
		return false, nil
	}

	if err := s.Save(); err != nil {
		logger.Warn("rm", "op", "rm", "alias", name, "via", via,
			"error", err, "error_class", fileutil.ClassifyWriteError(err))
		return true, err
	}

	logger.Info("rm", "op", "rm", "alias", name, "via", via)
	return true, nil
}

// Keys returns all alias names sorted — the key namespace for glob enumeration,
// without List's name-to-path shape.
func (s *Store) Keys() []string {
	keys := make([]string, 0, len(s.aliases))
	for name := range s.aliases {
		keys = append(keys, name)
	}
	slices.Sort(keys)
	return keys
}

// List returns all aliases sorted by name.
func (s *Store) List() []Alias {
	result := make([]Alias, 0, len(s.aliases))
	for name, path := range s.aliases {
		result = append(result, Alias{Name: name, Path: path})
	}

	slices.SortFunc(result, func(a, b Alias) int {
		return strings.Compare(a.Name, b.Name)
	})

	return result
}
