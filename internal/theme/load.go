package theme

import (
	"os"
	"path/filepath"
)

// Loader turns one theme file into a Theme, running §6.2's fixed rejection
// ladder over it.
//
// It HARDCODES NO SLUGS and RESOLVES NO PATHS. Both are injected, and both for
// the same reason: the loader is driven by four callers with different
// authority — TUI construction, the panel, portal doctor and portal theme
// export — and none of them may have config discovery imposed on it. The path to
// read arrives as an argument (§5.5's themes-directory chain lives in cmd), and
// the reserved built-in slugs arrive on the Loader.
type Loader struct {
	// ReservedSlugs is the set of built-in slugs a user file may not take
	// (§5.4). It is INJECTED rather than read from the embedded set here, so a
	// test can exercise the rung with a synthetic set and this package never
	// decides which names are Portal's.
	//
	// The zero value — a nil map — reserves nothing, which is exactly what a
	// Loader constructed before the built-in set exists wants: no input can
	// then produce `reserved name`.
	ReservedSlugs map[string]struct{}
}

// Result is one loaded theme: the slug its FILENAME yielded, and the palette its
// CONTENTS declared.
//
// The two are held side by side rather than folded together because a Theme
// carries no identity field (§3.2) — a slug belongs to whatever loaded the
// palette, not to the palette.
type Result struct {
	Slug  string
	Theme Theme
}

// LoadFile loads the theme file at path, returning either its slug and palette
// or EXACTLY ONE Rejection saying why it is not usable.
//
// The six checks are §6.2's ladder and run in its fixed order, short-circuiting
// at the first failure:
//
//  1. `bad name` — from filepath.Base(path) alone, BEFORE the file is opened, so
//     a bad-name file can never also report `unreadable` or anything about its
//     contents. Both of its causes live here, and both mean the file yields no
//     usable slug — which is what lets rung 2 assume one exists.
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
// single-reason row (§9.5) becomes a choice rather than a fact, and doctor and
// the panel become capable of disagreeing about the same file.
//
// `not found` — §6.2's seventh reason — is deliberately outside this ladder and
// is NEVER returned here. It applies only to a slug named by prefs.json with no
// corresponding file (§9.4), where there is nothing to check; producing it from
// a path would send a user to look for a file this function was just handed.
//
// The two filename rungs carry NO DETAIL, while the four that read the file all
// do. §14A gives `bad name` and `reserved name` their own pinned doctor lines,
// each composed from the filename and slug the caller already holds — naming the
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

	pairs, rejection := lexPairs(data)
	if rejection != nil {
		return Result{}, rejection
	}

	built, rejection := themeFromPairs(pairs)
	if rejection != nil {
		return Result{}, rejection
	}

	return Result{Slug: slug, Theme: built}, nil
}

// isReserved reports whether slug is one of the injected built-in slugs.
//
// The comparison is EXACT STRING EQUALITY, which §5.2's reject-never-normalise
// rule is what makes safe: a name is never lowercased into a slug, so
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
// It is rendered into the detail VERBATIM, which is §14A's format for this
// reason — the error text is the only thing distinguishing a permission denial
// from a dangling symlink, and doctor is where a verbatim system message
// belongs. The same error is retained on Err so a caller can match on it
// structurally instead of reading the rendered line back apart.
func unreadable(err error) *Rejection {
	return &Rejection{Reason: ReasonUnreadable, Detail: err.Error(), Err: err}
}
