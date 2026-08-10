package theme

import (
	"os"
	"path/filepath"

	"github.com/leeovery/portal/internal/log"
)

// Loader turns one theme file into a Theme, running the fixed rejection ladder
// over it.
//
// It hardcodes no slugs, resolves no paths and decides nothing about logging: it
// is driven by callers with different authority — TUI construction, the panel,
// portal doctor, portal theme export and the offline capture harness — and none
// of them may have config discovery or an emission policy imposed on it. The path
// to read arrives as an argument; the reserved slugs and the event seam arrive on
// the Loader.
type Loader struct {
	// ReservedSlugs is the set of built-in slugs a user file may not take. It is a
	// field rather than a lookup inside the rung so the collision check can be
	// exercised with a synthetic set.
	//
	// The zero value — a nil map — reserves nothing, which is why the production
	// constructors exist: anything resolving a user's theme goes through NewLoader
	// or NewSilentLoader, which populate it identically, or it has no shadowing
	// protection. A zero-value Loader is a test shape for driving the ladder with a
	// synthetic set.
	ReservedSlugs map[string]struct{}

	// BuiltinSource is where LoadBuiltin gets a built-in's bytes. Nil — the
	// production value — reads the embedded set through BuiltinBytes.
	//
	// It exists to stage one otherwise-unreachable state: a binary whose embedded
	// set cannot supply the theme a slot falls back to, which is fatal because
	// Portal has no runtime last-resort palette beneath that point. It does not
	// relax slug reservation, which is still derived from the embedded filenames.
	BuiltinSource func(slug string) ([]byte, bool)

	// events is the injected `theme` log-component seam: a real component logger
	// where a theme is used, log.Discard() where one is merely diagnosed.
	//
	// It is a pointer, so every copy of a Loader — the type is used by value —
	// shares one dedup set, which stops the construction-time by-name read and the
	// panel's enumeration reporting the same directory condition twice in one
	// process. Nil is a valid silent seam.
	events *EventLogger
}

// NewLoader returns a Loader reserving every built-in slug and emitting its
// events through events.
//
// The reserved set is derived from the embedded filenames, so a built-in added
// later reserves its own slug with no Go edit here, and it is built once per
// loader rather than per file.
//
// The property this makes real: an invalid theme falls back to a built-in, so the
// built-in Portal falls back to must never be a file the user can supply. A
// `tokyo-night.theme` with a typo'd hex is rejected before it is opened, and the
// fallback stays the embedded one.
//
// A nil seam panics. It would otherwise produce a loader indistinguishable from
// NewSilentLoader's, and silence is a decision that must be readable at the call
// site: with one named route to it, "where does Portal deliberately write no
// `theme` records" is answered by a single grep.
func NewLoader(events *EventLogger) Loader {
	if events == nil {
		panic("theme.NewLoader: nil event seam — a deliberately silent loader must be built with theme.NewSilentLoader")
	}

	return Loader{ReservedSlugs: builtinSlugSet(), events: events}
}

// NewSilentLoader returns a Loader that judges exactly as NewLoader's does and
// writes nothing at all.
//
// It is the shape a diagnose-shaped caller takes — `portal doctor`, `portal
// theme export`, the offline capture harness — and a constructor rather than a
// shape each assembles for itself, because the `theme` component records where a
// theme is used and never where one is diagnosed.
//
// Silence is about emission only: it reserves every built-in slug exactly as
// NewLoader does, since a diagnosis that let a user's file shadow a built-in
// would report a verdict the launch it is diagnosing would never reach.
func NewSilentLoader() Loader {
	return NewLoader(NewEventLogger(log.Discard()))
}

// Result is one loaded theme: the slug its filename yielded, the palette its
// contents declared, and the bytes those contents were.
//
// The slug and the palette are held side by side rather than folded together
// because a Theme carries no identity field — a slug belongs to whatever loaded
// the palette, not to the palette.
type Result struct {
	Slug  string
	Theme Theme

	// Source is the exact bytes that were parsed, whether they came off the disk
	// or out of the embedded set.
	//
	// It exists so `portal theme export` can write what the loader validated
	// rather than reading its input a second time. They are never a
	// re-serialisation of the Theme, which would drop every `#` comment — the
	// attribution header the format was chosen to carry.
	//
	// It is nil on every rejection, alongside the rest of the zero Result.
	Source []byte
}

// LoadFile loads the theme file at path, returning either its slug and palette
// or exactly one Rejection saying why it is not usable.
//
// The six checks are the rejection ladder and run in a fixed order,
// short-circuiting at the first failure:
//
//  1. `bad name` — from filepath.Base(path) alone, before the file is opened, so
//     a bad-name file can never also report `unreadable` or anything about its
//     contents. It means the file yields no usable slug, which lets the next rung
//     assume one exists.
//  2. `reserved name` — likewise decided from the slug alone, before any read.
//  3. `unreadable` — the read itself failed.
//  4. `bad syntax` — a lexical failure aborts the parse, so no value-level or
//     presence check runs.
//  5. `bad colour` — value validation across every known key.
//  6. `missing tokens` — the presence check runs last, on a file that parsed and
//     whose every known value is well-formed.
//
// The order is the point: without it a file that is both duplicate-keyed and
// missing tokens has two defensible answers, and doctor and the panel can
// disagree about the same file.
//
// Rungs 4 to 6 are parseThemeBytes, which LoadBuiltin runs on the embedded set,
// so a built-in and a stranger's file are judged by the same code.
//
// `not found` — the seventh reason — is never returned here. It applies only to a
// slug named by prefs.json with no corresponding file; producing it from a path
// would send a user to look for a file this function was just handed.
//
// The two filename rungs carry no detail, while the four that read the file all
// do: `bad name` and `reserved name` have their own pinned doctor lines composed
// from the filename and slug the caller already holds.
//
// On rejection the Result is the zero value.
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
// palette or exactly one Rejection from the four content rungs — `unreadable`,
// `bad syntax`, `bad colour`, `missing tokens` — in that order.
//
// It runs neither filename rung and derives no slug, which is the whole
// difference from LoadFile: a path handed in by a caller is an input rather than
// a directory entry, so judging it by its filename would reject
// `~/work/mytheme.txt` for rules that exist to keep the themes directory
// unambiguous, where this file does not live. The filename reasons still matter
// to a drop-in author, but as the caller's warning from the basename.
//
// Rungs 3 to 6 are the same code LoadFile and LoadBuiltin run, so a file judged
// here and the same file judged as a directory entry can differ only in the two
// rungs deliberately skipped.
//
// It is a function rather than a Loader method because none of a Loader's
// injected dependencies bears on it, and taking no receiver makes them
// unreachable rather than merely unread.
//
// On rejection the Result is the zero value, as LoadFile's is.
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

// parseThemeBytes turns one theme file's bytes into the Theme they describe, or
// into exactly one of the ladder's last three rungs — `bad syntax`,
// `bad colour`, `missing tokens` — in that order.
//
// It is the whole content half of the ladder and the only implementation of it,
// reached with bytes off the disk and with bytes out of the embedded set alike. A
// second parse path would let a format bug hide behind a Go-side built-in, so no
// caller may lex or validate on its own. The two rungs above it are decided from
// a filename, which an embedded built-in has no equivalent of.
//
// A failure here is an ordinary rejection, never a panic: escalating a broken
// built-in belongs to the place a fallback is needed, not to the place a file is
// read.
func parseThemeBytes(data []byte) (Theme, *Rejection) {
	pairs, rejection := lexPairs(data)
	if rejection != nil {
		return Theme{}, rejection
	}
	return themeFromPairs(pairs)
}

// isReserved reports whether slug is one of the injected built-in slugs.
//
// The comparison is exact string equality, which the reject-never-normalise rule
// makes safe: a name is never lowercased into a slug, so `Nord.theme` yields no
// slug at all rather than one that would collide here, and a case-insensitive
// filesystem cannot smuggle a shadowing file past the check.
func (l Loader) isReserved(slug string) bool {
	_, reserved := l.ReservedSlugs[slug]
	return reserved
}

// unreadable builds the one rejection a failed read produces, from the one
// source that knows anything about the failure: the OS error.
//
// It is rendered into the detail verbatim, the pinned format for this reason:
// the error text is the only thing distinguishing a permission denial from a
// dangling symlink. The same error is retained on Err so a caller can match on it
// structurally instead of reading the rendered line back apart.
func unreadable(err error) *Rejection {
	return &Rejection{Reason: ReasonUnreadable, Detail: err.Error(), Err: err}
}
