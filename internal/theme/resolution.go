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

// AttrName is the slot's name in the light/dark vocabulary, and whether the slot
// has one at all.
//
// It is the definition of those two words, so every surface that names a slot
// reads them from here rather than restating them: the `theme` component's `slot`
// attr and doctor's rendered `(light)` / `(dark)` parenthetical. The words are
// greppable in one and user-visible in the other, so a second declaration is a
// drift the reader has no way to see.
//
// A constant carries no name — not an empty string, and not the word "constant".
// The name says which half of an adaptive pair is meant, and a constant setting
// has no halves: a value there would assert a distinction the two-state setting
// does not have, and an empty one would grep as a slot named nothing.
func (s Slot) AttrName() (string, bool) {
	switch s {
	case SlotLight:
		return "light", true
	case SlotDark:
		return "dark", true
	default:
		return "", false
	}
}

// SlotResolution is what happened to one slot: the slug that was asked for, the
// slug that actually loaded, and — where those differ — why.
//
// The record is structured rather than folded into a palette because every later
// surface needs the parts separately and none can recover them from a Theme:
// `theme: fallback applied` carries the slug that failed and its reason, doctor's
// line renders `theme <slug> (<slot>) does not resolve: <reason>`, and the
// panel's badge table keeps the `●` on the persisted slug while the fallback's
// own row carries none.
type SlotResolution struct {
	// Slot is which of the setting's slots this record is about.
	Slot Slot

	// Requested is the slug that was nominated — the shipped default's slug when
	// the slot was unset.
	//
	// That the two converge is why this record carries no "was this slot set?"
	// flag: ResolveSetting substitutes the shipped default into the Setting before
	// resolution, so Setting{Light: DefaultLightSlug} is identical whether the slot
	// was unset or explicitly set to that slug. The distinction survives only in
	// RawKeys, which resolution is deliberately not given.
	//
	// No consumer needs it: the badge table keys all three of its cases on this one
	// field, doctor reads the raw keys directly, and the panel's per-slot commit
	// takes the raw slug as a parameter.
	Requested string

	// Resolved is the slug whose palette is in Theme: Requested where it loaded,
	// and the slot's mode-matched default where it did not.
	Resolved string

	// FellBack reports whether Requested and Resolved name different themes —
	// that is, whether the mode-matched fallback was applied.
	FellBack bool

	// Reason is why the nomination was not usable, populated iff FellBack. It is
	// exactly one reason, whatever the cause: a deleted file, a renamed file, a
	// typo in prefs.json, an illegal slug, a missing token, a bad colour and an
	// unreadable file or directory all take the same path and differ only here.
	Reason Reason

	// Theme is the palette that actually loaded — Resolved's, never Requested's.
	Theme Theme
}

// Resolution is the whole theme setting, loaded: the Nomination the TUI
// constructor takes, plus one SlotResolution per slot.
//
// The two ride together because they answer different questions from one
// evaluation, as the Setting and the raw keys do: the Nomination says what will
// be painted, the slots say what was asked for and what happened. A surface
// deriving either separately is how the picker and its badges would come to
// disagree about which theme is live.
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
// It writes nothing: falling back must never overwrite the persisted theme name
// in prefs.json. Portal keeps the user's choice and renders the fallback, so
// fixing the theme file restores it on the next launch with no re-selection,
// where persisting the fallback would turn a transient failure into a destructive
// one at the moment the user is least able to tell what happened.
//
// Both slots resolve independently and neither short-circuits the other: a broken
// light slot leaves a loadable dark slot as it was, and two broken slots in one
// launch produce two fallbacks rather than one plus an abandoned resolution.
//
// It emits the per-slot pair — `theme: fallback applied` where one was, then
// `theme: loaded` for the slug that actually loaded — through the injected seam,
// from the one place that holds every slot's outcome, so the cadence is one
// `loaded` per slot and never one combined line. See reportSlot.
//
// The error is the should-never-happen state: the fallback itself did not
// resolve. A nomination failing is ordinary and is absorbed silently by the
// fallback; a fallback failing means the embedded set cannot supply the theme
// Portal falls back to, and there is nothing left to paint. See resolveSlot.
//
// A caller simply returns it. It is BrokenBuiltinError — the pinned user-facing
// sentence — meant to travel the ordinary error path to main.go's single os.Exit
// owner, which prints it as one line and exits non-zero. Nothing on the way
// should inspect it, re-word it, log it or substitute a palette for it: the
// Resolution alongside it is the zero value so there is nothing to render.
func (l Loader) ResolveNomination(s Setting, themesDir string) (Resolution, error) {
	return l.resolveNomination(s, l.byNamePass(themesDir))
}

// ResolveNominationFrom re-runs that entire resolution — the same charset check,
// the same embedded-set-first ordering, the same per-slot mode-matched fallback,
// the same structured record and the same broken-builtin fatal — against a
// retained Enumeration instead of the themes directory.
//
// The only difference is where the slug is looked up once the embedded set has
// declined; everything else is the same code (see resolveNomination), so the two
// entry points cannot drift about what a slug means, which slot falls back to
// what, or when a fallback is fatal.
//
// It performs no directory read: an open-time or commit-time read would produce a
// third parse of the same slug, neither construction's nor the panel's, free to
// disagree with the row the user is looking at. The panel's retained parse is the
// fresher truth.
//
// The panel's paths share it: the open-time re-resolution, `Esc`'s close and the
// post-commit badge recompute. The mid-session slot load resolves against the
// same enumeration through the same rule body but takes ResolveSlot below: it is
// one slot rather than a setting, and it emits.
//
// A slug the enumeration has no entry for is `not found`, or `unreadable` where
// the directory itself could not be listed — the same discrimination
// unresolvedRejection draws for the union's own persisted rows, so a row and the
// theme that actually rendered can never state different reasons for one slug.
//
// It emits no `theme: loaded`: that event's cadence is construction plus the one
// commit-time load outside it, and a per-open/per-`Esc` line would turn a
// per-load INFO into running commentary. The `theme: fallback applied` WARN still
// fires, deduplicated per process. See resolutionPass.
func (l Loader) ResolveNominationFrom(e Enumeration, s Setting) (Resolution, error) {
	return l.resolveNomination(s, l.enumerationPass(e))
}

// ResolveSlot resolves one slot against a retained Enumeration and emits the
// commit-time `theme: loaded` — the single theme load that happens outside
// construction.
//
// It runs the same rule body its neighbour does: the charset check, the
// embedded-set-first ordering, the per-slot mode-matched fallback, the structured
// record and the broken-builtin fatal all arrive through resolveSlot and the
// enumeration's own loader (see commitPass), so the badge path and the load path
// cannot disagree about what one slug means.
//
// It performs no directory read, for ResolveNominationFrom's reason exactly.
//
// Only the cadence differs. Its neighbour re-resolves a setting construction
// already reported and emits no `theme: loaded`; this is a genuine load of a slot
// nothing has reported, so it emits one carrying the slug that actually rendered
// — the fallback's where one was applied. The two are separate methods rather
// than a flag so that pairing is stated in a type (see resolutionPass) and a
// later call site cannot pair them the other way round.
//
// The slug is an already-defaulted one: an unset slot holds the shipped default,
// which ResolveSetting and Setting.Slug substitute first, so an untouched slot
// resolves from the embedded set with FellBack false rather than being reported
// as a fallback.
func (l Loader) ResolveSlot(e Enumeration, slot Slot, slug string) (SlotResolution, error) {
	return l.resolveSlot(slot, slug, l.commitPass(e))
}

// resolutionPass is everything that differs between the entry points above: where
// a slug loads from, and how a resolved slot is reported.
//
// It is one value rather than two parameters threaded down two levels, and a pair
// of functions rather than a flag, because the two travel together and neither is
// meaningful alone: a pass resolving against a retained parse is by definition the
// one that must not emit `theme: loaded`. Naming that correspondence in a type
// stops another call site pairing them the other way round.
type resolutionPass struct {
	// load is the whole by-name ladder for this pass, so the fallback resolves
	// through the identical route the nomination did and the embedded set is
	// consulted first on both.
	load slugLoader

	// report is the per-slot emission at this pass's cadence, and it hands the
	// record straight back so a caller states the outcome once (see reportSlot).
	report func(SlotResolution) SlotResolution
}

// byNamePass is construction's pass: the themes directory read by name, at the
// per-load event cadence (`theme: fallback applied` where one was, then
// `theme: loaded` for the palette that rendered).
func (l Loader) byNamePass(themesDir string) resolutionPass {
	return resolutionPass{
		load:   func(slug string) (Result, *Rejection) { return l.ResolveByName(slug, themesDir) },
		report: l.reportSlot,
	}
}

// enumerationPass is the panel's re-resolution pass: the retained enumeration, at
// the re-resolution cadence — the fallback line alone, no `theme: loaded`.
func (l Loader) enumerationPass(e Enumeration) resolutionPass {
	return resolutionPass{load: l.enumerationLoad(e), report: l.reportFallback}
}

// commitPass is the panel's commit pass: the same retained enumeration, at the
// per-load event cadence its by-name sibling uses.
//
// It differs from enumerationPass in the reporter alone: re-resolving a setting
// construction already reported announces nothing, while the newly-live opposite
// slot is a load that has never been announced. Both share one loader (below) so
// the two cadences cannot come to read different parses of the same slug.
func (l Loader) commitPass(e Enumeration) resolutionPass {
	return resolutionPass{load: l.enumerationLoad(e), report: l.reportSlot}
}

// enumerationLoad is the retained enumeration as a slugLoader — the source both
// enumeration-backed passes resolve through.
func (l Loader) enumerationLoad(e Enumeration) slugLoader {
	return func(slug string) (Result, *Rejection) { return l.ResolveByNameFrom(e, slug) }
}

// ResolveByNameFrom resolves one slug through ResolveByName's identical ladder —
// the charset check, the embedded set first, then the source — with a retained
// Enumeration's entries in place of the themes directory.
//
// It performs no I/O: a directory read here would produce a second parse of the
// same slug, free to disagree with the one the caller already holds and is
// showing.
//
// It emits nothing, as ResolveByName does not either — the resolution's events
// belong to the slot report above.
func (l Loader) ResolveByNameFrom(e Enumeration, slug string) (Result, *Rejection) {
	return l.resolveNamed(slug, func(s string) (Result, *Rejection) {
		return entryResult(s, e)
	})
}

// entryResult answers what one slug loads to within a retained enumeration: the
// entry's own palette, the entry's own single rejection, or — where nothing
// answers to the slug — the verdict on the directory itself.
//
// Nothing is re-derived. The palette and the rejection ride across from the entry
// exactly as the ladder produced them at enumeration time, which makes the
// resolved theme and the row the user is looking at the same parse.
//
// The unresolved answer is unresolvedRejection — the same function the union's
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

// resolveNomination is the resolution both entry points are — the two setting
// states, resolved slot by slot through whichever pass it was handed.
//
// It is shared rather than duplicated because every rule stated in
// ResolveNomination's comment is a rule about the setting, not about the source:
// the two slots resolve independently and neither short-circuits the other, the
// nomination's shape mirrors the setting's, and a failing fallback abandons the
// whole resolution. A second copy for the panel is how a broken light slot would
// come to short-circuit a good dark one on one path and not the other.
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
		Nomination: AdaptivePair(MemberLight.Palette(light.Theme), dark.Theme),
		Slots:      []SlotResolution{light, dark},
	}, nil
}

// resolveSlot loads one slot's nominated slug, or the mode-matched default in its
// place.
//
// One not-loadable path serves every cause. A deleted file, a renamed file, a
// typo in prefs.json, an illegal persisted slug, a missing token, a bad colour
// and an unreadable file or directory all have one shape: the nomination did not
// load, so the slot's default does, the persisted name is kept, and the reason
// rides along for the surfaces to report. Nothing here branches on the cause,
// which keeps the rejection reasons a vocabulary rather than a control flow.
//
// An unset slot is not a fallback at all: ResolveSetting has already substituted
// the shipped default into the Setting, so it arrives here as an ordinary slug
// and resolves with FellBack false.
//
// The fallback resolves through the same by-name resolver the nomination did, so
// the ordering applies to it too: the embedded set is consulted first, and a
// user's `tokyo-night.theme` can never become the thing Portal falls back to.
//
// The two failures below are different failures, and telling them apart is the
// whole of the escalation. The first resolution is the nomination: when it fails
// nothing is fatal — the slot's mode-matched default takes its place, the
// persisted name is kept and the reason is reported. The second is the fallback
// itself: when that fails, the embedded set cannot supply the theme Portal falls
// back to, and there is nothing honest left to paint.
//
// A failed fallback therefore returns BrokenBuiltinError — the pinned user-facing
// sentence naming the fallback's slug — and never a second fallback or a
// compiled-in palette. There is deliberately no safety net beneath this point,
// and this path must not grow a last-resort palette later: one would paint values
// nobody chose while everything resting on the build-time guarantee still looked
// fine.
//
// The error is ordinary and travels the normal return path: no panic, no exit and
// no log line. Nothing is emitted for it either — the events report what a slot
// resolved to, and this slot resolved to nothing (see reportSlot).
//
// In a correctly built binary the second failure is unreachable, which is why
// Loader.BuiltinSource exists: the broken binary is staged by injecting a byte
// source that omits or corrupts a fallback.
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
// The order is fixed: the failure first, then the palette that replaced it. A
// reader scanning the log meets the cause before the consequence, and a slot that
// simply loaded emits the second line alone.
//
// It emits from the assembled record rather than from the branch that produced
// it, which makes `theme: loaded` structurally incapable of naming the slug that
// failed: the only slug it can reach is Resolved, the one whose palette is in
// Theme. If both events named the failed nomination, a `grep "theme:"` could not
// answer which palette is actually rendering on a broken install.
//
// A slot that could not resolve AT ALL emits nothing, because it never reaches
// here: the fallback failing is the should-never-happen state, nothing loaded,
// and no fallback was applied to report.
func (l Loader) reportSlot(r SlotResolution) SlotResolution {
	l.reportFallback(r)
	l.events.Loaded(r.Resolved, r.Slot)
	return r
}

// reportFallback is the re-resolution cadence: the failure line alone, with no
// `theme: loaded` behind it.
//
// It is the panel's reporter — the open-time re-resolution, `Esc`'s close and the
// post-commit recompute all resolve the same persisted setting they were already
// resolved for, so nothing was loaded that construction did not already report.
// The split holds on both sides: `theme: loaded` is construction plus the one
// commit-time load, while `theme: fallback applied` legitimately recurs on every
// panel open and every `Esc` and is deduplicated per process on `slug`+`reason`
// rather than suppressed at a site.
//
// The policy is single-sited here rather than restated at the panel's call sites:
// an emission wired onto the shared body would put a per-load INFO on a
// per-keypress path.
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
