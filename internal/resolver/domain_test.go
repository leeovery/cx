package resolver_test

import (
	"testing"

	"github.com/leeovery/portal/internal/resolver"
)

func TestDomain_String(t *testing.T) {
	cases := []struct {
		domain   resolver.Domain
		expected string
	}{
		{resolver.DomainBare, "bare"},
		{resolver.DomainSession, "session"},
		{resolver.DomainPath, "path"},
		{resolver.DomainAlias, "alias"},
		{resolver.DomainZoxide, "zoxide"},
		{resolver.DomainGlob, "glob"},
		{resolver.DomainMiss, "miss"},
	}
	for _, c := range cases {
		if got := c.domain.String(); got != c.expected {
			t.Errorf("Domain(%q).String() = %q, want %q", string(c.domain), got, c.expected)
		}
	}
}
