package resolver

// Domain discriminates an open target and a resolver result. It is a closed
// constant set rather than a bare string so a domain added to one routing
// vocabulary but not another is a compile error, not a silent misroute.
//
// The underlying strings of the result domains are the decision-log domain attr
// values verbatim. DomainBare and DomainGlob are routing-only: those targets are
// deterministic and emit no log line.
type Domain string

const (
	// DomainBare is a positional target running the full precedence chain.
	DomainBare Domain = "bare"
	// DomainSession is an existing-session (attach) hit.
	DomainSession Domain = "session"
	// DomainPath is a directory-path (mint) hit.
	DomainPath Domain = "path"
	// DomainAlias is an alias-key (mint) hit.
	DomainAlias Domain = "alias"
	// DomainZoxide is a zoxide-query (mint) hit.
	DomainZoxide Domain = "zoxide"
	// DomainGlob is a session-glob expansion match (attach).
	DomainGlob Domain = "glob"
	// DomainMiss is a total miss across every domain.
	DomainMiss Domain = "miss"
)

func (d Domain) String() string {
	return string(d)
}
