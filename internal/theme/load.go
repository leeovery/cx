package theme

import (
	"os"
	"path/filepath"

	"github.com/leeovery/portal/internal/log"
)

// Loader turns one theme file into a Theme, running the fixed rejection
// ladder over it. It hardcodes no slugs, resolves no paths and decides
// nothing about logging.
type Loader struct {
	// ReservedSlugs is the set of built-in slugs a user file may not take.
	// The zero value reserves nothing: anything resolving a user's theme
	// must go through NewLoader or NewSilentLoader, or it has no shadowing
	// protection.
	ReservedSlugs map[string]struct{}

	// BuiltinSource is where LoadBuiltin gets a built-in's bytes; nil reads
	// the embedded set. It exists to stage the otherwise-unreachable
	// broken-binary state and does not relax slug reservation.
	BuiltinSource func(slug string) ([]byte, bool)

	// A pointer so every copy of a Loader (used by value) shares one dedup
	// set; nil is a valid silent seam.
	events *EventLogger
}

// NewLoader returns a Loader reserving every built-in slug and emitting its
// events through events. A nil seam panics: silence must be readable at the
// call site as NewSilentLoader, not an accidental nil.
func NewLoader(events *EventLogger) Loader {
	if events == nil {
		panic("theme.NewLoader: nil event seam — a deliberately silent loader must be built with theme.NewSilentLoader")
	}

	return Loader{ReservedSlugs: builtinSlugSet(), events: events}
}

// NewSilentLoader returns a Loader that judges exactly as NewLoader's does
// and writes nothing — for diagnose-shaped callers. Silence is about emission
// only: it still reserves every built-in slug, or a diagnosis would report a
// verdict the launch it diagnoses would never reach.
func NewSilentLoader() Loader {
	return NewLoader(NewEventLogger(log.Discard()))
}

// Result is one loaded theme: the slug its filename yielded, the palette its
// contents declared, and the bytes those contents were.
type Result struct {
	Slug  string
	Theme Theme

	// Source is the exact bytes that were parsed — never a re-serialisation
	// of the Theme, which would drop every `#` comment. Nil on rejection.
	Source []byte
}

// LoadFile loads the theme file at path, returning either its slug and palette
// or exactly one Rejection saying why it is not usable. The rejection ladder
// runs in a fixed order, short-circuiting at the first failure: `bad name`,
// `reserved name` (both from the filename, before any read), `unreadable`,
// `bad syntax`, `bad colour`, `missing tokens`. The fixed order is the point —
// without it a file failing two rungs has two defensible answers, and doctor
// and the panel could disagree about the same file. `not found` is never
// returned here: it applies only to a slug with no corresponding file. On
// rejection the Result is the zero value.
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

// LoadPath loads the theme file at path as an explicit input, returning its
// palette or one Rejection from the four content rungs. It runs neither
// filename rung and derives no slug: a path handed in by a caller is an
// input, not a directory entry. On rejection the Result is the zero value.
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

// The whole content half of the ladder, for disk and embedded bytes alike: no
// caller may lex or validate on its own, or a format bug could hide behind a
// Go-side built-in. A failure here is an ordinary rejection, never a panic.
func parseThemeBytes(data []byte) (Theme, *Rejection) {
	pairs, rejection := lexPairs(data)
	if rejection != nil {
		return Theme{}, rejection
	}
	return themeFromPairs(pairs)
}

// Exact string equality is safe because a name is never normalised into a
// slug: `Nord.theme` yields no slug at all, so a case-insensitive filesystem
// cannot smuggle a shadowing file past the check.
func (l Loader) isReserved(slug string) bool {
	_, reserved := l.ReservedSlugs[slug]
	return reserved
}

// The OS error is rendered into the detail verbatim — the only thing
// distinguishing a permission denial from a dangling symlink — and retained
// on Err so a caller can match structurally instead of parsing the line.
func unreadable(err error) *Rejection {
	return &Rejection{Reason: ReasonUnreadable, Detail: err.Error(), Err: err}
}
