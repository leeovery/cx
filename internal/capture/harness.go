package capture

import (
	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

// This file is the fixture DRIVER: the seam §13.4's swap-and-diff completeness
// guard drives a fixture from, as distinct from fixtures.go, which is the fixture
// catalogue.
//
// It performs no config discovery of any kind — no XDG lookup, no prefs read, no
// themes-directory read. A palette arrives as an injected value, exactly as an
// explicit `--theme <path>` does, which is what keeps internal/capture's
// no-real-config guarantee intact (§13.3).

// keyRune builds the key-press message for a printable rune, carrying both the
// code and its text so the model's rune-key matchers see it exactly as a real
// keyboard would deliver it.
func keyRune(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// keyPageDown builds the `Ctrl+↓` key-press message — §12.2's page-forward key,
// which the panel's list binds through the shared pinArrowOnlyNav.
//
// It is a CODE PLUS A MODIFIER rather than a key type of its own, which is how
// Bubble Tea v2 models ctrl combos, and it carries NO Text: a modified key is not
// a printable character, and a Text-bearing one would additionally match the
// rune arms it must not reach.
func keyPageDown() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModCtrl}
}

// ModelAt builds the fixture's production tui.Model painted from th and drives it
// to the fixture's CAPTURED state — deterministically, through Update, with no
// tea program running.
//
// The theme is injected as the CONSTANT nomination shape, exactly as capturetool
// passes it (§13.3): a constant needs no light/dark detection, so the gate is
// resolved at construction and the frame is un-gated and byte-stable. It is
// handed to Deps rather than assigned afterwards, because the palette drives TWO
// seams — the nomination and the panel's faked enumerator, whose Resolve must
// report the same palette or task 8-8's open-time apply repaints the frame off
// `--theme`. See Fixture.Deps.
//
// Fixtures reach their captured state through Init/Update, so a bare Build returns
// a model that has ingested nothing — a blank or loading frame. The drive order
// below mirrors the production message sequence with every BLOCKING command left
// uninvoked (the LoadingMinDuration pad tick, the loading receiver's channel
// read), so nothing here can wait on a timer or a channel:
//
//  1. the window size, so both lists size to the inset content region;
//  2. the fixture's own session set as a SessionsMsg;
//  3. the fixture's own project set as a ProjectsLoadedMsg — after the sessions,
//     matching the ordering correction the production ProjectsLoadedMsg arm exists
//     to handle, so a grouped fixture re-groups against real project records;
//  4. each seeded loading event, then the seeded fatal when there is one;
//  5. the post-load key script (see Fixture.captureKeys).
//
// Deliberately absent: BootstrapCompleteMsg and LoadingMinElapsedMsg. The loading
// fixtures park on the loading page precisely because neither arrives, which is
// what keeps their captured frames deterministic.
func (f *Fixture) ModelAt(th theme.Theme, w, h int) tui.Model {
	var model tea.Model = tui.Build(f.Deps(th))
	model, _ = model.Update(tea.WindowSizeMsg{Width: w, Height: h})

	sessions, err := f.Lister.ListSessions()
	model, _ = model.Update(tui.SessionsMsg{Sessions: sessions, Err: err})

	projects, err := f.projectStore.List()
	model, _ = model.Update(tui.ProjectsLoadedMsg{Projects: projects, Err: err})

	for _, event := range f.loadingEvents {
		model, _ = model.Update(event)
	}
	if f.fatalEvent.FailedStep > 0 {
		model, _ = model.Update(f.fatalEvent)
	}
	for _, key := range f.captureKeys {
		model, _ = model.Update(key)
	}
	return model.(tui.Model)
}

// RenderSwapRender renders the fixture under theme a, swaps it live to theme b
// through the production entry point, and renders it again — returning both
// frames. It is the seam §13.4's completeness guard drives.
//
// EXACTLY ONE MODEL IS CONSTRUCTED, and that is the whole point. The caches the
// guard exists to catch are assigned once at construction, so a harness that built
// two models — one per theme — would assign every cached style correctly in each
// and hand back two perfectly correct frames while live swap was broken. That is
// §13.4's named vacuous-pass shape, and the fixture harness's one-shot render is
// precisely the shape that invites it.
//
// The A-render is therefore NOT optional set-up: it is what populates those
// caches. A fixture rendered only after the swap passes trivially.
//
// The swap goes through Model.ApplyTheme — the same entry point the panel's
// arrow-preview drives — so what the guard proves is the production mechanism and
// not a test-only setter or a rebuild.
func (f *Fixture) RenderSwapRender(a, b theme.Theme, w, h int) (before, after string) {
	m := f.ModelAt(a, w, h)
	before = m.View().Content
	m.ApplyTheme(b)
	return before, m.View().Content
}

// Colourless reports whether the fixture renders under the NO_COLOR carve-out.
//
// It is the discriminator §13.4's guard excludes on: a colourless render contains
// no theme hexes at all, so there is nothing to diff and inclusion would be
// meaningless rather than merely redundant. Reading the flag off the fixture's own
// Deps makes that exclusion STRUCTURAL rather than a hand-maintained name list, so
// a colourless fixture added later is excluded automatically.
//
// The palette handed to Deps is the ZERO one, and that is the one call site where
// it is the honest argument: a colourless fixture paints from no palette at all,
// and the flag being read is decided before any of them.
//
// No fixture sets it today; the accessor exists so that one which does is handled
// without anyone remembering the rule.
func (f *Fixture) Colourless() bool {
	return f.Deps(theme.Theme{}).NoColor
}
