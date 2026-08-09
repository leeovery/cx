package theme

// Badge is the theme panel's `●` marker: WHAT IS SET.
//
// It is the panel's entire answer to "what is actually set?", and it is the one
// signal the picker idiom rests on — THE CURSOR SAYS WHAT IS PREVIEWED, THE
// BADGE SAYS WHAT IS PERSISTED. The two are INDEPENDENT signals: `●` marks
// assignment, `▌` + tint marks browse position. So a badge legitimately sits on
// an UNSELECTABLE row — the persisted-but-broken theme is exactly the state the
// marker exists to make visible — and NO CONSUMER MAY INFER SELECTABILITY FROM
// THE PRESENCE OF A BADGE. Row.Selectable is the only answer to that question.
//
// It renders in `accent.primary` and NEVER `state.positive`. `●` marks
// ASSIGNMENT, which is the primary-accent role Portal already uses for active
// dots and the selector bar; `state.positive` is what `●` means on the Sessions
// list — an ATTACHED session — so painting the badge with it would read as
// liveness rather than as a setting. The token is applied by the row delegate;
// the decision is recorded here, with the enum it applies to, so the render site
// does not re-decide it.
//
// WHICH badge a row carries is a fact about the SETTING, and that fact is what
// this enum holds. The words a badge is DRAWN with are the panel's copy and live
// with the panel's other pinned strings.
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
	// BadgeBoth is one row carrying BOTH slots, because both name the same slug.
	BadgeBoth
)

// Badges is the panel's badge table: which slug carries a `●`, and in which of
// its four shapes.
//
// EVERY BADGE IS KEYED ON SlotResolution.Requested, AND THAT ONE FIELD COVERS
// ALL THREE CASES:
//
//   - SET AND LOADABLE → the persisted slug. Requested is what was nominated.
//   - SET BUT UNLOADABLE → STILL the persisted slug. Requested is the slug that
//     was nominated whether or not it loaded, so a fallback cannot move the badge
//     and the fallback's own row carries none.
//   - NEVER SET → the SHIPPED DEFAULT's slug, because ResolveSetting substitutes
//     the default into the Setting BEFORE resolution, so an unset slot arrives
//     here as an ordinary nomination.
//
// One field, three cases: nothing branches on FellBack and there is no set-ness
// flag to consult, because unset and set-to-the-default are the same state by
// construction (see SlotResolution.Requested).
//
// READING Resolved INSTEAD WOULD MOVE THE BADGE ONTO THE FALLBACK — the bug this
// derivation exists to prevent. Under a fallback the nomination holds the theme
// that LOADED, not the one the user SET, so a badge keyed on it would sit on a
// theme they never chose and silently claim the fallback was their choice. That
// is the "falling back must never overwrite the persisted theme name" rule
// reintroduced at the display layer, where it is if anything worse: nothing was
// written, so nothing looks wrong.
//
// The third case is the one that matters most, because it is the most common
// install: prefs.json is ABSENT on a virgin install, so a persisted-slug-only
// rule would show no marker anywhere at all — falsifying the reason the union is
// assembled in the first place, that the `●` marker always has something to sit
// on.
//
// IT IS PURE AND TOTAL: no I/O, no logging, no clock, deterministic, and a
// nil or empty slice yields an empty map rather than a panic. It reads no Theme
// and no palette — a badge is a fact about a SLUG.
//
// It supplies the derivation and NOTHING THAT MOVES A BADGE. A commit's visible
// recompute — two slot badges collapsing to one bare `●` on a virgin install,
// which is correct, since two inherited defaults just became one pin — is the
// commit path's, and it re-runs this same function against the new state.
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
// It is the row's IDENTITY — the slug wherever one exists, else the filename,
// else the raw persisted string — which is the same value SortKey derives from.
// That is what makes a charset-rejected persisted row match its badge: the row
// is keyed on its raw string, having neither a slug nor a file, and the badge is
// keyed on the same string.
//
// THE ONE EXCEPTION IS A `reserved name` ROW, WHICH RETURNS THE EMPTY STRING AND
// THEREFORE NEVER CARRIES A BADGE. Its slug is IDENTICAL to the built-in's by
// definition — that collision is the reason's entire content — so a bare identity
// lookup would paint `●` on BOTH rows. And the rejected file is not what is
// persisted: the persisted slug resolved to the BUILT-IN, which is the same
// discrimination doctor's persisted line draws. It is the one legitimate case in
// which two rows carry the same slug.
//
// THE PANEL MUST LOOK BADGES UP THROUGH THIS METHOD AND NEVER READ Slug
// DIRECTLY. Slug is what the two collided rows AGREE on, so a direct lookup
// re-opens exactly the double-badge this exclusion closes — and it would do so
// silently, on the one install that has a drop-in shadowing a built-in.
//
// The empty string is a safe "no badge" answer rather than a sentinel needing
// care: Badges never keys an entry on it, because the anchored slug charset makes
// an empty slug illegal and ResolveSetting treats an empty value as UNSET.
func (r Row) BadgeKey() string {
	if r.Rejection != nil && r.Rejection.Reason == ReasonReservedName {
		return ""
	}
	return r.SortKey()
}

// isConstantSetting reports whether these slots are the constant state: the
// degenerate ONE-slot shape.
//
// THE LENGTH IS PART OF THE TEST, not a redundancy on the Slot value. The two
// setting states never coexist, and they cannot: a non-empty `theme` wins the
// tiebreak and THE SLOTS ARE NOT READ AT ALL — a RESOLUTION rule rather than a
// file constraint, so a hand-edited prefs.json carrying all three keys still
// resolves to one state or the other. A slice mixing a SlotConstant with a slot
// is therefore a PROGRAMMING ERROR rather than an input shape, and requiring the
// length here is what keeps it from being rendered as both forms at once.
func isConstantSetting(slots []SlotResolution) bool {
	return len(slots) == 1 && slots[0].Slot == SlotConstant
}

// collapsed folds a second slot's badge onto a slug that already carries one,
// producing BadgeBoth.
//
// The collapse IS the map's own dedup rather than a pass over it — two slots
// naming the same slug meet on the same key, and the second occupant upgrades
// the first. That is what makes a single entry structural: there is no shape in
// which one slug comes back twice, because one key holds one badge.
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
// SlotConstant yields BadgeNone here rather than the bare `●`, which is the
// other half of isConstantSetting's guard: the constant's form is reachable
// ONLY through the one-slot shape, so a slice mixing the two states renders the
// slots it holds and never both forms at once.
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
