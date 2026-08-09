package tui

// Tests in this file parse the package's own sources and build models through
// the shared constructor, so they MUST NOT use t.Parallel.

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/tmux"
)

// fakeThemePersister records what the seam was asked to commit, and can be made
// to fail.
//
// `slugs` is every commit whatever its shape, which is what a "nothing was
// written" assertion wants; `constants` is the CommitTheme calls alone and
// `slots` the CommitThemeSlot ones, so §9.2's two commit keys are
// distinguishable — `Enter` writes the constant and clears both slots, `d`/`l`
// write a slot and clear the constant, and a test asserting one must not pass
// over the other.
//
// `slotCommits` is the slot calls with their SLUG AND SLOT TOGETHER. The flat
// slices above cannot express that pairing once a fixture drives both commit
// shapes — `slugs` then interleaves them — and §9.2's `d`/`l` is precisely a
// statement about which slug went into which slot.
//
// `err` is returned by both methods. §9.13's outstanding-failure state machine
// renders its line from the value the seam hands back, so a failed write has to
// be drivable end to end rather than only described.
type fakeThemePersister struct {
	slugs       []string
	constants   []string
	slots       []prefs.ThemeSlot
	slotCommits []slotCommit
	err         error
}

// slotCommit is one recorded CommitThemeSlot call.
type slotCommit struct {
	slug string
	slot prefs.ThemeSlot
}

func (f *fakeThemePersister) CommitTheme(slug string) error {
	f.slugs = append(f.slugs, slug)
	f.constants = append(f.constants, slug)
	return f.err
}

func (f *fakeThemePersister) CommitThemeSlot(slug string, slot prefs.ThemeSlot) error {
	f.slugs = append(f.slugs, slug)
	f.slots = append(f.slots, slot)
	f.slotCommits = append(f.slotCommits, slotCommit{slug: slug, slot: slot})
	return f.err
}

// TestBuild_NilThemePersisterIsTolerated pins the seam's nil-tolerance, the shape
// WithModePersister already has: a nil Deps field applies NO option, so the model
// holds a nil persister and a capture (or any test model) writes nowhere.
//
// The positive control is what stops the nil assertion passing for the wrong
// reason — an option that never wired anything would satisfy the first subtest
// forever.
func TestBuild_NilThemePersisterIsTolerated(t *testing.T) {
	t.Run("a nil dep leaves the seam unwired", func(t *testing.T) {
		m := Build(Deps{Lister: fakeLister{}})

		if m.themeState.persister != nil {
			t.Errorf("themePersister = %#v, want nil — a nil Deps.ThemePersister must apply no option", m.themeState.persister)
		}
	})

	t.Run("the model built that way drives without panicking", func(t *testing.T) {
		var model tea.Model = Build(Deps{Lister: fakeLister{}})

		model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
		model, _ = model.Update(SessionsMsg{Sessions: []tmux.Session{{Name: "dev", Windows: 1}}})
		model, _ = model.Update(ProjectsLoadedMsg{})
		for _, key := range []tea.KeyPressMsg{
			{Code: 's', Text: "s"},
			{Code: 'x', Text: "x"},
			{Code: '?', Text: "?"},
		} {
			model, _ = model.Update(key)
		}

		if content := model.(Model).View().Content; content == "" {
			t.Error("the nil-persister model painted nothing; the no-panic assertion would be vacuous")
		}
	})

	t.Run("an injected persister is the one the model holds", func(t *testing.T) {
		persister := &fakeThemePersister{}

		m := Build(Deps{Lister: fakeLister{}, ThemePersister: persister})

		if m.themeState.persister != persister {
			t.Fatalf("themePersister = %#v, want the injected recorder", m.themeState.persister)
		}
		// Exercised BY DIRECT CALL — nothing presses a key until Phase 9.
		if err := m.themeState.persister.CommitThemeSlot("nord", prefs.SlotDark); err != nil {
			t.Fatalf("CommitThemeSlot: %v", err)
		}
		if len(persister.slugs) != 1 || persister.slugs[0] != "nord" {
			t.Errorf("recorded slugs = %v, want [nord]", persister.slugs)
		}
		if len(persister.slots) != 1 || persister.slots[0] != prefs.SlotDark {
			t.Errorf("recorded slots = %v, want [SlotDark]", persister.slots)
		}
	})
}

// TestPrefsStore_DoesNotSatisfyThemePersister pins the deliberate method-name
// divergence (§8.9): the store's savers are SaveTheme / SaveThemeSlot, the seam's
// methods are CommitTheme / CommitThemeSlot, so *prefs.Store cannot be wired
// straight into the seam and bypass cmd's single emission site for
// `theme: commit failed`.
//
// The assertion is boxed through `any` because a direct `var _ ThemePersister =
// (*prefs.Store)(nil)` cannot express a NEGATIVE — it would simply fail to
// compile. The ModePersister control is the positive half: *prefs.Store does
// satisfy that seam, so this is about the theme seam's names rather than about
// the store having no methods at all.
func TestPrefsStore_DoesNotSatisfyThemePersister(t *testing.T) {
	var store any = (*prefs.Store)(nil)

	if _, satisfied := store.(ThemePersister); satisfied {
		t.Error("*prefs.Store satisfies ThemePersister; the seam's method names must differ from the store's savers so the logging site cannot be bypassed")
	}
	if _, satisfied := store.(ModePersister); !satisfied {
		t.Error("*prefs.Store no longer satisfies ModePersister; the negative assertion above is vacuous without this control")
	}
}

// TestCommitFailed_SingleEmissionSite pins §8.9's ownership from the side
// internal/tui owns: the model HOLDS the seam and logs nothing, so
// `theme: commit failed` has exactly one emission site (cmd's persister) rather
// than a doubled one.
//
// A source guard rather than a behavioural one, because "emitted nothing" has no
// observable trace here — the package would have to bind the component first, and
// a binding plus a message literal are precisely what a second site looks like.
// Both halves are checked: no `theme` component binding, and no `commit failed`
// message anywhere in the package's sources.
func TestCommitFailed_SingleEmissionSite(t *testing.T) {
	for name, file := range parsePackageFilesByName(t) {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok && isThemeComponentBinding(call) {
				t.Errorf(`%s binds the "theme" log component; the model holds the persister seam and emits nothing — cmd owns the component's records here`, name)
			}
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if strings.Contains(value, commitFailedEvent) {
				t.Errorf("%s carries the %q message; §8.9 keeps it single-sited on cmd's theme persister, which also returns the error for the panel to report", name, commitFailedEvent)
			}
			return true
		})
	}
}

// commitFailedEvent is §12.3's message, restated here as the string the guard
// above searches for. It is deliberately a TEST-side constant: production in this
// package must not carry it at all, so importing one from here would defeat the
// guard.
const commitFailedEvent = "commit failed"

// isThemeComponentBinding reports whether call is exactly log.For("theme").
func isThemeComponentBinding(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "For" || len(call.Args) != 1 {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "log" {
		return false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	return ok && lit.Kind == token.STRING && lit.Value == `"theme"`
}
