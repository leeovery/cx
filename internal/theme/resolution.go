package theme

// Slot names which of the theme setting's slots one resolution is about.
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
// a Theme: `theme: fallback applied` carries the slug that FAILED and its reason,
// doctor's line renders `theme <slug> (<slot>) does not resolve: <reason>`, and
// the panel's badge table keeps the `●` on the PERSISTED slug while the
// fallback's own row carries no badge.
type SlotResolution struct {
	// Slot is which of the setting's slots this record is about.
	Slot Slot

	// Requested is the slug that was nominated — the SHIPPED DEFAULT's slug when
	// the slot was unset.
	//
	// That the two converge is deliberate, and it is why this record carries no
	// "was this slot set?" flag. ResolveSetting substitutes the shipped default
	// into the Setting BEFORE this function sees it — "nothing set" and "pair
	// nominated" are the same state — so Setting{Light: DefaultLightSlug} is
	// identical whether the slot was unset or explicitly set to that slug. The
	// distinction survives only in RawKeys, which this function is deliberately
	// not given.
	//
	// Nothing needs it, either: the badge table keys all three of its cases on
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
	// that is, whether the mode-matched fallback was applied.
	FellBack bool

	// Reason is why the nomination was not usable, populated IFF FellBack. It is
	// exactly one reason, whatever the cause: a deleted file, a renamed
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
// constant, two under a pair — substituting the mode-matched default for any slot
// that will not load, and reports per slot what was asked for, what loaded and
// why they differ.
//
// It WRITES NOTHING, and that is a decision rather than an omission: falling back
// must never overwrite the persisted theme name in prefs.json. Portal keeps the
// user's choice and renders the fallback, so fixing the theme file restores it on
// the next launch with no re-selection — where persisting the fallback would turn
// a transient failure into a destructive one, at the moment the user is least able
// to tell what happened.
//
// Both slots resolve independently and NEITHER SHORT-CIRCUITS THE OTHER: a
// broken light slot leaves a loadable dark slot exactly as it was, and two broken
// slots in one launch produce two fallbacks rather than one plus an abandoned
// resolution.
//
// It EMITS the per-slot pair — `theme: fallback applied` where one was, then
// `theme: loaded` for the slug that actually loaded — through the injected seam,
// from the one place that holds every slot's outcome. That is what keeps the
// cadence single-sited: ONE `loaded` per slot, one under a constant and two under
// a pair, never one combined line. See reportSlot.
//
// The error is the should-never-happen state: the FALLBACK itself did not
// resolve. A nomination failing is ordinary and is absorbed silently by the
// fallback; a fallback failing means the embedded set cannot supply the theme
// Portal falls back to, and there is nothing left to paint. See resolveSlot.
//
// A CALLER SIMPLY RETURNS IT. It is BrokenBuiltinError — the pinned user-facing
// sentence — and it is meant to travel the ordinary error path to main.go's
// single os.Exit owner, which prints it as one line and exits non-zero. Nothing
// on the way is expected to inspect it, re-word it, log it or substitute a
// palette for it: the Resolution alongside it is the zero value precisely so
// there is nothing to be tempted to render.
func (l Loader) ResolveNomination(s Setting, themesDir string) (Resolution, error) {
	return l.resolveNomination(s, l.byNamePass(themesDir))
}

// ResolveNominationFrom re-runs that entire resolution — the same charset check,
// the same embedded-set-first ordering, the same per-slot mode-matched fallback,
// the same structured record and the same broken-builtin fatal — against a
// RETAINED Enumeration instead of the themes directory.
//
// ONE DIFFERENCE AND ONE ONLY: where the slug is looked up once the embedded set
// has declined. Everything else is literally the same code (see
// resolveNomination), so the two entry points cannot drift about what a slug
// means, which slot falls back to what, or when a fallback is fatal.
//
// IT PERFORMS NO DIRECTORY READ, AND THAT IS THE WHOLE POINT. A commit-time — and
// equally an open-time — directory read would produce a THIRD parse of the same
// slug, neither construction's nor the panel's, that can disagree with the row
// the user is looking at. The panel's retained parse is the fresher truth, so
// resolving against it is what keeps the panel row and the applied theme
// incapable of disagreeing.
//
// The panel's paths share it: the open-time re-resolution (the cursor lands on
// the theme actually rendering, and a mid-session edit applies here), `Esc`'s
// close (persisted state resolves against the panel's enumeration rather than
// against what construction loaded), and the post-commit badge recompute. The
// mid-session slot load resolves against the SAME enumeration through the same
// rule body, but takes ResolveSlot below: it is one slot rather than a setting,
// and it emits.
//
// A slug the enumeration has no entry for is `not found`, or `unreadable` where
// the directory itself could not be listed — the same discrimination
// unresolvedRejection draws for the union's own persisted rows, so a row and the
// theme that actually rendered can never state different reasons for one slug.
//
// It emits NO `theme: loaded`: that event's cadence is construction plus the one
// commit-time load outside it, and a per-open/per-`Esc` line would turn a per-load
// INFO into the running commentary its neighbours dedup to avoid. The
// `theme: fallback applied` WARN still fires, deduplicated per process — the panel
// open and the `Esc` are both sites that can apply one. See resolutionPass.
func (l Loader) ResolveNominationFrom(e Enumeration, s Setting) (Resolution, error) {
	return l.resolveNomination(s, l.enumerationPass(e))
}

// ResolveSlot resolves ONE slot against a retained Enumeration and emits the
// commit-time `theme: loaded` — the single theme load that happens outside
// construction.
//
// IT IS THE SAME RULE BODY ITS NEIGHBOUR RUNS. The charset check, the
// embedded-set-first ordering, the per-slot mode-matched fallback, the structured
// record and the broken-builtin fatal all arrive through resolveSlot and the
// enumeration's own loader (see commitPass), so the badge path and the load path
// CANNOT DISAGREE about what one slug means — which is the whole reason the
// panel's retained parse is the only source either of them reads.
//
// IT PERFORMS NO DIRECTORY READ, for ResolveNominationFrom's reason exactly: a
// read here would be a THIRD parse of the same slug, neither construction's nor
// the panel's, that can disagree with the row the user is looking at.
//
// WHAT DIFFERS IS THE CADENCE, AND ONLY THE CADENCE. Its neighbour re-resolves a
// setting construction already reported and therefore emits no `theme: loaded`
// (see reportFallback); this is a genuine LOAD of a slot nothing has reported, so
// it emits one — carrying the slug that actually RENDERED, the fallback's where
// one was applied. The two entry points are separate methods rather than a flag
// precisely so that pairing is stated in a type (see resolutionPass) and a later
// call site cannot pair them the other way round.
//
// The slug is the caller's already-defaulted one: an UNSET slot holds the shipped
// default, which ResolveSetting substitutes before anything here sees it, so an
// untouched slot resolves from the embedded set with FellBack false rather than
// arriving empty and being reported as a fallback.
func (l Loader) ResolveSlot(e Enumeration, slot Slot, slug string) (SlotResolution, error) {
	return l.resolveSlot(slot, slug, l.commitPass(e))
}

// resolutionPass is everything that differs between the entry points above: where
// a slug LOADS from, and how a resolved slot is REPORTED.
//
// It is ONE value rather than two parameters threaded down two levels, and a pair
// of functions rather than a flag, because the two travel together and neither is
// meaningful alone: a pass resolving against a retained parse is by definition the
// one that must not emit `theme: loaded`. Naming that correspondence in a type is
// what stops another call site pairing them the other way round.
type resolutionPass struct {
	// load is the by-name ladder for this pass — the whole of it, so the FALLBACK
	// resolves through the identical route the nomination did and the embedded set
	// is consulted first on both.
	load slugLoader

	// report is the per-slot emission at this pass's cadence, and it hands the
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

// enumerationPass is the panel's RE-RESOLUTION pass: the retained enumeration, at
// the re-resolution cadence — the fallback line alone, no `theme: loaded`.
func (l Loader) enumerationPass(e Enumeration) resolutionPass {
	return resolutionPass{load: l.enumerationLoad(e), report: l.reportFallback}
}

// commitPass is the panel's COMMIT pass: the SAME retained enumeration, at the
// per-LOAD event cadence its by-name sibling uses.
//
// It differs from enumerationPass in the reporter and in nothing else, which is
// the whole distinction: re-resolving a setting construction already reported
// announces nothing, while the newly-live opposite slot is a load that has never
// been announced at all. Both share one loader (below) so the two cadences cannot
// come to read different parses of the same slug.
func (l Loader) commitPass(e Enumeration) resolutionPass {
	return resolutionPass{load: l.enumerationLoad(e), report: l.reportSlot}
}

// enumerationLoad is the retained enumeration as a slugLoader — the ONE source both
// enumeration-backed passes resolve through.
func (l Loader) enumerationLoad(e Enumeration) slugLoader {
	return func(slug string) (Result, *Rejection) { return l.resolveFromEnumeration(slug, e) }
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
// answers to the slug — the verdict on the directory itself.
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
// A `bad name` entry can never match: it carries no slug, and the slug being
// looked up is valid by the time this is reached (resolveNamed's charset check).
// The Result carries no Source bytes, since an enumeration retains parses rather
// than files; that field is read by `portal theme export`, which resolves by
// name.
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

// resolveNomination is the resolution both entry points ARE — the two setting
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

// resolveSlot loads one slot's nominated slug, or the mode-matched default in its
// place.
//
// ONE NOT-LOADABLE PATH SERVES EVERY CAUSE. A deleted file, a renamed file, a
// typo in prefs.json, an illegal persisted slug, a missing token, a bad colour
// and an unreadable file or directory are seven stories with one shape: the
// nomination did not load, so the slot's default does, the persisted name is
// kept, and the reason rides along for the surfaces to report. Nothing here
// branches on the cause, which is what keeps the rejection reasons a vocabulary
// rather than a control flow.
//
// AN UNSET SLOT IS NOT A FALLBACK AT ALL. ResolveSetting has already substituted
// the shipped default into the Setting, so an unset slot arrives here as an
// ordinary slug and resolves with FellBack false — no new mechanism, just the one
// rule ("an unset slot holds the shipped default") applied to a slot that is SET
// BUT UNLOADABLE rather than unset. It is also why this record needs no set-ness
// flag: unset and set-to-the-default resolve identically, and Requested already
// says which slug the badge sits on.
//
// The fallback resolves through the SAME by-name resolver the nomination did, so
// the ordering applies to it too: the embedded set is consulted first, and a
// user's `tokyo-night.theme` can never become the thing Portal falls back to.
//
// THE TWO FAILURES ARE NOT THE SAME FAILURE, and the whole of the escalation is
// telling them apart. The FIRST resolution below is the NOMINATION: when it
// fails, nothing is fatal — the slot's mode-matched default takes its place, the
// persisted name is kept and the reason is reported. The SECOND is the FALLBACK
// itself: when THAT fails, the embedded set cannot supply the theme Portal falls
// back to, and there is nothing honest left to paint.
//
// So a failed fallback returns BrokenBuiltinError — the pinned user-facing
// sentence, naming the FALLBACK's slug — and never a second fallback and never a
// compiled-in palette. There is deliberately no safety net beneath this point: a
// build-time guarantee that the embedded set resolves beats a runtime crutch.
// THIS PATH MUST NOT GROW A LAST-RESORT PALETTE LATER — one here would paint
// values nobody chose while everything resting on the guarantee still looked fine.
//
// The error is ORDINARY and travels the normal return path: no panic, no exit
// and no log line. Nothing is emitted for it either — the events report what a
// slot RESOLVED TO, and this slot resolved to nothing (see reportSlot).
//
// In a correctly built binary the second failure is unreachable, which is
// precisely why Loader.BuiltinSource exists: an unreachable fatal nobody can
// exercise is a path nobody has ever run, so the broken binary is staged by
// injecting a byte source that omits or corrupts a fallback. Production carries a
// nil field and one branch.
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

// reportSlot emits the record of ONE slot's outcome and hands the resolution
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
// is in Theme. If both events named the failed nomination, a `grep "theme:"`
// could not answer which palette is actually rendering on a broken install — so
// the impossibility is worth more here than a saved line at each call site.
//
// A slot that could not resolve AT ALL emits nothing, because it never reaches
// here: the fallback failing is the should-never-happen state, nothing loaded,
// and no fallback was applied to report.
func (l Loader) reportSlot(r SlotResolution) SlotResolution {
	l.reportFallback(r)
	l.events.Loaded(r.Resolved, r.Slot)
	return r
}

// reportFallback is the RE-RESOLUTION cadence: the failure line alone, with no
// `theme: loaded` behind it.
//
// It is the panel's reporter — the open-time re-resolution, `Esc`'s close and the
// post-commit recompute all resolve the SAME persisted setting they were already
// resolved for, so nothing was loaded that construction did not already report.
// The split holds on both sides: `theme: loaded` is construction plus the one
// commit-time load, while `theme: fallback applied` legitimately recurs on every
// panel open and every `Esc` and is deduplicated per process on `slug`+`reason`
// rather than being suppressed at a site.
//
// THE POLICY IS SINGLE-SITED HERE, not restated at the panel's call sites: an
// emission wired onto the shared body instead would put a per-load INFO on a
// per-keypress path, which is the running commentary the neighbouring dedup rules
// exist to prevent.
func (l Loader) reportFallback(r SlotResolution) SlotResolution {
	if r.FellBack {
		l.events.FallbackApplied(r.Requested, r.Slot, r.Reason)
	}
	return r
}

// fallbackSlugFor is the fallback table: `theme_light` falls to the light
// default, `theme_dark` and a constant `theme` to the dark one.
//
// It is MODE-MATCHED because a single fixed fallback would throw a
// light-terminal user with a typo in their light slot onto a dark palette — a
// bigger surprise than falling to the light default, and the one outcome the
// whole mechanism exists to avoid.
//
// The values are DefaultLightSlug and DefaultDarkSlug rather than literals, and
// that is not merely tidiness: the fallback values ARE the shipped default's
// values, and the shipped adaptive pair degrades to a constant dark default ONLY
// because an unresolvable slot lands on the theme the shipped default already
// nominates. CHANGING THESE VALUES, OR ADOPTING A SINGLE FIXED FALLBACK, SILENTLY
// BREAKS THAT: nothing stops compiling and no contrast floor moves — the property
// simply stops holding.
func fallbackSlugFor(slot Slot) string {
	if slot == SlotLight {
		return DefaultLightSlug
	}
	return DefaultDarkSlug
}
