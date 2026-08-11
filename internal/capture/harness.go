package capture

import (
	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

// This file performs no config discovery — no XDG lookup, no prefs read, no
// themes-directory read. A palette arrives only as an injected value, which
// keeps the package's no-real-config guarantee intact.

func keyRune(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// No Text: a modified key is not printable, and a Text-bearing message would
// also match rune arms it must not reach.
func keyPageDown() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModCtrl}
}

// ModelAt builds the fixture's production tui.Model painted from th and drives
// it to its captured state through Update, with no tea program running.
// BootstrapCompleteMsg and LoadingMinElapsedMsg are deliberately never sent:
// loading fixtures stay parked precisely because neither arrives.
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
// through the production Model.ApplyTheme, and renders again, returning both
// frames. Exactly one model, deliberately: cached styles are assigned at
// construction, so two models would each render correctly while live swap was
// broken — the A-render populates those caches.
func (f *Fixture) RenderSwapRender(a, b theme.Theme, w, h int) (before, after string) {
	m := f.ModelAt(a, w, h)
	before = m.View().Content
	m.ApplyTheme(b)
	return before, m.View().Content
}

// Colourless reports whether the fixture renders under the NO_COLOR carve-out.
// Read off the fixture's own Deps so the exclusion stays structural rather
// than a hand-maintained name list.
func (f *Fixture) Colourless() bool {
	return f.Deps(theme.Theme{}).NoColor
}
