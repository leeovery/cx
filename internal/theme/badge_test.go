package theme_test

import (
	"go/ast"
	"maps"
	"testing"

	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

// TestBadges_ConstantIsBareDot pins the row-rendering rule's constant form: ONE slot, one
// entry, and NO SLOT WORD — the badge the panel draws as a bare `●`.
//
// With no slots there is nothing to qualify, so a slot label would be redundant
// with the marker itself. The entry is keyed on the constant's slug, which is
// the row the panel paints it on. The words each badge is drawn with are the
// panel's copy and are pinned there.
func TestBadges_ConstantIsBareDot(t *testing.T) {
	slots := []theme.SlotResolution{{Slot: theme.SlotConstant, Requested: "nord", Resolved: "nord"}}

	badges := theme.Badges(slots)

	want := map[string]theme.Badge{"nord": theme.BadgeConstant}
	if !maps.Equal(badges, want) {
		t.Fatalf("Badges(a constant) = %v, want %v", badgeNames(badges), badgeNames(want))
	}
}

// TestBadges_PairBadgesLightAndDark pins the row-rendering rule's adaptive form: each slot
// badges its OWN nominated slug, on its own row, with its own slot word.
//
// Both badges are present at all times, which is what lets a user see what light
// is set to without having to remember whether they ever set it.
func TestBadges_PairBadgesLightAndDark(t *testing.T) {
	slots := []theme.SlotResolution{
		{Slot: theme.SlotLight, Requested: "tokyo-night-day", Resolved: "tokyo-night-day"},
		{Slot: theme.SlotDark, Requested: "nord", Resolved: "nord"},
	}

	badges := theme.Badges(slots)

	want := map[string]theme.Badge{"tokyo-night-day": theme.BadgeLight, "nord": theme.BadgeDark}
	if !maps.Equal(badges, want) {
		t.Errorf("Badges(a pair) = %v, want %v", badgeNames(badges), badgeNames(want))
	}
}

// TestBadges_SameSlugInBothSlotsIsBoth pins the row-rendering rule's collapse: when both slots
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
		t.Fatalf("Badges(both slots on one slug) = %v, want a single entry", badgeNames(badges))
	}
	if got, want := badges["nord"], theme.BadgeBoth; got != want {
		t.Errorf("badge on %q = %s, want %s", "nord", badgeName(got), badgeName(want))
	}
}

// TestBadges_FallbackDoesNotMoveTheBadge pins the second row of the row-rendering rule's
// table: a slot that FELL BACK keeps its badge on the PERSISTED slug, and the
// fallback's own row carries NONE.
//
// This is the rejection-surface split's "falling back must never overwrite the persisted theme
// name" held at the display layer. A badge on the fallback would sit on a theme the
// user never chose and silently claim it as their choice — and because nothing
// was written, nothing would look wrong.
func TestBadges_FallbackDoesNotMoveTheBadge(t *testing.T) {
	slots := []theme.SlotResolution{
		{Slot: theme.SlotLight, Requested: theme.DefaultLightSlug, Resolved: theme.DefaultLightSlug},
		{Slot: theme.SlotDark, Requested: "gone-dark", Resolved: theme.DefaultDarkSlug, FellBack: true, Reason: theme.ReasonNotFound},
	}

	badges := theme.Badges(slots)

	want := map[string]theme.Badge{theme.DefaultLightSlug: theme.BadgeLight, "gone-dark": theme.BadgeDark}
	if !maps.Equal(badges, want) {
		t.Errorf("Badges(a fallen-back dark slot) = %v, want %v", badgeNames(badges), badgeNames(want))
	}
	if badge, ok := badges[theme.DefaultDarkSlug]; ok {
		t.Errorf("the fallback %q carries %s, want no badge — the marker means what is SET, and nobody set the fallback", theme.DefaultDarkSlug, badgeName(badge))
	}
}

// TestBadges_UnsetSlotBadgesShippedDefault pins the third row of the row-rendering rule's
// table over the MOST COMMON INSTALL: a virgin one, where the on-disk prefs shape leaves
// prefs.json absent entirely.
//
// It is the row that makes the union rule's justification for assembling the union true at
// all — "the `●` marker always has something to sit on" — because a
// persisted-slug-only rule would show no marker anywhere on this install. It is
// also the one place the no-unset rule's inherited-default-versus-pin distinction is visible
// to a user.
//
// The fixture runs the REAL read path (absent keys → the setting → the
// resolution) rather than hand-building the slots, because the claim under test
// is precisely that ResolveSetting's default substitution reaches Requested.
func TestBadges_UnsetSlotBadgesShippedDefault(t *testing.T) {
	requireDistinctDefaults(t)
	setting, _ := theme.ResolveSetting(theme.RawKeys{})

	resolution, err := nominationLoader().ResolveNomination(setting, t.TempDir())
	if err != nil {
		t.Fatalf("ResolveNomination(the shipped pair) = %v", err)
	}
	badges := theme.Badges(resolution.Slots)

	want := map[string]theme.Badge{theme.DefaultLightSlug: theme.BadgeLight, theme.DefaultDarkSlug: theme.BadgeDark}
	if !maps.Equal(badges, want) {
		t.Errorf("Badges(a virgin install) = %v, want %v — both shipped defaults badge their own row", badgeNames(badges), badgeNames(want))
	}
}

// TestBadges_CharsetRejectedValueKeepsItsBadge pins the badge on a persisted
// value the validate-before-use rule rejected before any file was sought: it sits on THAT RAW
// VALUE, so it meets the union row keyed on the same string.
//
// The row and the badge are derived by two different paths from one persisted
// value, and the value is the only thing either has to hold on to — it yields no
// slug and names no file. If the two keyed differently the badge would be lost on
// the one row whose entire purpose is to show the user what they set.
func TestBadges_CharsetRejectedValueKeepsItsBadge(t *testing.T) {
	const illegal = "../evil"
	loader := nominationLoader()
	setting, keys := theme.ResolveSetting(theme.RawKeys{Theme: illegal})

	_, union := theme.Assembler{Loader: loader}.Open(t.TempDir(), keys)
	resolution, err := loader.ResolveNomination(setting, t.TempDir())
	if err != nil {
		t.Fatalf("ResolveNomination(a charset-rejected constant) = %v", err)
	}
	badges := theme.Badges(resolution.Slots)

	if want := (map[string]theme.Badge{illegal: theme.BadgeConstant}); !maps.Equal(badges, want) {
		t.Fatalf("Badges = %v, want %v — the badge sits on the raw value", badgeNames(badges), badgeNames(want))
	}

	row := onlyPersistedRow(t, union)
	if got, want := row.BadgeKey(), illegal; got != want {
		t.Fatalf("the charset-rejected row's BadgeKey() = %q, want %q", got, want)
	}
	if badges[row.BadgeKey()] != theme.BadgeConstant {
		t.Errorf("the charset-rejected row carries %s, want %s — the row and its badge derive from one value by two paths and must meet", badgeName(badges[row.BadgeKey()]), badgeName(theme.BadgeConstant))
	}
}

// TestBadges_KeyedOnRequestedNotResolved pins the single field the row-rendering rule's whole
// three-row table reduces to, over a table where Requested and Resolved DIFFER
// on every slot.
//
// One rule covers all three rows and nothing branches on FellBack: the badge
// keys on what was NOMINATED, and what actually loaded is not an input.
func TestBadges_KeyedOnRequestedNotResolved(t *testing.T) {
	tests := []struct {
		name  string
		slots []theme.SlotResolution
		want  map[string]theme.Badge
	}{
		{
			name: "a constant that fell back",
			slots: []theme.SlotResolution{
				{Slot: theme.SlotConstant, Requested: "gone", Resolved: theme.DefaultDarkSlug, FellBack: true, Reason: theme.ReasonNotFound},
			},
			want: map[string]theme.Badge{"gone": theme.BadgeConstant},
		},
		{
			name: "both slots fell back",
			slots: []theme.SlotResolution{
				{Slot: theme.SlotLight, Requested: "gone-light", Resolved: theme.DefaultLightSlug, FellBack: true, Reason: theme.ReasonNotFound},
				{Slot: theme.SlotDark, Requested: "gone-dark", Resolved: theme.DefaultDarkSlug, FellBack: true, Reason: theme.ReasonBadColour},
			},
			want: map[string]theme.Badge{"gone-light": theme.BadgeLight, "gone-dark": theme.BadgeDark},
		},
		{
			name: "both slots fell back from the same slug",
			slots: []theme.SlotResolution{
				{Slot: theme.SlotLight, Requested: "gone", Resolved: theme.DefaultLightSlug, FellBack: true, Reason: theme.ReasonNotFound},
				{Slot: theme.SlotDark, Requested: "gone", Resolved: theme.DefaultDarkSlug, FellBack: true, Reason: theme.ReasonNotFound},
			},
			want: map[string]theme.Badge{"gone": theme.BadgeBoth},
		},
		{
			name: "a charset-rejected slot beside a loadable one",
			slots: []theme.SlotResolution{
				{Slot: theme.SlotLight, Requested: "../evil", Resolved: theme.DefaultLightSlug, FellBack: true, Reason: theme.ReasonBadName},
				{Slot: theme.SlotDark, Requested: "nord", Resolved: "nord"},
			},
			want: map[string]theme.Badge{"../evil": theme.BadgeLight, "nord": theme.BadgeDark},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			badges := theme.Badges(tt.slots)

			if !maps.Equal(badges, tt.want) {
				t.Errorf("Badges = %v, want %v", badgeNames(badges), badgeNames(tt.want))
			}
			for _, slot := range tt.slots {
				if slot.Resolved == slot.Requested {
					continue
				}
				if badge, ok := badges[slot.Resolved]; ok {
					t.Errorf("what %v resolved TO (%q) carries %s, want the badge on what it asked FOR (%q)", slot.Slot, slot.Resolved, badgeName(badge), slot.Requested)
				}
			}
		})
	}
}

// TestBadges_PureAndTotal pins the two properties every consumer rests on: the
// derivation is TOTAL — no input shape panics, including the ones that cannot
// legally arise — and it is PURE, so re-running it against unchanged state
// answers identically.
//
// Totality matters because the panel recomputes badges on a keypress path: a
// derivation that could panic on an unexpected slice would take the TUI down
// mid-commit. Purity is what makes the commit path's recompute a
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
			want  map[string]theme.Badge
		}{
			{name: "a nil slice", slots: nil, want: map[string]theme.Badge{}},
			{name: "an empty slice", slots: []theme.SlotResolution{}, want: map[string]theme.Badge{}},
			{
				name:  "a lone light slot",
				slots: []theme.SlotResolution{{Slot: theme.SlotLight, Requested: "nord"}},
				want:  map[string]theme.Badge{"nord": theme.BadgeLight},
			},
			{
				name: "a constant mixed with a slot",
				slots: []theme.SlotResolution{
					{Slot: theme.SlotConstant, Requested: "nord"},
					{Slot: theme.SlotDark, Requested: "tokyo-night"},
				},
				want: map[string]theme.Badge{"tokyo-night": theme.BadgeDark},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				badges := theme.Badges(tt.slots)

				if badges == nil {
					t.Fatal("Badges = nil, want an empty map — a nil map is a second shape of nothing")
				}
				if !maps.Equal(badges, tt.want) {
					t.Errorf("Badges = %v, want %v — a slice mixing the two setting states is a programming error, never both forms at once", badgeNames(badges), badgeNames(tt.want))
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
			t.Errorf("Badges answered %v then %v for one input, want them identical", badgeNames(first), badgeNames(second))
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
// It is the same value SortKey derives from, and that is what makes the
// charset-rejected persisted row — keyed on its raw string, having neither slug
// nor file — match the badge keyed on the same string. A key derived any other
// way would drop the badge on precisely the row the union rule minted to carry it.
//
// The rows are built directly rather than assembled through a loader, because
// the claim is about the Row shape rather than about how one comes to exist
// (the harness contract keeps Row an ordinary value for exactly this reason).
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
// — that collision is the reason's entire content — so a bare identity
// lookup would paint `●` on BOTH rows. And the rejected file is not what is
// persisted: the persisted slug resolved to the BUILT-IN, which is the same
// discrimination doctor's persisted line draws.
//
// This is the only place the union rule's one legitimate two-rows-for-one-slug case has an
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
		themetest.Write(t, dir, "nord.theme", themetest.Lines())
		loader := nominationLoader()
		setting, _ := theme.ResolveSetting(theme.RawKeys{Theme: "nord"})

		_, union := theme.Assembler{Loader: loader}.Open(dir, theme.RawKeys{Theme: "nord"})
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
			t.Fatalf("%d rows match the badge map %v, want exactly 1 — only one marker may render: %v", len(badged), badgeNames(badges), rowIdentities(union))
		}
		if badged[0].Source != theme.SourceBuiltin {
			t.Errorf("the badged row is %+v, want the built-in — the persisted slug resolved to it, not to the rejected file", badged[0])
		}
	})
}

// badgeNames renders a badge map by the enum's own constant names, so a failure
// message says which badge rather than printing its integer value. It does NOT
// render the badge's user-facing words: those are the panel's copy, asserted
// where they are declared.
func badgeNames(badges map[string]theme.Badge) map[string]string {
	names := make(map[string]string, len(badges))
	for key, badge := range badges {
		names[key] = badgeName(badge)
	}
	return names
}

// badgeName is one badge as its constant name.
func badgeName(badge theme.Badge) string {
	switch badge {
	case theme.BadgeConstant:
		return "BadgeConstant"
	case theme.BadgeLight:
		return "BadgeLight"
	case theme.BadgeDark:
		return "BadgeDark"
	case theme.BadgeBoth:
		return "BadgeBoth"
	default:
		return "BadgeNone"
	}
}
