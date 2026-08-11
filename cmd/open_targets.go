package cmd

import (
	"strings"

	"github.com/leeovery/portal/internal/resolver"
)

// Target is one element of the ordered open target-set union: a value plus the
// domain it resolves under. DomainBare marks a positional that runs the full
// precedence chain.
type Target struct {
	Value  string
	Domain resolver.Domain
}

// openTargetPins is a hand-maintained mirror of openCmd's live flag set: a new
// value-taking flag needs an entry here (long form and any short form), or
// orderedOpenTargets treats it as arity-0 and misroutes its value as a bare
// positional. An excluded flag maps to the empty domain — its value is consumed
// off the argv but never emitted as a target.
//
// Bundled value shorthands (`-sf`) are deliberately out of contract: absent from
// this map, such a token is skipped rather than attributed a value. That
// divergence from cobra's bundling is intended.
var openTargetPins = map[string]resolver.Domain{
	"-s": resolver.DomainSession, "--session": resolver.DomainSession,
	"-p": resolver.DomainPath, "--path": resolver.DomainPath,
	"-z": resolver.DomainZoxide, "--zoxide": resolver.DomainZoxide,
	"-a": resolver.DomainAlias, "--alias": resolver.DomainAlias,
	"-e": "", "--exec": "",
	"-f": "", "--filter": "",
	"--ack": "",
}

// orderedOpenTargets recovers the left-to-right union of positionals and pin
// occurrences from a raw open argv slice. cobra's StringP collapses repeated
// same-flag values and splits positionals from flags, losing the interleaved
// order and repeats this scan preserves.
//
// It is a pure classifier, not a validator: cobra already accepted the argv, so
// no token is rejected, only attributed. Repeats are honoured, never deduped.
func orderedOpenTargets(args []string) []Target {
	var targets []Target
	for i := 0; i < len(args); i++ {
		tok := args[i]

		// Everything after a bare `--` is command-passthrough, never a target.
		if tok == "--" {
			break
		}

		// A lone "-" is a positional, not a flag; no positional begins with "-".
		if !strings.HasPrefix(tok, "-") || tok == "-" {
			targets = append(targets, Target{Value: tok, Domain: resolver.DomainBare})
			continue
		}

		// The equals form (-s=api) splits on the first '='; the space form leaves
		// value empty until the next token.
		name, value, hasInlineValue := strings.Cut(tok, "=")

		domain, known := openTargetPins[name]
		if !known {
			// An unmodelled flag (boolean, --help, …) has unknown arity, so skip it
			// without consuming a following token as its value.
			continue
		}

		// Consumed even for excluded flags, so the value is never re-examined as a
		// positional; only the emission is suppressed.
		if !hasInlineValue && i+1 < len(args) {
			value = args[i+1]
			i++
		}

		if domain != "" {
			targets = append(targets, Target{Value: value, Domain: domain})
		}
	}
	return targets
}
