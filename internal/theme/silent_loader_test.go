package theme_test

import (
	"maps"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

func TestNewSilentLoader_JudgesIdenticallyAndWritesNothing(t *testing.T) {
	sink := logtest.Install(t)
	dir := stageMixedVerdictDir(t)

	silentEntries, silentDirRejection := theme.NewSilentLoader().Enumerate(dir)
	if got := sink.Records(); len(got) != 0 {
		t.Fatalf("the silent loader emitted %d records, want 0: %+v", len(got), got)
	}

	loudEntries, loudDirRejection := theme.NewLoader(theme.NewEventLogger(log.For(themeComponent))).Enumerate(dir)
	if len(sink.Records()) == 0 {
		t.Fatal("the emitting loader wrote nothing over the same directory — the silence assertion above is vacuous")
	}

	if silentDirRejection != nil || loudDirRejection != nil {
		t.Fatalf("directory verdicts = (%v, %v), want neither loader to reject a readable directory", silentDirRejection, loudDirRejection)
	}

	silent, loud := reasonsByFilename(silentEntries), reasonsByFilename(loudEntries)
	if want := stagedVerdicts(); !maps.Equal(loud, want) {
		t.Fatalf("the staged directory was judged %v, want %v — the fixture no longer spans the ladder, so the parity assertion below covers less than it reads", loud, want)
	}
	if !maps.Equal(silent, loud) {
		t.Errorf("silent loader verdicts = %v, want the emitting loader's %v", silent, loud)
	}
}

func TestNewSilentLoader_ReservesEveryBuiltinSlug(t *testing.T) {
	loader := theme.NewSilentLoader()

	for _, slug := range requireBuiltinSlugs(t) {
		t.Run(slug, func(t *testing.T) {
			path := themetest.Write(t, t.TempDir(), slug+".theme", themetest.Lines())

			got, rejection := loader.LoadFile(path)

			requireLoadRejection(t, got, rejection, theme.ReasonReservedName, "")
		})
	}
}

func TestNewLoader_NilSeamPanics(t *testing.T) {
	defer func() {
		raised := recover()
		if raised == nil {
			t.Fatal("NewLoader(nil) returned a loader, want a panic naming the silent constructor")
		}
		if message, ok := raised.(string); !ok || !strings.Contains(message, "NewSilentLoader") {
			t.Errorf("panic value = %v, want a string naming NewSilentLoader", raised)
		}
	}()

	theme.NewLoader(nil)
}

func stagedVerdicts() map[string]theme.Reason {
	return map[string]theme.Reason{
		validThemeBase:            "",
		badNameThemeBase:          theme.ReasonBadName,
		badColourThemeBase:        theme.ReasonBadColour,
		tokyoNightSlug + ".theme": theme.ReasonReservedName,
	}
}

const (
	validThemeBase     = "mine.theme"
	badNameThemeBase   = "Bad_Name.theme"
	badColourThemeBase = "broken.theme"
)

func stageMixedVerdictDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	themetest.Write(t, dir, validThemeBase, themetest.Lines())
	themetest.Write(t, dir, badNameThemeBase, themetest.Lines())
	themetest.WriteWithCanvas(t, dir, badColourThemeBase, brokenCanvasValue)
	themetest.Write(t, dir, tokyoNightSlug+".theme", themetest.Lines())
	return dir
}

func reasonsByFilename(entries []theme.Entry) map[string]theme.Reason {
	reasons := make(map[string]theme.Reason, len(entries))
	for _, entry := range entries {
		if entry.Rejection == nil {
			reasons[entry.Filename] = ""
			continue
		}
		reasons[entry.Filename] = entry.Rejection.Reason
	}
	return reasons
}
