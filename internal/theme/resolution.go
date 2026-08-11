package theme

// Slot names which of the theme setting's slots one resolution is about. The
// slot is what makes a fallback mode-matched — it is the only thing saying
// whether a light or dark default should replace a nomination. SlotConstant
// is the zero value: the degenerate one-slot case, not an unset sentinel.
type Slot int

const (
	// SlotConstant is prefs.json's `theme`.
	SlotConstant Slot = iota
	// SlotLight is prefs.json's `theme_light`.
	SlotLight
	// SlotDark is prefs.json's `theme_dark`.
	SlotDark
)

// AttrName is the slot's name in the light/dark vocabulary, and whether the
// slot has one at all — the definition every surface that names a slot reads
// from. A constant carries no name: the name says which half of an adaptive
// pair is meant, and a constant has no halves.
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

// SlotResolution is what happened to one slot: the slug asked for, the slug
// that loaded, and — where they differ — why.
type SlotResolution struct {
	Slot Slot

	// Requested is the slug nominated, or the shipped default's slug where
	// the slot was unset: ResolveSetting substitutes the default before
	// resolution, so unset and set-to-the-default are one state here. The
	// distinction survives only in RawKeys.
	Requested string

	// Resolved is the slug whose palette is in Theme.
	Resolved string

	FellBack bool

	// Reason is why the nomination was not usable, populated iff FellBack.
	Reason Reason

	// Theme is Resolved's palette, never Requested's.
	Theme Theme
}

// Resolution is the whole theme setting, loaded. The Nomination says what
// will be painted and the Slots what was asked for; they ride together from
// one evaluation so a surface deriving either separately cannot disagree with
// the other about which theme is live.
type Resolution struct {
	Nomination Nomination

	// Slots holds one record per slot: 1 under a constant, 2 under a pair in
	// light-then-dark order.
	Slots []SlotResolution
}

// ResolveNomination loads every theme a Setting nominates, substituting the
// mode-matched default for any slot that will not load, and reports per slot
// what was asked for and what loaded.
//
// It writes nothing: falling back must never overwrite the persisted theme
// name, so fixing the theme file restores it on the next launch with no
// re-selection. Slots resolve independently — a broken light slot leaves a
// loadable dark one alone. The error is the should-never-happen state (the
// fallback itself did not resolve); it is BrokenBuiltinError and a caller
// simply returns it, unwrapped and unlogged, with a zero Resolution
// alongside.
func (l Loader) ResolveNomination(s Setting, themesDir string) (Resolution, error) {
	return l.resolveNomination(s, l.byNamePass(themesDir))
}

// ResolveNominationFrom re-runs ResolveNomination's resolution against a
// retained Enumeration instead of the themes directory, through the same rule
// body. It performs no directory read: a read here would produce a third
// parse of the same slug, free to disagree with the row the user is looking
// at. A slug the enumeration has no entry for is `not found`, or `unreadable`
// where the directory could not be listed. It emits no `theme: loaded` — a
// per-open line would turn a per-load INFO into running commentary — but the
// fallback WARN still fires.
func (l Loader) ResolveNominationFrom(e Enumeration, s Setting) (Resolution, error) {
	return l.resolveNomination(s, l.enumerationPass(e))
}

// ResolveSlot resolves one slot against a retained Enumeration and emits the
// commit-time `theme: loaded` — a genuine load nothing else has reported,
// naming the slug that actually rendered. Same rule body and no directory
// read, as ResolveNominationFrom. The slug must be an already-defaulted one,
// or an untouched slot would be reported as a fallback.
func (l Loader) ResolveSlot(e Enumeration, slot Slot, slug string) (SlotResolution, error) {
	return l.resolveSlot(slot, slug, l.commitPass(e))
}

// resolutionPass is everything that differs between the entry points: where a
// slug loads from, and how a resolved slot is reported. The two travel
// together — a pass resolving against a retained parse is by definition the
// one that must not emit `theme: loaded` — so pairing them in a type stops a
// call site pairing them the other way round.
type resolutionPass struct {
	// load is the whole by-name ladder, so the fallback resolves through the
	// identical route the nomination did.
	load slugLoader

	report func(SlotResolution) SlotResolution
}

// byNamePass is construction's: the themes directory read by name, at the
// per-load event cadence.
func (l Loader) byNamePass(themesDir string) resolutionPass {
	return resolutionPass{
		load:   func(slug string) (Result, *Rejection) { return l.ResolveByName(slug, themesDir) },
		report: l.reportSlot,
	}
}

// enumerationPass is the panel's re-resolution: the retained enumeration, at
// the fallback-line-only cadence.
func (l Loader) enumerationPass(e Enumeration) resolutionPass {
	return resolutionPass{load: l.enumerationLoad(e), report: l.reportFallback}
}

// commitPass differs from enumerationPass in the reporter alone: the
// newly-live opposite slot is a load nothing has announced. Both share one
// loader so the cadences cannot read different parses of the same slug.
func (l Loader) commitPass(e Enumeration) resolutionPass {
	return resolutionPass{load: l.enumerationLoad(e), report: l.reportSlot}
}

func (l Loader) enumerationLoad(e Enumeration) slugLoader {
	return func(slug string) (Result, *Rejection) { return l.ResolveByNameFrom(e, slug) }
}

// ResolveByNameFrom resolves one slug through ResolveByName's ladder with a
// retained Enumeration's entries in place of the themes directory. It
// performs no I/O — a read here would produce a second parse of the same slug,
// free to disagree with the one the caller is showing — and emits nothing.
func (l Loader) ResolveByNameFrom(e Enumeration, slug string) (Result, *Rejection) {
	return l.resolveNamed(slug, func(s string) (Result, *Rejection) {
		return entryResult(s, e)
	})
}

// Nothing is re-derived: the palette and rejection ride across from the entry
// exactly as the ladder produced them, so the resolved theme and the row the
// user sees are the same parse. The unresolved answer goes through
// unresolvedRejection, the same function the union's persisted rows use, so a
// row and a resolution cannot state different reasons for one slug. The
// Result carries no Source bytes — an enumeration retains parses, not files.
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

// resolveSlot loads one slot's nominated slug, or the mode-matched default in
// its place. One not-loadable path serves every cause, so nothing branches on
// the cause and the rejection reasons stay a vocabulary rather than control
// flow. The fallback resolves through the same by-name route, so the embedded
// set is consulted first for it too.
//
// The two failures are different: a failed nomination is ordinary, a failed
// fallback means the embedded set cannot supply what Portal falls back to.
// That returns BrokenBuiltinError and nothing else — there is deliberately no
// safety net beneath this point, and one must not be added: a last-resort
// palette would paint values nobody chose while everything resting on the
// build-time guarantee still looked fine.
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

// Order matters: the failure line precedes the palette that replaced it.
// Emitting from the assembled record rather than the branch that produced it
// makes `theme: loaded` structurally incapable of naming the slug that
// failed — the only slug it can reach is Resolved.
func (l Loader) reportSlot(r SlotResolution) SlotResolution {
	l.reportFallback(r)
	l.events.Loaded(r.Resolved, r.Slot)
	return r
}

// reportFallback is the re-resolution cadence: the failure line alone. The
// panel's paths re-resolve a setting construction already reported, so an
// emission wired onto the shared body would put a per-load INFO on a
// per-keypress path.
func (l Loader) reportFallback(r SlotResolution) SlotResolution {
	if r.FellBack {
		l.events.FallbackApplied(r.Requested, r.Slot, r.Reason)
	}
	return r
}

// Mode-matched deliberately: a single fixed fallback would throw a
// light-terminal user with a typo in their light slot onto a dark palette.
// The values must stay the shipped defaults' — that identity is the only
// reason the shipped pair degrades to a constant dark default, and breaking
// it fails no build.
func fallbackSlugFor(slot Slot) string {
	if slot == SlotLight {
		return DefaultLightSlug
	}
	return DefaultDarkSlug
}
