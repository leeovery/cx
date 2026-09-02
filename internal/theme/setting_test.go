package theme_test

import (
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/leeovery/portal/internal/sourceguardtest"
	"github.com/leeovery/portal/internal/theme"
)

func TestResolveSetting_ConstantWins(t *testing.T) {
	got, raw := theme.ResolveSetting(theme.RawKeys{Theme: "nord"})

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

func TestResolveSetting_ConstantIgnoresSlots(t *testing.T) {
	got, raw := theme.ResolveSetting(theme.RawKeys{Theme: "nord", Light: "solarized", Dark: "gruvbox"})

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
			got, raw := theme.ResolveSetting(theme.RawKeys{Light: tt.light, Dark: tt.dark})

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

func TestResolveSetting_DefaultsAreTheSharedConstants(t *testing.T) {
	if theme.DefaultLightSlug == theme.DefaultDarkSlug {
		t.Fatalf("DefaultLightSlug and DefaultDarkSlug are both %q — a swapped substitution would be undetectable", theme.DefaultLightSlug)
	}

	got, _ := theme.ResolveSetting(theme.RawKeys{})

	if got.Light != theme.DefaultLightSlug {
		t.Errorf("Light = %q, want DefaultLightSlug (%q)", got.Light, theme.DefaultLightSlug)
	}
	if got.Dark != theme.DefaultDarkSlug {
		t.Errorf("Dark = %q, want DefaultDarkSlug (%q)", got.Dark, theme.DefaultDarkSlug)
	}
}

var settingKeyReaders = []struct {
	name string
	read func(value string) (slug, raw string)
}{
	{
		name: "theme",
		read: func(value string) (string, string) {
			got, raw := theme.ResolveSetting(theme.RawKeys{Theme: value})
			return got.Constant, raw.Theme
		},
	},
	{
		name: "theme_light",
		read: func(value string) (string, string) {
			got, raw := theme.ResolveSetting(theme.RawKeys{Light: value})
			return got.Light, raw.Light
		},
	},
	{
		name: "theme_dark",
		read: func(value string) (string, string) {
			got, raw := theme.ResolveSetting(theme.RawKeys{Dark: value})
			return got.Dark, raw.Dark
		},
	},
}

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

func TestResolveSetting_ControlOnlyValueIsUnset(t *testing.T) {
	t.Run("a control-only theme does not win the tiebreak", func(t *testing.T) {
		got, raw := theme.ResolveSetting(theme.RawKeys{Theme: "\x1b[31m\n\t"})

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
		got, raw := theme.ResolveSetting(theme.RawKeys{Light: "\n", Dark: "\x1b[0m"})

		if got.Light != theme.DefaultLightSlug || got.Dark != theme.DefaultDarkSlug {
			t.Errorf("Setting slots = {light %q, dark %q}, want the shipped pair {%q, %q}", got.Light, got.Dark, theme.DefaultLightSlug, theme.DefaultDarkSlug)
		}
		if want := (theme.RawKeys{}); raw != want {
			t.Errorf("RawKeys = %+v, want %+v", raw, want)
		}
	})
}

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

func TestResolveSetting_ReturnsRawKeysForTheSameEvaluation(t *testing.T) {
	t.Run("a default substituted into the setting never reaches the raw keys", func(t *testing.T) {
		got, raw := theme.ResolveSetting(theme.RawKeys{Dark: "nord"})

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
		constant, constantRaw := theme.ResolveSetting(theme.RawKeys{Theme: "nord", Light: "solarized\n", Dark: "gruvbox"})
		adaptive, adaptiveRaw := theme.ResolveSetting(theme.RawKeys{Light: "solarized\n", Dark: "gruvbox"})

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

func TestResolveSetting_IsPureAndDeterministic(t *testing.T) {
	t.Run("it answers the same triple identically", func(t *testing.T) {
		first, firstRaw := theme.ResolveSetting(theme.RawKeys{Theme: "\x1b[31mno\trd", Dark: "gruvbox"})
		theme.ResolveSetting(theme.RawKeys{Theme: "solarized", Light: "a", Dark: "b"})
		second, secondRaw := theme.ResolveSetting(theme.RawKeys{Theme: "\x1b[31mno\trd", Dark: "gruvbox"})

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
				t.Errorf("setting.go imports %q, which %s — resolution is a pure function of the keys it is handed", path, why)
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
		wantIn := []reflect.Type{reflect.TypeFor[theme.RawKeys]()}
		gotIn := slices.Collect(signature.Ins())
		if !slices.Equal(gotIn, wantIn) {
			t.Errorf("ResolveSetting takes %v, want %v — the whole value, so no call site can hand the slots over transposed", gotIn, wantIn)
		}
		wantOut := []reflect.Type{reflect.TypeFor[theme.Setting](), reflect.TypeFor[theme.RawKeys]()}
		gotOut := slices.Collect(signature.Outs())
		if !slices.Equal(gotOut, wantOut) {
			t.Errorf("ResolveSetting returns %v, want %v — no error, and nothing carrying a palette", gotOut, wantOut)
		}
	})
}

func TestSetting_Slug(t *testing.T) {
	tests := []struct {
		name    string
		setting theme.Setting
		slot    theme.Slot
		want    string
	}{
		{
			name:    "the constant slot of a constant setting",
			setting: theme.Setting{IsConstant: true, Constant: "nord"},
			slot:    theme.SlotConstant,
			want:    "nord",
		},
		{
			name:    "the light slot of a pair",
			setting: theme.Setting{Light: "nord", Dark: "tokyo-night"},
			slot:    theme.SlotLight,
			want:    "nord",
		},
		{
			name:    "the dark slot of a pair",
			setting: theme.Setting{Light: "nord", Dark: "tokyo-night"},
			slot:    theme.SlotDark,
			want:    "tokyo-night",
		},
		{
			name:    "an unset light slot takes the shipped default",
			setting: theme.Setting{Dark: "nord"},
			slot:    theme.SlotLight,
			want:    theme.DefaultLightSlug,
		},
		{
			name:    "an unset dark slot takes the shipped default",
			setting: theme.Setting{Light: "nord"},
			slot:    theme.SlotDark,
			want:    theme.DefaultDarkSlug,
		},
		{
			name:    "an unset constant has no default to substitute",
			setting: theme.Setting{},
			slot:    theme.SlotConstant,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.setting.Slug(tt.slot); got != tt.want {
				t.Errorf("Slug(%v) = %q, want %q", tt.slot, got, tt.want)
			}
		})
	}
}

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

func TestInForceKeys_UnsetSlotIsNeverInForce(t *testing.T) {
	setting, _ := theme.ResolveSetting(theme.RawKeys{Dark: "gruv"})
	if setting.Light != theme.DefaultLightSlug {
		t.Fatalf("Setting.Light = %q, want the substituted default %q — the assertion below would be vacuous", setting.Light, theme.DefaultLightSlug)
	}

	got := theme.InForceKeys(theme.RawKeys{Dark: "gruv"})

	want := []theme.InForceKey{{Value: "gruv", Slot: theme.SlotDark}}
	if !slices.Equal(got, want) {
		t.Errorf("InForceKeys = %+v, want %+v — a value Portal substituted is not one the user set", got, want)
	}
}

func TestInForceKeys_AcceptsAlreadyResolvedKeys(t *testing.T) {
	asRead := theme.RawKeys{Light: "\x1b[31msolar\n", Dark: "gruv\t"}
	_, resolved := theme.ResolveSetting(asRead)

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

func TestRawKeysWithConstant_HoldsOnlyTheConstant(t *testing.T) {
	keys := theme.RawKeys{Theme: "nord-lee", Light: "solar", Dark: "gruv"}

	got := keys.WithConstant("latte")

	if want := (theme.RawKeys{Theme: "latte"}); got != want {
		t.Errorf("WithConstant over %+v = %+v, want %+v — a constant clears both slots", keys, got, want)
	}
}

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

func TestNewRawKeys_StripsControlFromEveryKey(t *testing.T) {
	got := theme.NewRawKeys("\x1b[31mnord\x1b[0m", "so\tlar", "gruv\n")

	if want := (theme.RawKeys{Theme: "nord", Light: "solar", Dark: "gruv"}); got != want {
		t.Errorf("NewRawKeys = %+v, want %+v", got, want)
	}
}

func TestNewRawKeys_ControlOnlyValueStripsToEmpty(t *testing.T) {
	keys := theme.NewRawKeys("\x1b[31m\n\t", "\r", "\x1b[0m")

	if want := (theme.RawKeys{}); keys != want {
		t.Errorf("NewRawKeys = %+v, want %+v — a control-only value is unset", keys, want)
	}

	setting, raw := theme.ResolveSetting(keys)

	if setting.IsConstant {
		t.Errorf("IsConstant = true (Constant %q), want false — a value that strips to empty is unset, not a constant", setting.Constant)
	}
	if setting.Light != theme.DefaultLightSlug || setting.Dark != theme.DefaultDarkSlug {
		t.Errorf("Setting slots = {light %q, dark %q}, want the shipped pair {%q, %q}", setting.Light, setting.Dark, theme.DefaultLightSlug, theme.DefaultDarkSlug)
	}
	if want := (theme.RawKeys{}); raw != want {
		t.Errorf("RawKeys = %+v, want %+v", raw, want)
	}
}

func TestNewRawKeys_IsIdempotent(t *testing.T) {
	once := theme.NewRawKeys("\x1b[31mnord", "so\tlar", "gruv\n")

	if twice := theme.NewRawKeys(once.Theme, once.Light, once.Dark); twice != once {
		t.Errorf("NewRawKeys over the stripped %+v = %+v, want it unchanged", once, twice)
	}
}

func TestResolveSetting_StripsKeysItIsHandedUnstripped(t *testing.T) {
	asRead := theme.RawKeys{Light: "so\tlar", Dark: "\x1b[31mgruv\n"}

	setting, raw := theme.ResolveSetting(asRead)

	if want := (theme.RawKeys{Light: "solar", Dark: "gruv"}); raw != want {
		t.Errorf("RawKeys = %+v, want the stripped %+v", raw, want)
	}
	if setting.Light != "solar" || setting.Dark != "gruv" {
		t.Errorf("Setting slots = {light %q, dark %q}, want the stripped {%q, %q}", setting.Light, setting.Dark, "solar", "gruv")
	}
}

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

func settingSource(t *testing.T) sourceguardtest.ParsedSource {
	t.Helper()

	for _, source := range sourceguardtest.ParsePackageSources(t, ".", false) {
		if filepath.Base(source.Path) == "setting.go" {
			return source
		}
	}
	t.Fatal("setting.go not found among internal/theme's production sources")
	return sourceguardtest.ParsedSource{}
}

type fieldKind struct {
	name string
	kind reflect.Kind
}

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

func assertSingleLine(t *testing.T, what, value string) {
	t.Helper()

	if strings.ContainsFunc(value, unicode.IsControl) {
		t.Errorf("%s = %q still carries a control character — the value must render as one line", what, value)
	}
}

func TestInForceKeys_NoKeySetIsEmptyNotNil(t *testing.T) {
	got := theme.InForceKeys(theme.RawKeys{})

	if got == nil {
		t.Errorf("InForceKeys over unset keys = nil, want an empty slice")
	}
	if len(got) != 0 {
		t.Errorf("InForceKeys over unset keys = %+v, want no keys — nothing the user set is in force", got)
	}
}
