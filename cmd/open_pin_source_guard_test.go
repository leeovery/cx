package cmd

// Drift guard for the open domain-pin set: the pin flag names are declared once
// in openDomainPinFlags and consumed by both the exclusivity guard and the RunE
// dispatch loop, which must stay in lockstep.

import (
	"slices"
	"testing"
)

func TestPinResolversKeysCoveredByFlagList(t *testing.T) {
	for flag := range pinResolvers {
		if !slices.Contains(openDomainPinFlags, flag) {
			t.Errorf("pinResolvers has a resolver for %q but openDomainPinFlags omits it — anyOpenDomainPin iterates openDomainPinFlags and would miss this pin; add %q to openDomainPinFlags", flag, flag)
		}
	}
}

func TestFlagListFullyResolved(t *testing.T) {
	for _, flag := range openDomainPinFlags {
		if _, ok := pinResolvers[flag]; !ok {
			t.Errorf("openDomainPinFlags lists %q but pinResolvers has no resolver for it — dispatching this pin would call a nil resolver; add it to pinResolvers", flag)
		}
	}
}

func TestOpenDomainPinFlagsAreRegistered(t *testing.T) {
	for _, flag := range openDomainPinFlags {
		if openCmd.Flags().Lookup(flag) == nil {
			t.Errorf("openDomainPinFlags lists %q but openCmd registers no such flag — cmd.Flags().Changed(%q) is always false, silently disabling the exclusivity guard for this pin", flag, flag)
		}
	}
}

func TestAnyOpenDomainPinCoversEveryPin(t *testing.T) {
	for _, flag := range openDomainPinFlags {
		c := openProbeCmdWithFlags()
		if err := c.Flags().Set(flag, "x"); err != nil {
			t.Fatalf("set --%s: %v", flag, err)
		}
		if !anyOpenDomainPin(c) {
			t.Errorf("anyOpenDomainPin = false with --%s set, want true — the exclusivity guard must cover every declared pin", flag)
		}
	}
}
