package cmd

import (
	"log/slog"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

const themeCommitFailedEvent = "commit failed"

// themePersister lives in cmd because prefs is a deliberate no-logging leaf and
// the write needs prefs path resolution. It logs *and* returns: one that only
// logged would leave the panel silently "applied but not persisted".
type themePersister struct {
	store  *prefs.Store
	logger *slog.Logger
}

var _ tui.ThemePersister = themePersister{}

// newThemePersister binds the store to themeLogger; the logger is not a
// parameter because the rule is bind once per package.
func newThemePersister(store *prefs.Store) themePersister {
	return themePersister{store: store, logger: themeLogger}
}

func (p themePersister) CommitTheme(slug string) error {
	return p.reportCommit(p.store.SaveTheme(slug), "slug", slug)
}

func (p themePersister) CommitThemeSlot(slug string, member theme.Member) error {
	return p.reportCommit(
		p.store.SaveThemeSlot(slug, prefsSlotFor(member)),
		"slug", slug, "slot", themeSlotAttr(member),
	)
}

// `reason` is appended here rather than at the call sites so the attr order is
// one decision in one place.
func (p themePersister) reportCommit(err error, identity ...any) error {
	if err == nil {
		return nil
	}

	log.OrDiscard(p.logger).Warn(themeCommitFailedEvent, append(identity, "reason", err.Error())...)
	return err
}

func themeSlotAttr(member theme.Member) string {
	name, _ := member.Slot().AttrName()
	return name
}

// The two slot vocabularies stay separate types because prefs must not import
// internal/theme; this is where they convert.
func prefsSlotFor(member theme.Member) prefs.ThemeSlot {
	if member == theme.MemberLight {
		return prefs.SlotLight
	}
	return prefs.SlotDark
}
