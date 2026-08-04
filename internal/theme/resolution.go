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
	return l.resolveNomination(s, l.byNamePass(themesDir))
}

// ResolveNominationFrom re-runs that entire resolution — the same charset check,
// the same embedded-set-first ordering, the same per-slot mode-matched fallback,
// the same structured record and the same §7.6 fatal — against a RETAINED
// Enumeration instead of the themes directory.
//
// ONE DIFFERENCE AND ONE ONLY: where the slug is looked up once the embedded set
// has declined. Everything else is literally the same code (see
// resolveNomination), so the two entry points cannot drift about what a slug
// means, which slot falls back to what, or when a fallback is fatal.
//
// IT PERFORMS NO DIRECTORY READ, AND THAT IS THE WHOLE POINT. §8.4 refuses a
// commit-time — and equally an open-time — directory read because it would
// produce a THIRD parse of the same slug, neither construction's nor the panel's,
// that can disagree with the row the user is looking at. That reintroduces exactly
// the staleness split §5.8 exists to close: the panel's parse is the fresher truth
// by §5.8's own rule, so resolving against it is what keeps the panel row and the
// applied theme incapable of disagreeing.
//
// THREE CALLERS SHARE IT, all of them the panel's: the open-time re-resolution
// (§9.2 — the cursor lands on the theme actually rendering, and a mid-session
// edit applies here), `Esc`'s close (§5.8 — persisted state resolves against the
// panel's enumeration rather than against what construction loaded), and Phase
// 9's mid-session slot load (§8.4 — a stale hand-edited slot resolves from the
// retained enumeration, and only a slug it has no entry for falls through to the
// embedded set).
//
// A slug the enumeration has no entry for is `not found`, or `unreadable` where
// the directory itself could not be listed (§5.5) — the same discrimination
// unresolvedRejection draws for the union's own persisted rows, so a row and the
// theme that actually rendered can never state different reasons for one slug.
//
// It emits NO `theme: loaded` (§12.3): that event's cadence is construction plus
// the one commit-time load outside it, and a per-open/per-`Esc` line would turn a
// per-load INFO into the running commentary its neighbours dedup to avoid. The
// `theme: fallback applied` WARN still fires, deduplicated per process — §12.3
// names the panel open and the `Esc` as sites that apply one. See resolutionPass.
func (l Loader) ResolveNominationFrom(e Enumeration, s Setting) (Resolution, error) {
	return l.resolveNomination(s, l.enumerationPass(e))
}

// resolutionPass is everything that differs between the two entry points above:
// where a slug LOADS from, and how a resolved slot is REPORTED.
//
// It is ONE value rather than two parameters threaded down two levels, and a pair
// of functions rather than a flag, because the two travel together and neither is
// meaningful alone: a pass resolving against a retained parse is by definition the
// one that must not emit `theme: loaded` (§12.3). Naming that correspondence in a
// type is what stops a third call site pairing them the other way round.
type resolutionPass struct {
	// load is §8.4's by-name ladder for this pass — the whole of it, so the
	// FALLBACK resolves through the identical route the nomination did and the
	// embedded set is consulted first on both.
	load slugLoader

	// report is §12.3's per-slot emission at this pass's cadence, and it hands the
	// record straight back so a caller states the outcome once (see reportSlot).
	report func(SlotResolution) SlotResolution
}

// byNamePass is construction's pass: the themes directory read by name, at the
// per-LOAD event cadence (`theme: fallback applied` where one was, then
// `theme: loaded` for the palette that rendered).
func (l Loader) byNamePass(themesDir string) resolutionPass {
	return resolutionPass{
		load:   func(slug string) (Result, *Rejection) { return l.ResolveByName(slug, themesDir) },
		report: l.reportSlot,
	}
}

// enumerationPass is the panel's pass: the retained enumeration, at the
// RE-RESOLUTION cadence — the fallback line alone, no `theme: loaded` (§12.3).
func (l Loader) enumerationPass(e Enumeration) resolutionPass {
	return resolutionPass{
		load:   func(slug string) (Result, *Rejection) { return l.resolveFromEnumeration(slug, e) },
		report: l.reportFallback,
	}
}

// resolveFromEnumeration is the panel pass's third rung: the same ladder
// ResolveByName runs, with the RETAINED enumeration's entries in place of the
// directory.
func (l Loader) resolveFromEnumeration(slug string, e Enumeration) (Result, *Rejection) {
	return l.resolveNamed(slug, func(s string) (Result, *Rejection) {
		return entryResult(s, e)
	})
}

// entryResult answers what one slug loads to WITHIN a retained enumeration:
// the entry's own palette, the entry's own single rejection, or — where nothing
// answers to the slug — §5.5's verdict on the directory itself.
//
// NOTHING IS RE-DERIVED. The palette and the rejection ride across from the entry
// exactly as the ladder produced them at enumeration time, which is what makes the
// resolved theme and the row the user is looking at the same parse rather than two.
//
// The unresolved answer is unresolvedRejection — the SAME function the union's
// persisted rows are built through — so a slug nothing answers to reports one
// reason on the row and in the resolution: `not found` where the directory could
// be listed, `unreadable` where it could not, because the theme may be sitting
// right there in a directory nothing can read.
//
// A `bad name` entry can never match: it carries no slug (§6.2 rung 1), and the
// slug being looked up is valid by the time this is reached (resolveNamed's
// charset check). The Result carries no Source bytes, since an enumeration
// retains parses rather than files; only `portal theme export` reads that field,
// and it resolves by name.
func entryResult(slug string, e Enumeration) (Result, *Rejection) {
	for _, entry := range e.Entries {
		if entry.Slug != slug {
			continue
		}
		if entry.Rejection != nil {
			return Result{}, entry.Rejection
		}
		return Result{Slug: entry.Slug, Theme: entry.Theme}, nil
	}
	return Result{}, unresolvedRejection(e)
}

// resolveNomination is the resolution both entry points ARE — §8.2's two setting
// states, resolved slot by slot through whichever pass it was handed.
//
// It is shared rather than duplicated because every rule stated in
// ResolveNomination's comment is a rule about the SETTING, not about the source:
// the two slots resolve independently and neither short-circuits the other, the
// nomination's shape mirrors the setting's, and a failing fallback abandons the
// whole resolution. A second copy of that for the panel is how a broken light
// slot would come to short-circuit a good dark one on one path and not the other.
func (l Loader) resolveNomination(s Setting, pass resolutionPass) (Resolution, error) {
	if s.IsConstant {
		constant, err := l.resolveSlot(SlotConstant, s.Constant, pass)
		if err != nil {
			return Resolution{}, err
		}
		return Resolution{
			Nomination: ConstantNomination(constant.Theme),
			Slots:      []SlotResolution{constant},
		}, nil
	}

	light, err := l.resolveSlot(SlotLight, s.Light, pass)
	if err != nil {
		return Resolution{}, err
	}
	dark, err := l.resolveSlot(SlotDark, s.Dark, pass)
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
func (l Loader) resolveSlot(slot Slot, slug string, pass resolutionPass) (SlotResolution, error) {
	result, rejection := pass.load(slug)
	if rejection == nil {
		return pass.report(SlotResolution{Slot: slot, Requested: slug, Resolved: result.Slug, Theme: result.Theme}), nil
	}

	fallbackSlug := fallbackSlugFor(slot)
	fallback, fallbackRejection := pass.load(fallbackSlug)
	if fallbackRejection != nil {
		return SlotResolution{}, BrokenBuiltinError(fallbackSlug)
	}

	return pass.report(SlotResolution{
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
	l.reportFallback(r)
	l.events.Loaded(r.Resolved, r.Slot)
	return r
}

// reportFallback is the RE-RESOLUTION cadence: the failure line alone, with no
// `theme: loaded` behind it.
//
// It is the panel's reporter — the open-time re-resolution, task 8-10's `Esc` and
// Phase 9's recompute all resolve the SAME persisted setting they were already
// resolved for, so nothing was loaded that construction did not already report.
// §12.3 pins that split explicitly on both sides: `theme: loaded` is catalogued
// as construction plus the one commit-time load, while `theme: fallback applied`
// names "again on every panel open… and again on every `Esc`" among its cadences
// and deduplicates per process on `slug`+`reason` rather than being suppressed at
// a site.
//
// THE POLICY IS SINGLE-SITED HERE, not restated at the three panel call sites: an
// emission wired onto the shared body instead would put a per-load INFO on a
// per-keypress path, which is the running commentary the neighbouring dedup rules
// exist to prevent.
func (l Loader) reportFallback(r SlotResolution) SlotResolution {
	if r.FellBack {
		l.events.FallbackApplied(r.Requested, r.Slot, r.Reason)
	}
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
