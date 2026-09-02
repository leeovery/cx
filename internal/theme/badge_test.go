package theme_test

import (
	"go/ast"
	"maps"
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

func TestBadges_ConstantIsBareDot(t *testing.T) {
	slots := []theme.SlotResolution{{Slot: theme.SlotConstant, Requested: "nord", Resolved: "nord"}}

	badges := theme.Badges(slots)

	want := map[string]theme.Badge{"nord": theme.BadgeConstant}
	if !maps.Equal(badges, want) {
		t.Fatalf("Badges(a constant) = %v, want %v", badgeNames(badges), badgeNames(want))
	}
}

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
				t.Errorf("%s:%d reads a .Theme — a badge is a fact about a SLUG, and no palette, canvas or colour value may reach this derivation", source.Path, source.Fset.Position(sel.Pos()).Line)
			}
			return true
		})
	})
}

func badgeSource(t *testing.T) sourceguardtest.ParsedSource {
	t.Helper()

	for _, source := range sourceguardtest.ParsePackageSources(t, ".", false) {
		if filepath.Base(source.Path) == "badge.go" {
			return source
		}
	}
	t.Fatal("badge.go not found among internal/theme's production sources")
	return sourceguardtest.ParsedSource{}
}

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
			if got, want := tt.row.BadgeKey(), tt.row.Identity(); got != want {
				t.Errorf("BadgeKey() = %q, want the row's identity %q", got, want)
			}
		})
	}
}

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
		if got, want := row.Identity(), "nord"; got != want {
			t.Errorf("Identity() = %q, want %q — the row still IS the slug it collides on", got, want)
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
			t.Fatalf("%d rows match the badge map %v, want exactly 1 — only one marker may render: %v", len(badged), badgeNames(badges), rowFingerprints(union))
		}
		if badged[0].Source != theme.SourceBuiltin {
			t.Errorf("the badged row is %+v, want the built-in — the persisted slug resolved to it, not to the rejected file", badged[0])
		}
	})
}

func badgeNames(badges map[string]theme.Badge) map[string]string {
	names := make(map[string]string, len(badges))
	for key, badge := range badges {
		names[key] = badgeName(badge)
	}
	return names
}

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
