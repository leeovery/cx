package theme_test

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

func nominationLoader() theme.Loader {
	return theme.NewSilentLoader()
}

func dropInTheme(t *testing.T, path string) theme.Theme {
	t.Helper()

	result, rejection := nominationLoader().LoadFile(path)
	if rejection != nil {
		t.Fatalf("LoadFile(%s) = %v, want the drop-in's palette", path, rejection)
	}
	return result.Theme
}

func requireDistinctDefaults(t *testing.T) {
	t.Helper()

	if theme.DefaultLightSlug == theme.DefaultDarkSlug {
		t.Fatalf("DefaultLightSlug and DefaultDarkSlug are both %q — a swapped fallback map would be undetectable", theme.DefaultLightSlug)
	}
	if themetest.Builtin(t, theme.DefaultLightSlug) == themetest.Builtin(t, theme.DefaultDarkSlug) {
		t.Fatal("the two shipped defaults parse to the same palette — a swapped fallback map would be undetectable")
	}
}

func TestSlot_AttrName(t *testing.T) {
	tests := []struct {
		name      string
		slot      theme.Slot
		want      string
		wantNamed bool
	}{
		{name: "the light slot", slot: theme.SlotLight, want: "light", wantNamed: true},
		{name: "the dark slot", slot: theme.SlotDark, want: "dark", wantNamed: true},
		{name: "a constant", slot: theme.SlotConstant, want: "", wantNamed: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, named := tt.slot.AttrName()

			if got != tt.want || named != tt.wantNamed {
				t.Errorf("AttrName() = (%q, %v), want (%q, %v)", got, named, tt.want, tt.wantNamed)
			}
		})
	}
}

func TestResolveNomination_FallbackIsModeMatched(t *testing.T) {
	requireDistinctDefaults(t)

	tests := []struct {
		name         string
		setting      theme.Setting
		wantSlots    int
		index        int
		wantSlot     theme.Slot
		wantSlug     string
		wantFallback string
	}{
		{
			name:         "a broken light slot",
			setting:      theme.Setting{Light: "gone-light", Dark: "nord"},
			wantSlots:    2,
			index:        0,
			wantSlot:     theme.SlotLight,
			wantSlug:     "gone-light",
			wantFallback: theme.DefaultLightSlug,
		},
		{
			name:         "a broken dark slot",
			setting:      theme.Setting{Light: "nord", Dark: "gone-dark"},
			wantSlots:    2,
			index:        1,
			wantSlot:     theme.SlotDark,
			wantSlug:     "gone-dark",
			wantFallback: theme.DefaultDarkSlug,
		},
		{
			name:         "a broken constant",
			setting:      theme.Setting{IsConstant: true, Constant: "gone"},
			wantSlots:    1,
			index:        0,
			wantSlot:     theme.SlotConstant,
			wantSlug:     "gone",
			wantFallback: theme.DefaultDarkSlug,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := nominationLoader().ResolveNomination(tt.setting, t.TempDir())

			if err != nil {
				t.Fatalf("ResolveNomination(%+v) = %v, want the fallback applied", tt.setting, err)
			}
			if len(got.Slots) != tt.wantSlots {
				t.Fatalf("Slots = %+v, want %d", got.Slots, tt.wantSlots)
			}
			want := theme.SlotResolution{
				Slot:      tt.wantSlot,
				Requested: tt.wantSlug,
				Resolved:  tt.wantFallback,
				FellBack:  true,
				Reason:    theme.ReasonNotFound,
				Theme:     themetest.Builtin(t, tt.wantFallback),
			}
			if got.Slots[tt.index] != want {
				t.Errorf("slot resolution = %+v, want %+v", got.Slots[tt.index], want)
			}
		})
	}
}

func TestResolveNomination_StructuredOutcome(t *testing.T) {
	loader := nominationLoader()
	dir := t.TempDir()
	themetest.Write(t, dir, "broken-light.theme", themetest.WithoutKey(themetest.Lines(), "bg.subtle"))
	survivor := themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())
	setting := theme.Setting{Light: "broken-light", Dark: "nord-lee"}

	got, err := loader.ResolveNomination(setting, dir)

	if err != nil {
		t.Fatalf("ResolveNomination(%+v) = %v, want the fallback applied", setting, err)
	}
	want := []theme.SlotResolution{
		{
			Slot:      theme.SlotLight,
			Requested: "broken-light",
			Resolved:  theme.DefaultLightSlug,
			FellBack:  true,
			Reason:    theme.ReasonMissingTokens,
			Theme:     themetest.Builtin(t, theme.DefaultLightSlug),
		},
		{
			Slot:      theme.SlotDark,
			Requested: "nord-lee",
			Resolved:  "nord-lee",
			Theme:     dropInTheme(t, survivor),
		},
	}
	if len(got.Slots) != len(want) {
		t.Fatalf("Slots = %+v, want %d records", got.Slots, len(want))
	}
	for i := range want {
		if got.Slots[i] != want[i] {
			t.Errorf("Slots[%d] = %+v, want %+v", i, got.Slots[i], want[i])
		}
	}
}

func TestResolveNomination_NominationShapeMatchesSetting(t *testing.T) {
	t.Run("a constant yields one slot and a constant nomination", func(t *testing.T) {
		dir := t.TempDir()
		path := themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())
		setting := theme.Setting{IsConstant: true, Constant: "nord-lee"}

		got, err := nominationLoader().ResolveNomination(setting, dir)

		if err != nil {
			t.Fatalf("ResolveNomination(%+v) = %v", setting, err)
		}
		if len(got.Slots) != 1 || got.Slots[0].Slot != theme.SlotConstant {
			t.Fatalf("Slots = %+v, want exactly one SlotConstant record", got.Slots)
		}
		if !got.Nomination.IsConstant() {
			t.Error("IsConstant() = false, want true — the nomination mirrors the setting's state")
		}
		want := dropInTheme(t, path)
		if got.Nomination.Constant() != want {
			t.Errorf("Constant() = %+v, want the resolved palette %+v", got.Nomination.Constant(), want)
		}
		if got.Nomination.Select(theme.MemberDark) != want || got.Nomination.Select(theme.MemberLight) != want {
			t.Error("Select() answers differ under a constant, want the constant for either answer — detection is never consulted")
		}
	})

	t.Run("a pair yields two slots in light-then-dark order", func(t *testing.T) {
		dir := t.TempDir()
		light := themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())
		setting := theme.Setting{Light: "nord-lee", Dark: "nord"}

		got, err := nominationLoader().ResolveNomination(setting, dir)

		if err != nil {
			t.Fatalf("ResolveNomination(%+v) = %v", setting, err)
		}
		if len(got.Slots) != 2 {
			t.Fatalf("Slots = %+v, want exactly two records", got.Slots)
		}
		if got.Slots[0].Slot != theme.SlotLight || got.Slots[1].Slot != theme.SlotDark {
			t.Errorf("slot order = %v then %v, want light then dark", got.Slots[0].Slot, got.Slots[1].Slot)
		}
		if got.Nomination.IsConstant() {
			t.Error("IsConstant() = true, want false — an adaptive pair names no constant")
		}
		if want := (theme.Theme{}); got.Nomination.Constant() != want {
			t.Error("Constant() returned a palette under a pair, want the zero Theme — there is no active member to hand out")
		}
		if want := dropInTheme(t, light); got.Nomination.Select(theme.MemberLight) != want {
			t.Errorf("Select(MemberLight) = %+v, want the light slot's palette %+v", got.Nomination.Select(theme.MemberLight), want)
		}
		if want := themetest.Builtin(t, "nord"); got.Nomination.Select(theme.MemberDark) != want {
			t.Errorf("Select(MemberDark) = %+v, want the dark slot's palette %+v", got.Nomination.Select(theme.MemberDark), want)
		}
	})

	t.Run("a fallback substitutes a palette, never a state", func(t *testing.T) {
		constant, err := nominationLoader().ResolveNomination(theme.Setting{IsConstant: true, Constant: "gone"}, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveNomination(constant) = %v", err)
		}
		pair, err := nominationLoader().ResolveNomination(theme.Setting{Light: "gone-light", Dark: "gone-dark"}, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveNomination(pair) = %v", err)
		}

		if !constant.Nomination.IsConstant() || len(constant.Slots) != 1 {
			t.Errorf("a fallen-back constant = %+v, want one slot and a constant nomination", constant)
		}
		if pair.Nomination.IsConstant() || len(pair.Slots) != 2 {
			t.Errorf("a fallen-back pair = %+v, want two slots and an adaptive nomination", pair)
		}
	})
}

func withoutBuiltin(omit string) func(string) ([]byte, bool) {
	return func(slug string) ([]byte, bool) {
		if slug == omit {
			return nil, false
		}
		return theme.BuiltinBytes(slug)
	}
}

func corruptBuiltin(corrupt string) func(string) ([]byte, bool) {
	broken := themetest.Render(themetest.WithoutKey(themetest.Lines(), "canvas"))
	return func(slug string) ([]byte, bool) {
		if slug == corrupt {
			return broken, true
		}
		return theme.BuiltinBytes(slug)
	}
}

func requireZeroResolution(t *testing.T, got theme.Resolution) {
	t.Helper()

	if got.Slots != nil {
		t.Errorf("Slots = %+v alongside an error, want none", got.Slots)
	}
	if got.Nomination.IsConstant() {
		t.Error("IsConstant() = true alongside an error, want the zero Nomination")
	}
	zero := theme.Theme{}
	if got.Nomination.Constant() != zero || got.Nomination.Select(theme.MemberDark) != zero || got.Nomination.Select(theme.MemberLight) != zero {
		t.Error("the nomination carries a palette alongside an error, want the zero Theme — there is no runtime last-resort palette")
	}
}

func TestResolveNomination_UnresolvableFallbackErrors(t *testing.T) {
	tests := []struct {
		name    string
		omit    string
		setting theme.Setting
	}{
		{
			name:    "a constant whose dark fallback is missing",
			omit:    theme.DefaultDarkSlug,
			setting: theme.Setting{IsConstant: true, Constant: "gone"},
		},
		{
			name:    "a light slot whose light fallback is missing",
			omit:    theme.DefaultLightSlug,
			setting: theme.Setting{Light: "gone-light", Dark: "nord"},
		},
		{
			name:    "a dark slot whose dark fallback is missing",
			omit:    theme.DefaultDarkSlug,
			setting: theme.Setting{Light: "nord", Dark: "gone-dark"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := nominationLoader()
			loader.BuiltinSource = withoutBuiltin(tt.omit)

			got, err := loader.ResolveNomination(tt.setting, t.TempDir())

			if err == nil {
				t.Fatalf("ResolveNomination(%+v) = %+v, want an error — the fallback itself did not resolve", tt.setting, got)
			}
			requireZeroResolution(t, got)
		})
	}

	t.Run("a corrupt fallback fails the same way as a missing one", func(t *testing.T) {
		loader := nominationLoader()
		loader.BuiltinSource = corruptBuiltin(theme.DefaultDarkSlug)
		setting := theme.Setting{IsConstant: true, Constant: "gone"}

		got, err := loader.ResolveNomination(setting, t.TempDir())

		if err == nil {
			t.Fatalf("ResolveNomination(%+v) = %+v, want an error — the fallback parsed as invalid", setting, got)
		}
		requireZeroResolution(t, got)
	})

	t.Run("it never falls back a second time", func(t *testing.T) {
		var requested []string
		loader := nominationLoader()
		loader.BuiltinSource = func(slug string) ([]byte, bool) {
			requested = append(requested, slug)
			return withoutBuiltin(theme.DefaultDarkSlug)(slug)
		}

		if _, err := loader.ResolveNomination(theme.Setting{IsConstant: true, Constant: "gone"}, t.TempDir()); err == nil {
			t.Fatal("ResolveNomination succeeded with its fallback removed — the assertion below would be vacuous")
		}

		want := []string{"gone", theme.DefaultDarkSlug}
		if !slices.Equal(requested, want) {
			t.Errorf("built-ins asked for = %v, want %v — a failed fallback is fatal, never a second fallback", requested, want)
		}
	})
}

func resolutionSource(t *testing.T) parsedThemeSource {
	t.Helper()

	for _, source := range parseThemeSources(t) {
		if source.Name == "resolution.go" {
			return source
		}
	}
	t.Fatal("resolution.go not found among internal/theme's production sources")
	return parsedThemeSource{}
}

func TestResolveNomination_FallbackUsesSharedConstants(t *testing.T) {
	requireDistinctDefaults(t)

	t.Run("a fallen-back pair paints what a virgin install paints", func(t *testing.T) {
		shipped, _ := theme.ResolveSetting(theme.RawKeys{})

		fallen, err := nominationLoader().ResolveNomination(theme.Setting{Light: "gone-light", Dark: "gone-dark"}, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveNomination(a broken pair) = %v", err)
		}
		virgin, err := nominationLoader().ResolveNomination(shipped, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveNomination(the shipped pair) = %v", err)
		}

		for _, member := range []theme.Member{theme.MemberLight, theme.MemberDark} {
			if fallen.Nomination.Select(member) != virgin.Nomination.Select(member) {
				t.Errorf("Select(%v) differs between a fallen-back pair and the shipped default, want identical palettes — the degrades-to-a-constant-dark-default argument rests on them being the same values", member)
			}
		}
		if got := []string{fallen.Slots[0].Resolved, fallen.Slots[1].Resolved}; !slices.Equal(got, []string{theme.DefaultLightSlug, theme.DefaultDarkSlug}) {
			t.Errorf("fallback slugs = %v, want the shared constants %v", got, []string{theme.DefaultLightSlug, theme.DefaultDarkSlug})
		}
	})

	t.Run("the fallback map names the constants, never their values", func(t *testing.T) {
		source := resolutionSource(t)

		ast.Inspect(source.File, func(n ast.Node) bool {
			lit, isLit := n.(*ast.BasicLit)
			if !isLit || lit.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if value == theme.DefaultDarkSlug || value == theme.DefaultLightSlug {
				t.Errorf("%s:%d declares the literal %q — the fallback map is expressed in DefaultLightSlug/DefaultDarkSlug so it cannot drift from the shipped default", source.Name, source.Fset.Position(lit.Pos()).Line, value)
			}
			return true
		})
	})
}

type fallbackCause struct {
	name       string
	slug       string
	stage      func(t *testing.T) string
	wantReason theme.Reason
}

func fallbackCauses() []fallbackCause {
	dirWith := func(base string, lines []string) func(t *testing.T) string {
		return func(t *testing.T) string {
			t.Helper()
			dir := t.TempDir()
			themetest.Write(t, dir, base, lines)
			return dir
		}
	}

	return []fallbackCause{
		{
			name:       "a deleted file",
			slug:       "nord-lee",
			stage:      func(t *testing.T) string { return t.TempDir() },
			wantReason: theme.ReasonNotFound,
		},
		{
			name:       "a renamed file",
			slug:       "nord-lee",
			stage:      dirWith("nord-lee-renamed.theme", themetest.Lines()),
			wantReason: theme.ReasonNotFound,
		},
		{
			name:       "a typo in prefs.json",
			slug:       "nrod-lee",
			stage:      dirWith("nord-lee.theme", themetest.Lines()),
			wantReason: theme.ReasonNotFound,
		},
		{
			name:       "an illegal persisted slug",
			slug:       "../evil",
			stage:      dirWith("nord-lee.theme", themetest.Lines()),
			wantReason: theme.ReasonBadName,
		},
		{
			name:       "a missing token",
			slug:       "nord-lee",
			stage:      dirWith("nord-lee.theme", themetest.WithoutKey(themetest.Lines(), "bg.subtle")),
			wantReason: theme.ReasonMissingTokens,
		},
		{
			name:       "a bad colour",
			slug:       "nord-lee",
			stage:      dirWith("nord-lee.theme", themetest.LinesWithCanvas("blue")),
			wantReason: theme.ReasonBadColour,
		},
		{
			name:       "a duplicate key",
			slug:       "nord-lee",
			stage:      dirWith("nord-lee.theme", append(themetest.Lines(), "text.primary = #010203")),
			wantReason: theme.ReasonBadSyntax,
		},
		{
			name: "an unreadable file",
			slug: "nord-lee",
			stage: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				_ = themetest.DenyRead(t, themetest.Write(t, dir, "nord-lee.theme", themetest.Lines()))
				return dir
			},
			wantReason: theme.ReasonUnreadable,
		},
		{
			name: "an unreadable directory",
			slug: "nord-lee",
			stage: func(t *testing.T) string {
				dir := themesDirWithOneTheme(t)
				_ = themetest.DenyDir(t, dir)
				return dir
			},
			wantReason: theme.ReasonUnreadable,
		},
	}
}

var reachableFallbackReasons = []theme.Reason{
	theme.ReasonBadName,
	theme.ReasonUnreadable,
	theme.ReasonBadSyntax,
	theme.ReasonBadColour,
	theme.ReasonMissingTokens,
	theme.ReasonNotFound,
}

func TestResolveNomination_EveryCauseFallsBack(t *testing.T) {
	for _, tc := range fallbackCauses() {
		t.Run(tc.name, func(t *testing.T) {
			dir := tc.stage(t)
			setting := theme.Setting{IsConstant: true, Constant: tc.slug}

			got, err := nominationLoader().ResolveNomination(setting, dir)

			if err != nil {
				t.Fatalf("ResolveNomination(%+v) = %v, want the fallback applied", setting, err)
			}
			want := []theme.SlotResolution{{
				Slot:      theme.SlotConstant,
				Requested: tc.slug,
				Resolved:  theme.DefaultDarkSlug,
				FellBack:  true,
				Reason:    tc.wantReason,
				Theme:     themetest.Builtin(t, theme.DefaultDarkSlug),
			}}
			if !slices.Equal(got.Slots, want) {
				t.Errorf("Slots = %+v, want %+v", got.Slots, want)
			}
		})
	}

	t.Run("the causes cover every reason a nomination can fail with", func(t *testing.T) {
		var covered []theme.Reason
		for _, tc := range fallbackCauses() {
			if !slices.Contains(covered, tc.wantReason) {
				covered = append(covered, tc.wantReason)
			}
		}
		slices.Sort(covered)

		want := slices.Sorted(slices.Values(reachableFallbackReasons))
		if !slices.Equal(covered, want) {
			t.Errorf("causes cover %v, want %v — a reason with no case is a cause nobody has run through the fallback", covered, want)
		}
	})
}

func TestResolveNomination_BothSlotsCanFallBack(t *testing.T) {
	requireDistinctDefaults(t)

	t.Run("both slots missing", func(t *testing.T) {
		setting := theme.Setting{Light: "gone-light", Dark: "gone-dark"}

		got, err := nominationLoader().ResolveNomination(setting, t.TempDir())

		if err != nil {
			t.Fatalf("ResolveNomination(%+v) = %v, want two fallbacks", setting, err)
		}
		want := []theme.SlotResolution{
			{Slot: theme.SlotLight, Requested: "gone-light", Resolved: theme.DefaultLightSlug, FellBack: true, Reason: theme.ReasonNotFound, Theme: themetest.Builtin(t, theme.DefaultLightSlug)},
			{Slot: theme.SlotDark, Requested: "gone-dark", Resolved: theme.DefaultDarkSlug, FellBack: true, Reason: theme.ReasonNotFound, Theme: themetest.Builtin(t, theme.DefaultDarkSlug)},
		}
		if !slices.Equal(got.Slots, want) {
			t.Fatalf("Slots = %+v, want %+v", got.Slots, want)
		}
		if got.Nomination.Select(theme.MemberLight) != themetest.Builtin(t, theme.DefaultLightSlug) || got.Nomination.Select(theme.MemberDark) != themetest.Builtin(t, theme.DefaultDarkSlug) {
			t.Error("the nomination does not carry the two shipped defaults — a doubly-fallen-back pair is exactly the shipped pair")
		}
	})

	t.Run("both slots broken for different reasons", func(t *testing.T) {
		dir := t.TempDir()
		themetest.WriteWithCanvas(t, dir, "broken-light.theme", "blue")
		setting := theme.Setting{Light: "broken-light", Dark: "gone-dark"}

		got, err := nominationLoader().ResolveNomination(setting, dir)

		if err != nil {
			t.Fatalf("ResolveNomination(%+v) = %v, want two fallbacks", setting, err)
		}
		want := []theme.Reason{theme.ReasonBadColour, theme.ReasonNotFound}
		if gotReasons := []theme.Reason{got.Slots[0].Reason, got.Slots[1].Reason}; !slices.Equal(gotReasons, want) {
			t.Errorf("reasons = %v, want %v — each slot reports its own cause", gotReasons, want)
		}
	})
}

func TestResolveNomination_SurvivingSlotUnaffected(t *testing.T) {
	tests := []struct {
		name         string
		setting      theme.Setting
		survivor     int
		wantSlot     theme.Slot
		wantMember   theme.Member
		fallenSlot   theme.Slot
		wantFallback string
	}{
		{
			name:         "a broken light slot leaves dark alone",
			setting:      theme.Setting{Light: "gone-light", Dark: "nord-lee"},
			survivor:     1,
			wantSlot:     theme.SlotDark,
			wantMember:   theme.MemberDark,
			fallenSlot:   theme.SlotLight,
			wantFallback: theme.DefaultLightSlug,
		},
		{
			name:         "a broken dark slot leaves light alone",
			setting:      theme.Setting{Light: "nord-lee", Dark: "gone-dark"},
			survivor:     0,
			wantSlot:     theme.SlotLight,
			wantMember:   theme.MemberLight,
			fallenSlot:   theme.SlotDark,
			wantFallback: theme.DefaultDarkSlug,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())
			survivorTheme := dropInTheme(t, path)
			if survivorTheme == themetest.Builtin(t, tt.wantFallback) {
				t.Fatal("the drop-in parses to the shipped default's palette — an untouched slot and a fallen-back one would be indistinguishable")
			}

			got, err := nominationLoader().ResolveNomination(tt.setting, dir)

			if err != nil {
				t.Fatalf("ResolveNomination(%+v) = %v", tt.setting, err)
			}
			want := theme.SlotResolution{
				Slot:      tt.wantSlot,
				Requested: "nord-lee",
				Resolved:  "nord-lee",
				Theme:     survivorTheme,
			}
			if got.Slots[tt.survivor] != want {
				t.Errorf("the surviving slot = %+v, want %+v", got.Slots[tt.survivor], want)
			}
			if !got.Slots[1-tt.survivor].FellBack {
				t.Errorf("the other slot = %+v, want FellBack — this fixture is meant to stage one of each", got.Slots[1-tt.survivor])
			}
			if got.Nomination.Select(tt.wantMember) != survivorTheme {
				t.Error("the nomination's surviving member is not the drop-in's palette — a fallback substitutes one slot, never the setting")
			}
		})
	}
}

func TestResolveNomination_UnsetSlotIsNotAFallback(t *testing.T) {
	dirs := []struct {
		name  string
		stage func(t *testing.T) string
	}{
		{name: "an empty themes directory", stage: func(t *testing.T) string { return t.TempDir() }},
		{name: "an absent themes directory", stage: func(t *testing.T) string { return filepath.Join(t.TempDir(), "themes") }},
		{name: "no themes directory at all", stage: func(t *testing.T) string { return "" }},
	}

	for _, dir := range dirs {
		t.Run(dir.name, func(t *testing.T) {
			setting, _ := theme.ResolveSetting(theme.RawKeys{})

			got, err := nominationLoader().ResolveNomination(setting, dir.stage(t))

			if err != nil {
				t.Fatalf("ResolveNomination(the shipped pair) = %v", err)
			}
			want := []theme.SlotResolution{
				{Slot: theme.SlotLight, Requested: theme.DefaultLightSlug, Resolved: theme.DefaultLightSlug, Theme: themetest.Builtin(t, theme.DefaultLightSlug)},
				{Slot: theme.SlotDark, Requested: theme.DefaultDarkSlug, Resolved: theme.DefaultDarkSlug, Theme: themetest.Builtin(t, theme.DefaultDarkSlug)},
			}
			if !slices.Equal(got.Slots, want) {
				t.Errorf("Slots = %+v, want %+v — a virgin install produces zero fallbacks", got.Slots, want)
			}
		})
	}
}

func TestResolveNomination_SetAndUnsetDefaultsAreIndistinguishable(t *testing.T) {
	t.Run("the two settings resolve to identical records", func(t *testing.T) {
		unsetSetting, _ := theme.ResolveSetting(theme.RawKeys{})
		setSetting, _ := theme.ResolveSetting(theme.RawKeys{Light: theme.DefaultLightSlug, Dark: theme.DefaultDarkSlug})
		if unsetSetting != setSetting {
			t.Fatalf("Setting = %+v when unset and %+v when set to the same slugs — the claim below is about resolution, not about the setting", unsetSetting, setSetting)
		}

		unset, err := nominationLoader().ResolveNomination(unsetSetting, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveNomination(unset) = %v", err)
		}
		set, err := nominationLoader().ResolveNomination(setSetting, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveNomination(set) = %v", err)
		}

		if !slices.Equal(unset.Slots, set.Slots) {
			t.Errorf("Slots = %+v when unset and %+v when set to the same slugs, want identical", unset.Slots, set.Slots)
		}
	})

	t.Run("the record declares no set-ness flag", func(t *testing.T) {
		assertFieldKinds(t, reflect.TypeFor[theme.SlotResolution](), []fieldKind{
			{name: "Slot", kind: reflect.Int},
			{name: "Requested", kind: reflect.String},
			{name: "Resolved", kind: reflect.String},
			{name: "FellBack", kind: reflect.Bool},
			{name: "Reason", kind: reflect.String},
			{name: "Theme", kind: reflect.Struct},
		})
	})
}

func TestResolveNomination_NeverOverwritesPrefs(t *testing.T) {
	resolveThrough := func(t *testing.T, path string) theme.Resolution {
		t.Helper()

		keys, err := prefs.NewStore(path).LoadThemeKeys()
		if err != nil {
			t.Fatalf("LoadThemeKeys(%s): %v", path, err)
		}
		setting, _ := theme.ResolveSetting(theme.NewRawKeys(keys.Theme, keys.Light, keys.Dark))
		got, err := nominationLoader().ResolveNomination(setting, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveNomination(%+v) = %v", setting, err)
		}
		return got
	}

	t.Run("a persisted name survives the fallback byte for byte", func(t *testing.T) {
		const persisted = `{"session_list_mode":"by-project","appearance":"dark","theme_light":"gone-light","theme_dark":"gone-dark"}`
		path := writeFile(t, t.TempDir(), "prefs.json", persisted)

		got := resolveThrough(t, path)

		for _, slot := range got.Slots {
			if !slot.FellBack {
				t.Fatalf("slot %+v did not fall back — the fixture must exercise the path a write would be tempting on", slot)
			}
			if slot.Requested == slot.Resolved {
				t.Fatalf("slot %+v resolved what it requested — the persisted name is meant to differ from the rendered one here", slot)
			}
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read back %s: %v", path, err)
		}
		if string(after) != persisted {
			t.Errorf("prefs.json =\n%s\nwant it byte-identical:\n%s\n— fixing the theme file must restore the theme on the next launch, with no re-selection", after, persisted)
		}
	})

	t.Run("an absent prefs.json stays absent", func(t *testing.T) {
		configDir := t.TempDir()
		path := filepath.Join(configDir, "prefs.json")

		got := resolveThrough(t, path)

		if len(got.Slots) != 2 {
			t.Fatalf("Slots = %+v, want the shipped pair", got.Slots)
		}
		entries, err := os.ReadDir(configDir)
		if err != nil {
			t.Fatalf("read %s: %v", configDir, err)
		}
		if len(entries) != 0 {
			t.Errorf("resolution left %d entries in the config directory, want none — prefs.json stays absent on a virgin install and nothing here seeds it", len(entries))
		}
	})

	t.Run("it reaches no write of any kind", func(t *testing.T) {
		calls := osCallsReachableFrom(t, "ResolveNomination")

		for _, write := range []string{"os.WriteFile", "os.Create", "os.CreateTemp", "os.OpenFile", "os.Rename", "os.Remove", "os.RemoveAll", "os.Mkdir", "os.MkdirAll", "os.Chmod", "os.Symlink", "os.Truncate"} {
			if got := calls[write]; got != 0 {
				t.Errorf("ResolveNomination reaches %d %s call sites, want 0 — resolution reads, it never writes", got, write)
			}
		}
		if calls["os.ReadFile"] == 0 {
			t.Error("ResolveNomination reaches no os.ReadFile call site — the walk resolved nothing, so the assertions above would be vacuous")
		}
	})
}

func enumerationOf(t *testing.T, loader theme.Loader, dir string) theme.Enumeration {
	t.Helper()

	enumeration, _ := theme.Assembler{Loader: loader}.Open(dir, theme.RawKeys{})
	return enumeration
}

func TestResolveNominationFrom_ReadsNothing(t *testing.T) {
	t.Run("it resolves a drop-in whose directory is gone", func(t *testing.T) {
		dir := t.TempDir()
		path := themetest.Write(t, dir, "sunset.theme", themetest.Lines())
		want := dropInTheme(t, path)
		loader := nominationLoader()
		enumeration := enumerationOf(t, loader, dir)

		if err := os.RemoveAll(dir); err != nil {
			t.Fatalf("remove %s: %v", dir, err)
		}

		setting := theme.Setting{IsConstant: true, Constant: "sunset"}
		got, err := loader.ResolveNominationFrom(enumeration, setting)

		if err != nil {
			t.Fatalf("ResolveNominationFrom(%+v) = %v, want the retained parse", setting, err)
		}
		if len(got.Slots) != 1 || got.Slots[0].FellBack {
			t.Fatalf("Slots = %+v, want the drop-in resolved with no fallback — the enumeration still holds its parse", got.Slots)
		}
		if got.Nomination.Constant() != want {
			t.Errorf("the constant resolved to %s, want the drop-in's palette %s", got.Nomination.Constant().Canvas.Value, want.Canvas.Value)
		}
	})

	t.Run("it reaches no os call at all", func(t *testing.T) {
		for name, count := range osCallsReachableFrom(t, "ResolveNominationFrom") {
			if strings.HasPrefix(name, "os.") {
				t.Errorf("ResolveNominationFrom reaches %d %s call sites, want 0 — it resolves against the retained enumeration, never the filesystem", count, name)
			}
		}
		if osCallsReachableFrom(t, "ResolveNomination")["os.ReadFile"] == 0 {
			t.Error("the by-name entry point reaches no os.ReadFile call site either — the walk resolved nothing, so the assertion above would be vacuous")
		}
	})
}

func TestResolveNominationFrom_ResolvesAgainstTheEnumerationsEntries(t *testing.T) {
	loader := nominationLoader()

	t.Run("a valid entry resolves to its own palette", func(t *testing.T) {
		dir := t.TempDir()
		path := themetest.Write(t, dir, "sunset.theme", themetest.Lines())
		want := dropInTheme(t, path)
		setting := theme.Setting{IsConstant: true, Constant: "sunset"}

		got, err := loader.ResolveNominationFrom(enumerationOf(t, loader, dir), setting)

		if err != nil {
			t.Fatalf("ResolveNominationFrom(%+v) = %v", setting, err)
		}
		if slot := got.Slots[0]; slot.FellBack || slot.Resolved != "sunset" || slot.Theme != want {
			t.Errorf("slot = %+v, want `sunset` resolved to its own palette with no fallback", slot)
		}
	})

	for _, tt := range []struct {
		name        string
		enumeration func(*testing.T) theme.Enumeration
		want        theme.Reason
	}{
		{
			name: "an invalid entry falls back carrying its own reason",
			enumeration: func(t *testing.T) theme.Enumeration {
				dir := t.TempDir()
				themetest.WriteWithCanvas(t, dir, "sunset.theme", "not-a-colour")
				return enumerationOf(t, loader, dir)
			},
			want: theme.ReasonBadColour,
		},
		{
			name: "a slug the directory never held falls back as not found",
			enumeration: func(t *testing.T) theme.Enumeration {
				return enumerationOf(t, loader, t.TempDir())
			},
			want: theme.ReasonNotFound,
		},
		{
			name: "an unusable directory falls back as unreadable",
			enumeration: func(*testing.T) theme.Enumeration {
				return theme.Enumeration{DirUnusable: true}
			},
			want: theme.ReasonUnreadable,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setting := theme.Setting{IsConstant: true, Constant: "sunset"}

			got, err := loader.ResolveNominationFrom(tt.enumeration(t), setting)

			if err != nil {
				t.Fatalf("ResolveNominationFrom(%+v) = %v, want the fallback applied", setting, err)
			}
			slot := got.Slots[0]
			if !slot.FellBack {
				t.Fatalf("slot = %+v, want the fallback applied", slot)
			}
			if slot.Reason != tt.want {
				t.Errorf("reason = %q, want %q — the resolution and the panel row state one reason for one slug", slot.Reason, tt.want)
			}
			if slot.Requested != "sunset" {
				t.Errorf("Requested = %q, want the persisted slug %q — a fallback never moves the `●`", slot.Requested, "sunset")
			}
			if slot.Resolved != theme.DefaultDarkSlug {
				t.Errorf("Resolved = %q, want the constant's mode-matched default %q", slot.Resolved, theme.DefaultDarkSlug)
			}
			if slot.Theme != themetest.Builtin(t, theme.DefaultDarkSlug) {
				t.Error("the slot carries a palette other than the fallback's")
			}
		})
	}
}

func TestResolveNominationFrom_ConsultsTheEmbeddedSetFirst(t *testing.T) {
	loader := nominationLoader()
	dir := t.TempDir()
	themetest.Write(t, dir, "nord.theme", themetest.Lines())
	setting := theme.Setting{IsConstant: true, Constant: "nord"}

	got, err := loader.ResolveNominationFrom(enumerationOf(t, loader, dir), setting)

	if err != nil {
		t.Fatalf("ResolveNominationFrom(%+v) = %v", setting, err)
	}
	slot := got.Slots[0]
	if slot.FellBack {
		t.Fatalf("slot = %+v, want the embedded built-in resolved — the shadowing file must never be consulted", slot)
	}
	if want := themetest.Builtin(t, "nord"); slot.Theme != want {
		t.Errorf("slot resolved to canvas %s, want the embedded nord's %s", slot.Theme.Canvas.Value, want.Canvas.Value)
	}
}

func TestResolveNominationFrom_UnresolvableFallbackErrors(t *testing.T) {
	loader := nominationLoader()
	loader.BuiltinSource = withoutBuiltin(theme.DefaultDarkSlug)
	setting := theme.Setting{IsConstant: true, Constant: "gone"}

	got, err := loader.ResolveNominationFrom(theme.Enumeration{}, setting)

	if err == nil {
		t.Fatalf("ResolveNominationFrom(%+v) = %+v, want an error — the fallback itself did not resolve", setting, got)
	}
	requireZeroResolution(t, got)
}

func TestResolveNominationFrom_EmitsNoLoadedRecord(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	loader := theme.NewLoader(theme.NewEventLogger(logger))
	setting := theme.Setting{IsConstant: true, Constant: "gone"}

	got, err := loader.ResolveNominationFrom(theme.Enumeration{}, setting)

	if err != nil {
		t.Fatalf("ResolveNominationFrom(%+v) = %v, want the fallback applied", setting, err)
	}
	if !got.Slots[0].FellBack {
		t.Fatalf("slot = %+v, want the fallback applied — the assertions below need one", got.Slots[0])
	}
	if n := len(sink.RecordsWithMessage("loaded")); n != 0 {
		t.Errorf("a panel-open resolution emitted %d `theme: loaded` records, want 0 — the event's cadence is construction plus the commit-time load:\n%s", n, sink.Body())
	}
	if n := len(sink.RecordsWithMessage("fallback applied")); n != 1 {
		t.Errorf("emitted %d `theme: fallback applied` records, want exactly 1 — a panel open is a site that applies one:\n%s", n, sink.Body())
	}
}

func TestResolveByNameFrom_MatchesResolveByName(t *testing.T) {
	loader := nominationLoader()
	dir := t.TempDir()
	themetest.Write(t, dir, "sunset.theme", themetest.Lines())
	themetest.WriteWithCanvas(t, dir, "dusk.theme", "not-a-colour")
	enumeration := enumerationOf(t, loader, dir)

	builtin := theme.DefaultDarkSlug
	for _, slug := range []string{"sunset", "dusk", "gone", builtin, "Sunset"} {
		t.Run(slug, func(t *testing.T) {
			if slug != builtin && slices.Contains(theme.BuiltinSlugs(), slug) {
				t.Fatalf("%q is a built-in (the embedded set is %v) — it would answer from the embedded set and the case would prove nothing about the source", slug, theme.BuiltinSlugs())
			}

			wantResult, wantRejection := loader.ResolveByName(slug, dir)
			gotResult, gotRejection := loader.ResolveByNameFrom(enumeration, slug)

			if gotResult.Slug != wantResult.Slug || gotResult.Theme != wantResult.Theme {
				t.Errorf("ResolveByNameFrom(%q) resolved slug %q, want %q — the two sources must answer alike", slug, gotResult.Slug, wantResult.Slug)
			}
			if (gotRejection == nil) != (wantRejection == nil) {
				t.Fatalf("ResolveByNameFrom(%q) rejection = %v; ResolveByName's = %v", slug, gotRejection, wantRejection)
			}
			if gotRejection != nil && !reflect.DeepEqual(*gotRejection, *wantRejection) {
				t.Errorf("ResolveByNameFrom(%q) rejection = %+v, want %+v", slug, *gotRejection, *wantRejection)
			}
		})
	}
}

func TestResolveByNameFrom_ReadsNothing(t *testing.T) {
	t.Run("it resolves a drop-in whose directory is gone", func(t *testing.T) {
		dir := t.TempDir()
		path := themetest.Write(t, dir, "sunset.theme", themetest.Lines())
		want := dropInTheme(t, path)
		loader := nominationLoader()
		enumeration := enumerationOf(t, loader, dir)

		if err := os.RemoveAll(dir); err != nil {
			t.Fatalf("remove %s: %v", dir, err)
		}

		got, rejection := loader.ResolveByNameFrom(enumeration, "sunset")

		if rejection != nil {
			t.Fatalf("ResolveByNameFrom(sunset) = %v, want the retained parse", rejection)
		}
		if got.Theme != want {
			t.Errorf("resolved canvas %s, want the drop-in's %s", got.Theme.Canvas.Value, want.Canvas.Value)
		}
	})

	t.Run("it reaches no os call at all", func(t *testing.T) {
		for name, count := range osCallsReachableFrom(t, "ResolveByNameFrom") {
			if strings.HasPrefix(name, "os.") {
				t.Errorf("ResolveByNameFrom reaches %d %s call sites, want 0 — it resolves against the retained enumeration, never the filesystem", count, name)
			}
		}
		if osCallsReachableFrom(t, "ResolveByName")["os.ReadFile"] == 0 {
			t.Error("the by-name entry point reaches no os.ReadFile call site either — the walk resolved nothing, so the assertion above would be vacuous")
		}
	})
}
