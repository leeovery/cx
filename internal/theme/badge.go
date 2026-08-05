package theme

// Badge is §9.5's `●` marker: WHAT IS SET.
//
// It is the panel's entire answer to "what is actually set?", and it is the one
// signal the picker idiom rests on — THE CURSOR SAYS WHAT IS PREVIEWED, THE
// BADGE SAYS WHAT IS PERSISTED. The two are INDEPENDENT signals (§9.5): `●`
// marks assignment, `▌` + tint marks browse position. So a badge legitimately
// sits on an UNSELECTABLE row — the persisted-but-broken theme is exactly the
// state the marker exists to make visible — and NO CONSUMER MAY INFER
// SELECTABILITY FROM THE PRESENCE OF A BADGE. Row.Selectable is the only answer
// to that question.
//
// It renders in `accent.primary` and NEVER `state.positive` (§9.1's token
// table). `●` marks ASSIGNMENT, which is the primary-accent role Portal already
// uses for active dots and the selector bar; `state.positive` is what `●` means
// on the Sessions list — an ATTACHED session — so painting the badge with it
// would read as liveness rather than as a setting. The token is applied by the
// row delegate; the decision is recorded here, beside the vocabulary it applies
// to, so the two cannot drift.
//
// The slot vocabulary is NOVEL: §9.14 records that assigning a theme to a
// light/dark slot from inside a picker was found in no surveyed tool, so
// `● dark` / `● light` / `● both` / bare `●` has no established shape to borrow
// and is pinned here rather than inferred at a render site.
//
// BadgeNone is the zero value and the "no badge on this row" answer, which is
// what a map lookup for an unbadged row yields for free.
type Badge int

const (
	// BadgeNone is no badge: this row is not what is set.
	BadgeNone Badge = iota
	// BadgeConstant is the constant setting's bare `●` — one slug, no slot to
	// name (§8.2).
	BadgeConstant
	// BadgeLight is the adaptive pair's light slot.
	BadgeLight
	// BadgeDark is the adaptive pair's dark slot.
	BadgeDark
	// BadgeBoth is one row carrying BOTH slots, because both name the same slug.
	BadgeBoth
)

// The four badge texts, pinned VERBATIM because they are user-facing copy and
// because two of them are load-bearing beyond their wording.
//
// `● both` is chosen over a combined `● dark light` for two reasons, and both
// are §9.5's: with exactly two slots "both" is FULLY DETERMINED, so naming the
// slots adds nothing a reader cannot infer; and it is deliberately NO WIDER THAN
// `● light`, so the collapsed form cannot move the row-composition truncation
// budget the panel's ~27–34 columns are apportioned by. A wider collapsed badge
// would silently steal columns from the label on precisely the rows a user
// reaches in two keypresses. The width relation is asserted rather than left to
// this prose.
const (
	badgeConstantText = "●"
	badgeLightText    = "● light"
	badgeDarkText     = "● dark"
	badgeBothText     = "● both"
)

// Text is the badge as it is rendered — the empty string for BadgeNone, so an
// unbadged row renders nothing rather than needing a presence check at the call
// site.
func (b Badge) Text() string {
	switch b {
	case BadgeConstant:
		return badgeConstantText
	case BadgeLight:
		return badgeLightText
	case BadgeDark:
		return badgeDarkText
	case BadgeBoth:
		return badgeBothText
	default:
		return ""
	}
}

// Badges is §9.5's badge table: which slug carries a `●`, and in which of its
// four shapes.
//
// EVERY BADGE IS KEYED ON SlotResolution.Requested, AND THAT ONE FIELD IS THE
// WHOLE OF §9.5's THREE-ROW TABLE:
//
//   - SET AND LOADABLE → the persisted slug. Requested is what was nominated.
//   - SET BUT UNLOADABLE (§8.5) → STILL the persisted slug. Requested is the
//     slug that was nominated whether or not it loaded, so a fallback cannot
//     move the badge and the fallback's own row carries none.
//   - NEVER SET → the SHIPPED DEFAULT's slug (§8.3), because ResolveSetting
//     substitutes the default into the Setting BEFORE resolution, so an unset
//     slot arrives here as an ordinary nomination.
//
// One field, three rows: nothing branches on FellBack and there is no set-ness
// flag to consult, because unset and set-to-the-default are the same state by
// construction (see SlotResolution.Requested).
//
// READING Resolved INSTEAD WOULD MOVE THE BADGE ONTO THE FALLBACK — the bug this
// derivation exists to prevent. Under a fallback the nomination holds the theme
// that LOADED, not the one the user SET, so a badge keyed on it would sit on a
// theme they never chose and silently claim the fallback was their choice. That
// is §6.3's "falling back must never overwrite the persisted theme name"
// reintroduced at the display layer, where it is if anything worse: nothing was
// written, so nothing looks wrong.
//
// The third row is the one that matters most, because it is the most common
// install: §8.1 leaves prefs.json ABSENT on a virgin install, so a
// persisted-slug-only rule would show no marker anywhere at all — falsifying
// §9.4's entire justification for assembling the union, that "the `●` marker
// always has something to sit on".
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
// That is what makes task 8-1's charset-rejected persisted row match its badge:
// the row is keyed on its raw string, having neither a slug nor a file, and the
// badge is keyed on the same string.
//
// THE ONE EXCEPTION IS A `reserved name` ROW, WHICH RETURNS THE EMPTY STRING AND
// THEREFORE NEVER CARRIES A BADGE. Its slug is IDENTICAL to the built-in's by
// definition — that collision is the reason's entire content (§6.2) — so a bare
// identity lookup would paint `●` on BOTH rows. And the rejected file is not what
// is persisted: the persisted slug resolved to the BUILT-IN, which is the same
// discrimination doctor's persisted line draws. This is the ONLY place §9.4's one
// legitimate two-rows-for-one-slug case has an observable consequence.
//
// THE PANEL MUST LOOK BADGES UP THROUGH THIS METHOD AND NEVER READ Slug
// DIRECTLY. Slug is what the two collided rows AGREE on, so a direct lookup
// re-opens exactly the double-badge this exclusion closes — and it would do so
// silently, on the one install that has a drop-in shadowing a built-in.
//
// The empty string is a safe "no badge" answer rather than a sentinel needing
// care: Badges never keys an entry on it, because §5.2's anchored charset makes
// an empty slug illegal and ResolveSetting treats an empty value as UNSET.
func (r Row) BadgeKey() string {
	if r.Rejection != nil && r.Rejection.Reason == ReasonReservedName {
		return ""
	}
	return r.SortKey()
}

// isConstantSetting reports whether these slots are §8.2's constant state: the
// degenerate ONE-slot shape.
//
// THE LENGTH IS PART OF THE TEST, not a redundancy on the Slot value. The two
// setting states never coexist (§9.5), and they cannot: §8.2's tiebreak means a
// non-empty `theme` wins and THE SLOTS ARE NOT READ AT ALL — a RESOLUTION rule
// rather than a file constraint, so a hand-edited prefs.json carrying all three
// keys still resolves to one state or the other. A slice mixing a SlotConstant
// with a slot is therefore a PROGRAMMING ERROR rather than an input shape, and
// requiring the length here is what keeps it from being rendered as both forms
// at once.
func isConstantSetting(slots []SlotResolution) bool {
	return len(slots) == 1 && slots[0].Slot == SlotConstant
}

// collapsed folds a second slot's badge onto a slug that already carries one:
// §9.5's `● both`.
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
