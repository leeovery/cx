package cmd

import (
	"log/slog"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/tui"
)

// themeCommitFailedEvent is the message for a failed theme write, emitted under
// the `theme` component — `theme: commit failed` as it reads in the log.
const themeCommitFailedEvent = "commit failed"

// themePersister commits the theme panel's choice. It is the production
// tui.ThemePersister, and it is owned by cmd rather than by prefs or by the TUI
// because three constraints meet here: prefs is a deliberate leaf that must not
// import internal/log, the write needs prefs path resolution, and the `theme`
// component records the failure.
//
// It is therefore THE emission site for `theme: commit failed`, which otherwise
// has none: prefs cannot log it, and the model logging it would either double the
// event or make internal/tui another package emitting the component.
//
// It logs AND RETURNS. The panel's outstanding-failure state renders
// `⚠ couldn't save theme` from the returned value and holds the failure
// outstanding, so a persister that only logged would recreate the silent
// "applied but not persisted" state the picker idiom exists to close.
//
// The merge itself stays inside prefs, behind the field-specific savers: this
// type resolves nothing about the record's contents, holds no snapshot of it,
// and so cannot clobber a sibling field. Each instance persists its own
// change with no file watch and no cross-instance sync — other instances are
// unaffected until relaunch, exactly as session_list_mode already behaves.
type themePersister struct {
	store  *prefs.Store
	logger *slog.Logger
}

// Satisfying the seam is the whole point of the type, so a signature drift is a
// compile error here rather than a nil persister at the wiring site.
var _ tui.ThemePersister = themePersister{}

// newThemePersister binds the process's prefs store to the package's `theme`
// component logger. The logger is NOT a parameter: CLAUDE.md's rule is bind once
// per package, and themeLogger is that binding — a second log.For("theme")
// anywhere in cmd is what TestThemeComponent_BoundOnceInCmd exists to catch.
func newThemePersister(store *prefs.Store) themePersister {
	return themePersister{store: store, logger: themeLogger}
}

// CommitTheme persists slug as the constant theme, clearing both adaptive slots
// in the same write (mutual exclusion, enforced by the saver).
func (p themePersister) CommitTheme(slug string) error {
	return p.reportCommit(p.store.SaveTheme(slug), "slug", slug)
}

// CommitThemeSlot persists slug into one half of the adaptive pair, clearing the
// constant and leaving the other slot untouched — which is what makes a `● both`
// badge reachable in two keypresses.
//
// The slot is the existing typed prefs.ThemeSlot rather than a name this layer
// invents, so no caller can mint a third slot — and an out-of-range value is the
// saver's error, reported here like any other failed write, with no `slot` attr
// to carry.
func (p themePersister) CommitThemeSlot(slug string, slot prefs.ThemeSlot) error {
	identity := []any{"slug", slug}
	if name, named := themeSlotAttr(slot); named {
		identity = append(identity, "slot", name)
	}
	return p.reportCommit(p.store.SaveThemeSlot(slug, slot), identity...)
}

// reportCommit emits `theme: commit failed` for a failed write and hands err back
// verbatim. A successful commit emits NOTHING: the persister's one event is a
// failure, and the commit-time `theme: loaded` belongs to the loader.
//
// `reason` is appended HERE rather than at either call site, so the attr order —
// `slug`, `slot`, `reason` — is one decision in one place rather than two
// sites that happen to agree today. The logger is nil-tolerated for the reason
// every other emitter in the project tolerates one: a zero-valued persister must
// be silent rather than a panic.
func (p themePersister) reportCommit(err error, identity ...any) error {
	if err == nil {
		return nil
	}

	log.OrDiscard(p.logger).Warn(themeCommitFailedEvent, append(identity, "reason", err.Error())...)
	return err
}

// themeSlotAttr renders a prefs slot as the `slot` attr value, and reports
// whether the slot has one at all — the single source of that string on this
// side of the boundary.
//
// The values are deliberately the same two words internal/theme's own slotAttr
// renders for theme.Slot. They are separate types serving different layers
// (persistence versus resolution), so they cannot share a function; what they
// share is the user-visible vocabulary, and a test reads the loader's rendering
// back off a real emission to pin that they cannot drift apart.
//
// Anything that is not one half of the pair carries NO attr — not an empty
// string, and not the word "constant". The attr names which half of an ADAPTIVE
// pair a line is about, and a constant setting has no halves: a value there would
// assert a distinction the two-state setting does not have, and an empty one
// would grep as a slot named nothing.
func themeSlotAttr(slot prefs.ThemeSlot) (string, bool) {
	switch slot {
	case prefs.SlotLight:
		return "light", true
	case prefs.SlotDark:
		return "dark", true
	default:
		return "", false
	}
}
