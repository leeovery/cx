package theme

import (
	"os"
	"path/filepath"

	"github.com/leeovery/portal/internal/log"
)

// Loader turns one theme file into a Theme through the fixed rejection ladder.
// It resolves no paths and decides nothing about logging.
type Loader struct {
	// ReservedSlugs is the set of built-in slugs a user file may not take. The
	// zero value reserves nothing, so anything resolving a user's theme must be
	// built by NewLoader or NewSilentLoader or it has no shadowing protection.
	ReservedSlugs map[string]struct{}

	// BuiltinSource is where LoadBuiltin gets a built-in's bytes; nil reads the
	// embedded set. It exists to stage the otherwise-unreachable broken-binary
	// state and does not relax slug reservation.
	BuiltinSource func(slug string) ([]byte, bool)

	// A pointer so every copy of a Loader (used by value) shares one dedup set;
	// nil is a valid silent seam.
	events *EventLogger
}

// NewLoader returns a Loader reserving every built-in slug and emitting through
// events. A nil seam panics: silence must be readable at the call site as
// NewSilentLoader, not an accidental nil.
func NewLoader(events *EventLogger) Loader {
	if events == nil {
		panic("theme.NewLoader: nil event seam — a deliberately silent loader must be built with theme.NewSilentLoader")
	}

	return Loader{ReservedSlugs: builtinSlugSet(), events: events}
}

// NewSilentLoader returns a Loader that judges exactly as NewLoader's does and
// writes nothing — for diagnose-shaped callers. Silence is about emission only:
// it still reserves every built-in slug, or a diagnosis would report a verdict
// the launch it diagnoses would never reach.
func NewSilentLoader() Loader {
	return NewLoader(NewEventLogger(log.Discard()))
}

type Result struct {
	Slug  string
	Theme Theme

	// Source is the exact bytes that were parsed — never a re-serialisation of
	// the Theme, which would drop every `#` comment. Nil on rejection.
	Source []byte
}

// LoadFile loads the theme file at path, returning its slug and palette or
// exactly one Rejection with a zero Result. Rung order is load-bearing and
// short-circuits at the first failure: without a fixed order a file failing two
// rungs has two defensible answers, and doctor and the panel could disagree
// about the same file. `not found` is never returned here — it applies to a slug
// with no corresponding file.
func (l Loader) LoadFile(path string) (Result, *Rejection) {
	slug, rejection := SlugFromFilename(filepath.Base(path))
	if rejection != nil {
		return Result{}, rejection
	}
	if l.isReserved(slug) {
		return Result{}, &Rejection{Reason: ReasonReservedName}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, unreadable(err)
	}

	built, rejection := parseThemeBytes(data)
	if rejection != nil {
		return Result{}, rejection
	}

	return Result{Slug: slug, Theme: built, Source: data}, nil
}

// LoadPath loads the theme file at path as an explicit input: the content rungs
// only, no filename rungs and no slug, since a path handed in by a caller is an
// input rather than a directory entry. Rejection comes with a zero Result.
func LoadPath(path string) (Result, *Rejection) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, unreadable(err)
	}

	built, rejection := parseThemeBytes(data)
	if rejection != nil {
		return Result{}, rejection
	}

	return Result{Theme: built, Source: data}, nil
}

// The content half of the ladder, for disk and embedded bytes alike: no caller
// may lex or validate on its own, or a format bug could hide behind a Go-side
// built-in.
func parseThemeBytes(data []byte) (Theme, *Rejection) {
	pairs, rejection := lexPairs(data)
	if rejection != nil {
		return Theme{}, rejection
	}
	return themeFromPairs(pairs)
}

// Exact equality is safe because a name is never normalised into a slug: an
// upper-cased filename yields no slug at all, so a case-insensitive filesystem
// cannot smuggle a shadowing file past the check.
func (l Loader) isReserved(slug string) bool {
	_, reserved := l.ReservedSlugs[slug]
	return reserved
}

// The OS error goes into Detail verbatim — what distinguishes a permission
// denial from a dangling symlink — and stays on Err so a caller can match
// structurally instead of parsing the line.
func unreadable(err error) *Rejection {
	return &Rejection{Reason: ReasonUnreadable, Detail: err.Error(), Err: err}
}
