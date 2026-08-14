package theme_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

func TestLoadFile_ValidThemeReturnsSlugAndTheme(t *testing.T) {
	path := themetest.Write(t, t.TempDir(), "nord-lee.theme", themetest.Lines())

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
				return loader, themetest.WriteWithCanvas(t, dir, "nord.theme", "blue")
			},
			wantReason: theme.ReasonReservedName,
		},
		{
			name: "bad syntax beats bad colour",
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				lines := themetest.LinesWithCanvas("blue")
				lines = themetest.WithValue(lines, "text.primary", `"#C0CAF5"`)
				return theme.Loader{}, themetest.Write(t, dir, "nord-lee.theme", lines)
			},
			wantReason: theme.ReasonBadSyntax,
			wantDetail: "line 1: quoted value",
		},
		{
			name: "bad syntax beats missing tokens",
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				lines := append(themetest.WithoutKey(themetest.Lines(), "bg.subtle"), "text.primary = #010203")
				return theme.Loader{}, themetest.Write(t, dir, "nord-lee.theme", lines)
			},
			wantReason: theme.ReasonBadSyntax,
			wantDetail: "line 19: duplicate key text.primary",
		},
		{
			name: "bad colour beats missing tokens",
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				lines := themetest.WithValue(themetest.WithoutKey(themetest.Lines(), "bg.subtle"), "canvas", "blue")
				return theme.Loader{}, themetest.Write(t, dir, "nord-lee.theme", lines)
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

func TestLoadFile_ReservedNameDecidedFromSlugAlone(t *testing.T) {
	loader := theme.Loader{ReservedSlugs: map[string]struct{}{"nord": {}, "tokyo-night": {}}}

	states := []struct {
		name string
		make func(t *testing.T, dir string) string
	}{
		{
			name: "perfectly valid contents",
			make: func(t *testing.T, dir string) string {
				return themetest.Write(t, dir, "nord.theme", themetest.Lines())
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
				path := themetest.Write(t, dir, "nord.theme", themetest.Lines())
				_ = themetest.DenyRead(t, path)
				return path
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
		path := themetest.Write(t, t.TempDir(), "nord-lee.theme", themetest.Lines())

		got, rejection := loader.LoadFile(path)

		if rejection != nil {
			t.Fatalf("LoadFile(%q) rejected an unreserved slug: %v", path, rejection)
		}
		if want := "nord-lee"; got.Slug != want {
			t.Errorf("slug = %q, want %q", got.Slug, want)
		}
	})
}

func TestLoadFile_EmptyInjectedReservedSetNeverRejects(t *testing.T) {
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
		{name: "a built-in slug", base: "tokyo-night.theme", lines: themetest.Lines()},
		{name: "another built-in slug", base: "nord.theme", lines: themetest.Lines()},
		{name: "a built-in slug on an invalid file", base: "tokyo-night.theme", lines: themetest.WithoutKey(themetest.Lines(), "canvas")},
	}

	for _, tl := range loaders {
		for _, tf := range files {
			t.Run(tl.name+"/"+tf.name, func(t *testing.T) {
				path := themetest.Write(t, t.TempDir(), tf.base, tf.lines)

				_, rejection := tl.loader.LoadFile(path)

				if rejection != nil && rejection.Reason == theme.ReasonReservedName {
					t.Errorf("LoadFile(%q) = %q, want no reserved-name rejection from an empty built-in set", path, rejection.Reason)
				}
			})
		}
	}
}

func TestLoadFile_UnreadableKeepsOSErrorVerbatim(t *testing.T) {
	tests := []struct {
		name string
		make func(t *testing.T, dir, base string) string
	}{
		{
			name: "unreadable file",
			make: func(t *testing.T, dir, base string) string {
				path := themetest.Write(t, dir, base, themetest.Lines())
				_ = themetest.DenyRead(t, path)
				return path
			},
		},
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

func TestLoadFile_TokensCarriedOnlyByTheReasonsThatNameTokens(t *testing.T) {
	naming := map[theme.Reason]bool{
		theme.ReasonMissingTokens: true,
		theme.ReasonBadColour:     true,
	}

	for _, tt := range rejectionCorpus() {
		t.Run(tt.name, func(t *testing.T) {
			loader, path := tt.setup(t, t.TempDir())

			_, rejection := loader.LoadFile(path)

			if rejection == nil {
				return
			}
			if naming[rejection.Reason] {
				if len(rejection.Tokens) == 0 {
					t.Fatalf("reason %q carried no tokens, want the list its detail %q renders", rejection.Reason, rejection.Detail)
				}
				return
			}
			if len(rejection.Tokens) != 0 {
				t.Errorf("reason %q carried tokens %v, want none — the reason names no token", rejection.Reason, rejection.Tokens)
			}
		})
	}
}

func TestLoadFile_DetailNeverSpansTwoReasons(t *testing.T) {
	badColourAndMissing := themetest.WithValue(themetest.WithoutKey(themetest.Lines(), "bg.subtle"), "canvas", "blue")

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
			lines:         themetest.WithValue(badColourAndMissing, "canvas", "#0b0c14"),
			wantReason:    theme.ReasonMissingTokens,
			wantDetail:    "missing bg.subtle",
			wantNoMention: []string{"line ", "canvas", "blue"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := themetest.Write(t, t.TempDir(), "nord-lee.theme", tt.lines)

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

func TestLoadPath_DerivesNoSlugAndRunsNoFilenameRung(t *testing.T) {
	reserving := theme.Loader{ReservedSlugs: map[string]struct{}{"nord": {}, "tokyo-night": {}}}

	tests := []struct {
		name             string
		loader           theme.Loader
		base             string
		wantFileRejected theme.Reason
	}{
		{
			name:             "an upper-case stem",
			loader:           theme.Loader{},
			base:             "Nord.theme",
			wantFileRejected: theme.ReasonBadName,
		},
		{
			name:             "a shouted extension",
			loader:           theme.Loader{},
			base:             "nord.THEME",
			wantFileRejected: theme.ReasonBadName,
		},
		{
			name:             "an unexpected extension",
			loader:           theme.Loader{},
			base:             "mytheme.txt",
			wantFileRejected: theme.ReasonBadName,
		},
		{
			name:             "a reserved slug",
			loader:           reserving,
			base:             "nord.theme",
			wantFileRejected: theme.ReasonReservedName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := themetest.Write(t, t.TempDir(), tt.base, themetest.Lines())

			if _, rejection := tt.loader.LoadFile(path); rejection == nil || rejection.Reason != tt.wantFileRejected {
				t.Fatalf("LoadFile(%q) = %v, want %q — the fixture does not exercise a filename rung", path, rejection, tt.wantFileRejected)
			}

			got, rejection := theme.LoadPath(path)

			if rejection != nil {
				t.Fatalf("LoadPath(%q) rejected the file: %v", path, rejection)
			}
			if got.Slug != "" {
				t.Errorf("LoadPath(%q).Slug = %q, want empty — an explicit path yields no identity", path, got.Slug)
			}
			if tokens, want := got.Theme.All(), wantThemeTokens(); !slices.Equal(tokens, want) {
				t.Errorf("theme = %+v, want %+v", tokens, want)
			}
			if want, err := os.ReadFile(path); err != nil {
				t.Fatalf("read %s: %v", path, err)
			} else if !slices.Equal(got.Source, want) {
				t.Errorf("Source = %q, want the file's bytes %q", got.Source, want)
			}
		})
	}
}

func TestLoadPath_RunsTheContentRungs(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, dir string) (path, wantDetail string)
		wantReason theme.Reason
	}{
		{
			name:       "a duplicate key",
			wantReason: theme.ReasonBadSyntax,
			setup: func(t *testing.T, dir string) (string, string) {
				lines := append(themetest.Lines(), "text.primary = #010203")
				return themetest.Write(t, dir, "nord-lee.theme", lines), "line 20: duplicate key text.primary"
			},
		},
		{
			name:       "a bad colour",
			wantReason: theme.ReasonBadColour,
			setup: func(t *testing.T, dir string) (string, string) {
				return themetest.WriteWithCanvas(t, dir, "nord-lee.theme", "blue"), "canvas = blue"
			},
		},
		{
			name:       "a missing token",
			wantReason: theme.ReasonMissingTokens,
			setup: func(t *testing.T, dir string) (string, string) {
				lines := themetest.WithoutKey(themetest.Lines(), "bg.subtle")
				return themetest.Write(t, dir, "nord-lee.theme", lines), "missing bg.subtle"
			},
		},
		{
			name:       "an unreadable file",
			wantReason: theme.ReasonUnreadable,
			setup: func(t *testing.T, dir string) (string, string) {
				path := themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())
				return path, themetest.DenyRead(t, path).Error()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, wantDetail := tt.setup(t, t.TempDir())

			got, rejection := theme.LoadPath(path)

			requireLoadRejection(t, got, rejection, tt.wantReason, wantDetail)
		})
	}
}

type loadCase struct {
	name       string
	setup      func(t *testing.T, dir string) (theme.Loader, string)
	wantReason theme.Reason
}

func rejectionCorpus() []loadCase {
	return []loadCase{
		{
			name: "a valid file",
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				return theme.Loader{}, themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())
			},
		},
		{
			name:       "a bad name",
			wantReason: theme.ReasonBadName,
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				return theme.Loader{}, themetest.Write(t, dir, "Nord.theme", themetest.Lines())
			},
		},
		{
			name:       "a reserved name",
			wantReason: theme.ReasonReservedName,
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				loader := theme.Loader{ReservedSlugs: map[string]struct{}{"nord": {}}}
				return loader, themetest.Write(t, dir, "nord.theme", themetest.Lines())
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
				path := themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())
				_ = themetest.DenyRead(t, path)
				return theme.Loader{}, path
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
				lines := append(themetest.Lines(), "text.primary = #010203")
				return theme.Loader{}, themetest.Write(t, dir, "nord-lee.theme", lines)
			},
		},
		{
			name:       "a bad colour",
			wantReason: theme.ReasonBadColour,
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				return theme.Loader{}, themetest.WriteWithCanvas(t, dir, "nord-lee.theme", "blue")
			},
		},
		{
			name:       "a missing token",
			wantReason: theme.ReasonMissingTokens,
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				return theme.Loader{}, themetest.Write(t, dir, "nord-lee.theme", themetest.WithoutKey(themetest.Lines(), "bg.subtle"))
			},
		},
		{
			name:       "an empty file",
			wantReason: theme.ReasonMissingTokens,
			setup: func(t *testing.T, dir string) (theme.Loader, string) {
				return theme.Loader{}, themetest.Write(t, dir, "nord-lee.theme", nil)
			},
		},
	}
}

func wantThemeTokens() []theme.Token {
	lines := themetest.Lines()
	tokens := make([]theme.Token, 0, len(lines))
	for _, line := range lines {
		name, value, _ := strings.Cut(line, " = ")
		tokens = append(tokens, theme.Token{Name: name, Value: strings.ToUpper(value)})
	}
	return tokens
}

func writeDanglingThemeLink(t *testing.T, dir, base string) string {
	t.Helper()

	path := filepath.Join(dir, base)
	if err := os.Symlink(filepath.Join(dir, "absent-target.theme"), path); err != nil {
		t.Fatalf("symlink %s: %v", path, err)
	}
	return path
}

func requireLoadRejection(t *testing.T, got theme.Result, rejection *theme.Rejection, wantReason theme.Reason, wantDetail string) {
	t.Helper()

	if rejection == nil {
		t.Fatalf("the loader accepted the file as %+v, want the rejection %q: %s", got, wantReason, wantDetail)
	}
	if !reflect.DeepEqual(got, theme.Result{}) {
		t.Errorf("the loader returned %+v alongside a rejection, want the zero Result", got)
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

func TestLoadEntryPoints_CarryTheExactSourceBytes(t *testing.T) {
	body := themetest.Body()

	for _, tc := range loadEntryPoints() {
		t.Run(tc.name, func(t *testing.T) {
			got, rejection := tc.load(t, body)

			if rejection != nil {
				t.Fatalf("the loader rejected a valid theme: %v", rejection)
			}
			if !bytes.Equal(got.Source, body) {
				t.Errorf("Source = %q, want the exact input bytes %q", got.Source, body)
			}
			if got.Slug != tc.wantSlug {
				t.Errorf("Slug = %q, want %q", got.Slug, tc.wantSlug)
			}
		})
	}
}

func TestLoadEntryPoints_RejectionReturnsTheZeroResult(t *testing.T) {
	broken := themetest.Render(themetest.WithoutKey(themetest.Lines(), "bg.subtle"))

	for _, tc := range loadEntryPoints() {
		t.Run(tc.name, func(t *testing.T) {
			got, rejection := tc.load(t, broken)

			if rejection == nil {
				t.Fatalf("the loader accepted %+v, want the rejection %q", got, theme.ReasonMissingTokens)
			}
			if rejection.Reason != theme.ReasonMissingTokens {
				t.Errorf("rejection reason = %q, want %q", rejection.Reason, theme.ReasonMissingTokens)
			}
			if !reflect.DeepEqual(got, theme.Result{}) {
				t.Errorf("the loader returned %+v alongside a rejection, want the zero Result", got)
			}
		})
	}
}

type loadEntryPoint struct {
	name     string
	wantSlug string
	load     func(t *testing.T, data []byte) (theme.Result, *theme.Rejection)
}

func loadEntryPoints() []loadEntryPoint {
	return []loadEntryPoint{
		{
			name:     "LoadFile",
			wantSlug: "nord-lee",
			load: func(t *testing.T, data []byte) (theme.Result, *theme.Rejection) {
				return theme.Loader{}.LoadFile(writeThemeBytes(t, data))
			},
		},
		{
			name: "LoadPath",
			load: func(t *testing.T, data []byte) (theme.Result, *theme.Rejection) {
				return theme.LoadPath(writeThemeBytes(t, data))
			},
		},
		{
			name:     "LoadBuiltin",
			wantSlug: "nord-lee",
			load: func(t *testing.T, data []byte) (theme.Result, *theme.Rejection) {
				loader := theme.Loader{BuiltinSource: func(string) ([]byte, bool) { return data, true }}
				result, rejection, _ := loader.LoadBuiltin("nord-lee")
				return result, rejection
			},
		},
	}
}

func writeThemeBytes(t *testing.T, data []byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "nord-lee.theme")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
