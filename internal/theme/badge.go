package theme

// Badge is the theme panel's `●` marker: what is set.
//
// The cursor says what is previewed, the badge says what is persisted; `●` marks
// assignment and `▌` + tint marks browse position. A badge therefore legitimately
// sits on an unselectable row — a persisted-but-broken theme is exactly what the
// marker exists to make visible — so selectability must never be inferred from
// the presence of a badge. Row.Selectable is the only answer to that question.
//
// It renders in `accent.primary` and never `state.positive`: on the Sessions list
// `●` in that token means an attached session, so painting the badge with it
// would read as liveness rather than as a setting. The token is applied by the
// row delegate; the decision is recorded here so the render site does not
// re-decide it.
//
// Which badge a row carries is a fact about the setting, and that fact is what
// this enum holds. The words a badge is drawn with are the panel's copy.
//
// BadgeNone is the zero value and the "no badge on this row" answer, which is
// what a map lookup for an unbadged row yields for free.
type Badge int

const (
	// BadgeNone is no badge: this row is not what is set.
	BadgeNone Badge = iota
	// BadgeConstant is the constant setting's bare `●` — one slug, no slot to
	// name.
	BadgeConstant
	// BadgeLight is the adaptive pair's light slot.
	BadgeLight
	// BadgeDark is the adaptive pair's dark slot.
	BadgeDark
	// BadgeBoth is one row carrying both slots, because both name the same slug.
	BadgeBoth
)

// Badges is the panel's badge table: which slug carries a `●`, and in which of
// its four shapes.
//
// Every badge is keyed on SlotResolution.Requested, and that one field covers all
// three cases:
//
//   - Set and loadable → the persisted slug, which is what was nominated.
//   - Set but unloadable → still the persisted slug, because Requested holds the
//     nomination whether or not it loaded, so a fallback cannot move the badge and
//     the fallback's own row carries none.
//   - Never set → the shipped default's slug, because ResolveSetting substitutes
//     the default into the Setting before resolution, so an unset slot arrives
//     here as an ordinary nomination.
//
// Nothing branches on FellBack and there is no set-ness flag to consult, because
// unset and set-to-the-default are the same state by construction (see
// SlotResolution.Requested).
//
// Reading Resolved instead would move the badge onto the fallback: under a
// fallback the nomination holds the theme that loaded rather than the one the
// user set, so the badge would sit on a theme they never chose and silently claim
// the fallback was their choice — the "falling back must never overwrite the
// persisted theme name" rule reintroduced at the display layer, where nothing was
// written and so nothing looks wrong.
//
// A nil or empty slice yields an empty map rather than a panic.
func Badges(slots []SlotResolution) map[string]Badge {
	if isConstantSetting(slots) {
		return map[string]Badge{slots[0].Requested: BadgeConstant}
	}

	badges := make(map[string]Badge, len(slots))
	for _, slot := range slots {
		badge := slotBadge(slot.Slot)
		if badge == BadgeNone {
			continue
		}
		badges[slot.Requested] = collapsed(badges[slot.Requested], badge)
	}
	return badges
}

// BadgeKey is the value a Row is looked up in Badges' map by.
//
// It is the row's identity — the slug wherever one exists, else the filename,
// else the raw persisted string — the same value SortKey derives from. That is
// what makes a charset-rejected persisted row match its badge: the row is keyed
// on its raw string, having neither a slug nor a file, and the badge is keyed on
// the same string.
//
// A `reserved name` row is the exception and returns the empty string, so it
// never carries a badge: its slug is identical to the built-in's by definition —
// that collision is the reason's entire content — so a bare identity lookup would
// paint `●` on both rows. The rejected file is also not what is persisted; the
// persisted slug resolved to the built-in.
//
// The panel must look badges up through this method and never read Slug directly:
// Slug is what the two collided rows agree on, so a direct lookup silently
// re-opens the double-badge this exclusion closes, on the one install that has a
// drop-in shadowing a built-in.
//
// The empty string is a safe "no badge" answer rather than a sentinel needing
// care: Badges never keys an entry on it, because the anchored slug charset makes
// an empty slug illegal and ResolveSetting treats an empty value as unset.
func (r Row) BadgeKey() string {
	if r.Rejection != nil && r.Rejection.Reason == ReasonReservedName {
		return ""
	}
	return r.SortKey()
}

// isConstantSetting reports whether these slots are the constant state: the
// degenerate one-slot shape.
//
// The length is part of the test rather than a redundancy on the Slot value. The
// two setting states never coexist — a non-empty `theme` wins the tiebreak and
// the slots are not read at all, a resolution rule rather than a file constraint
// — so a slice mixing a SlotConstant with a slot is a programming error rather
// than an input shape, and requiring the length keeps it from rendering as both
// forms at once.
func isConstantSetting(slots []SlotResolution) bool {
	return len(slots) == 1 && slots[0].Slot == SlotConstant
}

// collapsed folds a second slot's badge onto a slug that already carries one,
// producing BadgeBoth.
//
// The collapse is the map's own dedup rather than a pass over it — two slots
// naming the same slug meet on the same key, and the second occupant upgrades
// the first — so there is no shape in which one slug comes back twice.
//
// A slug with no badge yet keeps the arriving one, so the ordinary
// two-different-slugs pair never reaches the collapse at all.
func collapsed(existing, arriving Badge) Badge {
	if existing == BadgeNone {
		return arriving
	}
	return BadgeBoth
}

// slotBadge is the badge one slot of an adaptive pair carries.
//
// SlotConstant yields BadgeNone here rather than the bare `●`, the other half of
// isConstantSetting's guard: the constant's form is reachable only through the
// one-slot shape, so a slice mixing the two states renders the slots it holds and
// never both forms at once.
func slotBadge(s Slot) Badge {
	switch s {
	case SlotLight:
		return BadgeLight
	case SlotDark:
		return BadgeDark
	default:
		return BadgeNone
	}
}
