package theme_test

import (
	"go/ast"
	"maps"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

// TestBadges_ConstantIsBareDot pins §9.5's constant form: ONE slot, one entry,
// and NO SLOT WORD.
//
// With no slots there is nothing to qualify, so a slot label would be redundant
// with the marker itself. The entry is keyed on the constant's slug, which is
// the row the panel paints it on.
func TestBadges_ConstantIsBareDot(t *testing.T) {
	slots := []theme.SlotResolution{{Slot: theme.SlotConstant, Requested: "nord", Resolved: "nord"}}

	badges := theme.Badges(slots)

	want := map[string]theme.Badge{"nord": theme.BadgeConstant}
	if !maps.Equal(badges, want) {
		t.Fatalf("Badges(a constant) = %v, want %v", badgeTexts(badges), badgeTexts(want))
	}
	if got, want := badges["nord"].Text(), "●"; got != want {
		t.Errorf("constant badge text = %q, want the bare %q — a constant has no slot to name", got, want)
	}
}

// TestBadges_PairBadgesLightAndDark pins §9.5's adaptive form: each slot badges
// its OWN nominated slug, on its own row, with its own slot word.
//
// Both badges are present at all times, which is what lets a user see what light
// is set to without having to remember whether they ever set it (§9.5).
func TestBadges_PairBadgesLightAndDark(t *testing.T) {
	slots := []theme.SlotResolution{
		{Slot: theme.SlotLight, Requested: "tokyo-night-day", Resolved: "tokyo-night-day"},
		{Slot: theme.SlotDark, Requested: "nord", Resolved: "nord"},
	}

	badges := theme.Badges(slots)

	want := map[string]string{"tokyo-night-day": "● light", "nord": "● dark"}
	if got := badgeTexts(badges); !maps.Equal(got, want) {
		t.Errorf("Badges(a pair) = %v, want %v", got, want)
	}
}

// TestBadges_SameSlugInBothSlotsIsBoth pins §9.5's collapse: when both slots
// name the SAME slug, that one row carries `● both` — a SINGLE entry, never two.
//
// It is reachable in two keypresses (`d` then `l` on one row) and is a likely
// path: it is where a user lands wanting "this theme everywhere" without
// realising `Enter` is the idiom for it. Two entries cannot express it at all —
// one map key holds one badge — so the collapse is the only shape that renders
// the state honestly.
func TestBadges_SameSlugInBothSlotsIsBoth(t *testing.T) {
	slots := []theme.SlotResolution{
		{Slot: theme.SlotLight, Requested: "nord", Resolved: "nord"},
		{Slot: theme.SlotDark, Requested: "nord", Resolved: "nord"},
	}

	badges := theme.Badges(slots)

	if len(badges) != 1 {
		t.Fatalf("Badges(both slots on one slug) = %v, want a single entry", badgeTexts(badges))
	}
	if got, want := badges["nord"], theme.BadgeBoth; got != want {
		t.Errorf("badge on %q = %q, want %q", "nord", got.Text(), want.Text())
	}
}

// TestBadges_FallbackDoesNotMoveTheBadge pins the second row of §9.5's table:
// a slot that FELL BACK keeps its badge on the PERSISTED slug, and the
// fallback's own row carries NONE.
//
// This is §6.3's "falling back must never overwrite the persisted theme name"
// held at the display layer. A badge on the fallback would sit on a theme the
// user never chose and silently claim it as their choice — and because nothing
// was written, nothing would look wrong.
func TestBadges_FallbackDoesNotMoveTheBadge(t *testing.T) {
	slots := []theme.SlotResolution{
		{Slot: theme.SlotLight, Requested: theme.DefaultLightSlug, Resolved: theme.DefaultLightSlug},
		{Slot: theme.SlotDark, Requested: "gone-dark", Resolved: theme.DefaultDarkSlug, FellBack: true, Reason: theme.ReasonNotFound},
	}

	badges := theme.Badges(slots)

	want := map[string]string{theme.DefaultLightSlug: "● light", "gone-dark": "● dark"}
	if got := badgeTexts(badges); !maps.Equal(got, want) {
		t.Errorf("Badges(a fallen-back dark slot) = %v, want %v", got, want)
	}
	if badge, ok := badges[theme.DefaultDarkSlug]; ok {
		t.Errorf("the fallback %q carries %q, want no badge — the `●` means what is SET, and nobody set the fallback", theme.DefaultDarkSlug, badge.Text())
	}
}

// TestBadges_UnsetSlotBadgesShippedDefault pins the third row of §9.5's table
// over the MOST COMMON INSTALL: a virgin one, where §8.1 leaves prefs.json
// absent entirely.
//
// It is the row that makes §9.4's justification for assembling the union true at
// all — "the `●` marker always has something to sit on" — because a
// persisted-slug-only rule would show no marker anywhere on this install. It is
// also the one place §9.9's inherited-default-versus-pin distinction is visible
// to a user.
//
// The fixture runs the REAL read path (absent keys → the setting → the
// resolution) rather than hand-building the slots, because the claim under test
// is precisely that ResolveSetting's default substitution reaches Requested.
func TestBadges_UnsetSlotBadgesShippedDefault(t *testing.T) {
	requireDistinctDefaults(t)
	setting, _ := theme.ResolveSetting("", "", "")

	resolution, err := nominationLoader().ResolveNomination(setting, t.TempDir())
	if err != nil {
		t.Fatalf("ResolveNomination(the shipped pair) = %v", err)
	}
	badges := theme.Badges(resolution.Slots)

	want := map[string]string{theme.DefaultLightSlug: "● light", theme.DefaultDarkSlug: "● dark"}
	if got := badgeTexts(badges); !maps.Equal(got, want) {
		t.Errorf("Badges(a virgin install) = %v, want %v — both shipped defaults badge their own row", got, want)
	}
}

// TestBadges_CharsetRejectedValueKeepsItsBadge pins the badge on a persisted
// value §8.6 rejected before any file was sought: it sits on THAT RAW VALUE, so
// it meets the union row task 8-1 keyed on the same string.
//
// The row and the badge are derived by two different paths from one persisted
// value, and the value is the only thing either has to hold on to — it yields no
// slug and names no file. If the two keyed differently the badge would be lost on
// the one row whose entire purpose is to show the user what they set.
func TestBadges_CharsetRejectedValueKeepsItsBadge(t *testing.T) {
	const illegal = "../evil"
	loader := nominationLoader()
	setting, keys := theme.ResolveSetting(illegal, "", "")

	_, union := loader.Open(t.TempDir(), keys)
	resolution, err := loader.ResolveNomination(setting, t.TempDir())
	if err != nil {
		t.Fatalf("ResolveNomination(a charset-rejected constant) = %v", err)
	}
	badges := theme.Badges(resolution.Slots)

	if got, want := badgeTexts(badges), map[string]string{illegal: "●"}; !maps.Equal(got, want) {
		t.Fatalf("Badges = %v, want %v — the badge sits on the raw value", got, want)
	}

	row := onlyPersistedRow(t, union)
	if got, want := row.BadgeKey(), illegal; got != want {
		t.Fatalf("the charset-rejected row's BadgeKey() = %q, want %q", got, want)
	}
	if badges[row.BadgeKey()] != theme.BadgeConstant {
		t.Errorf("the charset-rejected row carries %q, want %q — the row and its badge derive from one value by two paths and must meet", badges[row.BadgeKey()].Text(), theme.BadgeConstant.Text())
	}
}

// TestBadges_KeyedOnRequestedNotResolved pins the single field §9.5's whole
// three-row table reduces to, over a table where Requested and Resolved DIFFER
// on every slot.
//
// One rule covers all three rows and nothing branches on FellBack: the badge
// keys on what was NOMINATED, and what actually loaded is not an input.
func TestBadges_KeyedOnRequestedNotResolved(t *testing.T) {
	tests := []struct {
		name  string
		slots []theme.SlotResolution
		want  map[string]string
	}{
		{
			name: "a constant that fell back",
			slots: []theme.SlotResolution{
				{Slot: theme.SlotConstant, Requested: "gone", Resolved: theme.DefaultDarkSlug, FellBack: true, Reason: theme.ReasonNotFound},
			},
			want: map[string]string{"gone": "●"},
		},
		{
			name: "both slots fell back",
			slots: []theme.SlotResolution{
				{Slot: theme.SlotLight, Requested: "gone-light", Resolved: theme.DefaultLightSlug, FellBack: true, Reason: theme.ReasonNotFound},
				{Slot: theme.SlotDark, Requested: "gone-dark", Resolved: theme.DefaultDarkSlug, FellBack: true, Reason: theme.ReasonBadColour},
			},
			want: map[string]string{"gone-light": "● light", "gone-dark": "● dark"},
		},
		{
			name: "both slots fell back from the same slug",
			slots: []theme.SlotResolution{
				{Slot: theme.SlotLight, Requested: "gone", Resolved: theme.DefaultLightSlug, FellBack: true, Reason: theme.ReasonNotFound},
				{Slot: theme.SlotDark, Requested: "gone", Resolved: theme.DefaultDarkSlug, FellBack: true, Reason: theme.ReasonNotFound},
			},
			want: map[string]string{"gone": "● both"},
		},
		{
			name: "a charset-rejected slot beside a loadable one",
			slots: []theme.SlotResolution{
				{Slot: theme.SlotLight, Requested: "../evil", Resolved: theme.DefaultLightSlug, FellBack: true, Reason: theme.ReasonBadName},
				{Slot: theme.SlotDark, Requested: "nord", Resolved: "nord"},
			},
			want: map[string]string{"../evil": "● light", "nord": "● dark"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			badges := theme.Badges(tt.slots)

			if got := badgeTexts(badges); !maps.Equal(got, tt.want) {
				t.Errorf("Badges = %v, want %v", got, tt.want)
			}
			for _, slot := range tt.slots {
				if slot.Resolved == slot.Requested {
					continue
				}
				if badge, ok := badges[slot.Resolved]; ok {
					t.Errorf("what %v resolved TO (%q) carries %q, want the badge on what it asked FOR (%q)", slot.Slot, slot.Resolved, badge.Text(), slot.Requested)
				}
			}
		})
	}
}

// TestBadges_BothIsNoWiderThanLight asserts §9.5's width relation DIRECTLY
// rather than leaving it to prose: the collapsed badge is no wider than the
// widest slot badge, so it cannot move the row-composition truncation budget.
//
// The collapse is reachable in two keypresses, so a wider `● both` would steal
// columns from the label on rows users routinely produce. Width is measured with
// lipgloss rather than len, because these are multi-byte glyphs and the budget is
// counted in terminal cells.
func TestBadges_BothIsNoWiderThanLight(t *testing.T) {
	both := lipgloss.Width(theme.BadgeBoth.Text())
	for _, badge := range []theme.Badge{theme.BadgeLight, theme.BadgeDark} {
		if got := lipgloss.Width(badge.Text()); both > got {
			t.Errorf("%q is %d cells wide, wider than %q at %d — the collapsed badge must not move the truncation budget", theme.BadgeBoth.Text(), both, badge.Text(), got)
		}
	}
}

// TestBadges_PureAndTotal pins the two properties every consumer rests on: the
// derivation is TOTAL — no input shape panics, including the ones that cannot
// legally arise — and it is PURE, so re-running it against unchanged state
// answers identically.
//
// Totality matters because the panel recomputes badges on a keypress path: a
// derivation that could panic on an unexpected slice would take the TUI down
// mid-commit. Purity is what makes the commit path's recompute (Phase 9) a
// re-run of this function rather than a second, drifting derivation.
//
// The source half asserts the third property, which no input can demonstrate:
// badge.go reads NO palette. A badge is a fact about a slug, and a Theme reaching
// this derivation would be the first step toward a badge that depends on the
// colours it marks.
func TestBadges_PureAndTotal(t *testing.T) {
	t.Run("no input shape panics", func(t *testing.T) {
		tests := []struct {
			name  string
			slots []theme.SlotResolution
			want  map[string]string
		}{
			{name: "a nil slice", slots: nil, want: map[string]string{}},
			{name: "an empty slice", slots: []theme.SlotResolution{}, want: map[string]string{}},
			{
				name:  "a lone light slot",
				slots: []theme.SlotResolution{{Slot: theme.SlotLight, Requested: "nord"}},
				want:  map[string]string{"nord": "● light"},
			},
			{
				name: "a constant mixed with a slot",
				slots: []theme.SlotResolution{
					{Slot: theme.SlotConstant, Requested: "nord"},
					{Slot: theme.SlotDark, Requested: "tokyo-night"},
				},
				want: map[string]string{"tokyo-night": "● dark"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				badges := theme.Badges(tt.slots)

				if badges == nil {
					t.Fatal("Badges = nil, want an empty map — a nil map is a second shape of nothing")
				}
				if got := badgeTexts(badges); !maps.Equal(got, tt.want) {
					t.Errorf("Badges = %v, want %v — a slice mixing the two setting states is a programming error, never both forms at once", got, tt.want)
				}
			})
		}
	})

	t.Run("repeat calls answer identically", func(t *testing.T) {
		slots := []theme.SlotResolution{
			{Slot: theme.SlotLight, Requested: "gone", Resolved: theme.DefaultLightSlug, FellBack: true, Reason: theme.ReasonNotFound},
			{Slot: theme.SlotDark, Requested: "nord", Resolved: "nord"},
		}

		first := theme.Badges(slots)
		second := theme.Badges(slots)

		if !maps.Equal(first, second) {
			t.Errorf("Badges answered %v then %v for one input, want them identical", badgeTexts(first), badgeTexts(second))
		}
	})

	t.Run("the derivation reads no palette", func(t *testing.T) {
		source := badgeSource(t)

		ast.Inspect(source.File, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == "Theme" {
				t.Errorf("%s:%d reads a .Theme — a badge is a fact about a SLUG, and no palette, canvas or colour value may reach this derivation", source.Name, source.Fset.Position(sel.Pos()).Line)
			}
			return true
		})
	})
}

// badgeSource returns the parsed badge.go, failing rather than passing
// vacuously when the file is not found.
func badgeSource(t *testing.T) parsedThemeSource {
	t.Helper()

	for _, source := range parseThemeSources(t) {
		if source.Name == "badge.go" {
			return source
		}
	}
	t.Fatal("badge.go not found among internal/theme's production sources")
	return parsedThemeSource{}
}

// TestBadgeKey_MatchesRowIdentity pins the value a row is looked up in the badge
// map by: its IDENTITY — the slug where one exists, else the filename, else the
// raw persisted string.
//
// It is the same value SortKey derives from, and that is what makes task 8-1's
// charset-rejected persisted row — keyed on its raw string, having neither slug
// nor file — match the badge keyed on the same string. A key derived any other
// way would drop the badge on precisely the row §9.4 minted to carry it.
//
// The rows are built directly rather than assembled through a loader, because
// the claim is about the Row shape rather than about how one comes to exist
// (§13.3 keeps Row an ordinary value for exactly this reason).
func TestBadgeKey_MatchesRowIdentity(t *testing.T) {
	tests := []struct {
		name string
		row  theme.Row
		want string
	}{
		{
			name: "a built-in",
			row:  theme.Row{Slug: theme.DefaultDarkSlug, Source: theme.SourceBuiltin},
			want: theme.DefaultDarkSlug,
		},
		{
			name: "a valid drop-in file",
			row:  theme.Row{Slug: "nord-lee", Filename: "nord-lee.theme", Source: theme.SourceFile},
			want: "nord-lee",
		},
		{
			name: "a not-found persisted slug",
			row:  theme.Row{Slug: "ghost", Source: theme.SourcePersisted, Rejection: &theme.Rejection{Reason: theme.ReasonNotFound}},
			want: "ghost",
		},
		{
			name: "a charset-rejected persisted value",
			row:  theme.Row{Persisted: "../evil", Source: theme.SourcePersisted, Rejection: &theme.Rejection{Reason: theme.ReasonBadName}},
			want: "../evil",
		},
		{
			name: "a bad-name file",
			row:  theme.Row{Filename: "Nord Lee.theme", Source: theme.SourceFile, Rejection: &theme.Rejection{Reason: theme.ReasonBadName}},
			want: "Nord Lee.theme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.row.BadgeKey(); got != tt.want {
				t.Errorf("BadgeKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestBadgeKey_ReservedNameRowHasNone pins the ONE row that can never carry a
// badge, and the collision it exists to exclude.
//
// A `reserved name` row's slug is IDENTICAL to the built-in's by definition
// (§6.2) — that collision is the reason's entire content — so a bare identity
// lookup would paint `●` on BOTH rows. And the rejected file is not what is
// persisted: the persisted slug resolved to the BUILT-IN, which is the same
// discrimination doctor's persisted line draws.
//
// This is the only place §9.4's one legitimate two-rows-for-one-slug case has an
// observable consequence, so the exclusion is asserted end to end as well as on
// the method: with `nord` persisted and a `nord.theme` drop-in present, EXACTLY
// ONE of the union's rows matches the badge map.
func TestBadgeKey_ReservedNameRowHasNone(t *testing.T) {
	t.Run("the method returns no key", func(t *testing.T) {
		row := theme.Row{
			Slug:      "nord",
			Filename:  "nord.theme",
			Source:    theme.SourceFile,
			Rejection: &theme.Rejection{Reason: theme.ReasonReservedName},
		}

		if got := row.BadgeKey(); got != "" {
			t.Errorf("BadgeKey() = %q, want no key — the rejected file is not what is persisted", got)
		}
		if got, want := row.SortKey(), "nord"; got != want {
			t.Errorf("SortKey() = %q, want %q — the row still sorts beside the built-in it collides with", got, want)
		}
	})

	t.Run("only one row of a collided pair can render the dot", func(t *testing.T) {
		dir := t.TempDir()
		writeTheme(t, dir, "nord.theme", themeLines())
		loader := nominationLoader()
		setting, _ := theme.ResolveSetting("nord", "", "")

		_, union := loader.Open(dir, theme.RawKeys{Theme: "nord"})
		resolution, err := loader.ResolveNomination(setting, dir)
		if err != nil {
			t.Fatalf("ResolveNomination(a constant %q) = %v", "nord", err)
		}
		badges := theme.Badges(resolution.Slots)

		var badged []theme.Row
		for _, row := range union.Rows {
			if _, ok := badges[row.BadgeKey()]; ok {
				badged = append(badged, row)
			}
		}
		if len(badged) != 1 {
			t.Fatalf("%d rows match the badge map %v, want exactly 1 — only one `●` may render: %v", len(badged), badgeTexts(badges), rowIdentities(union))
		}
		if badged[0].Source != theme.SourceBuiltin {
			t.Errorf("the badged row is %+v, want the built-in — the persisted slug resolved to it, not to the rejected file", badged[0])
		}
	})
}

// badgeTexts renders a badge map as the strings a user would see, so a failure
// message names the badges rather than the enum's integer values.
func badgeTexts(badges map[string]theme.Badge) map[string]string {
	texts := make(map[string]string, len(badges))
	for key, badge := range badges {
		texts[key] = badge.Text()
	}
	return texts
}
