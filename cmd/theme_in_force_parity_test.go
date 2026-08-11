package cmd

import (
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// The slugs are deliberately unresolvable (a table of resolvable slugs would
// agree on the empty set everywhere). Expectations are sorted and both sides
// sort: the panel's rows arrive alphabetically, doctor's lines in slot order,
// and each order is pinned by the surface that owns it.
var inForceKeyShapes = []struct {
	name string
	keys theme.RawKeys
	want []string
}{
	{name: "a constant alone", keys: theme.RawKeys{Theme: "ghost"}, want: []string{"ghost"}},
	{name: "a constant beside two slots", keys: theme.RawKeys{Theme: "ghost", Light: "phantom", Dark: "spectre"}, want: []string{"ghost"}},
	{name: "the light slot alone", keys: theme.RawKeys{Light: "ghost"}, want: []string{"ghost"}},
	{name: "the dark slot alone", keys: theme.RawKeys{Dark: "ghost"}, want: []string{"ghost"}},
	{name: "two slots naming different slugs", keys: theme.RawKeys{Light: "phantom", Dark: "ghost"}, want: []string{"ghost", "phantom"}},
	{name: "two slots naming one slug", keys: theme.RawKeys{Light: "ghost", Dark: "ghost"}, want: []string{"ghost"}},
	{name: "two slots naming one illegal string", keys: theme.RawKeys{Light: "../evil", Dark: "../evil"}, want: []string{"../evil"}},
	{name: "an unresolvable slot beside a resolvable one", keys: theme.RawKeys{Light: theme.DefaultLightSlug, Dark: "ghost"}, want: []string{"ghost"}},
	{name: "no keys at all", keys: theme.RawKeys{}, want: nil},
}

func TestThemeKeysInForce_PanelAndDoctorReportTheSameSlugs(t *testing.T) {
	for _, slug := range []string{"ghost", "phantom", "spectre"} {
		requireDropInSlug(t, slug)
	}

	for _, shape := range inForceKeyShapes {
		t.Run(shape.name, func(t *testing.T) {
			dir := t.TempDir()

			panelSlugs := persistedRowSlugs(dir, shape.keys)
			doctorSlugs := persistedAdvisorySlugs(t, dir, shape.keys)

			if !slices.Equal(panelSlugs, doctorSlugs) {
				t.Errorf("the panel reports %v and doctor reports %v; the two surfaces read one rule about which keys are in force", panelSlugs, doctorSlugs)
			}
			if !slices.Equal(panelSlugs, shape.want) {
				t.Errorf("the panel reports %v, want %v", panelSlugs, shape.want)
			}
			if !slices.Equal(doctorSlugs, shape.want) {
				t.Errorf("doctor reports %v, want %v", doctorSlugs, shape.want)
			}
		})
	}
}

func persistedRowSlugs(themesDir string, keys theme.RawKeys) []string {
	_, union := theme.Assembler{Loader: theme.NewSilentLoader()}.Open(themesDir, keys)

	var slugs []string
	for _, row := range union.Rows {
		if row.Source == theme.SourcePersisted {
			slugs = append(slugs, row.Label())
		}
	}
	slices.Sort(slugs)
	return slugs
}

func persistedAdvisorySlugs(t *testing.T, themesDir string, keys theme.RawKeys) []string {
	t.Helper()

	deps := persistedThemeDeps(t, prefsJSONForKeys(t, keys), themesDir)

	var slugs []string
	for _, a := range persistedAdvisoriesUnder(t, deps, theme.NewSilentLoader()) {
		slugs = append(slugs, a.slug)
	}
	slices.Sort(slugs)
	return slugs
}

// An unset key is absent from the file rather than present-and-empty — the
// shape prefs itself writes.
func prefsJSONForKeys(t *testing.T, keys theme.RawKeys) string {
	t.Helper()

	fields := map[string]string{}
	for _, field := range []struct{ name, value string }{
		{name: "theme", value: keys.Theme},
		{name: "theme_light", value: keys.Light},
		{name: "theme_dark", value: keys.Dark},
	} {
		if field.value != "" {
			fields[field.name] = field.value
		}
	}
	return prefsJSONWith(t, fields)
}

// Two silent surfaces agree on everything, so the parity above is only an
// assertion while the table expects reports.
func TestThemeKeysInForce_TableIsNotVacuous(t *testing.T) {
	var reported, illegal []string
	for _, shape := range inForceKeyShapes {
		reported = append(reported, shape.want...)
		for _, value := range shape.want {
			if !theme.ValidSlug(value) {
				illegal = append(illegal, value)
			}
		}
	}

	if len(reported) == 0 {
		t.Fatal("no shape in the table expects a reported value; the parity assertions would hold over two silent surfaces")
	}
	if len(illegal) == 0 {
		t.Errorf("the table expects %v, none of which is an illegal value — a value yielding no slug is what proves the collapse keys on the persisted value", reported)
	}
}
