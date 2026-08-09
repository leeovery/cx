package theme_test

import (
	"maps"
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

// TestNewSilentLoader_JudgesIdenticallyAndWritesNothing pins both halves of the
// diagnose-shaped constructor: it runs the SAME ladder an emitting loader runs —
// the derived reserved set included — and writes not one record while doing it.
//
// Neither half stands alone. Silence is satisfied by the zero-value Loader,
// which reserves nothing and would let a user's file shadow the built-in Portal
// falls back to; identical verdicts are satisfied by the emitting loader itself.
// The emitting run doubles as the positive control, so the silence assertion
// cannot pass for want of anything to emit over.
func TestNewSilentLoader_JudgesIdenticallyAndWritesNothing(t *testing.T) {
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)
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

// stagedVerdicts is the verdict stageMixedVerdictDir's directory must produce,
// file by file — the fixture's own claim, made checkable.
//
// It is asserted as a WHOLE map rather than as a spot check on the reserved rung,
// because the way this fixture degrades is by silently holding fewer files than
// it appears to: a staged name that collides with another entry leaves the
// enumeration a rung short, and a parity comparison between two loaders passes
// just as happily over three files as over four.
func stagedVerdicts() map[string]theme.Reason {
	return map[string]theme.Reason{
		validThemeBase:            "",
		badNameThemeBase:          theme.ReasonBadName,
		badColourThemeBase:        theme.ReasonBadColour,
		tokyoNightSlug + ".theme": theme.ReasonReservedName,
	}
}

// The four files stageMixedVerdictDir stages, as their bases. No two differ only
// by CASE: the temp directory may sit on a case-insensitive volume, where a
// case-varying twin is the same file — the second write lands on the first, the
// entry keeps the original casing, and the directory quietly holds one rung fewer
// than it was written to hold. The bad name is therefore made bad by a character
// the slug charset rejects rather than by capitalising a neighbour.
const (
	validThemeBase     = "mine.theme"
	badNameThemeBase   = "Bad_Name.theme"
	badColourThemeBase = "broken.theme"
)

// stageMixedVerdictDir writes a themes directory spanning the ladder's outcomes:
// a valid drop-in, a bad name, a bad colour, and a filename taking a built-in's
// slug — the last being what separates a loader carrying the derived reserved
// set from a hand-assembled one.
func stageMixedVerdictDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	themetest.Write(t, dir, validThemeBase, themetest.Lines())
	themetest.Write(t, dir, badNameThemeBase, themetest.Lines())
	themetest.Write(t, dir, badColourThemeBase, themetest.WithValue(themetest.Lines(), "canvas", brokenCanvasValue))
	themetest.Write(t, dir, tokyoNightSlug+".theme", themetest.Lines())
	return dir
}

// reasonsByFilename renders one enumeration as filename → the reason its entry
// was rejected for, with the empty reason standing for an accepted file.
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
