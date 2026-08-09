// Tests in this file seed prefs.json through t.Setenv, so they MUST NOT use
// t.Parallel.
//
// They own the ONE claim neither surface's own suite can make: the theme panel's
// persisted rows and doctor's persisted lines name the same slugs for the same
// prefs.json. Each surface is pinned against hand-written expectations elsewhere
// — the union's in internal/theme, doctor's in doctor_persisted_theme_test.go —
// and a table pinning one of them cannot notice the other drifting away from it.
//
// It is sited in cmd because this is the package that can reach BOTH: the
// assembler that builds the panel's row model, and doctor's persisted producer.
package cmd

import (
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// inForceKeyShapes is the shared table both surfaces are driven over: every
// shape prefs.json's three theme keys can take, with the slugs each surface owes
// the user a report about.
//
// `ghost`, `phantom` and `spectre` are deliberately unresolvable and `../evil`
// is deliberately illegal, so a shape that should report something really does —
// a table of resolvable slugs would agree on the empty set everywhere and prove
// nothing.
//
// The expectations are SORTED rather than in either surface's own order, and the
// comparison below sorts both sides for the same reason: the panel's rows arrive
// alphabetically (the union's display order) while doctor's lines arrive in slot
// order, and each of those orders is pinned by the surface that owns it. What
// the two must agree on is WHICH slugs are reported, not in what sequence.
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

// TestThemeKeysInForce_PanelAndDoctorReportTheSameSlugs: it reports one slug set
// on both surfaces — the panel's persisted rows and doctor's persisted lines.
//
// The union rule's "one slug is one row" and the pinned copy's "one slug is one line" are two
// renderings of ONE rule about which persisted keys Portal is reading: the constant-or-pair
// rule's `theme`-wins tiebreak, then only the slots the user actually SET, then two
// slots naming one value collapsed to one report. A panel listing a row doctor
// does not report — or the reverse — is the failure this pins, and no
// single-surface table can see it.
//
// Both surfaces are driven over the SAME themes directory, so a divergence can
// only come from the selection rule rather than from what is on disk.
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

// persistedRowSlugs lists what the panel union contributes for these keys: the
// rows prefs.json produced that nothing else in the list answers to, by the label
// each row is listed under — which is the raw value for a row whose value yields
// no slug at all.
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

// persistedAdvisorySlugs lists the slugs doctor's persisted producer reports for
// these keys, read from each advisory's own identity field rather than parsed
// back out of its rendered line.
func persistedAdvisorySlugs(t *testing.T, themesDir string, keys theme.RawKeys) []string {
	t.Helper()

	deps := persistedThemeDeps(t, prefsJSONForKeys(t, keys), themesDir)

	var slugs []string
	for _, a := range persistedThemeAdvisories(deps, theme.NewSilentLoader()) {
		slugs = append(slugs, a.slug)
	}
	slices.Sort(slugs)
	return slugs
}

// prefsJSONForKeys renders the prefs.json a set of raw keys came from, so one
// table shape drives the panel (which takes the keys directly) and doctor (which
// reads them back off disk). An unset key is ABSENT from the file rather than
// present-and-empty, which is the shape prefs itself writes.
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

// TestThemeKeysInForce_TableIsNotVacuous fails when the shared table has stopped
// exercising the rule: two silent surfaces agree on everything, so the parity
// above is only an assertion while the table expects reports — including one for
// a value that is not a legal slug, which is what makes the collapse's keying on
// the persisted VALUE observable.
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
