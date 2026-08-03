package theme

// Slot names which of §8.2's setting slots one resolution is about.
//
// The three values are the three positions a slug can occupy, and they are what
// makes the fallback MODE-MATCHED: the slot a nomination came from is the only
// thing that says whether a light or a dark default should replace it.
//
// SlotConstant is the zero value, which is the degenerate one-slot case rather
// than an "unset" sentinel — a Resolution always names its slots, and there is no
// state in which one is unknown.
type Slot int

const (
	// SlotConstant is the constant setting's single slug — prefs.json's `theme`.
	SlotConstant Slot = iota
	// SlotLight is the adaptive pair's light slug — prefs.json's `theme_light`.
	SlotLight
	// SlotDark is the adaptive pair's dark slug — prefs.json's `theme_dark`.
	SlotDark
)

// SlotResolution is what happened to ONE slot: the slug that was asked for, the
// slug that actually loaded, and — where those differ — why.
//
// The record is structured rather than folded into a palette because every later
// surface needs the parts separately, and none of them can recover the parts from
// a Theme: task 5-5's `theme: fallback applied` carries the slug that FAILED and
// its reason, Phase 7's doctor line renders `theme <slug> (<slot>) does not
// resolve: <reason>`, and §9.5's badge table keeps the `●` on the PERSISTED slug
// while the fallback's own row carries no badge.
type SlotResolution struct {
	// Slot is which of §8.2's slots this record is about.
	Slot Slot

	// Requested is the slug that was nominated — the SHIPPED DEFAULT's slug when
	// the slot was unset.
	//
	// That the two converge is deliberate, and it is why this record carries no
	// "was this slot set?" flag. ResolveSetting substitutes the shipped default
	// into the Setting BEFORE this function sees it (§8.3: "nothing set" and
	// "pair nominated" are the same state), so Setting{Light: DefaultLightSlug} is
	// identical whether the slot was unset or explicitly set to that slug. The
	// distinction survives only in RawKeys, which this function is deliberately
	// not given.
	//
	// Nothing needs it, either: §9.5's badge table keys all three of its rows on
	// this one field — a set-and-loadable slot badges the persisted slug, a set
	// but unloadable one still badges the persisted slug, and a never-set one
	// badges the shipped default's slug, which is exactly the value here in each
	// case. Doctor reads the raw keys directly, and the panel's per-slot commit
	// takes the raw slug as a parameter. A flag that cannot be computed here and
	// that no consumer reads would be a second, wrong source of truth for which
	// slug carries the `●`.
	Requested string

	// Resolved is the slug whose palette is in Theme: Requested where it loaded,
	// and the slot's mode-matched default where it did not.
	Resolved string

	// FellBack reports whether Requested and Resolved name different themes —
	// that is, whether §8.5's fallback was applied.
	FellBack bool

	// Reason is why the nomination was not usable, populated IFF FellBack. It is
	// exactly one of §6.2's reasons, whatever the cause: a deleted file, a renamed
	// file, a typo in prefs.json, an illegal slug, a missing token, a bad colour
	// and an unreadable file or directory all take the same path and differ only
	// here.
	Reason Reason

	// Theme is the palette that actually loaded — Resolved's, never Requested's.
	Theme Theme
}

// Resolution is the whole theme setting, loaded: the Nomination the TUI
// constructor takes, plus one SlotResolution per slot.
//
// The two ride together because they answer different questions from ONE
// evaluation, exactly as the Setting and the raw keys do: the Nomination says
// what will be PAINTED, the slots say what was asked for and what happened. A
// surface deriving either separately is how the picker and its badges would come
// to disagree about which theme is live.
type Resolution struct {
	// Nomination is the loaded setting the constructor takes — one palette under
	// a constant, both palettes under a pair, and no active member either way.
	Nomination Nomination

	// Slots is one record per slot: exactly 1 under a constant, exactly 2 under a
	// pair, in light-then-dark order.
	Slots []SlotResolution
}

// ResolveNomination loads every theme a Setting nominates — one under a
// constant, two under a pair — substituting §8.5's mode-matched default for any
// slot that will not load, and reports per slot what was asked for, what loaded
// and why they differ.
//
// It WRITES NOTHING, and that is a decision rather than an omission (§6.3):
// falling back must never overwrite the persisted theme name in prefs.json.
// Portal keeps the user's choice and renders the fallback, so fixing the theme
// file restores it on the next launch with no re-selection — where persisting the
// fallback would turn a transient failure into a destructive one, at the moment
// the user is least able to tell what happened.
//
// Both slots resolve independently and NEITHER SHORT-CIRCUITS THE OTHER: a
// broken light slot leaves a loadable dark slot exactly as it was, and two broken
// slots in one launch produce two fallbacks rather than one plus an abandoned
// resolution.
//
// It EMITS NOTHING itself. Task 5-5 wires `theme: loaded` and `theme: fallback
// applied` onto exactly these outcomes, from the one place that holds them all.
//
// The error is §7.6's should-never-happen state: the FALLBACK itself did not
// resolve. A nomination failing is ordinary and is absorbed silently by the
// fallback; a fallback failing means the embedded set cannot supply the theme
// Portal falls back to, and there is nothing left to paint. See resolveSlot.
func (l Loader) ResolveNomination(s Setting, themesDir string) (Resolution, error) {
	if s.IsConstant {
		constant, err := l.resolveSlot(SlotConstant, s.Constant, themesDir)
		if err != nil {
			return Resolution{}, err
		}
		return Resolution{
			Nomination: ConstantNomination(constant.Theme),
			Slots:      []SlotResolution{constant},
		}, nil
	}

	light, err := l.resolveSlot(SlotLight, s.Light, themesDir)
	if err != nil {
		return Resolution{}, err
	}
	dark, err := l.resolveSlot(SlotDark, s.Dark, themesDir)
	if err != nil {
		return Resolution{}, err
	}
	return Resolution{
		Nomination: AdaptivePair(light.Theme, dark.Theme),
		Slots:      []SlotResolution{light, dark},
	}, nil
}

// resolveSlot loads one slot's nominated slug, or §8.5's mode-matched default in
// its place.
//
// ONE NOT-LOADABLE PATH SERVES EVERY CAUSE. A deleted file, a renamed file, a
// typo in prefs.json, an illegal persisted slug, a missing token, a bad colour
// and an unreadable file or directory are seven stories with one shape: the
// nomination did not load, so the slot's default does, the persisted name is
// kept, and the reason rides along for the surfaces to report. Nothing here
// branches on the cause, which is what keeps §6.2's reasons a vocabulary rather
// than a control flow.
//
// AN UNSET SLOT IS NOT A FALLBACK AT ALL. ResolveSetting has already substituted
// the shipped default into the Setting, so an unset slot arrives here as an
// ordinary slug and resolves with FellBack false — which is precisely §8.5's "no
// new mechanism": one rule ("an unset slot holds the shipped default") applied to
// a slot that is SET BUT UNLOADABLE rather than unset. It is also why this record
// needs no set-ness flag: unset and set-to-the-default resolve identically, and
// Requested already says which slug the badge sits on.
//
// The fallback resolves through the SAME by-name resolver the nomination did, so
// §8.4's ordering applies to it too: the embedded set is consulted first, and a
// user's `tokyo-night.theme` can never become the thing Portal falls back to.
//
// A fallback that itself fails to resolve returns an ERROR and never a second
// fallback. §7.6 deliberately removed the safety net beneath this point — there
// is no runtime last-resort hardcoded palette, and adding one here would trade a
// build-time guarantee for a runtime crutch — so a binary whose embedded set
// cannot supply its own fallback has nothing honest left to paint. Task 5-6 pins
// the user-facing sentence that failure is reported with; the rejection travels
// up unaltered until then.
func (l Loader) resolveSlot(slot Slot, slug, themesDir string) (SlotResolution, error) {
	result, rejection := l.ResolveByName(slug, themesDir)
	if rejection == nil {
		return SlotResolution{Slot: slot, Requested: slug, Resolved: result.Slug, Theme: result.Theme}, nil
	}

	fallbackSlug := fallbackSlugFor(slot)
	fallback, fallbackRejection := l.ResolveByName(fallbackSlug, themesDir)
	if fallbackRejection != nil {
		return SlotResolution{}, fallbackRejection
	}

	return SlotResolution{
		Slot:      slot,
		Requested: slug,
		Resolved:  fallback.Slug,
		FellBack:  true,
		Reason:    rejection.Reason,
		Theme:     fallback.Theme,
	}, nil
}

// fallbackSlugFor is §8.5's table: `theme_light` falls to the light default,
// `theme_dark` and a constant `theme` to the dark one.
//
// It is MODE-MATCHED because a single fixed fallback would throw a
// light-terminal user with a typo in their light slot onto a dark palette — a
// bigger surprise than falling to the light default, and the one outcome the
// whole mechanism exists to avoid. That alternative was considered and rejected.
//
// The values are DefaultLightSlug and DefaultDarkSlug rather than literals, and
// that is not merely tidiness: the fallback values ARE the shipped default's
// values, and §8.3's second reason for shipping an adaptive pair — "it degrades
// to a constant dark default" — is true ONLY because an unresolvable slot lands
// on the theme the shipped default already nominates. CHANGING THESE VALUES, OR
// ADOPTING THE REJECTED SINGLE-FIXED-FALLBACK ALTERNATIVE, SILENTLY INVALIDATES
// THAT ARGUMENT: nothing stops compiling and no floor moves — the reasoning
// simply stops holding.
func fallbackSlugFor(slot Slot) string {
	if slot == SlotLight {
		return DefaultLightSlug
	}
	return DefaultDarkSlug
}
