package cmd

import (
	"log/slog"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
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
// whether the slot has one at all.
//
// The words are not this layer's to choose: they come off theme.Slot's own name
// mapping, which is where the constant's absent attr is decided and where every
// other surface naming a slot reads them too. Persistence and resolution keep
// SEPARATE slot types — prefs is a deliberate no-logging leaf that must not
// import internal/theme — but they share one vocabulary, so the conversion
// happens here, at the boundary the two types already meet on.
func themeSlotAttr(slot prefs.ThemeSlot) (string, bool) {
	return themeSlotFor(slot).AttrName()
}

// themeSlotFor reads a persisted slot as the resolution layer's.
//
// Anything that is not one half of the adaptive pair — including the zero value
// the saver rejects — is the constant, which is the slot with no name and
// therefore no attr.
func themeSlotFor(slot prefs.ThemeSlot) theme.Slot {
	switch slot {
	case prefs.SlotLight:
		return theme.SlotLight
	case prefs.SlotDark:
		return theme.SlotDark
	default:
		return theme.SlotConstant
	}
}
