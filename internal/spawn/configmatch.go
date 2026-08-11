package spawn

import "strings"

// A friendly alias is just a nicer spelling of a bundle-id family, so config
// matching stays on families.
var friendlyAliases = map[string]string{
	"ghostty": "com.mitchellh.ghostty*",
	"warp":    "dev.warp.Warp-*",
}

// Specificity tiers: an exact bundle id beats a named form (friendly alias or
// .app name), which beats a *-glob family.
const (
	tierGlob     = 1
	tierNamed    = 2
	tierBundleID = 3
)

// literals refines glob matches only; non-glob tiers are separated by tier
// alone and leave it zero.
type matchScore struct {
	tier     int
	literals int
}

func (s matchScore) better(o matchScore) bool {
	if s.tier != o.tier {
		return s.tier > o.tier
	}
	return s.literals > o.literals
}

// Go map iteration order is randomised, so a tie must not decide the winner: a
// residual exact tie keeps the lexicographically-smaller key.
func matchConfig(cfg TerminalsConfig, id Identity) (key string, entry TerminalEntry, ok bool) {
	var bestScore matchScore
	for k, e := range cfg {
		score, matched := scoreKey(k, id)
		if !matched {
			continue
		}
		switch {
		case !ok, score.better(bestScore):
			key, entry, bestScore, ok = k, e, score, true
		case !bestScore.better(score) && k < key:
			key, entry = k, e
		}
	}
	return key, entry, ok
}

// Branch order is load-bearing: a key containing "*" is always a glob first,
// never an exact or named match.
func scoreKey(key string, id Identity) (matchScore, bool) {
	if strings.Contains(key, "*") {
		if MatchesFamily(id.BundleID, key) {
			return matchScore{tier: tierGlob, literals: countLiterals(key)}, true
		}
		return matchScore{}, false
	}

	if key == id.BundleID {
		return matchScore{tier: tierBundleID}, true
	}

	if family, isAlias := friendlyAliases[key]; isAlias && MatchesFamily(id.BundleID, family) {
		return matchScore{tier: tierNamed}, true
	}

	if id.Name != "" && key == id.Name {
		return matchScore{tier: tierNamed}, true
	}

	return matchScore{}, false
}

func countLiterals(key string) int {
	n := 0
	for _, r := range key {
		if r != '*' {
			n++
		}
	}
	return n
}
