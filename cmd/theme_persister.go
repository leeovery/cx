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
// The failure is therefore recorded here rather than beneath or above: prefs
// cannot log it, and the model logging it would either double the event or make
// internal/tui another package emitting the component.
//
// It logs and returns. The panel's outstanding-failure state renders
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
// component logger. The logger is not a parameter: the rule is bind once per
// package, and themeLogger is cmd's single log.For("theme") binding.
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
// The half is named in the domain's own light/dark type, which the caller carries
// end to end; this seam is where it becomes the value prefs.json is keyed by.
func (p themePersister) CommitThemeSlot(slug string, member theme.Member) error {
	return p.reportCommit(
		p.store.SaveThemeSlot(slug, prefsSlotFor(member)),
		"slug", slug, "slot", themeSlotAttr(member),
	)
}

// reportCommit emits `theme: commit failed` for a failed write and hands err back
// verbatim. A successful commit emits nothing: the persister's one event is a
// failure, and the commit-time `theme: loaded` belongs to the loader.
//
// `reason` is appended here rather than at the call sites, so the attr order —
// `slug`, `slot`, `reason` — is one decision in one place. The logger is
// nil-tolerated so a zero-valued persister is silent rather than a panic.
func (p themePersister) reportCommit(err error, identity ...any) error {
	if err == nil {
		return nil
	}

	log.OrDiscard(p.logger).Warn(themeCommitFailedEvent, append(identity, "reason", err.Error())...)
	return err
}

// themeSlotAttr renders one half of the adaptive pair as the `slot` attr value.
//
// The words come off theme.Slot's own name mapping, where every other surface
// naming a slot reads them. A member is always one half of a pair, so it always
// has a name — the nameless case is the constant, which reaches this component's
// commit line through CommitTheme and carries no attr at all.
func themeSlotAttr(member theme.Member) string {
	name, _ := member.Slot().AttrName()
	return name
}

// prefsSlotFor is the domain light/dark answer as the value prefs.json is keyed
// by — the single conversion between the two vocabularies, which stay separate
// types because prefs is a deliberate no-logging leaf that must not import
// internal/theme.
func prefsSlotFor(member theme.Member) prefs.ThemeSlot {
	if member == theme.MemberLight {
		return prefs.SlotLight
	}
	return prefs.SlotDark
}
