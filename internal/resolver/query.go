package resolver

import (
	"errors"
	"fmt"
	"os"
	"slices"
)

// SessionLister returns the user-visible (leading-underscore-filtered) set of
// running tmux session names. An error is treated as "no sessions".
type SessionLister interface {
	ListSessionNames() ([]string, error)
}

// AliasLookup retrieves the path for an alias key and enumerates the finite
// alias-key namespace that glob expansion runs against.
type AliasLookup interface {
	Get(name string) (string, bool)
	Keys() []string
}

// ZoxideQuerier queries zoxide for a directory matching the given terms.
type ZoxideQuerier interface {
	Query(terms string) (string, error)
}

// DirValidator checks whether a directory exists on disk.
type DirValidator interface {
	Exists(path string) bool
}

// QueryResult is the interface for resolution outcomes.
type QueryResult interface {
	queryResult()
}

// PathResult is a directory-domain hit: a resolved path that mints a session.
type PathResult struct {
	Path   string
	Domain Domain
}

func (*PathResult) queryResult() {}

// SessionResult is a session-domain hit: an existing session to attach. Domain
// is DomainSession for an exact-name hit, DomainGlob for a glob match.
type SessionResult struct {
	Name   string
	Domain Domain
}

func (*SessionResult) queryResult() {}

// MissResult is a total miss across every domain. The caller hard-fails on it;
// there is no implicit picker fallback.
type MissResult struct {
	Target string
}

func (*MissResult) queryResult() {}

// DirNotFoundError reports that a resolved directory does not exist on disk.
type DirNotFoundError struct {
	Path string
}

func (e *DirNotFoundError) Error() string {
	return fmt.Sprintf("Directory not found: %s", e.Path)
}

// OSDirValidator checks directory existence using the real filesystem.
type OSDirValidator struct{}

func (v *OSDirValidator) Exists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// QueryResolver applies the resolution chain: exact session-name match, path
// detection, alias lookup, zoxide query, then a total miss.
type QueryResolver struct {
	sessions     SessionLister
	aliases      AliasLookup
	zoxide       ZoxideQuerier
	dirValidator DirValidator
}

func NewQueryResolver(sessions SessionLister, aliases AliasLookup, zoxide ZoxideQuerier, dirValidator DirValidator) *QueryResolver {
	return &QueryResolver{
		sessions:     sessions,
		aliases:      aliases,
		zoxide:       zoxide,
		dirValidator: dirValidator,
	}
}

func (qr *QueryResolver) isExactSession(query string) bool {
	names, err := qr.sessions.ListSessionNames()
	if err != nil {
		return false
	}
	return slices.Contains(names, query)
}

// Resolve applies the single-target resolution chain in precedence order: exact
// session name → path → alias → zoxide → total miss (the caller hard-fails, with
// no picker fallback). It does not expand globs: a glob matches nothing in the
// chain and falls through to a miss rather than silently taking a first match.
func (qr *QueryResolver) Resolve(query string) (QueryResult, error) {
	if qr.isExactSession(query) {
		return &SessionResult{Name: query, Domain: DomainSession}, nil
	}

	if IsPathArgument(query) {
		resolved, err := ResolvePath(query)
		if err != nil {
			return nil, err
		}
		return &PathResult{Path: resolved, Domain: DomainPath}, nil
	}

	if path, ok := qr.aliases.Get(query); ok {
		return qr.validatedPath(path, DomainAlias)
	}

	// A zoxide error (not installed, no match) falls through to the miss tail;
	// only the -z pin surfaces it.
	if path, err := qr.zoxide.Query(query); err == nil {
		return qr.validatedPath(path, DomainZoxide)
	}

	return &MissResult{Target: query}, nil
}

func expandSessionGlobAll(pattern string, names []string) []QueryResult {
	matches := MatchGlob(pattern, names)
	if len(matches) == 0 {
		return []QueryResult{&MissResult{Target: pattern}}
	}
	results := make([]QueryResult, 0, len(matches))
	for _, m := range matches {
		results = append(results, &SessionResult{Name: m, Domain: DomainGlob})
	}
	return results
}

// ResolveBareAll is the multi-target form of Resolve: a glob expands to one
// result per matching session, and every failure is collected as a *MissResult
// so pre-flight reports all of them at once. The returned error is always nil.
func (qr *QueryResolver) ResolveBareAll(query string) ([]QueryResult, error) {
	if HasGlobMeta(query) {
		names, _ := qr.sessions.ListSessionNames()
		return expandSessionGlobAll(query, names), nil
	}

	r, err := qr.Resolve(query)
	if err != nil {
		return []QueryResult{&MissResult{Target: query}}, nil
	}
	return []QueryResult{r}, nil
}

// ResolveSessionPinAll is the multi-target form of ResolveSessionPin: a glob
// expands over the session set and a miss is collected as a *MissResult instead
// of erroring. The returned error is always nil.
func (qr *QueryResolver) ResolveSessionPinAll(query string) ([]QueryResult, error) {
	if HasGlobMeta(query) {
		names, _ := qr.sessions.ListSessionNames()
		return expandSessionGlobAll(query, names), nil
	}

	if qr.isExactSession(query) {
		return []QueryResult{&SessionResult{Name: query, Domain: DomainSession}}, nil
	}
	return []QueryResult{&MissResult{Target: query}}, nil
}

// ResolveAliasPinAll is the multi-target form of ResolveAliasPin: a glob expands
// over the alias-key namespace, and a failure is collected as a *MissResult —
// for a glob, under the matched key, so surviving keys still resolve. The
// returned error is always nil.
func (qr *QueryResolver) ResolveAliasPinAll(value string) ([]QueryResult, error) {
	if HasGlobMeta(value) {
		matches := MatchGlob(value, qr.aliases.Keys())
		if len(matches) == 0 {
			return []QueryResult{&MissResult{Target: value}}, nil
		}
		results := make([]QueryResult, 0, len(matches))
		for _, k := range matches {
			// matches are drawn from Keys(), so Get always finds the key.
			path, _ := qr.aliases.Get(k)
			r, err := qr.validatedPath(path, DomainAlias)
			if err != nil {
				results = append(results, &MissResult{Target: k})
				continue
			}
			results = append(results, r)
		}
		return results, nil
	}

	path, ok := qr.aliases.Get(value)
	if !ok {
		return []QueryResult{&MissResult{Target: value}}, nil
	}
	r, err := qr.validatedPath(path, DomainAlias)
	if err != nil {
		return []QueryResult{&MissResult{Target: value}}, nil
	}
	return []QueryResult{r}, nil
}

// ResolveSessionPin resolves query in the session domain only, by exact name:
// no aliases, no zoxide, no filesystem, no minting, no picker fallback. It does
// not expand globs — a glob is never a literal name, so it hard-fails rather
// than taking a first match.
func (qr *QueryResolver) ResolveSessionPin(query string) (QueryResult, error) {
	if qr.isExactSession(query) {
		return &SessionResult{Name: query, Domain: DomainSession}, nil
	}

	return nil, fmt.Errorf("No session found: %s", query) //nolint:staticcheck // user-facing message per spec
}

// ResolvePathPin resolves dir in the path domain only, and always mints. It
// stats the literal path, so a directory whose name contains glob
// metacharacters is reachable here, unlike the same value as a bare positional.
func (qr *QueryResolver) ResolvePathPin(dir string) (QueryResult, error) {
	resolved, err := ResolvePath(dir)
	if err != nil {
		return nil, err
	}
	return &PathResult{Path: resolved, Domain: DomainPath}, nil
}

// ResolveAliasPin resolves value in the alias domain only, by exact key, so an
// alias shadowed by a same-named session stays reachable. It does not expand
// globs. An unknown key hard-fails, a gone directory with *DirNotFoundError.
func (qr *QueryResolver) ResolveAliasPin(value string) (QueryResult, error) {
	path, ok := qr.aliases.Get(value)
	if !ok {
		return nil, unknownAliasError(value)
	}
	return qr.validatedPath(path, DomainAlias)
}

// ResolveZoxidePin resolves query in the zoxide domain only. Unlike the bare
// chain, which swallows zoxide failures, it returns ErrZoxideNotInstalled
// verbatim; any other failure hard-fails.
func (qr *QueryResolver) ResolveZoxidePin(query string) (QueryResult, error) {
	path, err := qr.zoxide.Query(query)
	if err != nil {
		if errors.Is(err, ErrZoxideNotInstalled) {
			return nil, ErrZoxideNotInstalled
		}
		return nil, fmt.Errorf("No zoxide match for: %s", query) //nolint:staticcheck // user-facing message per spec
	}
	return qr.validatedPath(path, DomainZoxide)
}

func unknownAliasError(value string) error {
	return fmt.Errorf("No alias found: %s", value) //nolint:staticcheck // user-facing message per spec
}

func (qr *QueryResolver) validatedPath(path string, domain Domain) (QueryResult, error) {
	if !qr.dirValidator.Exists(path) {
		return nil, &DirNotFoundError{Path: path}
	}
	return &PathResult{Path: path, Domain: domain}, nil
}
