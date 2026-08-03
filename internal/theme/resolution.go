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
// a Theme: §12.3's `theme: fallback applied` carries the slug that FAILED and
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
// It EMITS §12.3's per-slot pair — `theme: fallback applied` where one was, then
// `theme: loaded` for the slug that actually loaded — through the injected seam,
// from the one place that holds every slot's outcome. That is what keeps the
// cadence single-sited: ONE `loaded` per slot, one under a constant and two under
// a pair, never one combined line. See reportSlot.
//
// The error is §7.6's should-never-happen state: the FALLBACK itself did not
// resolve. A nomination failing is ordinary and is absorbed silently by the
// fallback; a fallback failing means the embedded set cannot supply the theme
// Portal falls back to, and there is nothing left to paint. See resolveSlot.
//
// A CALLER SIMPLY RETURNS IT. It is BrokenBuiltinError — §14A's pinned sentence
// — and it is meant to travel the ordinary error path to main.go's single
// os.Exit owner, which prints it as one line and exits non-zero. Nothing on the
// way is expected to inspect it, re-word it, log it or substitute a palette for
// it: the Resolution alongside it is the zero value precisely so there is
// nothing to be tempted to render.
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
// THE TWO FAILURES ARE NOT THE SAME FAILURE, and the whole of the escalation is
// telling them apart. The FIRST resolution below is the NOMINATION: when it
// fails, nothing is fatal — the slot's mode-matched default takes its place, the
// persisted name is kept and the reason is reported. The SECOND is the FALLBACK
// itself: when THAT fails, the embedded set cannot supply the theme Portal falls
// back to, and there is nothing honest left to paint.
//
// So a failed fallback returns BrokenBuiltinError — §14A's pinned sentence,
// naming the FALLBACK's slug — and never a second fallback and never a
// compiled-in palette. §7.6 removed the safety net beneath this point on
// purpose, rejecting "a compiled-in last-resort palette equal to Tokyo Night
// Dark" in exactly those terms: a build-time guarantee beats a runtime crutch.
// THIS PATH MUST NOT GROW ONE LATER — a palette here would paint values nobody
// chose while every test resting on the guarantee still passed.
//
// The error is ORDINARY and travels the normal return path: no panic, no exit
// and no log line. Nothing is emitted for it either — §12.3's events report what
// a slot RESOLVED TO, and this slot resolved to nothing (see reportSlot).
//
// In a correctly built binary the second failure is unreachable, which is
// precisely why Loader.BuiltinSource exists: an unreachable fatal with no test
// is a path nobody has ever run, so a test stages the broken binary by injecting
// a byte source that omits or corrupts a fallback. Production carries a nil
// field and one branch.
func (l Loader) resolveSlot(slot Slot, slug, themesDir string) (SlotResolution, error) {
	result, rejection := l.ResolveByName(slug, themesDir)
	if rejection == nil {
		return l.reportSlot(SlotResolution{Slot: slot, Requested: slug, Resolved: result.Slug, Theme: result.Theme}), nil
	}

	fallbackSlug := fallbackSlugFor(slot)
	fallback, fallbackRejection := l.ResolveByName(fallbackSlug, themesDir)
	if fallbackRejection != nil {
		return SlotResolution{}, BrokenBuiltinError(fallbackSlug)
	}

	return l.reportSlot(SlotResolution{
		Slot:      slot,
		Requested: slug,
		Resolved:  fallback.Slug,
		FellBack:  true,
		Reason:    rejection.Reason,
		Theme:     fallback.Theme,
	}), nil
}

// reportSlot emits §12.3's record of ONE slot's outcome and hands the resolution
// straight back, so a caller states the outcome once (the reportDirectoryUnusable
// shape, for the same reason).
//
// THE ORDER IS FIXED: the failure first, then the palette that replaced it. A
// reader scanning the log meets the cause before the consequence, and a slot that
// simply loaded emits the second line alone.
//
// It emits from the ASSEMBLED RECORD rather than from the branch that produced
// it, which is what makes `theme: loaded` structurally incapable of naming the
// slug that FAILED: the only slug it can reach is Resolved, the one whose palette
// is in Theme. Both events naming the failed nomination is exactly the state
// §12.3 says a broken install must never be logged in — a `grep "theme:"` could
// then not answer which palette is actually rendering — so the impossibility is
// worth more here than a saved line at each call site.
//
// A slot that could not resolve AT ALL emits nothing, because it never reaches
// here: the fallback failing is §7.6's should-never-happen state, nothing loaded,
// and no fallback was applied to report.
func (l Loader) reportSlot(r SlotResolution) SlotResolution {
	if r.FellBack {
		l.events.FallbackApplied(r.Requested, r.Slot, r.Reason)
	}
	l.events.Loaded(r.Resolved, r.Slot)
	return r
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
