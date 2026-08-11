package project

import (
	"errors"
	"slices"
	"strings"

	"github.com/leeovery/portal/internal/fileutil"
)

// ErrProjectNotFound reports that no project matches the given path: an unknown
// path is an addressing error rather than a no-op, and nothing is written.
var ErrProjectNotFound = errors.New("project not found")

// NormaliseTag trims surrounding whitespace, reporting ok==false for input that
// is empty or whitespace-only. Case is preserved, so matching is case-sensitive:
// "Work" and "work" are distinct tags. Every comparison must go through this
// rather than re-implementing the trim, so the stored value, the grouping key and
// the displayed heading stay identical.
func NormaliseTag(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}

func findByPath(projects []Project, path string) (int, bool) {
	idx := slices.IndexFunc(projects, func(p Project) bool {
		return p.Path == path
	})
	return idx, idx >= 0
}

// AddTag adds rawTag, in canonical form, to the tag set of the project matched
// by exact path. A blank tag or one already present is a silent no-op that
// writes nothing; an unmatched path returns ErrProjectNotFound.
func (s *Store) AddTag(path, rawTag string) error {
	projects, err := s.Load()
	if err != nil {
		return err
	}

	idx, ok := findByPath(projects, path)
	if !ok {
		return ErrProjectNotFound
	}

	tag, ok := NormaliseTag(rawTag)
	if !ok {
		return nil
	}

	if slices.Contains(projects[idx].Tags, tag) {
		return nil
	}

	projects[idx].Tags = append(projects[idx].Tags, tag)
	return s.saveTagMutation(projects, projects[idx].Name, path, tag)
}

// RemoveTag removes the canonical form of rawTag from the tag set of the project
// matched by exact path. A blank or absent tag is a silent no-op that writes
// nothing; an unmatched path returns ErrProjectNotFound.
func (s *Store) RemoveTag(path, rawTag string) error {
	projects, err := s.Load()
	if err != nil {
		return err
	}

	idx, ok := findByPath(projects, path)
	if !ok {
		return ErrProjectNotFound
	}

	tag, ok := NormaliseTag(rawTag)
	if !ok {
		return nil
	}

	before := len(projects[idx].Tags)
	projects[idx].Tags = slices.DeleteFunc(projects[idx].Tags, func(existing string) bool {
		return existing == tag
	})
	if len(projects[idx].Tags) == before {
		return nil
	}

	return s.saveTagMutation(projects, projects[idx].Name, path, tag)
}

func (s *Store) saveTagMutation(projects []Project, name, path, tag string) error {
	if err := s.Save(projects); err != nil {
		logger.Warn("modify", "op", "modify", "project", name, "path", path, "value", tag, "via", "cli",
			"error", err, "error_class", fileutil.ClassifyWriteError(err))
		return err
	}
	logger.Info("modify", "op", "modify", "project", name, "path", path, "value", tag, "via", "cli")
	return nil
}
