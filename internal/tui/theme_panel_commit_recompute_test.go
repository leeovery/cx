package tui

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
)

// §9.2's POST-COMMIT RECOMPUTE: "a successful commit recomputes the panel's full
// row set, not just the badges."
//
// The row SET moves in both directions, which is the half that is easy to miss —
// `Enter` clears both slots, so a row that existed only because a slot named it
// loses its reason to exist and DISAPPEARS, while a slot commit makes the other
// slot live so a slug with no file and no built-in gains one and a row APPEARS.
//
// Three traps sit on that recompute and each is a named test below:
//
//   - THE MERGED BYTES. Re-deriving from what the commit's read-modify-write just
//     read would import another instance's writes at the moment the user presses a
//     key — the cross-instance sync §8.4 declines, arrived at through the write
//     path instead of the open path.
//   - THE DIRECTORY. Re-reading it would break §5.8's pin (enumeration belongs to
//     panel open; a commit changes prefs, not the directory) and would mint a third
//     parse that can disagree with the row on screen.
//   - THE INDEX. Anchoring the cursor to one silently breaks §9.2's invariant the
//     moment a row is inserted above it: the screen keeps previewing one theme
//     while the cursor sits on another.
//
// No t.Parallel() — the package-level mock convention makes parallelism unsafe
// across this package's tests.

// newRecomputePanelModel is the recompute fixture: a REAL loader over a REAL
// themes directory, a recording persister, and the panel already open.
//
// The loader is real because the recompute's whole subject is the DERIVATION —
// which rows the union holds for a given set of persisted keys, and in what order.
// A stub seam answering with a fixed union would make every row assertion a
// statement about the fixture rather than about theme.Reassemble.
func newRecomputePanelModel(t *testing.T, dir string, keys theme.RawKeys) (Model, *realThemeEnumerator, *fakeThemePersister) {
	t.Helper()

	m, enumerator, _ := newClosePanelModel(t, dir, keys)
	persister := &fakeThemePersister{}
	WithThemePersister(persister)(&m)
	m.termWidth, m.termHeight = arrowTermW, arrowTermH
	m.applySessions(closePanelSessions())

	m = pressThemeKey(t, m)
	if !m.themePanel.open {
		t.Fatal("fixture: the panel did not open")
	}
	return m, enumerator, persister
}

// requireRowLabels fails unless the panel lists exactly these labels, in this
// order — the ORDER being half of what a recompute has to get right (§9.5).
func requireRowLabels(t *testing.T, m Model, want ...string) {
	t.Helper()

	got := themePanelRowLabels(m)
	if len(got) != len(want) {
		t.Fatalf("the panel lists %v, want %v", got, want)
	}
	for i, label := range want {
		if got[i] != label {
			t.Fatalf("panel row %d is %q, want %q — the full list is %v, want %v", i, got[i], label, got, want)
		}
	}
}

// TestPanelRecompute_RowDisappearsOnConstantCommit: it removes a row whose only
// reason to exist was a cleared slot.
//
// §9.2: "`Enter` clears both slots, so a `not found` or charset-rejected row that
// existed *only* because a slot named it loses its reason to exist and
// **disappears**." The row is §9.4's persisted-slug row — minted so the `●` always
// has something to sit on — and with the slot cleared there is no longer a `●` for
// it to hold.
//
// The pre-commit assertion is the control: the row IS there before the keypress,
// so its absence afterwards is a recompute rather than a union that never held it.
func TestPanelRecompute_RowDisappearsOnConstantCommit(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
	m, _, persister := newRecomputePanelModel(t, dir, theme.RawKeys{Light: "ghost", Dark: "sunset"})

	requireRowLabels(t, m, "ghost", "nord", "sunset", theme.DefaultDarkSlug, theme.DefaultLightSlug)
	requireCursorOn(t, m, "sunset")

	m, _ = pressCommitKey(t, m)

	requireCommitted(t, persister, "sunset")
	requireConstantKeys(t, m, "sunset")
	requireRowLabels(t, m, "nord", "sunset", theme.DefaultDarkSlug, theme.DefaultLightSlug)
	requireCursorOn(t, m, "sunset")
}

// commitSlotForTest drives a SLOT commit through the PRODUCTION path (task 9-3's
// commitSlot): the named slot written, the constant cleared in the same write and
// the other slot left untouched — prefs.SaveThemeSlot's on-disk shape (§8.2)
// mirrored onto the model's raw keys — followed by this task's recompute.
//
// IT TAKES (slug, slot) RATHER THAN THE FINISHED KEYS, so the recompute is driven
// from a state a single slot commit can actually produce: a helper assigning keys
// of its own could hand the panel a pair no keypress could reach and the row
// assertions below would be about that instead.
//
// The KEY that reaches this commit is gated behind task 9-5's confirm while a
// constant is set (§9.2), which is why the helper calls the commit directly rather
// than pressing `d` — the recompute's half is identical whichever route gets there.
func commitSlotForTest(t *testing.T, m Model, slug string, slot prefs.ThemeSlot) Model {
	t.Helper()

	if err := (&m).commitSlot(slug, slot); err != nil {
		t.Fatalf("commitSlot(%s, %v): %v", slug, slot, err)
	}
	if m.themeKeys.Theme != "" {
		t.Fatalf("the slot commit left the constant %q set; §8.2 clears it in the SAME write", m.themeKeys.Theme)
	}
	return m
}

// TestPanelRecompute_RowAppearsForNewlyLiveSlot: it inserts a row for a
// newly-live slot.
//
// §9.2: "`d`/`l` on a constant makes the other slot live (§8.2). If that slot names
// a slug with no file and no built-in, §9.4 requires it to have a row — and the
// open-time union never minted one, because a `theme`-wins file's slots are not
// read at all. So a row **appears**."
//
// The fixture is therefore a hand-edited prefs.json carrying all three keys, which
// §8.2 makes legal and resolves as a CONSTANT — the one shape in which a persisted
// slot names something the open-time union is blind to.
func TestPanelRecompute_RowAppearsForNewlyLiveSlot(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
	keys := theme.RawKeys{Theme: "sunset", Light: "ghost", Dark: "sunset"}
	m, _, _ := newRecomputePanelModel(t, dir, keys)

	requireRowLabels(t, m, "nord", "sunset", theme.DefaultDarkSlug, theme.DefaultLightSlug)
	requireCursorOn(t, m, "sunset")

	m = commitSlotForTest(t, m, keys.Dark, prefs.SlotDark)

	requireRowLabels(t, m, "ghost", "nord", "sunset", theme.DefaultDarkSlug, theme.DefaultLightSlug)
	ghost := themePanelRowFor(t, m, "ghost")
	if ghost.Row.Selectable() {
		t.Error("the minted `ghost` row is selectable; a slug with no file and no built-in is unselectable with its reason (§9.4)")
	}
	if got := ghost.Row.Rejection.Reason; got != theme.ReasonNotFound {
		t.Errorf("the minted `ghost` row carries reason %q, want %q — the slug resolves to nothing (§9.4)", got, theme.ReasonNotFound)
	}
}

// TestPanelRecompute_ReSortsThroughTheComparator: it re-sorts an inserted row into
// place.
//
// §9.5's order is applied INSIDE theme.Reassemble, so the recompute states no
// comparator of its own and performs no caller-side sort. The inserted slug is
// chosen to land in the MIDDLE of the union: appending it, prepending it, or
// leaving the pre-commit order alone all produce a different list, so the
// assertion cannot pass on an accident of position.
func TestPanelRecompute_ReSortsThroughTheComparator(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
	keys := theme.RawKeys{Theme: "sunset", Light: "prism", Dark: "sunset"}
	m, _, _ := newRecomputePanelModel(t, dir, keys)

	requireRowLabels(t, m, "nord", "sunset", theme.DefaultDarkSlug, theme.DefaultLightSlug)

	m = commitSlotForTest(t, m, keys.Dark, prefs.SlotDark)

	requireRowLabels(t, m, "nord", "prism", "sunset", theme.DefaultDarkSlug, theme.DefaultLightSlug)
}

// renderRecomputePanel renders the panel block at the height it is composited into,
// so a badge assertion can be made against what the user actually sees rather than
// against the item field alone.
func renderRecomputePanel(m Model) string {
	return renderThemePanel(m.themePanel, m.contentHeight(), m.activeTheme, m.colourless)
}

// requireBadgeText fails unless the rendered panel carries exactly the given badge
// texts, counted by occurrence.
//
// The bare `●` is counted as OCCURRENCES OF THE GLYPH minus the slot forms, because
// `● light` and `● dark` both contain it: counting the glyph alone would report a
// pair as three badges, and counting the constant's text as a whole string would
// report every slot badge as one.
func requireBadgeText(t *testing.T, m Model, wantBare, wantLight, wantDark int) {
	t.Helper()

	rendered := ansi.Strip(renderRecomputePanel(m))
	light := strings.Count(rendered, "● light")
	dark := strings.Count(rendered, "● dark")
	bare := strings.Count(rendered, "●") - light - dark
	if bare != wantBare || light != wantLight || dark != wantDark {
		t.Errorf("the panel renders %d bare `●`, %d `● light` and %d `● dark`; want %d/%d/%d\n%s",
			bare, light, dark, wantBare, wantLight, wantDark, rendered)
	}
}

// TestPanelRecompute_VirginInstallBadgeCollapse: it collapses two slot badges to
// one bare dot.
//
// The MOST COMMON first write in the product (§8.9): §8.1 leaves a fresh install
// with no prefs.json at all, so both slots arrive as the shipped defaults and the
// panel opens carrying `● light` and `● dark` (§9.5's third badge row — unset and
// set-to-the-default are the same state). `Enter` writes one constant and clears
// both slots, so the pair collapses to a single bare `●` — "two inherited defaults
// just became one pin".
//
// Both the item BADGES and the RENDERED text are asserted: the map is what the
// recompute writes, the glyphs are what the user is told.
func TestPanelRecompute_VirginInstallBadgeCollapse(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
	m, _, persister := newRecomputePanelModel(t, dir, theme.RawKeys{})

	requireCursorOn(t, m, theme.DefaultDarkSlug)
	requireBadge(t, m, theme.DefaultDarkSlug, theme.BadgeDark)
	requireBadge(t, m, theme.DefaultLightSlug, theme.BadgeLight)
	requireBadgeText(t, m, 0, 1, 1)

	m, _ = pressCommitKey(t, m)

	requireCommitted(t, persister, theme.DefaultDarkSlug)
	requireConstantKeys(t, m, theme.DefaultDarkSlug)
	requireBadge(t, m, theme.DefaultDarkSlug, theme.BadgeConstant)
	requireBadge(t, m, theme.DefaultLightSlug, theme.BadgeNone)
	requireBadge(t, m, "nord", theme.BadgeNone)
	requireBadge(t, m, "sunset", theme.BadgeNone)
	requireBadgeText(t, m, 1, 0, 0)
}

// TestPanelRecompute_ReadsNothing: it reads no directory and no prefs.
//
// The two halves are §5.8's and §8.4's, and each is driven the only way it cannot
// be faked. The DIRECTORY is deleted after the panel opened, so a re-read would
// have nothing to find; PREFS.JSON on disk is seeded to name a slug this instance
// has never held, so a re-read — of the file OR of the merged bytes the persister's
// read-modify-write just had in hand (§8.9) — would show up as a row.
func TestPanelRecompute_ReadsNothing(t *testing.T) {
	t.Run("no directory read", func(t *testing.T) {
		dir := t.TempDir()
		writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
		m, enumerator, persister := newRecomputePanelModel(t, dir, theme.RawKeys{Light: "ghost", Dark: "sunset"})
		requireRowLabels(t, m, "ghost", "nord", "sunset", theme.DefaultDarkSlug, theme.DefaultLightSlug)

		if err := os.RemoveAll(dir); err != nil {
			t.Fatalf("remove the themes directory: %v", err)
		}
		m, _ = pressCommitKey(t, m)

		requireCommitted(t, persister, "sunset")
		requireRowLabels(t, m, "nord", "sunset", theme.DefaultDarkSlug, theme.DefaultLightSlug)
		if enumerator.opens != 1 {
			t.Errorf("the commit ran %d enumerations in total, want the single one the open performed — §5.8 pins enumeration to panel OPEN, and a commit changes prefs rather than the directory", enumerator.opens)
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("the themes directory exists again after the commit (err %v); nothing on this path touches it", err)
		}
	})

	t.Run("no prefs read", func(t *testing.T) {
		const onDisk = `{"theme":"sunset","theme_light":"phantom","theme_dark":"sunset"}`
		prefsFile := filepath.Join(t.TempDir(), "prefs.json")
		if err := os.WriteFile(prefsFile, []byte(onDisk), 0o644); err != nil {
			t.Fatalf("write prefs: %v", err)
		}
		t.Setenv("PORTAL_PREFS_FILE", prefsFile)
		dir := t.TempDir()
		writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
		// The IN-MEMORY light slot names `ghost`; the file names `phantom`. §8.4's
		// construction-time snapshot is what this instance holds, and the recompute
		// re-derives from THAT plus its own mutation.
		keys := theme.RawKeys{Theme: "sunset", Light: "ghost", Dark: "sunset"}
		m, _, _ := newRecomputePanelModel(t, dir, keys)

		m = commitSlotForTest(t, m, keys.Dark, prefs.SlotDark)

		requireRowLabels(t, m, "ghost", "nord", "sunset", theme.DefaultDarkSlug, theme.DefaultLightSlug)
		after, err := os.ReadFile(prefsFile)
		if err != nil {
			t.Fatalf("read back prefs: %v", err)
		}
		if string(after) != onDisk {
			t.Errorf("prefs.json =\n%s\nwant it byte-identical:\n%s", after, onDisk)
		}
	})
}

// sgrPattern matches one SGR escape sequence, which is how "the frame's colours"
// is measured as a SET rather than as bytes: a recompute legitimately changes the
// frame (rows move), so a byte comparison cannot say whether the palette moved
// with them.
var sgrPattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

// frameColours is the sorted, deduplicated set of SGR sequences in a frame.
func frameColours(frame string) []string {
	seqs := sgrPattern.FindAllString(frame, -1)
	slices.Sort(seqs)
	return slices.Compact(seqs)
}

// TestPanelRecompute_DoesNotApplyTheme: it never re-themes.
//
// §9.2: a commit is a WRITE, NOT A NAVIGATION. The re-resolution the recompute runs
// is for the BADGES alone — it selects no new active member and calls no
// Model.ApplyTheme — so the previewed palette and the frame's colours survive a
// keypress that visibly changes the rows.
//
// THE FIXTURE ARROWS AWAY FIRST, and what that buys is narrower than it looks —
// worth stating exactly, because a later task could otherwise read this as a
// guarantee it is not. `Enter` commits the CURSOR's slug, so after the write the
// persisted row IS the previewed row: a recompute that re-resolved and applied
// THAT would be an invisible no-op here rather than a caught defect. What this
// test does catch is an apply of any OTHER row — a re-resolution reading the
// pre-commit keys, the other slot, or §8.5's fallback — and the no-op direction is
// closed elsewhere, structurally by task 9-1's "the commit path calls no
// ApplyTheme" scan and observably by its byte-identical frame comparison. The
// claim bites directly only on task 9-3's SLOT commit, where committing the
// non-active slot leaves the previewed and the resolved themes genuinely
// different.
//
// The row-set assertion is the control — without it, "the colours did not move" is
// equally true of a recompute that never ran.
func TestPanelRecompute_DoesNotApplyTheme(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
	writeThemeFileForTest(t, dir, "sunset.theme", "#202020")
	m, _, persister := newRecomputePanelModel(t, dir, theme.RawKeys{Light: "ghost", Dark: "aurora"})
	requireCursorOn(t, m, "aurora")

	m = arrowToThemeRow(t, m, "sunset")
	previewed := m.activeTheme
	if got := previewed.Canvas.Value; got != "#202020" {
		t.Fatalf("fixture: arrowing to `sunset` rendered canvas %s, want #202020 — the preview is what the commit must leave alone", got)
	}
	colours := frameColours(m.View().Content)

	m, _ = pressCommitKey(t, m)

	requireCommitted(t, persister, "sunset")
	// The control: the recompute genuinely ran, so the assertions below are about a
	// keypress that changed the panel rather than one that changed nothing.
	requireRowLabels(t, m, "aurora", "nord", "sunset", theme.DefaultDarkSlug, theme.DefaultLightSlug)
	if m.activeTheme != previewed {
		t.Errorf("the recompute rendered canvas %s, want the previewed %s left alone — the re-resolution is for the badges, never for selecting a new active member (§9.2)", m.activeTheme.Canvas.Value, previewed.Canvas.Value)
	}
	if got := frameColours(m.View().Content); !slices.Equal(got, colours) {
		t.Errorf("the recompute changed the frame's colours\nbefore: %v\nafter:  %v", colours, got)
	}
}

// TestPanelRecompute_CursorAnchoredByIdentity: it anchors the cursor by identity.
//
// §9.2: the recompute "re-anchors the cursor to the previewed theme's identity,
// never to its index. Anchoring to an index would silently break §9.2's invariant
// the moment a row is inserted above the cursor: the screen would keep previewing
// one theme while the cursor sat on another."
//
// So the fixture inserts a row ABOVE the cursor — the one shape in which an index
// anchor and an identity anchor disagree — and the INDEX MOVING is as much the
// assertion as the label not moving.
func TestPanelRecompute_CursorAnchoredByIdentity(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
	keys := theme.RawKeys{Theme: "sunset", Light: "ghost", Dark: "sunset"}
	m, _, _ := newRecomputePanelModel(t, dir, keys)

	requireCursorOn(t, m, "sunset")
	previewed := m.activeTheme
	before := m.themePanel.list.Index()
	if before != 1 {
		t.Fatalf("fixture: the cursor opened at index %d, want 1 — the inserted row must land above it", before)
	}

	m = commitSlotForTest(t, m, keys.Dark, prefs.SlotDark)

	requireRowLabels(t, m, "ghost", "nord", "sunset", theme.DefaultDarkSlug, theme.DefaultLightSlug)
	requireCursorOn(t, m, "sunset")
	if got := m.themePanel.list.Index(); got != before+1 {
		t.Errorf("the cursor sits at index %d, want %d — the inserted row pushed `sunset` down and the anchor followed the IDENTITY", got, before+1)
	}
	if m.activeTheme != previewed {
		t.Errorf("the recompute rendered canvas %s, want the previewed %s — the cursor's row is always what is painted behind the panel (§9.2)", m.activeTheme.Canvas.Value, previewed.Canvas.Value)
	}
}

// TestPanelRecompute_NoChangeCommitIsStable: it is idempotent for an unchanged
// commit.
//
// Committing the slug that is ALREADY the constant changes nothing on disk, so the
// recompute must produce an identical row set, an identical badge map and an
// identical cursor — which makes it provably idempotent rather than merely usually
// harmless. The rendered frame is compared byte for byte, because with nothing
// moving there is nothing a recompute may legitimately change.
func TestPanelRecompute_NoChangeCommitIsStable(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
	m, _, persister := newRecomputePanelModel(t, dir, theme.RawKeys{Theme: "sunset"})

	requireCursorOn(t, m, "sunset")
	rows := themePanelRowLabels(m)
	badges := maps.Clone(m.themePanel.badges)
	index := m.themePanel.list.Index()
	frame := m.View().Content

	m, _ = pressCommitKey(t, m)

	requireCommitted(t, persister, "sunset")
	requireConstantKeys(t, m, "sunset")
	requireRowLabels(t, m, rows...)
	if got := m.themePanel.badges; !maps.Equal(got, badges) {
		t.Errorf("the badge map is %v after an unchanged commit, want the identical %v", got, badges)
	}
	if got := m.themePanel.list.Index(); got != index {
		t.Errorf("the cursor sits at index %d after an unchanged commit, want the identical %d", got, index)
	}
	if got := m.View().Content; got != frame {
		t.Errorf("an unchanged commit changed the composed frame\nbefore: %q\nafter:  %q", escSeq(frame), escSeq(got))
	}
}

// errThemeResolveFatal is §7.6's fatal as the seam hands it back — the ONLY error
// Resolve can return, from a binary whose embedded set cannot supply a fallback.
// A real loader cannot produce it against a correctly built binary, which is why
// the degrade path is driven through a fake.
var errThemeResolveFatal = errors.New("theme: the embedded set cannot supply a fallback")

// splitThemeEnumerator answers Open and Reassemble with DIFFERENT unions, which is
// how the recompute's own effect is isolated: whatever the panel lists after a
// commit came from the REASSEMBLY, and whatever it lists without one came from the
// open.
//
// It is a fake rather than a real loader because the three states below are ones a
// correct loader cannot reach — a selectable row vanishing from under the cursor
// (no directory is re-read, so a valid file's row cannot go away), a Resolve
// returning §7.6's fatal, and a reassembly the panel must render in the assembler's
// order rather than in its own.
type splitThemeEnumerator struct {
	opened      theme.Union
	reassembled theme.Union
	resolution  theme.Resolution
	resolveErr  error
	reassembles int
	slotLoads   []slotLoad
}

func (e *splitThemeEnumerator) Open(theme.RawKeys) (theme.Enumeration, theme.Union) {
	return theme.Enumeration{DirPath: fixtureThemesDir}, e.opened
}

func (e *splitThemeEnumerator) Reassemble(theme.Enumeration, theme.RawKeys) theme.Union {
	e.reassembles++
	return e.reassembled
}

// Resolve answers with the ZERO Resolution alongside an error, which is the
// loader's own contract (§7.6: "the Resolution alongside it is the zero value
// precisely so there is nothing to be tempted to render") — and therefore the
// shape the degrade policy has to survive, since theme.Badges yields an EMPTY map
// for the empty slot slice a zero Resolution carries.
func (e *splitThemeEnumerator) Resolve(theme.Enumeration, theme.Setting) (theme.Resolution, error) {
	if e.resolveErr != nil {
		return theme.Resolution{}, e.resolveErr
	}
	return e.resolution, nil
}

// ResolveSlot records §8.4's commit-time asks and answers from the declared
// resolution — its record for that slot, or the declared nomination's member where
// no record was declared for it — so a split-union fixture can state whether the
// conversion's load ran at all without a real loader, and answers it with a palette
// the fixture chose either way (see stubThemeEnumerator.ResolveSlot).
func (e *splitThemeEnumerator) ResolveSlot(_ theme.Enumeration, slot theme.Slot, slug string) (theme.SlotResolution, error) {
	e.slotLoads = append(e.slotLoads, slotLoad{slot: slot, slug: slug})
	if e.resolveErr != nil {
		return theme.SlotResolution{}, e.resolveErr
	}
	for _, declared := range e.resolution.Slots {
		if declared.Slot == slot {
			return declared, nil
		}
	}
	return theme.SlotResolution{
		Slot:      slot,
		Requested: slug,
		Resolved:  slug,
		Theme:     e.resolution.Nomination.Select(slot == theme.SlotDark),
	}, nil
}

// themeRowsUnion wraps hand-declared rows as the union shape the seam hands back.
func themeRowsUnion(rows []theme.Row) theme.Union {
	return theme.Union{Rows: rows, Count: len(rows), Rejected: arrowRejectedCount(rows)}
}

// newSplitPanelModel opens the panel over `opened`, with the seam primed to answer
// the recompute's reassembly with `reassembled`.
func newSplitPanelModel(t *testing.T, opened, reassembled []theme.Row, cursorSlug string) (Model, *splitThemeEnumerator, *fakeThemePersister) {
	t.Helper()

	target := arrowRowBySlug(t, opened, cursorSlug)
	enumerator := &splitThemeEnumerator{
		opened:      themeRowsUnion(opened),
		reassembled: themeRowsUnion(reassembled),
		resolution: theme.Resolution{
			Nomination: theme.ConstantNomination(target.Theme),
			Slots: []theme.SlotResolution{{
				Slot:      theme.SlotConstant,
				Requested: cursorSlug,
				Resolved:  cursorSlug,
				Theme:     target.Theme,
			}},
		},
	}
	persister := &fakeThemePersister{}
	deps := Deps{
		Lister:          fakeLister{},
		Theme:           theme.ConstantNomination(target.Theme),
		ThemeEnumerator: enumerator,
		ThemeKeys:       theme.RawKeys{Theme: cursorSlug},
		ThemePersister:  persister,
	}
	return openCommitPanel(t, deps, PageSessions, cursorSlug), enumerator, persister
}

// TestPanelRecompute_CursorClampsOnMissingIdentity: it degrades when the previewed
// identity is gone.
//
// A STRUCTURAL GUARD, NOT A LIVE PATH. The cursor is always on a selectable row
// (§9.2) and the recompute re-reads no directory, so a valid file's row cannot go
// away underneath it — the reassembly can only add or drop the persisted-slug rows,
// which are unselectable. The guard is here because the alternative to degrading is
// indexing out of range inside a list the user is looking at.
//
// The clamp is anchorThemePanelCursor's own: the FIRST SELECTABLE row, which is why
// the reassembly leads with an unselectable one — landing on index 0 would put the
// cursor somewhere the arrows, which skip unselectable rows, cannot return to.
func TestPanelRecompute_CursorClampsOnMissingIdentity(t *testing.T) {
	opened := arrowValidRows(4)
	reassembled := []theme.Row{
		arrowInvalidRow(arrowSlug(0)),
		arrowValidRow(arrowSlug(1), 1),
		arrowValidRow(arrowSlug(3), 3),
	}
	m, _, persister := newSplitPanelModel(t, opened, reassembled, arrowSlug(2))

	m, _ = pressCommitKey(t, m)

	requireCommitted(t, persister, arrowSlug(2))
	requireRowLabels(t, m, arrowSlug(0), arrowSlug(1), arrowSlug(3))
	requireCursorOn(t, m, arrowSlug(1))
	if row := themePanelCursorRow(t, m); !row.Selectable() {
		t.Errorf("the clamp landed the cursor on the unselectable %q; §9.2's invariant is that the cursor is always on a SELECTABLE row", row.Label())
	}
}

// TestPanelRecompute_ResolveErrorKeepsBadges: it keeps the badges when the
// re-resolution errors.
//
// §7.6's fatal is the only error Resolve can return, and the panel DEGRADES on it
// rather than escalating — the policy applyInForceTheme states once for all three
// panel call sites. Here degrading specifically means KEEPING THE EXISTING TABLE:
// theme.Badges returns an EMPTY map for an empty slice, so deriving from a zero
// Resolution would wipe every `●` off the panel at the exact moment the user
// committed one — the marker lying in the direction §9.13's "a failed commit does
// not move the `●`" rule exists to forbid.
//
// The other steps still run on that path, because they read the mutated keys and
// the retained enumeration rather than the resolution: the reassembly's rows are
// rendered IN ITS OWN ORDER (deliberately not alphabetical here, so a caller-side
// sort would show up) and the cursor is re-anchored by identity.
func TestPanelRecompute_ResolveErrorKeepsBadges(t *testing.T) {
	opened := arrowValidRows(4)
	reassembled := []theme.Row{
		arrowValidRow(arrowSlug(1), 1),
		arrowValidRow(arrowSlug(0), 0),
		arrowValidRow(arrowSlug(3), 3),
	}
	m, enumerator, persister := newSplitPanelModel(t, opened, reassembled, arrowSlug(0))
	badges := maps.Clone(m.themePanel.badges)
	if badges[arrowSlug(0)] != theme.BadgeConstant {
		t.Fatalf("fixture: the open left badges %v, want a constant `●` on %q", badges, arrowSlug(0))
	}
	enumerator.resolveErr = errThemeResolveFatal

	m, _ = pressCommitKey(t, m)

	requireCommitted(t, persister, arrowSlug(0))
	if got := m.themePanel.badges; !maps.Equal(got, badges) {
		t.Errorf("the failed re-resolution left badges %v, want the existing %v — an empty map would wipe every `●` off the panel", got, badges)
	}
	requireBadge(t, m, arrowSlug(0), theme.BadgeConstant)
	requireRowLabels(t, m, arrowSlug(1), arrowSlug(0), arrowSlug(3))
	requireCursorOn(t, m, arrowSlug(0))
}

// TestPanelRecompute_SkippedOnFailedCommit: it does not run on a failed write.
//
// §9.13: a failed commit "does not move the `●` — the marker means 'what is
// persisted' and would be lying if it moved". The badges are derived from the raw
// keys, which a failed write leaves untouched, so the mechanism is that the
// recompute is never reached at all.
//
// The seam's two unions are what make that observable: a reassembly that ran would
// replace the panel's rows outright, so an unchanged list plus a zero reassembly
// count is the recompute genuinely not happening. The positive control is the same
// fixture with the write succeeding.
func TestPanelRecompute_SkippedOnFailedCommit(t *testing.T) {
	opened := arrowValidRows(4)
	reassembled := arrowValidRows(2)

	m, enumerator, persister := newSplitPanelModel(t, opened, reassembled, arrowSlug(0))
	badges := maps.Clone(m.themePanel.badges)
	persister.err = errThemeCommitFailed

	m, _ = pressCommitKey(t, m)

	requireCommitted(t, persister, arrowSlug(0))
	requireRowLabels(t, m, arrowSlug(0), arrowSlug(1), arrowSlug(2), arrowSlug(3))
	if got := m.themePanel.badges; !maps.Equal(got, badges) {
		t.Errorf("the failed commit left badges %v, want the untouched %v — a failed commit does not move the `●` (§9.13)", got, badges)
	}
	if enumerator.reassembles != 0 {
		t.Errorf("the failed commit ran %d reassemblies, want 0 — only a SUCCESSFUL commit recomputes (§9.2)", enumerator.reassembles)
	}

	// Positive control: the same fixture with the write succeeding does recompute,
	// so the untouched rows above are the failure path rather than an unwired one.
	wired, control, _ := newSplitPanelModel(t, opened, reassembled, arrowSlug(0))
	wired, _ = pressCommitKey(t, wired)
	requireRowLabels(t, wired, arrowSlug(0), arrowSlug(1))
	if control.reassembles != 1 {
		t.Fatalf("positive control: a successful commit ran %d reassemblies, want 1", control.reassembles)
	}
}

// TestPanelRecompute_ItemsReplacedNotRebuilt: it keeps the list instance and its
// re-pointed styles.
//
// §11.1 rules a rebuild out as the expensive path and §11.2 names this instance the
// WORST CASE of the cached-style class, so the recompute replaces the list's ITEMS
// and nothing else. The `bubbles/list`-owned styles stay RE-POINTED by task 8-9's
// restyle path rather than reassigned here — a commit changes no theme, so there is
// nothing to re-point.
//
// The PAGINATION DOTS are the class's exemplar and the reason a rebuild would be
// visible: `bubbles/list` reads its dot STRINGS out of the styles once at
// construction, so a list rebuilt here would print the library's hardcoded greys
// under every theme. The TITLE is the instance probe — the panel draws its own
// header and disables the list's (newThemePanelList), so nothing in production ever
// reads or writes this field, and a value written into it survives an item swap
// while a fresh list.Model could not carry it.
func TestPanelRecompute_ItemsReplacedNotRebuilt(t *testing.T) {
	const (
		sentinel   = "recompute-instance-probe"
		dotsHeight = 16
	)
	opened := arrowValidRows(20)
	m, _, persister := newSplitPanelModel(t, opened, opened[:19], arrowSlug(0))
	m.themePanel.list.Title = sentinel
	width, height := m.themePanel.list.Width(), m.themePanel.list.Height()
	if width <= 0 || height <= 0 {
		t.Fatalf("fixture: the panel list is sized %dx%d, so an unchanged size says nothing", width, height)
	}

	m, _ = pressCommitKey(t, m)

	requireCommitted(t, persister, arrowSlug(0))
	if got := len(m.themePanel.list.Items()); got != 19 {
		t.Fatalf("the panel lists %d rows after the recompute, want the reassembly's 19", got)
	}
	if got := m.themePanel.list.Title; got != sentinel {
		t.Errorf("the panel list's Title is %q, want the sentinel %q — the recompute replaced the list MODEL rather than its items", got, sentinel)
	}
	if w, h := m.themePanel.list.Width(), m.themePanel.list.Height(); w != width || h != height {
		t.Errorf("the panel list is sized %dx%d after the recompute, want the unchanged %dx%d", w, h, width, height)
	}
	if got := m.themePanel.list.KeyMap.CursorUp.Keys(); len(got) == 0 {
		t.Error("the panel list's keymap binds nothing after the recompute; the list was replaced by a fresh model")
	}

	dots := themePanelDotRow(t, renderThemePanel(m.themePanel, dotsHeight, m.activeTheme, m.colourless))
	for _, tc := range []struct {
		what  string
		token theme.Token
	}{
		{what: "active dot", token: m.activeTheme.AccentPrimary},
		{what: "inactive dot", token: m.activeTheme.TextFaint},
	} {
		if want := tokenFgSeq(t, tc.token); !strings.Contains(dots, want) {
			t.Errorf("the rendered %s does not carry the previewed theme's SGR %q — a rebuilt list prints `bubbles/list`'s own greys: %q", tc.what, want, escSeq(dots))
		}
	}
}
