package capture_test

import (
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/capture"
	"github.com/leeovery/portal/internal/sourceguard"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
	"github.com/leeovery/portal/internal/tui"
)

func messagePanelFixtureNames() []string {
	return []string{
		panelFixtureNamePrefix + "confirm",
		panelFixtureNamePrefix + "commit-failed",
		panelFixtureNamePrefix + "min-height-message",
	}
}

const (
	messagePanelTermWidth = 54

	messagePanelFloorTermHeight = 10
)

const themePanelConfirmCopy = "clear constant nord?  y / n"

var (
	confirmFooterRows  = []string{"y confirm", "n cancel"}
	standingFooterRows = []string{"⏎ set theme", "d set as dark", "l set as light", "esc close"}
)

func footerBlock(t *testing.T, lines []panelLine, want []string) (start int, rows []string) {
	t.Helper()

	start = len(lines) - len(want)
	if start < 0 {
		t.Fatalf("the panel renders %d lines, fewer than the footer's %d rows:\n%s", len(lines), len(want), panelText(lines))
	}
	for _, line := range lines[start:] {
		rows = append(rows, strings.Join(strings.Fields(line.visible), " "))
	}
	return start, rows
}

func TestPanelFixture_ConfirmFrame(t *testing.T) {
	palette := themetest.Builtin(t, theme.DefaultLightSlug)
	lines := panelLines(t, panelFrameAt(t, "theme-panel-confirm", palette, messagePanelTermWidth, harnessHeight))
	start := panelLineIndex(t, lines, "clear constant")
	footerStart, footerRows := footerBlock(t, lines, confirmFooterRows)

	t.Run("the slot carries the pinned confirm verbatim", func(t *testing.T) {
		if got, want := joinMessageLines(lines[start:footerStart]), strings.Join(strings.Fields(themePanelConfirmCopy), " "); got != want {
			t.Errorf("the message slot reads %q, want the pinned %q", got, want)
		}
	})

	t.Run("it renders in text.secondary with no band", func(t *testing.T) {
		raw := lines[start].raw
		if !strings.Contains(raw, fgSeq(t, palette.TextSecondary)) {
			t.Error("the confirm is not text.secondary; the slot's confirm state carries that token and no other")
		}
		for _, band := range []struct {
			name  string
			token theme.Token
		}{
			{"bg.attention", palette.BgAttention},
			{"bg.selection", palette.BgSelection},
			{"bg.subtle", palette.BgSubtle},
		} {
			if strings.Contains(raw, bgSeq(t, band.token)) {
				t.Errorf("the confirm carries a %s band; it takes NO band — it is text on the panel's own canvas", band.name)
			}
		}
		if !strings.Contains(raw, bgSeq(t, palette.Canvas)) {
			t.Error("the confirm does not carry the panel's canvas, so its cells are a terminal-bg island inside the panel body")
		}
	})

	t.Run("the footer is the confirm's two keys and nothing else", func(t *testing.T) {
		if got := footerRows; !slicesEqual(got, confirmFooterRows) {
			t.Errorf("the footer renders %v, want exactly %v — the standing scope lists four keys of which none would act", got, confirmFooterRows)
		}
		for _, standing := range standingFooterRows {
			if strings.Contains(panelFieldText(lines), standing) {
				t.Errorf("the panel still advertises %q while the confirm is live; the confirm substitutes the scope rather than extending it", standing)
			}
		}
	})
}

func TestPanelFixture_ConfirmWrapsAtMinWidth(t *testing.T) {
	palette := themetest.Builtin(t, theme.DefaultLightSlug)
	lines := panelLines(t, panelFrameAt(t, "theme-panel-confirm", palette, messagePanelTermWidth, harnessHeight))
	start := panelLineIndex(t, lines, "clear constant")
	footerStart, _ := footerBlock(t, lines, confirmFooterRows)
	messageRows := footerStart - start

	t.Run("the slot occupies two rows", func(t *testing.T) {
		if messageRows != 2 {
			t.Errorf("the confirm renders on %d row(s) at the panel's minimum width, want the 2 rows it wraps to:\n%s", messageRows, panelText(lines))
		}
	})

	t.Run("the same copy fits on one row at the preferred width", func(t *testing.T) {
		wide := panelLines(t, panelFixtureFrame(t, "theme-panel-confirm", palette))
		wideStart := panelLineIndex(t, wide, "clear constant")
		wideFooter, _ := footerBlock(t, wide, confirmFooterRows)
		if got := wideFooter - wideStart; got != 1 {
			t.Errorf("the confirm renders on %d row(s) at the preferred width, want 1 — the wrap must be a function of the WIDTH rather than of the copy", got)
		}
	})

	t.Run("the wrapped rows are charged to the list body", func(t *testing.T) {
		firstRow := panelLineIndex(t, lines, "catppuccin-latte")
		emptySlot := len(lines) - firstRow - len(confirmFooterRows)
		if got, want := start-firstRow, emptySlot-messageRows; got != want {
			t.Errorf("the list body renders %d rows, want %d — the slot's %d rows must come out of the BODY, not off the bottom of the panel where the footer is:\n%s", got, want, messageRows, panelText(lines))
		}
	})
}

const themePanelCommitFailedCopy = "⚠ couldn't save theme"

var foregroundRuns = regexp.MustCompile(`38;2;[0-9]+;[0-9]+;[0-9]+`)

func distinctForegrounds(raw string) []string {
	found := foregroundRuns.FindAllString(raw, -1)
	slices.Sort(found)
	return slices.Compact(found)
}

func TestPanelFixture_CommitFailedFrame(t *testing.T) {
	palette := themetest.Builtin(t, "nord")
	lines := panelLines(t, panelFixtureFrame(t, "theme-panel-commit-failed", palette))
	start := panelLineIndex(t, lines, "couldn't save theme")
	footerStart, footerRows := footerBlock(t, lines, standingFooterRows)

	t.Run("the slot carries the pinned failed-commit line verbatim", func(t *testing.T) {
		if got, want := joinMessageLines(lines[start:footerStart]), themePanelCommitFailedCopy; got != want {
			t.Errorf("the message slot reads %q, want the pinned %q", got, want)
		}
	})

	t.Run("the glyph and the text are one accent.attention run", func(t *testing.T) {
		got := distinctForegrounds(lines[start].raw)
		if want := []string{fgSeq(t, palette.AccentAttention)}; !slicesEqual(got, want) {
			t.Errorf("the line is painted with foregrounds %v, want the single accent.attention run %v — the `⚠` and the text are one signal", got, want)
		}
	})

	t.Run("it carries no bg.attention band", func(t *testing.T) {
		if strings.Contains(lines[start].raw, bgSeq(t, palette.BgAttention)) {
			t.Error("the failed-commit line carries a bg.attention band; it takes none — the warning band is a full-width main-screen treatment that reads as heavy inside a ~30-column panel")
		}
		if !strings.Contains(lines[start].raw, bgSeq(t, palette.Canvas)) {
			t.Error("the line does not carry the panel's canvas, so its cells are a terminal-bg island inside the panel body")
		}
	})

	t.Run("the standing footer is still in force", func(t *testing.T) {
		if !slicesEqual(footerRows, standingFooterRows) {
			t.Errorf("the footer renders %v, want the standing %v — only the confirm substitutes a scope, and a failed commit raises no question", footerRows, standingFooterRows)
		}
	})
}

func TestPanelFixture_CommitFailedBadgeUnmoved(t *testing.T) {
	palette := themetest.Builtin(t, "nord")
	failed := panelRows(t, panelFixtureFrame(t, "theme-panel-commit-failed", palette))
	prior := panelRows(t, panelFixtureFrame(t, "theme-panel-adaptive-pair", palette))

	for _, slug := range panelUnionSlugs() {
		t.Run(slug, func(t *testing.T) {
			if got, want := failed[slug].badge, prior[slug].badge; got != want {
				t.Errorf("the %s row's badge = %q with the failure live, want the pre-failure %q — a write that did not land must never move the `●`", slug, got, want)
			}
		})
	}

	t.Run("the pre-failure frame carries badges at all", func(t *testing.T) {
		if prior["nord"].badge == "" || prior[theme.DefaultLightSlug].badge == "" {
			t.Fatal("the sibling frame renders no badges, so comparing against it would prove nothing about the marker staying put")
		}
	})
}

func TestPanelFixture_MinHeightMessageFrame(t *testing.T) {
	palette := themetest.Builtin(t, "nord")
	frame := panelFrameAt(t, "theme-panel-min-height-message", palette, messagePanelTermWidth, messagePanelFloorTermHeight)
	lines := panelLines(t, frame)

	t.Run("one row shorter the panel refuses to open", func(t *testing.T) {
		short := ansi.Strip(panelFrameAt(t, "theme-panel-min-height-message", palette, messagePanelTermWidth, messagePanelFloorTermHeight-1))
		if strings.Contains(short, panelBorder) {
			t.Errorf("the panel still opens one row below the captured height, so that height is above the floor rather than on it:\n%s", short)
		}
		if !strings.Contains(short, "terminal too short for the theme picker") {
			t.Errorf("the refusal does not carry the pinned short-terminal copy:\n%s", short)
		}
	})

	t.Run("the panel opens at the captured height", func(t *testing.T) {
		visible := ansi.Strip(frame)
		if !strings.Contains(visible, "Themes") {
			t.Errorf("the frame carries no `Themes` header; the panel did not open at the floor:\n%s", visible)
		}
		for _, refusal := range panelEntryRefusalCopy {
			if strings.Contains(visible, refusal) {
				t.Errorf("the frame carries the blocked-entry flash %q, so the captured height is BELOW the floor", refusal)
			}
		}
	})

	body := panelLineIndex(t, lines, "nord")
	message := panelLineIndex(t, lines, "couldn't save theme")
	footerStart, footerRows := footerBlock(t, lines, standingFooterRows)

	t.Run("the body is the floor's one list row", func(t *testing.T) {
		if got := message - body; got != 1 {
			t.Errorf("the list body renders %d rows, want the ONE row the floor guarantees:\n%s", got, panelText(lines))
		}
	})

	t.Run("the slot is the floor's one message row", func(t *testing.T) {
		if got := footerStart - message; got != 1 {
			t.Errorf("the message slot renders %d rows, want the ONE row the floor counts:\n%s", got, panelText(lines))
		}
	})

	t.Run("the standing footer is in force and intact", func(t *testing.T) {
		if !slicesEqual(footerRows, standingFooterRows) {
			t.Errorf("the footer renders %v, want the standing %v — the floor is the standing scope's arithmetic, and a footer cut from the bottom loses `esc close` first", footerRows, standingFooterRows)
		}
	})
}

func TestPanelFixture_MinHeightMessageTruncates(t *testing.T) {
	palette := themetest.Builtin(t, "nord")

	t.Run("the seeded failure is one line", func(t *testing.T) {
		lines := panelLines(t, panelFrameAt(t, "theme-panel-min-height-message", palette, messagePanelTermWidth, messagePanelFloorTermHeight))
		start := panelLineIndex(t, lines, "couldn't save theme")
		footerStart, _ := footerBlock(t, lines, standingFooterRows)
		if got := footerStart - start; got != 1 {
			t.Errorf("the slot renders %d rows at the floor, want 1:\n%s", got, panelText(lines))
		}
	})

	t.Run("the copy that wraps above the floor is truncated at it", func(t *testing.T) {
		lines := panelLines(t, panelFrameAt(t, "theme-panel-confirm", themetest.Builtin(t, theme.DefaultLightSlug), messagePanelTermWidth, messagePanelFloorTermHeight))
		start := panelLineIndex(t, lines, "clear constant")
		footerStart, _ := footerBlock(t, lines, confirmFooterRows)
		if got := footerStart - start; got != 1 {
			t.Errorf("the confirm renders %d rows at the floor, want the 1 row the height rule pins:\n%s", got, panelText(lines))
		}
		if got := strings.TrimSpace(lines[start].visible); !strings.HasSuffix(got, "…") {
			t.Errorf("the confirm reads %q at the floor; the same copy wraps at this width above the floor, so at it the line must be TRUNCATED rather than laid out whole", got)
		}
	})
}

func joinMessageLines(lines []panelLine) string {
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		parts = append(parts, strings.TrimSpace(line.visible))
	}
	return strings.Join(parts, " ")
}

var messageSeedFields = []string{"ThemeConfirm", "ThemeCommitFailed"}

var pinnedMessageCopy = []string{"clear constant", "couldn't save theme"}

func TestPanelFixture_MessageSeedsAreStateOnly(t *testing.T) {
	t.Run("the seams carry a boolean and nothing else", func(t *testing.T) {
		seeds := reflect.TypeFor[tui.CaptureSeeds]()
		for _, name := range messageSeedFields {
			field, ok := seeds.FieldByName(name)
			if !ok {
				t.Errorf("tui.CaptureSeeds has no %s field; the message frames declare their state through it", name)
				continue
			}
			if field.Type.Kind() != reflect.Bool {
				t.Errorf("tui.CaptureSeeds.%s is a %s; a seed that can carry text can carry a paraphrase of the pinned copy", name, field.Type.Kind())
			}
		}
	})

	t.Run("no fixture source declares the copy", func(t *testing.T) {
		for _, path := range packageSourceFiles(t) {
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			for _, pinned := range pinnedMessageCopy {
				if strings.Contains(string(body), pinned) {
					t.Errorf("%s contains %q; the message slot's copy is single-sourced in internal/tui, and a catalogue holding its own would be free to drift from it", path, pinned)
				}
			}
		}
	})

	t.Run("the frames render it all the same", func(t *testing.T) {
		confirm := panelFixtureFrame(t, "theme-panel-confirm", themetest.Builtin(t, theme.DefaultLightSlug))
		if !strings.Contains(ansi.Strip(confirm), themePanelConfirmCopy) {
			t.Errorf("the confirm frame does not carry %q, so the absence above says nothing about where the copy comes from", themePanelConfirmCopy)
		}
		failed := panelFixtureFrame(t, "theme-panel-commit-failed", themetest.Builtin(t, "nord"))
		if !strings.Contains(ansi.Strip(failed), themePanelCommitFailedCopy) {
			t.Errorf("the failed-commit frame does not carry %q, so the absence above says nothing about where the copy comes from", themePanelCommitFailedCopy)
		}
	})
}

func packageSourceFiles(t *testing.T) []string {
	t.Helper()

	paths, err := sourceguard.PackageGoFiles(".", false)
	if err != nil {
		t.Fatalf("enumerate the internal/capture package sources: %v", err)
	}
	return paths
}

func TestPanelFixture_MessageFramesWireNoThemePersister(t *testing.T) {
	for _, name := range messagePanelFixtureNames() {
		t.Run(name, func(t *testing.T) {
			fx, err := capture.FixtureByName(name)
			if err != nil {
				t.Fatalf("FixtureByName(%s): %v", name, err)
			}
			if deps := fx.Deps(themetest.Builtin(t, theme.DefaultDarkSlug)); deps.ThemePersister != nil {
				t.Errorf("the fixture wires a ThemePersister (%#v); the seeded failure must write nowhere", deps.ThemePersister)
			}
		})
	}
}

func slicesEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
