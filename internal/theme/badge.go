package theme

// Badge is the theme panel's `●` marker: what is persisted, as distinct from
// the cursor's browse position. A badge legitimately sits on an unselectable
// row (a persisted-but-broken theme) — never infer selectability from one.
// Badges render in `accent.primary`, never `state.positive`, which would read
// as session liveness.
type Badge int

const (
	BadgeNone Badge = iota
	// BadgeConstant is the constant setting's bare `●` — one slug, no slot.
	BadgeConstant
	BadgeLight
	BadgeDark
	// BadgeBoth is one row carrying both slots, because both name the same slug.
	BadgeBoth
)

// Badges is the panel's badge table: which slug carries a `●`, in which shape.
// Keyed on SlotResolution.Requested, never Resolved — Resolved would move the
// badge onto a fallback theme the user never chose. Nil or empty slice yields
// an empty map.
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

// BadgeKey is the value a Row is looked up in Badges' map by: the row's
// Identity, except a reserved-name rejection returns "" — its slug equals the
// built-in's, and an identity lookup would paint `●` on both rows. Look
// badges up through this method, never via Slug directly, or that
// double-badge reopens.
func (r Row) BadgeKey() string {
	if r.Rejection != nil && r.Rejection.Reason == ReasonReservedName {
		return ""
	}
	return r.Identity()
}

// The length check is deliberate: a slice mixing SlotConstant with adaptive
// slots is a programming error and must not render as both forms at once.
func isConstantSetting(slots []SlotResolution) bool {
	return len(slots) == 1 && slots[0].Slot == SlotConstant
}

func collapsed(existing, arriving Badge) Badge {
	if existing == BadgeNone {
		return arriving
	}
	return BadgeBoth
}

// SlotConstant deliberately yields BadgeNone: the constant form is reachable
// only through the one-slot shape, so a mixed slice never renders both forms.
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
