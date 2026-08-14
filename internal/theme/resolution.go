package theme

// Slot is what makes a fallback mode-matched: it says whether a light or a dark
// default should replace a nomination.
type Slot int

const (
	// SlotConstant is prefs.json's `theme`, and the zero value: the degenerate
	// one-slot case, not an unset sentinel.
	SlotConstant Slot = iota
	// SlotLight is prefs.json's `theme_light`.
	SlotLight
	// SlotDark is prefs.json's `theme_dark`.
	SlotDark
)

// AttrName is the slot's name in the light/dark vocabulary. A constant has no
// half of a pair to name, so it reports false.
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

type SlotResolution struct {
	Slot Slot

	// Requested is the nominated slug, or the shipped default's where the slot
	// was unset: ResolveSetting substitutes the default before resolution, so
	// unset and set-to-the-default are one state here. RawKeys still separates
	// them.
	Requested string

	Resolved string

	FellBack bool

	// Reason is why the nomination was not usable, populated iff FellBack.
	Reason Reason

	// Theme is Resolved's palette, never Requested's.
	Theme Theme
}

// Resolution is the whole theme setting, loaded. Nomination and Slots ride
// together from one evaluation, so two surfaces cannot disagree about which
// theme is live.
type Resolution struct {
	Nomination Nomination

	// Slots holds one record per slot: the constant alone, or light then dark.
	Slots []SlotResolution
}

// ResolveNomination loads every theme a Setting nominates, substituting the
// mode-matched default for any slot that will not load. It writes nothing:
// falling back must never overwrite the persisted name, so fixing the theme
// file restores it on the next launch. A non-nil error is BrokenBuiltinError —
// the fallback itself did not load — returned with a zero Resolution.
func (l Loader) ResolveNomination(s Setting, themesDir string) (Resolution, error) {
	return l.resolveNomination(s, l.byNamePass(themesDir))
}

// ResolveNominationFrom re-runs ResolveNomination against a retained Enumeration
// instead of the themes directory, reading nothing: a fresh read would parse the
// slug again, free to disagree with the row the user is looking at. A slug with
// no entry is `not found`, or `unreadable` where the directory could not be
// listed. It emits the fallback WARN only, never `theme: loaded`.
func (l Loader) ResolveNominationFrom(e Enumeration, s Setting) (Resolution, error) {
	return l.resolveNomination(s, l.enumerationPass(e))
}

// ResolveSlot resolves one slot against a retained Enumeration, reading nothing,
// and does emit `theme: loaded` — a commit is a genuine load, not a
// re-derivation of one already reported, which is what the re-resolution passes
// do. The slug must already have the default substituted in, or an untouched
// slot is reported as a fallback.
func (l Loader) ResolveSlot(e Enumeration, slot Slot, slug string) (SlotResolution, error) {
	return l.resolveSlot(slot, slug, l.commitPass(e))
}

// resolutionPass pairs where a slug loads from with how the resolved slot is
// reported. The two are independent: a commit resolves from a retained
// enumeration and still reports a genuine load. The three constructors below are
// the only pairings.
type resolutionPass struct {
	// load is the whole by-name ladder, so the fallback resolves through the
	// identical route the nomination did.
	load slugLoader

	report func(SlotResolution) SlotResolution
}

func (l Loader) byNamePass(themesDir string) resolutionPass {
	return resolutionPass{
		load:   func(slug string) (Result, *Rejection) { return l.ResolveByName(slug, themesDir) },
		report: l.reportSlot,
	}
}

func (l Loader) enumerationPass(e Enumeration) resolutionPass {
	return resolutionPass{load: l.enumerationLoad(e), report: l.reportFallback}
}

func (l Loader) commitPass(e Enumeration) resolutionPass {
	return resolutionPass{load: l.enumerationLoad(e), report: l.reportSlot}
}

func (l Loader) enumerationLoad(e Enumeration) slugLoader {
	return func(slug string) (Result, *Rejection) { return l.ResolveByNameFrom(e, slug) }
}

// ResolveByNameFrom runs ResolveByName's ladder against a retained
// Enumeration's entries instead of the themes directory: no I/O, no events. A
// drop-in resolves Source-less, because an enumeration retains parses rather
// than files; a built-in still comes back carrying its embedded bytes.
func (l Loader) ResolveByNameFrom(e Enumeration, slug string) (Result, *Rejection) {
	return l.resolveNamed(slug, func(s string) (Result, *Rejection) {
		return entryResult(s, e)
	})
}

// The unresolved answer goes through unresolvedRejection, as the union's
// persisted rows do, so a row and a resolution cannot state different reasons
// for one slug.
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
		Nomination: AdaptivePair(light.Theme, dark.Theme),
		Slots:      []SlotResolution{light, dark},
	}, nil
}

// A failed fallback returns BrokenBuiltinError and nothing else: no safety net
// belongs beneath this point, since a last-resort palette would paint values
// nobody chose while a broken embedded set still looked fine.
func (l Loader) resolveSlot(slot Slot, slug string, pass resolutionPass) (SlotResolution, error) {
	result, rejection := pass.load(slug)
	if rejection == nil {
		return pass.report(SlotResolution{Slot: slot, Requested: slug, Resolved: result.Slug, Theme: result.Theme}), nil
	}

	fallbackSlug := defaultSlugFor(slot)
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
func (l Loader) reportSlot(r SlotResolution) SlotResolution {
	l.reportFallback(r)
	l.events.Loaded(r.Resolved, r.Slot)
	return r
}

// reportFallback is the re-resolution cadence: the failure line alone. Adding
// the load line here would put a per-load INFO on a per-keypress path.
func (l Loader) reportFallback(r SlotResolution) SlotResolution {
	if r.FellBack {
		l.events.FallbackApplied(r.Requested, r.Slot, r.Reason)
	}
	return r
}

// defaultSlugFor is the slot's shipped default, mode-matched deliberately: one
// fixed default would throw a light-terminal user with a typo in their light
// slot onto a dark palette. A constant names no mode to match, so it takes the
// dark default.
func defaultSlugFor(slot Slot) string {
	if slot == SlotLight {
		return DefaultLightSlug
	}
	return DefaultDarkSlug
}
