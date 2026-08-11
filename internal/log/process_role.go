package log

import "strings"

// The closed value space for the per-record process_role baseline attr.
//
// roleClean is unreachable — no invocation maps to it — and its switch arm in
// ResolveProcessRole is dead. Both stay: pruning a value out of the closed space
// is a log-taxonomy change, not a cleanup.
const (
	roleDaemon    = "daemon"
	roleHydrate   = "hydrate"
	roleHooksCLI  = "hooks_cli"
	roleClean     = "clean"
	roleTUI       = "tui"
	roleBootstrap = "bootstrap"
)

// ResolveProcessRole maps a process invocation's argument vector (os.Args[1:]) to
// one of the closed process_role values, defaulting to bootstrap. Flag tokens are
// ignored and the longest subcommand prefix wins.
//
// It is a lightweight argv inspection rather than a parse because Init runs from
// main before Cobra sees argv. It reads nothing global.
func ResolveProcessRole(args []string) string {
	path := subcommandPath(args)

	if len(path) >= 2 && path[0] == "state" {
		switch path[1] {
		case "daemon":
			return roleDaemon
		case "hydrate", "signal-hydrate":
			return roleHydrate
		}
	}

	if len(path) == 0 {
		// Bare `portal` prints help; it does not launch the picker. roleTUI is kept
		// here only so the closed role space stays stable.
		return roleTUI
	}

	switch path[0] {
	case "hook", "hooks":
		return roleHooksCLI
	// Dead arm, retained with roleClean: see the const block.
	case "clean":
		return roleClean
	case "open", "x":
		return roleTUI
	}

	return roleBootstrap
}

// subcommandPath drops every token beginning with "-", including the lone "-",
// preserving the order of the rest.
func subcommandPath(args []string) []string {
	path := make([]string, 0, len(args))
	for _, tok := range args {
		if strings.HasPrefix(tok, "-") {
			continue
		}
		path = append(path, tok)
	}
	return path
}
