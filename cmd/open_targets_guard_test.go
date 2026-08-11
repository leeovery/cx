package cmd

// openTargetPins is structurally decoupled from openCmd's live cobra flag
// set: a value-taking flag added to one but not the other would be treated
// as arity-0, misrouting its value as a bare positional target.

import (
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/resolver"
	"github.com/spf13/pflag"
)

// A pflag takes a value unless it is a bool or carries a non-empty
// NoOptDefVal, and orderedOpenTargets correctly skips those, so they report
// nothing missing.
func valueTakingFlagMissingPins(f *pflag.Flag, pins map[string]resolver.Domain) []string {
	if f.Value.Type() == "bool" || f.NoOptDefVal != "" {
		return nil
	}
	var missing []string
	if _, ok := pins["--"+f.Name]; !ok {
		missing = append(missing, "--"+f.Name)
	}
	if f.Shorthand != "" {
		if _, ok := pins["-"+f.Shorthand]; !ok {
			missing = append(missing, "-"+f.Shorthand)
		}
	}
	return missing
}

func TestValueTakingFlagMissingPins_DetectsDrift(t *testing.T) {
	// A crafted flag set — a pflag.FlagSet cannot cleanly un-register a flag, so
	// the real openCmd is never mutated.
	fs := pflag.NewFlagSet("crafted", pflag.ContinueOnError)
	fs.StringP("zzz", "Z", "", "throwaway value-taking flag absent from openTargetPins")

	got := valueTakingFlagMissingPins(fs.Lookup("zzz"), openTargetPins)
	want := []string{"--zzz", "-Z"}
	if !slices.Equal(got, want) {
		t.Errorf("valueTakingFlagMissingPins(--zzz/-Z) = %#v, want %#v — a value-taking flag missing from openTargetPins must be flagged", got, want)
	}
}

func TestValueTakingFlagMissingPins_SkipsAndCovers(t *testing.T) {
	fs := pflag.NewFlagSet("crafted", pflag.ContinueOnError)
	fs.BoolP("verbose", "v", false, "bool flag — arity-0, correctly skipped")
	fs.String("opt", "", "optional-value flag — skipped via NoOptDefVal")
	fs.Lookup("opt").NoOptDefVal = "sentinel"
	fs.StringP("session", "s", "", "value-taking flag already present in openTargetPins")

	if got := valueTakingFlagMissingPins(fs.Lookup("verbose"), openTargetPins); got != nil {
		t.Errorf("bool flag --verbose should be skipped, got %#v", got)
	}
	if got := valueTakingFlagMissingPins(fs.Lookup("opt"), openTargetPins); got != nil {
		t.Errorf("optional-value flag --opt should be skipped, got %#v", got)
	}
	if got := valueTakingFlagMissingPins(fs.Lookup("session"), openTargetPins); got != nil {
		t.Errorf("fully-pinned flag --session/-s should report nothing missing, got %#v", got)
	}
}

func TestOpenTargetPinsCoverValueTakingFlags(t *testing.T) {
	openCmd.Flags().VisitAll(func(f *pflag.Flag) {
		for _, key := range valueTakingFlagMissingPins(f, openTargetPins) {
			t.Errorf("openCmd flag --%s is value-taking but %q is absent from openTargetPins — orderedOpenTargets would misroute its value as a positional target; add it to openTargetPins", f.Name, key)
		}
	})
}

// The reverse guard: a stale pin naming a flag cobra no longer accepts would
// make orderedOpenTargets consume the following token as its value.
func TestOpenTargetPinsKeysAreLiveFlags(t *testing.T) {
	for key := range openTargetPins {
		var f *pflag.Flag
		switch {
		case strings.HasPrefix(key, "--"):
			f = openCmd.Flags().Lookup(strings.TrimPrefix(key, "--"))
		case strings.HasPrefix(key, "-"):
			f = openCmd.Flags().ShorthandLookup(strings.TrimPrefix(key, "-"))
		}
		if f == nil {
			t.Errorf("openTargetPins key %q does not name a live openCmd flag — a stale pin misroutes argv scanning; remove it or restore the flag", key)
		}
	}
}
