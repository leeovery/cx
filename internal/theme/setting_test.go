package theme_test

import (
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/leeovery/portal/internal/theme"
)

// TestResolveSetting_ConstantWins pins the constant-or-pair rule's tiebreak: a non-empty
// `theme` key resolves to the CONSTANT state, whatever else the file carries.
//
// The rule is what keeps "two states, not three" a resolution rule rather than a
// file constraint — a hand-edited prefs.json may legally carry every key at once,
// and the deterministic answer is that the constant is the setting.
func TestResolveSetting_ConstantWins(t *testing.T) {
	got, raw := theme.ResolveSetting("nord", "", "")

	if !got.IsConstant {
		t.Errorf("IsConstant = false, want true — a non-empty `theme` resolves to the constant state")
	}
	if got.Constant != "nord" {
		t.Errorf("Constant = %q, want %q", got.Constant, "nord")
	}
	if want := (theme.RawKeys{Theme: "nord"}); raw != want {
		t.Errorf("RawKeys = %+v, want %+v", raw, want)
	}
}

// TestResolveSetting_ConstantIgnoresSlots pins the half of the tiebreak that is
// easy to lose: `theme` winning means the slots are NOT READ AT ALL.
//
// The resolved Setting carries no slot values, which is what makes the row-rendering rule's
// "the two setting states never coexist on screen" hold — the panel has no pair to
// render badges for. The stale slots are still returned in RawKeys, because they
// are on disk and untouched: nothing prunes them, and the surfaces that report
// what the file says (the union rule's list, the pinned copy's advisory line) need them.
func TestResolveSetting_ConstantIgnoresSlots(t *testing.T) {
	got, raw := theme.ResolveSetting("nord", "solarized", "gruvbox")

	if !got.IsConstant || got.Constant != "nord" {
		t.Errorf("Setting = %+v, want the constant %q", got, "nord")
	}
	if got.Light != "" || got.Dark != "" {
		t.Errorf("Setting slots = {light %q, dark %q}, want both empty — a winning `theme` leaves the slots unread", got.Light, got.Dark)
	}
	if want := (theme.RawKeys{Theme: "nord", Light: "solarized", Dark: "gruvbox"}); raw != want {
		t.Errorf("RawKeys = %+v, want %+v — the stale slots are on disk and still reach the panel and doctor", raw, want)
	}
}

// TestResolveSetting_UnsetSlotsTakeShippedDefaults pins the shipped adaptive default: "nothing
// set" and "pair nominated" are THE SAME STATE. There is no unconfigured branch — only a
// default value per slot.
//
// Partial pairs therefore do not exist. `theme_dark = nord` alone is the whole
// pair {tokyo-night-day, nord}, because light was never overridden rather than
// half-missing, so there is no incomplete-pair state to validate, explain or
// render around.
func TestResolveSetting_UnsetSlotsTakeShippedDefaults(t *testing.T) {
	tests := []struct {
		name       string
		light      string
		dark       string
		wantLight  string
		wantDark   string
		wantRawKey theme.RawKeys
	}{
		{
			name:       "neither slot set",
			wantLight:  theme.DefaultLightSlug,
			wantDark:   theme.DefaultDarkSlug,
			wantRawKey: theme.RawKeys{},
		},
		{
			name:       "light only",
			light:      "nord",
			wantLight:  "nord",
			wantDark:   theme.DefaultDarkSlug,
			wantRawKey: theme.RawKeys{Light: "nord"},
		},
		{
			name:       "dark only",
			dark:       "nord",
			wantLight:  theme.DefaultLightSlug,
			wantDark:   "nord",
			wantRawKey: theme.RawKeys{Dark: "nord"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, raw := theme.ResolveSetting("", tt.light, tt.dark)

			if got.IsConstant {
				t.Errorf("IsConstant = true, want false — an empty `theme` resolves to the adaptive pair")
			}
			if got.Constant != "" {
				t.Errorf("Constant = %q, want empty — an adaptive setting names no constant", got.Constant)
			}
			if got.Light != tt.wantLight || got.Dark != tt.wantDark {
				t.Errorf("Setting slots = {light %q, dark %q}, want {light %q, dark %q}", got.Light, got.Dark, tt.wantLight, tt.wantDark)
			}
			if raw != tt.wantRawKey {
				t.Errorf("RawKeys = %+v, want %+v — a default is substituted into the Setting, never into the raw keys", raw, tt.wantRawKey)
			}
		})
	}
}

// TestResolveSetting_DefaultsAreTheSharedConstants pins WHERE the substituted
// values come from: DefaultLightSlug and DefaultDarkSlug, the same two constants
// the per-slot fallback resolves to.
//
// The coincidence is load-bearing rather than incidental — the shipped adaptive default's "the
// adaptive pair degrades to a constant dark default" argument is true only because an
// unresolvable slot lands on the theme the shipped default already nominates. A
// literal here would leave that argument silently untrue the day either constant
// moves, so the substitution is asserted against the constants themselves.
//
// The substitution is also MODE-MATCHED: the light slot takes the light default
// and the dark slot the dark one, which the distinctness check below is what
// makes a real assertion rather than one a swapped pair would also pass.
func TestResolveSetting_DefaultsAreTheSharedConstants(t *testing.T) {
	if theme.DefaultLightSlug == theme.DefaultDarkSlug {
		t.Fatalf("DefaultLightSlug and DefaultDarkSlug are both %q — a swapped substitution would be undetectable", theme.DefaultLightSlug)
	}

	got, _ := theme.ResolveSetting("", "", "")

	if got.Light != theme.DefaultLightSlug {
		t.Errorf("Light = %q, want DefaultLightSlug (%q)", got.Light, theme.DefaultLightSlug)
	}
	if got.Dark != theme.DefaultDarkSlug {
		t.Errorf("Dark = %q, want DefaultDarkSlug (%q)", got.Dark, theme.DefaultDarkSlug)
	}
}

// settingKeyReaders names the three prefs.json theme keys and pulls the resolved
// slug and the raw key for each back out of ONE evaluation, so a payload can be
// exercised in every position without three near-identical tables.
var settingKeyReaders = []struct {
	name string
	read func(value string) (slug, raw string)
}{
	{
		name: "theme",
		read: func(value string) (string, string) {
			got, raw := theme.ResolveSetting(value, "", "")
			return got.Constant, raw.Theme
		},
	},
	{
		name: "theme_light",
		read: func(value string) (string, string) {
			got, raw := theme.ResolveSetting("", value, "")
			return got.Light, raw.Light
		},
	},
	{
		name: "theme_dark",
		read: func(value string) (string, string) {
			got, raw := theme.ResolveSetting("", "", value)
			return got.Dark, raw.Dark
		},
	},
}

// TestResolveSetting_ControlStripsAllThree pins the row-rendering rule: a slug that came from
// prefs.json is control-stripped AT THE POINT IT IS READ, not at the point it is
// drawn.
//
// Stripping here makes it a property of the value, so every consumer inherits it
// — the panel row and doctor's advisory line alike. A pasted newline, tab or
// ANSI escape would otherwise corrupt whichever of those the user is reading to
// FIND the problem. All three keys are covered because all three are echoed:
// truncation is the only display concern left, and that stays panel-local.
func TestResolveSetting_ControlStripsAllThree(t *testing.T) {
	payloads := []struct {
		name string
		in   string
		want string
	}{
		{name: "a trailing newline", in: "nord\n", want: "nord"},
		{name: "an interior tab", in: "no\trd", want: "nord"},
		{name: "a carriage return", in: "nord\r", want: "nord"},
		{name: "an ANSI colour escape", in: "\x1b[31mnord\x1b[0m", want: "nord"},
		{name: "a mixed payload", in: "\x1b[1mno\trd\r\n", want: "nord"},
	}

	for _, key := range settingKeyReaders {
		for _, payload := range payloads {
			t.Run(key.name+"/"+payload.name, func(t *testing.T) {
				slug, raw := key.read(payload.in)

				if slug != payload.want {
					t.Errorf("Setting slug for %s = %q, want %q", key.name, slug, payload.want)
				}
				if raw != payload.want {
					t.Errorf("RawKeys.%s = %q, want %q — the raw keys are the STRIPPED values, not the bytes on disk", key.name, raw, payload.want)
				}
				assertSingleLine(t, key.name+" setting slug", slug)
				assertSingleLine(t, key.name+" raw key", raw)
			})
		}
	}
}

// TestResolveSetting_ControlOnlyValueIsUnset records a deliberate resolution of
// an ambiguity the spec leaves open: it pins control-stripping and it pins
// "a non-empty `theme` wins", but not which happens first for a value that is
// ONLY control characters.
//
// Stripping happens first, so such a value is UNSET rather than an illegal slug.
// The row-rendering rule makes the stripped form "the value" for every consumer, and the
// alternative would mint a panel row labelled with an empty string.
func TestResolveSetting_ControlOnlyValueIsUnset(t *testing.T) {
	t.Run("a control-only theme does not win the tiebreak", func(t *testing.T) {
		got, raw := theme.ResolveSetting("\x1b[31m\n\t", "", "")

		if got.IsConstant {
			t.Errorf("IsConstant = true (Constant %q), want false — a value that strips to empty is unset, not a constant", got.Constant)
		}
		if got.Light != theme.DefaultLightSlug || got.Dark != theme.DefaultDarkSlug {
			t.Errorf("Setting slots = {light %q, dark %q}, want the shipped pair {%q, %q}", got.Light, got.Dark, theme.DefaultLightSlug, theme.DefaultDarkSlug)
		}
		if raw.Theme != "" {
			t.Errorf("RawKeys.Theme = %q, want empty", raw.Theme)
		}
	})

	t.Run("a control-only slot takes the shipped default", func(t *testing.T) {
		got, raw := theme.ResolveSetting("", "\n", "\x1b[0m")

		if got.Light != theme.DefaultLightSlug || got.Dark != theme.DefaultDarkSlug {
			t.Errorf("Setting slots = {light %q, dark %q}, want the shipped pair {%q, %q}", got.Light, got.Dark, theme.DefaultLightSlug, theme.DefaultDarkSlug)
		}
		if want := (theme.RawKeys{}); raw != want {
			t.Errorf("RawKeys = %+v, want %+v", raw, want)
		}
	})
}

// TestResolveSetting_NoTrimOrLowercase pins the negative half of the stripping
// rule: it normalises NOTHING ELSE.
//
// A value that is merely wrong — a stray leading space, the wrong case — arrives
// at the charset check unaltered, so it is reported as `bad name` for what
// the user typed rather than quietly corrected into a different slug. Trimming
// `"  nord"` to `"nord"` would silently resolve a theme the file does not name;
// lowercasing `"Nord"` would let a hand-edit shadow a built-in, which is the very
// thing the reserved-slug rule exists to prevent.
func TestResolveSetting_NoTrimOrLowercase(t *testing.T) {
	values := []struct {
		name string
		in   string
	}{
		{name: "leading spaces", in: "  nord"},
		{name: "trailing space", in: "nord "},
		{name: "interior space", in: "no rd"},
		{name: "uppercase", in: "Nord"},
	}

	for _, key := range settingKeyReaders {
		for _, value := range values {
			t.Run(key.name+"/"+value.name, func(t *testing.T) {
				slug, raw := key.read(value.in)

				if slug != value.in {
					t.Errorf("Setting slug for %s = %q, want %q unchanged — the charset check rejects it, resolution never corrects it", key.name, slug, value.in)
				}
				if raw != value.in {
					t.Errorf("RawKeys.%s = %q, want %q unchanged", key.name, raw, value.in)
				}
			})
		}
	}
}

// TestResolveSetting_ReturnsRawKeysForTheSameEvaluation pins why the raw keys
// ride back alongside the Setting: the construction-time load rule's "the nomination alone is
// insufficient for the panel".
//
// The two answer different questions from ONE evaluation, so they cannot
// disagree. The Setting says what is ACTIVE — with the shipped defaults already
// substituted, which is why a never-set slot is indistinguishable from one set to
// the same slug. The raw keys say what is PERSISTED — empty where the user set
// nothing, and still populated for slots a winning constant left unread. Deriving
// them separately is what would let the panel's list and its badges drift.
func TestResolveSetting_ReturnsRawKeysForTheSameEvaluation(t *testing.T) {
	t.Run("a default substituted into the setting never reaches the raw keys", func(t *testing.T) {
		got, raw := theme.ResolveSetting("", "", "nord")

		if got.Light != theme.DefaultLightSlug {
			t.Errorf("Light = %q, want the shipped default %q", got.Light, theme.DefaultLightSlug)
		}
		if raw.Light != "" {
			t.Errorf("RawKeys.Light = %q, want empty — the raw keys report what is PERSISTED, and nothing is", raw.Light)
		}
		if raw.Dark != "nord" {
			t.Errorf("RawKeys.Dark = %q, want %q", raw.Dark, "nord")
		}
	})

	t.Run("the raw keys are the same whichever state the setting resolved to", func(t *testing.T) {
		constant, constantRaw := theme.ResolveSetting("nord", "solarized\n", "gruvbox")
		adaptive, adaptiveRaw := theme.ResolveSetting("", "solarized\n", "gruvbox")

		if !constant.IsConstant || adaptive.IsConstant {
			t.Fatalf("the two calls did not resolve to different states: %+v and %+v", constant, adaptive)
		}
		if constantRaw.Light != adaptiveRaw.Light || constantRaw.Dark != adaptiveRaw.Dark {
			t.Errorf("slot raw keys = %+v under a constant and %+v under a pair, want identical — resolution reads the keys, it never rewrites them", constantRaw, adaptiveRaw)
		}
		if want := "solarized"; constantRaw.Light != want {
			t.Errorf("RawKeys.Light = %q, want the stripped %q", constantRaw.Light, want)
		}
	})
}

// TestResolveSetting_IsPureAndDeterministic pins the property every later phase
// leans on without re-checking: resolution is a total function of its three
// inputs.
//
// It reads no file, no environment and no clock, returns no error, and answers
// the same triple identically every time — so a surface may resolve whenever it
// needs to, and two surfaces resolving the same keys cannot disagree.
func TestResolveSetting_IsPureAndDeterministic(t *testing.T) {
	t.Run("it answers the same triple identically", func(t *testing.T) {
		first, firstRaw := theme.ResolveSetting("\x1b[31mno\trd", "", "gruvbox")
		theme.ResolveSetting("solarized", "a", "b")
		second, secondRaw := theme.ResolveSetting("\x1b[31mno\trd", "", "gruvbox")

		if first != second {
			t.Errorf("Setting = %+v then %+v for the same input, want identical", first, second)
		}
		if firstRaw != secondRaw {
			t.Errorf("RawKeys = %+v then %+v for the same input, want identical", firstRaw, secondRaw)
		}
	})

	t.Run("it reaches no file, environment or clock", func(t *testing.T) {
		for _, spec := range settingSource(t).File.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatalf("unquote import %s: %v", spec.Path.Value, err)
			}
			if why, impure := impureSettingImports[path]; impure {
				t.Errorf("setting.go imports %q, which %s — resolution is a pure function of its three inputs", path, why)
			}
		}
	})

	t.Run("it deals in slugs only", func(t *testing.T) {
		assertFieldKinds(t, reflect.TypeFor[theme.Setting](), []fieldKind{
			{name: "IsConstant", kind: reflect.Bool},
			{name: "Constant", kind: reflect.String},
			{name: "Light", kind: reflect.String},
			{name: "Dark", kind: reflect.String},
		})
		assertFieldKinds(t, reflect.TypeFor[theme.RawKeys](), []fieldKind{
			{name: "Theme", kind: reflect.String},
			{name: "Light", kind: reflect.String},
			{name: "Dark", kind: reflect.String},
		})

		signature := reflect.TypeOf(theme.ResolveSetting)
		for i := range signature.NumIn() {
			if got := signature.In(i).Kind(); got != reflect.String {
				t.Errorf("ResolveSetting parameter %d is a %s, want a string — the function is handed slugs, never palettes", i, got)
			}
		}
		wantOut := []reflect.Type{reflect.TypeFor[theme.Setting](), reflect.TypeFor[theme.RawKeys]()}
		gotOut := slices.Collect(signature.Outs())
		if !slices.Equal(gotOut, wantOut) {
			t.Errorf("ResolveSetting returns %v, want %v — no error, and nothing carrying a palette", gotOut, wantOut)
		}
	})
}

// TestInForceKeys_SelectsTheKeysInForce pins all three clauses of the construction-time load
// rule's "the keys in force" rule in the one place they are now decided: the `theme`-wins
// tiebreak, the non-empty-raw-value rule, and the same-value collapse.
//
// The three used to be authored twice — once for the union rule's panel rows and once for
// the pinned copy's doctor lines — and it is their single home, not this table, that moves
// both surfaces at once; the table holds that home to its rule. The slot each
// entry carries is what doctor's parenthetical is rendered from, so it is
// asserted alongside the value rather than left to the caller's own reading of
// the keys.
func TestInForceKeys_SelectsTheKeysInForce(t *testing.T) {
	tests := []struct {
		name string
		keys theme.RawKeys
		want []theme.InForceKey
	}{
		{
			name: "a constant alone",
			keys: theme.RawKeys{Theme: "nord-lee"},
			want: []theme.InForceKey{{Value: "nord-lee", Slot: theme.SlotConstant}},
		},
		{
			name: "a constant leaves the slots unread",
			keys: theme.RawKeys{Theme: "nord-lee", Light: "solar", Dark: "gruv"},
			want: []theme.InForceKey{{Value: "nord-lee", Slot: theme.SlotConstant}},
		},
		{
			name: "the light slot alone",
			keys: theme.RawKeys{Light: "solar"},
			want: []theme.InForceKey{{Value: "solar", Slot: theme.SlotLight}},
		},
		{
			name: "the dark slot alone",
			keys: theme.RawKeys{Dark: "gruv"},
			want: []theme.InForceKey{{Value: "gruv", Slot: theme.SlotDark}},
		},
		{
			name: "two slots naming different values, light then dark",
			keys: theme.RawKeys{Light: "solar", Dark: "gruv"},
			want: []theme.InForceKey{{Value: "solar", Slot: theme.SlotLight}, {Value: "gruv", Slot: theme.SlotDark}},
		},
		{
			name: "two slots naming one value collapse",
			keys: theme.RawKeys{Light: "solar", Dark: "solar"},
			want: []theme.InForceKey{{Value: "solar", Slot: theme.SlotLight, Both: true}},
		},
		{
			name: "two slots naming one value that yields no slug collapse too",
			keys: theme.RawKeys{Light: "../evil", Dark: "../evil"},
			want: []theme.InForceKey{{Value: "../evil", Slot: theme.SlotLight, Both: true}},
		},
		{
			name: "no keys at all",
			keys: theme.RawKeys{},
			want: nil,
		},
		{
			name: "a control-only constant leaves the slots in force",
			keys: theme.RawKeys{Theme: "\x1b[0m\n", Dark: "gruv"},
			want: []theme.InForceKey{{Value: "gruv", Slot: theme.SlotDark}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := theme.InForceKeys(tt.keys); !slices.Equal(got, tt.want) {
				t.Errorf("InForceKeys(%+v) = %+v, want %+v", tt.keys, got, tt.want)
			}
		})
	}
}

// TestInForceKeys_UnsetSlotIsNeverInForce pins the half of the rule the Setting
// alone cannot express: an unset slot holds the SHIPPED DEFAULT, and that
// substituted value is not something the user set.
//
// The vacuity guard is the whole test: the default really is in the Setting, so
// its absence from the keys in force is evidence that the RAW value is what was
// read rather than the resolved slot.
func TestInForceKeys_UnsetSlotIsNeverInForce(t *testing.T) {
	setting, _ := theme.ResolveSetting("", "", "gruv")
	if setting.Light != theme.DefaultLightSlug {
		t.Fatalf("Setting.Light = %q, want the substituted default %q — the assertion below would be vacuous", setting.Light, theme.DefaultLightSlug)
	}

	got := theme.InForceKeys(theme.RawKeys{Dark: "gruv"})

	want := []theme.InForceKey{{Value: "gruv", Slot: theme.SlotDark}}
	if !slices.Equal(got, want) {
		t.Errorf("InForceKeys = %+v, want %+v — a value Portal substituted is not one the user set", got, want)
	}
}

// TestInForceKeys_AcceptsAlreadyResolvedKeys pins the property the RawKeys
// parameter rests on: handing back the keys ResolveSetting already produced
// answers identically to handing over the values as they were read.
//
// That is what lets a caller holding either form ask the same question of the
// same function — stripping is idempotent, and the resolution is pure and total.
func TestInForceKeys_AcceptsAlreadyResolvedKeys(t *testing.T) {
	asRead := theme.RawKeys{Light: "\x1b[31msolar\n", Dark: "gruv\t"}
	_, resolved := theme.ResolveSetting(asRead.Theme, asRead.Light, asRead.Dark)

	if resolved == asRead {
		t.Fatalf("the fixture strips to itself (%+v) — the assertion below would not be about stripping at all", resolved)
	}
	if got, want := theme.InForceKeys(asRead), theme.InForceKeys(resolved); !slices.Equal(got, want) {
		t.Errorf("InForceKeys over the keys as read = %+v, over the stripped keys = %+v, want identical", got, want)
	}
	if got, want := theme.InForceKeys(asRead)[0].Value, "solar"; got != want {
		t.Errorf("first value = %q, want the stripped %q", got, want)
	}
}

// TestRawKeysWithConstant_HoldsOnlyTheConstant pins the constant direction of the
// setting's mutual exclusion as a value: the named slug, and NOTHING of the
// receiver.
//
// The clear is what the rule is FOR, so the fixture arrives carrying both slots
// and a stale constant — over empty keys the transformation and a plain field
// assignment are the same value.
func TestRawKeysWithConstant_HoldsOnlyTheConstant(t *testing.T) {
	keys := theme.RawKeys{Theme: "nord-lee", Light: "solar", Dark: "gruv"}

	got := keys.WithConstant("latte")

	if want := (theme.RawKeys{Theme: "latte"}); got != want {
		t.Errorf("WithConstant over %+v = %+v, want %+v — a constant clears both slots", keys, got, want)
	}
}

// TestRawKeysWithMember_ReplacesTheNamedHalf pins the other direction: one half
// of the pair replaced, the other carried across verbatim, and the constant gone.
func TestRawKeysWithMember_ReplacesTheNamedHalf(t *testing.T) {
	tests := []struct {
		name   string
		keys   theme.RawKeys
		member theme.Member
		slug   string
		want   theme.RawKeys
	}{
		{
			name:   "the light half over a constant",
			keys:   theme.RawKeys{Theme: "nord-lee"},
			member: theme.MemberLight,
			slug:   "solar",
			want:   theme.RawKeys{Light: "solar"},
		},
		{
			name:   "the dark half over a constant",
			keys:   theme.RawKeys{Theme: "nord-lee"},
			member: theme.MemberDark,
			slug:   "gruv",
			want:   theme.RawKeys{Dark: "gruv"},
		},
		{
			name:   "the light half of a pair, the dark one carried across",
			keys:   theme.RawKeys{Light: "solar", Dark: "gruv"},
			member: theme.MemberLight,
			slug:   "latte",
			want:   theme.RawKeys{Light: "latte", Dark: "gruv"},
		},
		{
			name:   "the dark half of a pair, the light one carried across",
			keys:   theme.RawKeys{Light: "solar", Dark: "gruv"},
			member: theme.MemberDark,
			slug:   "latte",
			want:   theme.RawKeys{Light: "solar", Dark: "latte"},
		},
		{
			name:   "a constant beside a stale pair takes the pair and drops the constant",
			keys:   theme.RawKeys{Theme: "nord-lee", Light: "solar", Dark: "gruv"},
			member: theme.MemberDark,
			slug:   "latte",
			want:   theme.RawKeys{Light: "solar", Dark: "latte"},
		},
		{
			name:   "an empty slug empties the named half",
			keys:   theme.RawKeys{Light: "solar", Dark: "gruv"},
			member: theme.MemberLight,
			slug:   "",
			want:   theme.RawKeys{Dark: "gruv"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.keys.WithMember(tt.member, tt.slug); got != tt.want {
				t.Errorf("WithMember(%d, %q) over %+v = %+v, want %+v", tt.member, tt.slug, tt.keys, got, tt.want)
			}
		})
	}
}

// TestRawKeys_TransformationsLeaveTheReceiverAlone pins that both are VALUES
// rather than mutations: the caller's own keys are untouched, so a caller that
// discards the result changes nothing.
func TestRawKeys_TransformationsLeaveTheReceiverAlone(t *testing.T) {
	original := theme.RawKeys{Theme: "nord-lee", Light: "solar", Dark: "gruv"}

	keys := original
	constant, slotted := keys.WithConstant("latte"), keys.WithMember(theme.MemberLight, "latte")

	if keys != original {
		t.Errorf("the receiver is now %+v, want the original %+v — both transformations return a NEW value", keys, original)
	}
	if constant == original || slotted == original {
		t.Errorf("WithConstant returned %+v and WithMember %+v over %+v; neither moved, so an untouched receiver says nothing", constant, slotted, original)
	}
}

// impureSettingImports names the routes out of a pure function, and what each
// one would let in. It is a deny-list rather than an allow-list because the
// property being guarded is "no I/O, no environment, no clock", not "these exact
// imports" — a later phase reaching for another pure stdlib helper is fine.
var impureSettingImports = map[string]string{
	"bufio":         "reads streams",
	"crypto/rand":   "is nondeterministic",
	"embed":         "carries files",
	"io":            "streams",
	"io/fs":         "reads filesystems",
	"log/slog":      "logs",
	"math/rand":     "is nondeterministic",
	"math/rand/v2":  "is nondeterministic",
	"net":           "talks to the network",
	"os":            "reads files and the environment",
	"os/exec":       "runs programs",
	"path/filepath": "resolves paths",
	"time":          "reads the clock",
}

// settingSource returns the parsed setting.go, failing rather than passing
// vacuously when the file is not found.
func settingSource(t *testing.T) parsedThemeSource {
	t.Helper()

	for _, source := range parseThemeSources(t) {
		if source.Name == "setting.go" {
			return source
		}
	}
	t.Fatal("setting.go not found among internal/theme's production sources")
	return parsedThemeSource{}
}

// fieldKind is one expected struct field: its name and the kind of its type.
type fieldKind struct {
	name string
	kind reflect.Kind
}

// assertFieldKinds fails unless the struct declares exactly the given fields, in
// order, each of the given kind. It is what pins "slugs only": a field carrying a
// Theme, a Token or a canvas value would be a struct kind, not a string.
func assertFieldKinds(t *testing.T, structType reflect.Type, want []fieldKind) {
	t.Helper()

	got := make([]fieldKind, 0, structType.NumField())
	for field := range structType.Fields() {
		got = append(got, fieldKind{name: field.Name, kind: field.Type.Kind()})
	}
	if !slices.Equal(got, want) {
		t.Errorf("%s fields = %+v, want %+v", structType.Name(), got, want)
	}
}

// assertSingleLine fails when a value still carries a control character, which is
// the checkable form of the row-rendering rule's "renders as one line of ordinary text".
func assertSingleLine(t *testing.T, what, value string) {
	t.Helper()

	if strings.ContainsFunc(value, unicode.IsControl) {
		t.Errorf("%s = %q still carries a control character — the value must render as one line", what, value)
	}
}
