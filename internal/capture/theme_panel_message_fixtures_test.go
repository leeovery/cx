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
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
	"github.com/leeovery/portal/internal/tui"
)

// §13.3's THREE REMAINING panel surfaces, as frames: the message slot in both of
// its states, and the panel at its minimum height with a message live.
//
// Each carries something no other frame does, and §13.4's guard structurally
// cannot report their absence — it enumerates whatever fixtures exist, so a
// missing one READS AS COVERAGE:
//
//   - the CONFIRM is the only visual proof of the footer substitution: while it is
//     live the standing four keys would ALL be inert, so the footer becomes
//     `y confirm` / `n cancel`, and nothing else on the frame says so;
//   - the FAILED-COMMIT line is the only place the deliberate absence of a
//     `bg.attention` band is checkable — the whole point of that decision is that a
//     full-width main-screen band reads as heavy inside a 27-column panel, which is
//     a judgement only an image settles;
//   - the MINIMUM-HEIGHT frame is the only surface on which §9.8's floor arithmetic
//     is observable at all, and the only check that the slot TRUNCATES to one line
//     there rather than wrapping.
//
// The assertions read the RENDERED frame rather than model internals, exactly as
// the sibling panel assertions do: internal/tui exports no panel accessor, and the
// frame is what the tapes screenshot.
//
// No t.Parallel() anywhere — the project bans it outright.

// messagePanelFixtureNames is the set this task registers, named once so a
// per-fixture assertion cannot cover two and forget the third.
func messagePanelFixtureNames() []string {
	return []string{
		"theme-panel-confirm",
		"theme-panel-commit-failed",
		"theme-panel-min-height-message",
	}
}

// The sizes the message frames are captured at, declared here because they are
// what their tapes resolve to and what these assertions must render at.
//
// VHS sets the terminal in PIXELS (`Set Width` / `Set Height` against
// `Set FontSize`), so a tape cannot state a column count directly — each tape's
// comment records the count its pixel size resolves to, and these constants are the
// same numbers on the Go side.
const (
	// messagePanelTermWidth lands the panel on §9.8's MINIMUM width, which is where
	// §9.1's wrap case lives: the ladder is `min(max(contentW/2, minimum), preferred)`
	// against a content region of termW − 2·tui.Hinset, so 54 columns gives 50 content
	// columns, half of which is below the minimum and clamps to it.
	messagePanelTermWidth = 54

	// messagePanelFloorTermHeight lands the panel EXACTLY on §9.8's height floor —
	// header + footer + one list row + one message row. 13 rows leaves 11 content
	// rows, which is that floor; a taller terminal gives the body a second row and
	// the frame stops being about the arithmetic.
	messagePanelFloorTermHeight = 13
)

// panelLine is one rendered line of the slide-over: the panel side of a frame
// line, with and without its SGR runs.
type panelLine struct {
	visible string
	raw     string
}

// panelLinesOf is every line of the rendered slide-over that carries its left
// border, top to bottom — the panel's own rows below the header rule.
//
// It exists beside panelLineWith because the message frames are about ADJACENCY as
// much as content: a wrapped message is two CONSECUTIVE rows, and what the slot
// costs the list body is observable only as a position. A content-matching lookup
// can state neither.
func panelLinesOf(t *testing.T, frame string) []panelLine {
	t.Helper()

	var lines []panelLine
	for line := range strings.SplitSeq(frame, "\n") {
		_, panel, onPanel := strings.Cut(line, panelBorder)
		if !onPanel {
			continue
		}
		lines = append(lines, panelLine{visible: strings.TrimRight(ansi.Strip(panel), " "), raw: panel})
	}
	if len(lines) == 0 {
		t.Fatalf("no frame line carries the panel's left border %q, so the panel did not render:\n%s", panelBorder, ansi.Strip(frame))
	}
	return lines
}

// panelLineIndex is the index within panelLinesOf's slice of the ONE panel line
// whose visible text carries want — fataling when there is none or more than one.
func panelLineIndex(t *testing.T, lines []panelLine, want string) int {
	t.Helper()

	found := -1
	for i, line := range lines {
		if !strings.Contains(line.visible, want) {
			continue
		}
		if found >= 0 {
			t.Fatalf("%q renders on panel lines %d AND %d; the assertions below locate blocks by position, which two matches make meaningless", want, found, i)
		}
		found = i
	}
	if found < 0 {
		t.Fatalf("no panel line carries %q:\n%s", want, panelText(lines))
	}
	return found
}

// panelText is the whole rendered slide-over as plain text, for a failure message
// that shows the frame the assertion was reading.
func panelText(lines []panelLine) string {
	visible := make([]string, 0, len(lines))
	for _, line := range lines {
		visible = append(visible, line.visible)
	}
	return strings.Join(visible, "\n")
}

// panelFieldText is the same frame with each line's runs of spaces collapsed — the
// form a footer row is SEARCHED for, since the fixed key column pads every glyph
// out and a literal `esc close` therefore appears nowhere in the raw text. A
// search against the raw form would pass whatever the footer said.
func panelFieldText(lines []panelLine) string {
	fields := make([]string, 0, len(lines))
	for _, line := range lines {
		fields = append(fields, strings.Join(strings.Fields(line.visible), " "))
	}
	return strings.Join(fields, "\n")
}

// themePanelConfirmCopy is §14A's slot-from-constant confirm as the confirm
// fixture renders it — the pinned format around that fixture's persisted constant.
//
// It is a literal here because internal/tui keeps the format unexported, and the
// point of the assertion is that the frame carries §14A's wording VERBATIM: a copy
// derived from the same place the renderer reads would agree with a paraphrase.
const themePanelConfirmCopy = "clear constant nord?  y / n"

// confirmFooterRows / standingFooterRows are the two scopes' footer labels, in
// descriptor order — §9.2's nested confirm scope and the panel's standing one.
var (
	confirmFooterRows  = []string{"y confirm", "n cancel"}
	standingFooterRows = []string{"⏎ set theme", "d set as dark", "l set as light", "esc close"}
)

// footerBlock is the panel's vertical keymap footer as rendered: the index its
// first row sits on within panelLinesOf's slice, and the rows themselves.
//
// The footer is the LAST block the panel assembles, so it is located from the
// BOTTOM — which is what makes "the footer survived" a statement the caller can
// make rather than an assumption. A footer pushed off the block by an over-long
// message loses `esc close` first, the one key that closes a panel the user can no
// longer read the way out of.
func footerBlock(t *testing.T, lines []panelLine, want []string) (start int, rows []string) {
	t.Helper()

	start = len(lines) - len(want)
	if start < 0 {
		t.Fatalf("the panel renders %d lines, fewer than the footer's %d rows:\n%s", len(lines), len(want), panelText(lines))
	}
	for _, line := range lines[start:] {
		// The key column is padded to a FIXED width so the labels share a left edge,
		// which puts a variable run of spaces between glyph and label. That is layout
		// rather than copy, so the comparison is on the fields.
		rows = append(rows, strings.Join(strings.Fields(line.visible), " "))
	}
	return start, rows
}

// TestPanelFixture_ConfirmFrame: it renders the confirm with its substituted
// footer.
//
// §13.3 mandates the message slot in BOTH states, and this frame is the ONLY
// visual proof of the footer substitution. While §9.2's confirm is live it is
// key-exclusive within the panel and resolves on `y`/`n`/`Esc` alone, so the
// standing `⏎ set theme` / `d set as dark` / `l set as light` / `esc close` would
// advertise four keys of which NONE would act — the dead end §14.3 is firm about.
// A reviewer cannot see the substitution anywhere else.
//
// # THE COHERENCE RULE: `--theme` MUST NAME THE THEME UNDER THE CURSOR
//
// Rendered under `tokyo-night-day`, the row the fixture seeds its cursor onto —
// deliberately NOT the persisted constant `nord` the confirm names.
func TestPanelFixture_ConfirmFrame(t *testing.T) {
	palette := themetest.Builtin(t, theme.DefaultLightSlug)
	lines := panelLinesOf(t, panelFrameAt(t, "theme-panel-confirm", palette, messagePanelTermWidth, harnessHeight))
	start := panelLineIndex(t, lines, "clear constant")
	footerStart, footerRows := footerBlock(t, lines, confirmFooterRows)

	t.Run("the slot carries §14A's confirm verbatim", func(t *testing.T) {
		if got, want := joinMessageLines(lines[start:footerStart]), themePanelConfirmCopy; got != want {
			t.Errorf("the message slot reads %q, want §14A's pinned %q", got, want)
		}
	})

	t.Run("it renders in text.secondary with no band", func(t *testing.T) {
		raw := lines[start].raw
		if !strings.Contains(raw, fgSeq(t, palette.TextSecondary)) {
			t.Error("the confirm is not text.secondary; §9.1's table gives the slot's confirm state that token and no other")
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
				t.Errorf("the confirm carries a %s band; §9.1's table gives it NO band — it is text on the panel's own canvas", band.name)
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
				t.Errorf("the panel still advertises %q while the confirm is live; §9.2 substitutes the scope rather than extending it", standing)
			}
		}
	})
}

// TestPanelFixture_ConfirmWrapsAtMinWidth: it wraps the confirm at the minimum
// width.
//
// §9.1 warns that "at the minimum panel width the slot may wrap to two rows", and
// §14A calls panel wording "a layout constraint as much as a copy choice" — so the
// one copy the spec says might not fit has to be capturable AT the width it might
// not fit at. This is that frame, and the fixture is captured there for this reason
// alone.
//
// THE SECOND ROW COMES OUT OF THE LIST BODY, which is the half a reader is most
// likely to assume rather than check. §9.1's slot is not reserved when empty, so
// its measured height is subtracted from the list's budget — and the alternative,
// an assembly that let the message push the block down, would take the row off the
// BOTTOM, where the footer is: `esc close` first, the one key that closes a panel
// the user can no longer read the way out of.
func TestPanelFixture_ConfirmWrapsAtMinWidth(t *testing.T) {
	palette := themetest.Builtin(t, theme.DefaultLightSlug)
	lines := panelLinesOf(t, panelFrameAt(t, "theme-panel-confirm", palette, messagePanelTermWidth, harnessHeight))
	start := panelLineIndex(t, lines, "clear constant")
	footerStart, _ := footerBlock(t, lines, confirmFooterRows)
	messageRows := footerStart - start

	t.Run("the slot occupies two rows", func(t *testing.T) {
		if messageRows != 2 {
			t.Errorf("the confirm renders on %d row(s) at the panel's minimum width, want the 2 §9.1 names:\n%s", messageRows, panelText(lines))
		}
	})

	t.Run("the same copy fits on one row at the preferred width", func(t *testing.T) {
		wide := panelLinesOf(t, panelFixtureFrame(t, "theme-panel-confirm", palette))
		wideStart := panelLineIndex(t, wide, "clear constant")
		wideFooter, _ := footerBlock(t, wide, confirmFooterRows)
		if got := wideFooter - wideStart; got != 1 {
			t.Errorf("the confirm renders on %d row(s) at the preferred width, want 1 — the wrap must be a function of the WIDTH rather than of the copy", got)
		}
	})

	t.Run("the wrapped rows are charged to the list body", func(t *testing.T) {
		firstRow := panelLineIndex(t, lines, "catppuccin-latte")
		// What the body would hold with the slot empty and this same footer: every
		// panel row from the first list row down, less the footer's own.
		emptySlot := len(lines) - firstRow - len(confirmFooterRows)
		if got, want := start-firstRow, emptySlot-messageRows; got != want {
			t.Errorf("the list body renders %d rows, want %d — the slot's %d rows must come out of the BODY, not off the bottom of the panel where the footer is:\n%s", got, want, messageRows, panelText(lines))
		}
	})
}

// themePanelCommitFailedCopy is §14A's failed-commit line, VERBATIM. It is a
// literal here for the reason the confirm's is: internal/tui keeps the constant
// unexported, and reading the copy from the same place the renderer reads it would
// agree with a paraphrase as readily as with §14A's wording.
const themePanelCommitFailedCopy = "⚠ couldn't save theme"

// foregroundRuns matches every truecolor FOREGROUND SGR core in a rendered line.
// Styled output carries no hex, so "which tokens painted this line" is answered by
// enumerating these.
var foregroundRuns = regexp.MustCompile(`38;2;[0-9]+;[0-9]+;[0-9]+`)

// distinctForegrounds is the set of foreground tokens a rendered line was painted
// with, deduped and sorted — the observable form of "the glyph and the text are ONE
// run" (§9.1's table gives the failed-commit line a single `accent.attention` run
// covering both).
func distinctForegrounds(raw string) []string {
	found := foregroundRuns.FindAllString(raw, -1)
	slices.Sort(found)
	return slices.Compact(found)
}

// TestPanelFixture_CommitFailedFrame: it renders the failed-commit line with no
// band.
//
// §13.3 mandates the message slot in BOTH states, and this frame is the ONLY place
// the deliberate ABSENCE of a `bg.attention` band is checkable. §9.1's table gives
// the failed-commit line `accent.attention` for the `⚠` and the text and NO band —
// the reasoning being that the band is a full-width main-screen flash treatment
// that would read as heavy inside a 27–34 column panel, which is a judgement only
// an image settles.
//
// # THE COHERENCE RULE: `--theme` MUST NAME THE THEME UNDER THE CURSOR
//
// Rendered under `nord`, the adaptive pair's dark slot — the row §9.2's open
// anchors the cursor to under capturetool's gate-free dark fallback (§8.8).
func TestPanelFixture_CommitFailedFrame(t *testing.T) {
	palette := themetest.Builtin(t, "nord")
	lines := panelLinesOf(t, panelFixtureFrame(t, "theme-panel-commit-failed", palette))
	start := panelLineIndex(t, lines, "couldn't save theme")
	footerStart, footerRows := footerBlock(t, lines, standingFooterRows)

	t.Run("the slot carries §14A's failed-commit line verbatim", func(t *testing.T) {
		if got, want := joinMessageLines(lines[start:footerStart]), themePanelCommitFailedCopy; got != want {
			t.Errorf("the message slot reads %q, want §14A's pinned %q", got, want)
		}
	})

	t.Run("the glyph and the text are one accent.attention run", func(t *testing.T) {
		got := distinctForegrounds(lines[start].raw)
		if want := []string{fgSeq(t, palette.AccentAttention)}; !slicesEqual(got, want) {
			t.Errorf("the line is painted with foregrounds %v, want the single accent.attention run %v — the `⚠` and the text are one signal (§9.1's table)", got, want)
		}
	})

	t.Run("it carries no bg.attention band", func(t *testing.T) {
		if strings.Contains(lines[start].raw, bgSeq(t, palette.BgAttention)) {
			t.Error("the failed-commit line carries a bg.attention band; §9.1's table refuses it — the warning band is a full-width main-screen treatment that reads as heavy inside a ~30-column panel")
		}
		if !strings.Contains(lines[start].raw, bgSeq(t, palette.Canvas)) {
			t.Error("the line does not carry the panel's canvas, so its cells are a terminal-bg island inside the panel body")
		}
	})

	t.Run("the standing footer is still in force", func(t *testing.T) {
		if !slicesEqual(footerRows, standingFooterRows) {
			t.Errorf("the footer renders %v, want the standing %v — only §9.2's confirm substitutes a scope, and a failed commit raises no question", footerRows, standingFooterRows)
		}
	})
}

// TestPanelFixture_CommitFailedBadgeUnmoved: it leaves the badge in place on a
// failure.
//
// §9.13: a failed commit "does not move the `●` — the marker means what is
// PERSISTED and would be lying if it moved". The frame is where that is visible,
// and the adaptive pair is what makes it legible: two slot badges on two different
// rows, both still where the persisted setting put them.
//
// It is asserted against the SIBLING FRAME the fixture is built from rather than
// against restated labels, so the claim is "the failure changed nothing about the
// badges" rather than "the badges are on the rows this test guessed".
func TestPanelFixture_CommitFailedBadgeUnmoved(t *testing.T) {
	palette := themetest.Builtin(t, "nord")
	failed := panelRows(t, panelFixtureFrame(t, "theme-panel-commit-failed", palette))
	prior := panelRows(t, panelFixtureFrame(t, "theme-panel-adaptive-pair", palette))

	for _, slug := range panelUnionSlugs() {
		t.Run(slug, func(t *testing.T) {
			if got, want := failed[slug].badge, prior[slug].badge; got != want {
				t.Errorf("the %s row's badge = %q with the failure live, want the pre-failure %q — §9.13 forbids a write that did not land moving the `●`", slug, got, want)
			}
		})
	}

	t.Run("the pre-failure frame carries badges at all", func(t *testing.T) {
		if prior["nord"].badge == "" || prior[theme.DefaultLightSlug].badge == "" {
			t.Fatal("the sibling frame renders no badges, so comparing against it would prove nothing about the marker staying put")
		}
	})
}

// TestPanelFixture_MinHeightMessageFrame: it renders the floor arithmetic with a
// message.
//
// §9.8 defines the height floor as header + footer + ONE LIST ROW + ONE MESSAGE
// ROW, and §13.3 requires a frame of it because "that arithmetic is only observable
// on a frame that renders it". The message row is counted although §9.1 leaves the
// slot unreserved when empty, because neither contender can be suppressed — so a
// floor computed without it puts the panel one row short at exactly the moment a
// message appears.
//
// THAT THE CAPTURED HEIGHT IS THE FLOOR IS PROVEN, NOT ASSERTED: one row shorter
// and §9.7's entry gate REFUSES with §14A's pinned copy. Without that leg the frame
// would only be of some short terminal, and the arithmetic it is captured for would
// be a coincidence of the size someone picked.
func TestPanelFixture_MinHeightMessageFrame(t *testing.T) {
	palette := themetest.Builtin(t, "nord")
	frame := panelFrameAt(t, "theme-panel-min-height-message", palette, messagePanelTermWidth, messagePanelFloorTermHeight)
	lines := panelLinesOf(t, frame)

	t.Run("one row shorter the panel refuses to open", func(t *testing.T) {
		short := ansi.Strip(panelFrameAt(t, "theme-panel-min-height-message", palette, messagePanelTermWidth, messagePanelFloorTermHeight-1))
		if strings.Contains(short, panelBorder) {
			t.Errorf("the panel still opens one row below the captured height, so that height is above §9.8's floor rather than on it:\n%s", short)
		}
		if !strings.Contains(short, "terminal too short for the theme picker") {
			t.Errorf("the refusal does not carry §14A's pinned short-terminal copy:\n%s", short)
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
			t.Errorf("the list body renders %d rows, want the ONE §9.8's floor guarantees:\n%s", got, panelText(lines))
		}
	})

	t.Run("the slot is the floor's one message row", func(t *testing.T) {
		if got := footerStart - message; got != 1 {
			t.Errorf("the message slot renders %d rows, want the ONE §9.8's floor counts:\n%s", got, panelText(lines))
		}
	})

	t.Run("the standing footer is in force and intact", func(t *testing.T) {
		if !slicesEqual(footerRows, standingFooterRows) {
			t.Errorf("the footer renders %v, want the standing %v — the floor is the standing scope's arithmetic, and a footer cut from the bottom loses `esc close` first", footerRows, standingFooterRows)
		}
	})
}

// TestPanelFixture_MinHeightMessageTruncates: it truncates rather than wraps at
// the floor height.
//
// §9.1's two dimensions degrade DIFFERENTLY on purpose: at the minimum WIDTH the
// slot may wrap to two rows, but "it does cost a row of vertical budget, so at the
// minimum HEIGHT the message is truncated to one line rather than wrapped". A
// two-row message at the floor would leave zero list rows or overflow the frame.
//
// THE CONTROL IS THE CONFIRM FIXTURE, and it is what makes this a statement about
// the RULE rather than about a message that happened to fit. The failed-commit line
// is short enough for the minimum width either way, so its single row proves
// nothing on its own; the confirm's copy provably wraps at that same width — this
// file's own wrap assertion captures it doing so — and must collapse to one
// truncated line at the floor.
func TestPanelFixture_MinHeightMessageTruncates(t *testing.T) {
	palette := themetest.Builtin(t, "nord")

	t.Run("the seeded failure is one line", func(t *testing.T) {
		lines := panelLinesOf(t, panelFrameAt(t, "theme-panel-min-height-message", palette, messagePanelTermWidth, messagePanelFloorTermHeight))
		start := panelLineIndex(t, lines, "couldn't save theme")
		footerStart, _ := footerBlock(t, lines, standingFooterRows)
		if got := footerStart - start; got != 1 {
			t.Errorf("the slot renders %d rows at the floor, want 1:\n%s", got, panelText(lines))
		}
	})

	t.Run("the copy that wraps above the floor is truncated at it", func(t *testing.T) {
		lines := panelLinesOf(t, panelFrameAt(t, "theme-panel-confirm", themetest.Builtin(t, theme.DefaultLightSlug), messagePanelTermWidth, messagePanelFloorTermHeight))
		start := panelLineIndex(t, lines, "clear constant")
		footerStart, _ := footerBlock(t, lines, confirmFooterRows)
		if got := footerStart - start; got != 1 {
			t.Errorf("the confirm renders %d rows at the floor, want the 1 §9.1's height rule pins:\n%s", got, panelText(lines))
		}
		if got := strings.TrimSpace(lines[start].visible); !strings.HasSuffix(got, "…") {
			t.Errorf("the confirm reads %q at the floor; the same copy wraps at this width above the floor, so at it the line must be TRUNCATED rather than laid out whole", got)
		}
	})
}

// joinMessageLines reassembles a wrapped message slot into the one line its copy
// was written as — the wrap broke it at a space, so the space is what rejoins it.
func joinMessageLines(lines []panelLine) string {
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		parts = append(parts, strings.TrimSpace(line.visible))
	}
	return strings.Join(parts, " ")
}

// messageSeedFields are the two capture-seed fields the message frames declare
// their state through. They are named here so the structural check below fails on
// a RENAME as loudly as on a retyping — a seed that quietly became something else
// is exactly what would let a fixture start carrying copy.
var messageSeedFields = []string{"ThemeConfirm", "ThemeCommitFailed"}

// pinnedMessageCopy is §14A's message-slot copy in both states, as the strings a
// fixture must NOT contain.
//
// The confirm is matched on its invariant PREFIX rather than on the whole line: the
// slug is fixture data (it is the persisted constant), so the format around it is
// the part that belongs to the panel.
var pinnedMessageCopy = []string{"clear constant", "couldn't save theme"}

// TestPanelFixture_MessageSeedsAreStateOnly: it seeds state, never text.
//
// §14A pins the message slot's copy, and in the panel that wording is "a layout
// constraint as much as a copy choice" — so a fixture able to supply its own text
// could ship a frame that reads plausibly, reviews cleanly and says something the
// spec never wrote. The seeds are therefore BOOLEANS that declare which contender
// is live, and the copy is composed by the slot from its own pinned constants.
//
// Both halves are checked, because either alone is weak: a boolean seam proves
// nothing if the catalogue holds the copy anyway, and an absent literal proves
// nothing if the seam could carry one tomorrow.
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
				t.Errorf("tui.CaptureSeeds.%s is a %s; a seed that can carry text can carry a paraphrase of §14A's copy", name, field.Type.Kind())
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
					t.Errorf("%s contains %q; the message slot's copy is §14A's, single-sourced in internal/tui, and a catalogue holding its own would be free to drift from it", path, pinned)
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

// packageSourceFiles is every non-test Go file of internal/capture — the fixture
// catalogue and the seams it wires, which is where a fixture-supplied string would
// have to live.
func packageSourceFiles(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the package directory: %v", err)
	}
	var paths []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		paths = append(paths, name)
	}
	if len(paths) == 0 {
		t.Fatal("the package directory holds no non-test Go files; the scan below would be vacuous")
	}
	return paths
}

// TestPanelFixture_MessageFramesRegistered: it registers all three in both lists.
//
// FixtureByName's switch and FixtureNames()'s slice are two hand-maintained lists,
// and a fixture present in one and absent from the other is INVISIBLE to §13.4's
// enumerating guard — it exists, it renders, and nothing ever swaps it. Absence
// reads as coverage, which is the shape task 4-3's drift check exists to close;
// this states the same claim for the three by name, so a half-registration fails
// here with the fixture named.
func TestPanelFixture_MessageFramesRegistered(t *testing.T) {
	for _, name := range messagePanelFixtureNames() {
		t.Run(name, func(t *testing.T) {
			if _, err := capture.FixtureByName(name); err != nil {
				t.Errorf("FixtureByName(%s): %v — the fixture is not resolvable", name, err)
			}
			if !slices.Contains(capture.FixtureNames(), name) {
				t.Errorf("FixtureNames() %v omits %s — an unenumerated fixture is never driven by the guard", capture.FixtureNames(), name)
			}
		})
	}
}

// TestPanelFixture_MessageFramesUnderTheGuard: it is enumerated by the
// swap-and-diff guard.
//
// §13.4's guard NEVER NAMES A FIXTURE — it iterates the registry — so the claim
// worth pinning is that these three arrive in the set its assertions range over,
// with no edit to the guard itself. That is the whole reason the specified frames
// are ENUMERATED rather than listed.
func TestPanelFixture_MessageFramesUnderTheGuard(t *testing.T) {
	guarded := make([]string, 0, len(capture.FixtureNames()))
	for _, fx := range guardedFixtures(t) {
		guarded = append(guarded, fx.Name())
	}
	for _, name := range messagePanelFixtureNames() {
		if !slices.Contains(guarded, name) {
			t.Errorf("%s is not in the guarded set %v; the guard enumerates, so a fixture it never sees is uncovered while reading as covered", name, guarded)
		}
	}
}

// TestPanelFixture_MessageFramesWireNoThemePersister: their seeded states write
// nowhere.
//
// THE WRITE HALF MATTERS DISPROPORTIONATELY ON THESE THREE. One of them is seeded
// into a FAILED-COMMIT state, which in production is reached by a write — so a
// reader is entitled to ask what the seed wrote. The answer is nothing at all: the
// theme-commit seam is unwired, which is what this states outright and what the
// untouched config directory in the sibling no-config-access assertion corroborates.
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

// slicesEqual reports whether two rendered row sets match element for element.
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
