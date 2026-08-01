package theme_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// TestLoadFile_ValidThemeReturnsSlugAndTheme pins the accepting path end to end:
// the slug comes from the filename, the palette from the contents, and the 19
// values arrive in §4.3's canonical upper case even though the file wrote them
// lower.
func TestLoadFile_ValidThemeReturnsSlugAndTheme(t *testing.T) {
	path := writeTheme(t, t.TempDir(), "nord-lee.theme", themeLines())

	got, rejection := theme.Loader{}.LoadFile(path)

	if rejection != nil {
		t.Fatalf("LoadFile(%q) rejected the file: %v", path, rejection)
	}
	if want := "nord-lee"; got.Slug != want {
		t.Errorf("slug = %q, want %q", got.Slug, want)
	}
	if tokens, want := got.Theme.All(), wantThemeTokens(); !slices.Equal(tokens, want) {
		t.Errorf("theme = %+v, want %+v", tokens, want)
	}
}

// TestLoadFile_LadderShortCircuits is the pin the whole function exists for:
// every file below fails TWO of §6.2's rungs at once, and each reports only the
// higher one.
//
// Nothing else in the feature asserts this ordering, and without it a file that
// is simultaneously duplicate-keyed and missing tokens has two defensible
// answers — which would make the panel's single-reason row a choice rather than
// a fact, and let doctor and the panel disagree about the same file.
func TestLoadFile_LadderShortCircuits(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, dir string) (theme.Loader, string)
		wantReason theme.Reason
		wantDetail string
	}{
		{
			name: "bad name beats unreadable",
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				return theme.Loader{}, filepath.Join(dir, "no-such-directory", "Nord.theme")
			},
			wantReason: theme.ReasonBadName,
		},
		{
			name: "reserved name beats bad colour",
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				loader := theme.Loader{ReservedSlugs: map[string]struct{}{"nord": {}}}
				return loader, writeTheme(t, dir, "nord.theme", withValue(themeLines(), "canvas", "blue"))
			},
			wantReason: theme.ReasonReservedName,
		},
		{
			name: "bad syntax beats bad colour",
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				lines := withValue(themeLines(), "canvas", "blue")
				lines = withValue(lines, "text.primary", `"#C0CAF5"`)
				return theme.Loader{}, writeTheme(t, dir, "nord-lee.theme", lines)
			},
			wantReason: theme.ReasonBadSyntax,
			wantDetail: "line 1: quoted value",
		},
		{
			name: "bad syntax beats missing tokens",
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				// 19 tokens less bg.subtle is 18 lines, so the duplicate lands on line 19.
				lines := append(withoutKey(themeLines(), "bg.subtle"), "text.primary = #010203")
				return theme.Loader{}, writeTheme(t, dir, "nord-lee.theme", lines)
			},
			wantReason: theme.ReasonBadSyntax,
			wantDetail: "line 19: duplicate key text.primary",
		},
		{
			name: "bad colour beats missing tokens",
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				lines := withValue(withoutKey(themeLines(), "bg.subtle"), "canvas", "blue")
				return theme.Loader{}, writeTheme(t, dir, "nord-lee.theme", lines)
			},
			wantReason: theme.ReasonBadColour,
			wantDetail: "canvas = blue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader, path := tt.setup(t, t.TempDir())

			got, rejection := loader.LoadFile(path)

			requireLoadRejection(t, got, rejection, tt.wantReason, tt.wantDetail)
		})
	}
}

// TestLoadFile_BadNameDecidedBeforeOpen pins rung 1's position: the FILENAME is
// checked before the file is opened, so a bad-name file can never also report
// `unreadable` or anything about its contents.
//
// Every path here is inside a directory that does not exist, so a read would
// certainly fail — which is what makes the verdict evidence of ordering rather
// than a coincidence. Both causes are covered, because both live on this rung.
func TestLoadFile_BadNameDecidedBeforeOpen(t *testing.T) {
	tests := []struct {
		name      string
		base      string
		wantCause theme.BadNameCause
	}{
		{name: "uppercase stem", base: "Nord.theme", wantCause: theme.BadNameSlug},
		{name: "leading hyphen stem", base: "-nord.theme", wantCause: theme.BadNameSlug},
		{name: "shouted extension", base: "nord.THEME", wantCause: theme.BadNameExtension},
		{name: "no extension at all", base: "nord", wantCause: theme.BadNameExtension},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "no-such-directory", tt.base)

			got, rejection := theme.Loader{}.LoadFile(path)

			requireLoadRejection(t, got, rejection, theme.ReasonBadName, "")
			if rejection.BadNameCause != tt.wantCause {
				t.Errorf("rejection cause = %v, want %v", rejection.BadNameCause, tt.wantCause)
			}
			if rejection.Err != nil {
				t.Errorf("rejection carries Err %v, want nil — the file was never opened", rejection.Err)
			}
		})
	}
}

// TestLoadFile_ReservedNameDecidedFromSlugAlone pins rung 2 and §5.4's
// no-shadowing rule: a user file whose slug collides with a built-in is rejected
// whatever its contents are — including when there are no contents to have.
//
// The three fixtures are the three states the file can be in behind the same
// name, and all three land on the same rung, which is what "decided from the
// slug alone, before any read" means. The unreserved neighbour is the near miss:
// reservation is EXACT STRING EQUALITY, so `nord-lee` is not a collision with
// `nord` and loads normally.
func TestLoadFile_ReservedNameDecidedFromSlugAlone(t *testing.T) {
	loader := theme.Loader{ReservedSlugs: map[string]struct{}{"nord": {}, "tokyo-night": {}}}

	states := []struct {
		name string
		make func(t *testing.T, dir string) string
	}{
		{
			name: "perfectly valid contents",
			make: func(t *testing.T, dir string) string {
				return writeTheme(t, dir, "nord.theme", themeLines())
			},
		},
		{
			name: "no file at all",
			make: func(t *testing.T, dir string) string {
				return filepath.Join(dir, "nord.theme")
			},
		},
		{
			name: "an unreadable file",
			make: func(t *testing.T, dir string) string {
				return writeUnreadableTheme(t, dir, "nord.theme")
			},
		},
	}

	for _, tt := range states {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.make(t, t.TempDir())

			got, rejection := loader.LoadFile(path)

			requireLoadRejection(t, got, rejection, theme.ReasonReservedName, "")
		})
	}

	t.Run("an unreserved neighbour still loads", func(t *testing.T) {
		path := writeTheme(t, t.TempDir(), "nord-lee.theme", themeLines())

		got, rejection := loader.LoadFile(path)

		if rejection != nil {
			t.Fatalf("LoadFile(%q) rejected an unreserved slug: %v", path, rejection)
		}
		if want := "nord-lee"; got.Slug != want {
			t.Errorf("slug = %q, want %q", got.Slug, want)
		}
	})
}

// TestLoadFile_EmptyReservedSetNeverRejects pins this phase's wiring: the
// built-in slug set is INJECTED and empty until the embedded themes exist, so
// rung 2 is implemented and dormant — no input can produce `reserved name` yet.
//
// Both empty forms are covered because both are reachable: a zero-value Loader
// carries a nil map, and a caller assembling a set from an empty built-in
// collection produces an allocated one.
//
// The slugs are the ones that WILL be reserved, so the day the set is populated
// this test says exactly which wiring changed rather than failing obscurely.
func TestLoadFile_EmptyReservedSetNeverRejects(t *testing.T) {
	loaders := []struct {
		name   string
		loader theme.Loader
	}{
		{name: "zero value — a nil set", loader: theme.Loader{}},
		{name: "an allocated empty set", loader: theme.Loader{ReservedSlugs: map[string]struct{}{}}},
	}

	files := []struct {
		name  string
		base  string
		lines []string
	}{
		{name: "a would-be built-in slug", base: "nord.theme", lines: themeLines()},
		{name: "a second would-be built-in slug", base: "tokyo-night.theme", lines: themeLines()},
		{name: "a would-be built-in slug on an invalid file", base: "nord.theme", lines: withoutKey(themeLines(), "canvas")},
	}

	for _, tl := range loaders {
		for _, tf := range files {
			t.Run(tl.name+"/"+tf.name, func(t *testing.T) {
				path := writeTheme(t, t.TempDir(), tf.base, tf.lines)

				_, rejection := tl.loader.LoadFile(path)

				if rejection != nil && rejection.Reason == theme.ReasonReservedName {
					t.Errorf("LoadFile(%q) = %q, want no reserved-name rejection from an empty built-in set", path, rejection.Reason)
				}
			})
		}
	}
}

// TestLoadFile_UnreadableKeepsOSErrorVerbatim pins rung 3 and §14A's detail for
// it. `unreadable` covers EVERY read failure, not only permissions, and the OS
// error is kept verbatim because it is the only thing distinguishing a
// permission denial from a dangling symlink — the two cases below.
//
// The expected error is taken from os.ReadFile on the same path rather than
// spelled out, because its text is the platform's rather than Portal's; what is
// pinned is that nothing rewords, wraps or truncates it.
func TestLoadFile_UnreadableKeepsOSErrorVerbatim(t *testing.T) {
	tests := []struct {
		name string
		make func(t *testing.T, dir, base string) string
	}{
		{name: "unreadable file", make: writeUnreadableTheme},
		{name: "dangling symlink", make: writeDanglingThemeLink},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.make(t, t.TempDir(), "nord-lee.theme")

			_, readErr := os.ReadFile(path)
			if readErr == nil {
				t.Fatalf("os.ReadFile(%q) succeeded, the fixture is not unreadable", path)
			}

			got, rejection := theme.Loader{}.LoadFile(path)

			requireLoadRejection(t, got, rejection, theme.ReasonUnreadable, readErr.Error())
			if rejection.Err == nil {
				t.Fatalf("rejection carries no Err, want the OS error %v", readErr)
			}
			if rejection.Err.Error() != readErr.Error() {
				t.Errorf("rejection.Err = %v, want the OS error verbatim: %v", rejection.Err, readErr)
			}
		})
	}
}

// TestLoadFile_NotFoundIsOutsideTheLadder pins §6.2's seventh reason as
// deliberately unreachable from a path: `not found` applies only to a slug named
// by prefs.json with no corresponding file (§9.4), where there is nothing to
// check. An absent file handed to LoadFile is `unreadable`, which is the reason
// that sends the user to check permissions rather than the filename.
//
// The corpus is every rung plus the accepting path, and each case states the
// reason it DOES produce, so the test cannot pass by failing to reach the loader.
func TestLoadFile_NotFoundIsOutsideTheLadder(t *testing.T) {
	for _, tt := range rejectionCorpus() {
		t.Run(tt.name, func(t *testing.T) {
			loader, path := tt.setup(t, t.TempDir())

			_, rejection := loader.LoadFile(path)

			if tt.wantReason == "" {
				if rejection != nil {
					t.Fatalf("LoadFile(%q) rejected a valid file: %v", path, rejection)
				}
				return
			}
			if rejection == nil {
				t.Fatalf("LoadFile(%q) accepted the file, want %q", path, tt.wantReason)
			}
			if rejection.Reason == theme.ReasonNotFound {
				t.Errorf("LoadFile(%q) = %q, want %q — not found is outside the ladder", path, rejection.Reason, tt.wantReason)
			}
			if rejection.Reason != tt.wantReason {
				t.Errorf("LoadFile(%q) = %q, want %q", path, rejection.Reason, tt.wantReason)
			}
		})
	}
}

// TestLoadFile_DetailNeverSpansTwoReasons pins §6.2's other half: doctor
// enumerates WITHIN the reason, never across reasons, so it never reports a file
// as both `bad colour` and `missing tokens`.
//
// One file is repaired one fault at a time. It starts with three faults at once,
// and each state asserts the whole detail plus the absence of every fault the
// file still has — which is what distinguishes "the detail is scoped" from "the
// detail happens to look right".
func TestLoadFile_DetailNeverSpansTwoReasons(t *testing.T) {
	// 19 tokens less bg.subtle is 18 lines, so an appended duplicate lands on line 19.
	badColourAndMissing := withValue(withoutKey(themeLines(), "bg.subtle"), "canvas", "blue")

	tests := []struct {
		name          string
		lines         []string
		wantReason    theme.Reason
		wantDetail    string
		wantNoMention []string
	}{
		{
			name:          "duplicate key, bad colour and a missing token",
			lines:         append(slices.Clone(badColourAndMissing), "text.primary = #010203"),
			wantReason:    theme.ReasonBadSyntax,
			wantDetail:    "line 19: duplicate key text.primary",
			wantNoMention: []string{"canvas", "blue", "missing", "bg.subtle"},
		},
		{
			name:          "the duplicate repaired, bad colour and a missing token left",
			lines:         badColourAndMissing,
			wantReason:    theme.ReasonBadColour,
			wantDetail:    "canvas = blue",
			wantNoMention: []string{"line ", "missing", "bg.subtle"},
		},
		{
			name:          "the colour repaired, a missing token left",
			lines:         withValue(badColourAndMissing, "canvas", "#0b0c14"),
			wantReason:    theme.ReasonMissingTokens,
			wantDetail:    "missing bg.subtle",
			wantNoMention: []string{"line ", "canvas", "blue"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTheme(t, t.TempDir(), "nord-lee.theme", tt.lines)

			got, rejection := theme.Loader{}.LoadFile(path)

			requireLoadRejection(t, got, rejection, tt.wantReason, tt.wantDetail)
			for _, foreign := range tt.wantNoMention {
				if strings.Contains(rejection.Detail, foreign) {
					t.Errorf("detail %q mentions %q, which belongs to another reason", rejection.Detail, foreign)
				}
			}
		})
	}
}

// loadCase is one staged file plus the verdict the ladder must reach on it. An
// empty wantReason means the file loads.
type loadCase struct {
	name       string
	setup      func(t *testing.T, dir string) (theme.Loader, string)
	wantReason theme.Reason
}

// rejectionCorpus stages one file per §6.2 rung, plus the accepting path and the
// three shapes of read failure — the absent file among them, which is the case
// `not found` would otherwise be tempting for.
func rejectionCorpus() []loadCase {
	return []loadCase{
		{
			name: "a valid file",
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				return theme.Loader{}, writeTheme(t, dir, "nord-lee.theme", themeLines())
			},
		},
		{
			name:       "a bad name",
			wantReason: theme.ReasonBadName,
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				return theme.Loader{}, writeTheme(t, dir, "Nord.theme", themeLines())
			},
		},
		{
			name:       "a reserved name",
			wantReason: theme.ReasonReservedName,
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				loader := theme.Loader{ReservedSlugs: map[string]struct{}{"nord": {}}}
				return loader, writeTheme(t, dir, "nord.theme", themeLines())
			},
		},
		{
			name:       "an absent file",
			wantReason: theme.ReasonUnreadable,
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				return theme.Loader{}, filepath.Join(dir, "nord-lee.theme")
			},
		},
		{
			name:       "an unreadable file",
			wantReason: theme.ReasonUnreadable,
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				return theme.Loader{}, writeUnreadableTheme(t, dir, "nord-lee.theme")
			},
		},
		{
			name:       "a dangling symlink",
			wantReason: theme.ReasonUnreadable,
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				return theme.Loader{}, writeDanglingThemeLink(t, dir, "nord-lee.theme")
			},
		},
		{
			name:       "a duplicate key",
			wantReason: theme.ReasonBadSyntax,
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				lines := append(themeLines(), "text.primary = #010203")
				return theme.Loader{}, writeTheme(t, dir, "nord-lee.theme", lines)
			},
		},
		{
			name:       "a bad colour",
			wantReason: theme.ReasonBadColour,
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				return theme.Loader{}, writeTheme(t, dir, "nord-lee.theme", withValue(themeLines(), "canvas", "blue"))
			},
		},
		{
			name:       "a missing token",
			wantReason: theme.ReasonMissingTokens,
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				return theme.Loader{}, writeTheme(t, dir, "nord-lee.theme", withoutKey(themeLines(), "bg.subtle"))
			},
		},
		{
			name:       "an empty file",
			wantReason: theme.ReasonMissingTokens,
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				return theme.Loader{}, writeTheme(t, dir, "nord-lee.theme", nil)
			},
		},
	}
}

// themeLines renders a complete, valid theme file: one `key = value` line per
// §2.4 token, in table order, each carrying a distinct value.
//
// The names come from TokenNames() rather than a restatement — theme_test.go
// holds the one deliberate restatement of the vocabulary — and the values are
// LOWER case, so a theme parsed from these lines proves canonicalisation rather
// than merely echoing the file.
func themeLines() []string {
	names := theme.TokenNames()
	lines := make([]string, 0, len(names))
	for i, name := range names {
		lines = append(lines, name+" = "+lowerHexForRow(i+1))
	}
	return lines
}

// lowerHexForRow renders a distinct well-formed value for the given §2.4 row,
// carrying lower-case letters so upper-casing has something to bite on.
func lowerHexForRow(row int) string {
	return fmt.Sprintf("#abcd%02x", row)
}

// wantThemeTokens renders the tokens themeLines() must parse into: the same
// values in §4.3's canonical upper case, in §2.4 order.
func wantThemeTokens() []theme.Token {
	names := theme.TokenNames()
	tokens := make([]theme.Token, 0, len(names))
	for i, name := range names {
		tokens = append(tokens, theme.Token{Name: name, Value: strings.ToUpper(lowerHexForRow(i + 1))})
	}
	return tokens
}

// withValue returns the lines with the named key's value replaced, in the same
// file order.
func withValue(lines []string, key, value string) []string {
	replaced := slices.Clone(lines)
	for i, line := range replaced {
		if strings.HasPrefix(line, key+" = ") {
			replaced[i] = key + " = " + value
		}
	}
	return replaced
}

// withoutKey returns the lines with the named key's line removed — a file that
// never declared it.
func withoutKey(lines []string, key string) []string {
	return slices.DeleteFunc(slices.Clone(lines), func(line string) bool {
		return strings.HasPrefix(line, key+" = ")
	})
}

// writeTheme writes lines as a file named base inside dir and returns its path.
func writeTheme(t *testing.T, dir, base string, lines []string) string {
	t.Helper()

	path := filepath.Join(dir, base)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// writeUnreadableTheme writes a perfectly valid theme file and then removes
// every permission bit, so the only thing wrong with it is that the bytes cannot
// be read.
//
// Mode bits do not deny root, so the fixture is impossible there and the test
// skips rather than asserting something the platform will not do. The mode is
// restored on cleanup so the temp dir tears down predictably.
func writeUnreadableTheme(t *testing.T, dir, base string) string {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("running as root: mode bits do not deny, so an unreadable file cannot be staged")
	}

	path := writeTheme(t, dir, base, themeLines())
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(path, 0o644); err != nil {
			t.Errorf("restore mode on %s: %v", path, err)
		}
	})

	return path
}

// writeDanglingThemeLink stages a theme file that enumerates but cannot be read
// for an entirely different reason: it is a symlink to a file that is not there.
func writeDanglingThemeLink(t *testing.T, dir, base string) string {
	t.Helper()

	path := filepath.Join(dir, base)
	if err := os.Symlink(filepath.Join(dir, "absent-target.theme"), path); err != nil {
		t.Fatalf("symlink %s: %v", path, err)
	}
	return path
}

// requireLoadRejection fails the test unless LoadFile rejected with the one
// expected reason and the complete §14A detail.
//
// It also pins the invariants every rejection from the ladder shares: no
// populated Result comes back alongside one — never a slug, never a partial
// palette, never the source bytes — and Line is 0 for every reason but
// `bad syntax`, which is the only one that has a line.
//
// The zero-Result check is a DeepEqual rather than a ==: a Result carries the
// bytes it parsed, which makes the struct uncomparable. The assertion is
// unchanged.
func requireLoadRejection(t *testing.T, got theme.Result, rejection *theme.Rejection, wantReason theme.Reason, wantDetail string) {
	t.Helper()

	if rejection == nil {
		t.Fatalf("LoadFile() accepted the file as %+v, want the rejection %q: %s", got, wantReason, wantDetail)
	}
	if !reflect.DeepEqual(got, theme.Result{}) {
		t.Errorf("LoadFile() returned %+v alongside a rejection, want the zero Result", got)
	}
	if rejection.Reason != wantReason {
		t.Errorf("rejection reason = %q, want %q", rejection.Reason, wantReason)
	}
	if rejection.Detail != wantDetail {
		t.Errorf("rejection detail = %q, want %q", rejection.Detail, wantDetail)
	}
	if rejection.Reason != theme.ReasonBadSyntax && rejection.Line != 0 {
		t.Errorf("rejection line = %d, want 0 — only bad syntax carries a line", rejection.Line)
	}
}
