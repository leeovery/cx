package cmd

import (
	"testing"

	"github.com/leeovery/portal/internal/resolver"
)

func TestGlobExpandableDomain_TypedConstants(t *testing.T) {
	// These expand a glob against a finite Portal-owned namespace: bare
	// positionals over session names, -s over session names, -a over alias keys.
	for _, d := range []resolver.Domain{resolver.DomainBare, resolver.DomainSession, resolver.DomainAlias} {
		if !globExpandableDomain(d) {
			t.Errorf("globExpandableDomain(%q) = false, want true", d)
		}
	}

	// Every other domain, including the zero value, is a literal or
	// already-expanded value with nothing to expand against.
	for _, d := range []resolver.Domain{resolver.DomainPath, resolver.DomainZoxide, resolver.DomainGlob, resolver.DomainMiss, resolver.Domain("")} {
		if globExpandableDomain(d) {
			t.Errorf("globExpandableDomain(%q) = true, want false", d)
		}
	}
}
