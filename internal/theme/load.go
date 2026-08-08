package theme

import (
	"os"
	"path/filepath"
)

// Loader turns one theme file into a Theme, running the fixed rejection ladder
// over it.
//
// It HARDCODES NO SLUGS, RESOLVES NO PATHS and DECIDES NOTHING ABOUT LOGGING.
// All three are injected, and all three for the same reason: the loader is
// driven by callers with different authority — TUI construction, the panel,
// portal doctor, portal theme export and the offline capture harness — and none
// of them may have config discovery or an emission policy imposed on it. The
// path to read arrives as an argument (the themes-directory resolution chain
// lives in cmd), while the reserved built-in slugs and the event seam arrive on
// the Loader.
type Loader struct {
	// ReservedSlugs is the set of built-in slugs a user file may not take.
	// NewLoader populates it from the embedded set, and it stays a FIELD rather
	// than a lookup inside the rung so a caller can exercise the collision check
	// with a synthetic set — which is also what keeps the reserved-name rung
	// decidable without the embedded files being involved at all.
	//
	// The zero value — a nil map — reserves nothing. That is the shape every
	// caller uninterested in the rung carries, and it is why the production
	// constructor exists: a Loader assembled by hand has no shadowing
	// protection, so anything resolving a user's theme goes through NewLoader.
	ReservedSlugs map[string]struct{}

	// BuiltinSource is where LoadBuiltin gets a built-in's bytes. The zero value
	// — nil — is the production one and reads the embedded set through
	// BuiltinBytes, so no call site passes anything.
	//
	// It exists for exactly one state, and BECAUSE that state is otherwise
	// unreachable: a binary whose embedded set cannot supply the theme a slot
	// falls back to. Portal has no runtime last-resort palette beneath that point
	// by decision, so the failure is fatal — and an unreachable fatal nobody can
	// exercise is a path nobody has ever run. Injecting a source that omits or
	// corrupts a fallback slug stages it; production carries a nil field and one
	// branch.
	//
	// It does NOT relax slug reservation. The reserved-slug set is still derived
	// from the embedded filenames, so a user's file can never take a built-in's
	// slug whatever this source answers.
	BuiltinSource func(slug string) ([]byte, bool)

	// events is the injected `theme` log-component seam. It is unexported and
	// arrives through NewLoader because it is not configuration the loader reads
	// but a decision the CALLER makes: a real component logger where a theme is
	// used, log.Discard() where one is merely diagnosed.
	//
	// It is a pointer, so every copy of a Loader — the type is used by value —
	// shares one dedup set, which is what stops the construction-time by-name read
	// and the panel's enumeration reporting the same directory condition twice in
	// one process. A nil one is a valid silent seam, so the zero-value Loader
	// emits nothing at all.
	events *EventLogger
}

// NewLoader returns a Loader reserving every built-in slug and emitting its
// events through events.
//
// The reserved set is DERIVED from the embedded set — builtinSlugSet reads the
// embedded filenames — so a built-in added later reserves its own slug with no Go
// edit here, and reservation stays true as the set grows. It is built ONCE per
// loader rather than per file, because enumeration classifies a whole directory
// through one Loader and the set is the same for every candidate in it.
//
// The property this constructor makes real: an invalid theme falls back to a
// built-in, so the built-in Portal falls back to must never be a file the user
// can supply. A `tokyo-night.theme` with a typo'd hex is rejected before it is
// opened, and the fallback stays the embedded one.
//
// The event logger, by contrast, is a dependency the CALLER owns rather than one
// this package can derive: passing log.Discard() is how `portal doctor`,
// `portal theme export` and capturetool stay silent, and passing a fresh one is
// how a caller controls dedup. The reserved-slug set stays an ordinary field so
// the reserved-name rung can still be driven with a synthetic set, or with none.
func NewLoader(events *EventLogger) Loader {
	return Loader{ReservedSlugs: builtinSlugSet(), events: events}
}

// Result is one loaded theme: the slug its FILENAME yielded, the palette its
// CONTENTS declared, and the bytes those contents were.
//
// The slug and the palette are held side by side rather than folded together
// because a Theme carries no identity field — a slug belongs to whatever loaded
// the palette, not to the palette.
type Result struct {
	Slug  string
	Theme Theme

	// Source is the exact bytes that were parsed, whether they came off the
	// disk or out of the embedded set.
	//
	// It exists so `portal theme export` can write what the loader VALIDATED
	// rather than reading its input a second time: the bytes printed are the
	// bytes checked, comments and trailing newline included, with no window in
	// which the two could differ. They are the file's bytes and never a
	// re-serialisation of the Theme, because re-serialising would drop every
	// `#` comment — the attribution header the format was chosen to carry, and
	// the derivation notes that are the only surviving record of a judgement
	// nothing else can re-derive.
	//
	// It is nil on every rejection, alongside the rest of the zero Result.
	Source []byte
}

// LoadFile loads the theme file at path, returning either its slug and palette
// or EXACTLY ONE Rejection saying why it is not usable.
//
// The six checks are the rejection ladder and run in a fixed order,
// short-circuiting at the first failure:
//
//  1. `bad name` — from filepath.Base(path) alone, BEFORE the file is opened, so
//     a bad-name file can never also report `unreadable` or anything about its
//     contents. Both of its causes live here, and both mean the file yields no
//     usable slug — which is what lets the next rung assume one exists.
//  2. `reserved name` — likewise decided from the slug alone, before any read.
//     Unreachable for a bad-name file, which has no slug to collide.
//  3. `unreadable` — the read itself failed.
//  4. `bad syntax` — a lexical failure aborts the parse, so no value-level or
//     presence check runs.
//  5. `bad colour` — value validation across every known key.
//  6. `missing tokens` — the presence check runs last, on a file that parsed and
//     whose every known value is well-formed.
//
// The order is the whole point of the function: without it a file that is both
// duplicate-keyed and missing tokens has two defensible answers, the panel's
// single-reason row becomes a choice rather than a fact, and doctor and the
// panel become capable of disagreeing about the same file.
//
// Rungs 4 to 6 are parseThemeBytes, which LoadBuiltin runs on the embedded set —
// so a built-in and a stranger's file are judged by the same code. The first
// three rungs are this function's alone, being about a path.
//
// `not found` — the seventh reason — is deliberately outside this ladder and is
// NEVER returned here. It applies only to a slug named by prefs.json with no
// corresponding file, where there is nothing to check; producing it from a path
// would send a user to look for a file this function was just handed.
//
// The two filename rungs carry NO DETAIL, while the four that read the file all
// do. `bad name` and `reserved name` have their own pinned doctor lines, each
// composed from the filename and slug the caller already holds — naming the
// conflict and, for a reserved slug, the fix — so a detail here would be a
// second, competing copy of that copy.
//
// On rejection the Result is the zero value. A caller never sees a slug or a
// partly populated palette alongside a rejection.
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

// LoadPath loads the theme file at path as an EXPLICIT INPUT, returning its
// palette or exactly one Rejection from the four CONTENT rungs — `unreadable`,
// `bad syntax`, `bad colour`, `missing tokens` — in that order.
//
// It runs neither filename rung and derives NO SLUG, which is the whole
// difference from LoadFile. A path handed in by a caller is an input rather than
// a directory entry: nothing enumerated it, nothing will resolve it by name
// again, and a Theme has no identity field — so a theme loaded this way simply
// has none, and the Result's Slug is empty. Judging such a file by its filename
// would reject `~/work/mytheme.txt` for rules that exist to keep the THEMES
// DIRECTORY unambiguous, where this file does not live.
//
// Its caller is `capturetool --theme <path>`, the visual-verification route a
// drop-in author has. The filename reasons still MATTER to that author — a fatal
// name is worth catching before the file reaches the themes directory — but they
// are the caller's warning to emit from the basename, not this function's verdict
// to refuse on.
//
// Rungs 3 to 6 are the same code LoadFile and LoadBuiltin run (parseThemeBytes,
// preceded by the same read), so a file judged here and the same file judged as a
// directory entry can differ only in the two rungs deliberately skipped.
//
// The receiver is unused for the same reason LoadBuiltin's is: neither the
// reserved-slug set nor the event seam bears on an input the caller named itself
// — the log component reports on the directory enumeration, not on a file handed
// in. It stays a method so a caller reaches every entry point through the one
// Loader it holds.
//
// On rejection the Result is the zero value, exactly as LoadFile's is.
func (l Loader) LoadPath(path string) (Result, *Rejection) {
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
// It is the WHOLE of the content half of the ladder, and it is deliberately the
// only one: LoadFile reaches it with bytes off the disk and LoadBuiltin with
// bytes out of the embedded set, so a built-in and a stranger's drop-in are
// parsed by the same code and judged by the same rule. A second parse path is
// what would let a format bug hide behind a Go-side built-in, so no caller may
// lex or validate on its own.
//
// The two rungs ABOVE it — `bad name` and `reserved name` — are the file
// path's, and are deliberately outside: they are decided from a FILENAME, which
// an embedded built-in has no equivalent of.
//
// A failure here is an ordinary rejection, never a panic. Escalating a broken
// built-in is the job of the place a fallback is needed, not of the place a file
// is read.
func parseThemeBytes(data []byte) (Theme, *Rejection) {
	pairs, rejection := lexPairs(data)
	if rejection != nil {
		return Theme{}, rejection
	}
	return themeFromPairs(pairs)
}

// isReserved reports whether slug is one of the injected built-in slugs.
//
// The comparison is EXACT STRING EQUALITY, which the reject-never-normalise rule
// is what makes safe: a name is never lowercased into a slug, so
// `Nord.theme` yields no slug at all rather than one that would collide here —
// and a case-insensitive filesystem cannot smuggle a shadowing file past the
// check.
func (l Loader) isReserved(slug string) bool {
	_, reserved := l.ReservedSlugs[slug]
	return reserved
}

// unreadable builds the one rejection a failed read produces, from the one
// source that knows anything about the failure: the OS error.
//
// It is rendered into the detail VERBATIM, which is the pinned format for this
// reason — the error text is the only thing distinguishing a permission denial
// from a dangling symlink, and doctor is where a verbatim system message
// belongs. The same error is retained on Err so a caller can match on it
// structurally instead of reading the rendered line back apart.
func unreadable(err error) *Rejection {
	return &Rejection{Reason: ReasonUnreadable, Detail: err.Error(), Err: err}
}
