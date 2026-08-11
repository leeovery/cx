package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/resolver"
	"github.com/leeovery/portal/internal/session"
	"github.com/leeovery/portal/internal/spawn"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tui"
	"github.com/spf13/cobra"
)

// resolveLogger binds the "resolve" log component once for the whole open
// command. openCmd.RunE emits exactly one INFO decision line per bare positional
// resolved through the guessing chain, so a confusing guess is reconstructable
// from portal.log. internal/resolver stays a pure, log-free library: the binding
// and emission live only here.
var resolveLogger = log.For("resolve")

// themeLogger binds the "theme" log component once for the whole cmd package,
// with `spawn` and `resolve` as direct precedent in the closed taxonomy.
//
// The component is legitimately emitted from more than one PACKAGE — the loader
// in internal/theme, the appearance translation and the theme persister here —
// because CLAUDE.md's rule is bind once per PACKAGE, which `spawn` and
// `bootstrap` already span several files under. This is that one binding for
// cmd: every theme seam constructed here is handed this logger, and nothing else
// in the package calls log.For("theme").
//
// Emission is the CALLER's decision, not the loader's: a real component logger
// goes where a theme is USED — TUI construction, the panel and the persister —
// while `portal doctor`, `portal theme export` and capturetool pass
// log.Discard() because the component records where a theme is used, never where
// one is diagnosed.
var themeLogger = log.For("theme")

// openTUIFunc is the function used to launch the TUI. Tests override it via
// t.Cleanup-restored assignment to capture arguments without launching the
// real Bubble Tea program.
var openTUIFunc = openTUI

// openPathFunc is the function used to open a session at a resolved path.
// Tests override it via t.Cleanup-restored assignment to capture the resolved
// path without performing real tmux create / exec hand-off (which would
// require a live attached tmux client and replace the test process via
// syscall.Exec).
var openPathFunc = openPath

// openSessionFunc is the function used to attach this terminal to an existing
// session (the session-domain outcome). Tests override it via t.Cleanup-restored
// assignment to capture the target without building a real connector.
var openSessionFunc = openSession

// openDeps holds injectable dependencies for the open command.
// When nil, real implementations are used.
var openDeps *OpenDeps

// OpenDeps allows injecting dependencies for testing.
type OpenDeps struct {
	SessionLister resolver.SessionLister
	AliasLookup   resolver.AliasLookup
	Zoxide        resolver.ZoxideQuerier
	DirValidator  resolver.DirValidator
	// AckWriter writes the @portal-spawn-<batch>-<token> confirmation marker for
	// a spawned window (the hidden --ack carrier). When nil, buildAckWriter falls
	// back to a real @portal-spawn- server-option channel over the shared tmux
	// client. See writeAckMarker.
	AckWriter spawn.AckWriter
	// ThemeLoader is the loader TUI construction resolves the persisted theme
	// setting through. When nil, buildThemeLoader builds the production one (the
	// embedded built-in set, this package's `theme` component logger).
	//
	// It exists for ONE state, and because that state is otherwise unreachable: a
	// binary whose embedded set cannot supply the theme a slot falls back to.
	// Injecting a loader carrying a BuiltinSource that omits or corrupts a
	// fallback slug — the seam theme.Loader declares for exactly this — is what
	// makes the fatal openTUI returns on that path a path something has run.
	ThemeLoader *theme.Loader
}

// SessionConnector connects the user to a tmux session.
// The implementation differs based on whether Portal is inside or outside tmux.
type SessionConnector interface {
	Connect(name string) error
}

// SwitchClienter defines the interface for switching tmux clients.
type SwitchClienter interface {
	SwitchClient(name string) error
}

// SwitchConnector connects to a session by issuing tmux switch-client.
// Used when Portal is running inside an existing tmux session.
type SwitchConnector struct {
	client SwitchClienter
}

// Connect switches the current tmux client to the named session.
func (sc *SwitchConnector) Connect(name string) error {
	return sc.client.SwitchClient(name)
}

// AttachConnector connects to a session by exec-ing tmux attach-session.
// Used when Portal is running outside tmux (bare shell).
//
// The execer and tmuxPath fields are optional injection seams for tests —
// they are exclusively for unit-test substitution to avoid the real
// syscall.Exec replacing the test process. When either is unset, Connect
// falls back to production defaults (realExecer + exec.LookPath("tmux")).
type AttachConnector struct {
	execer   execer
	tmuxPath string
}

// Connect replaces the current process with tmux attach-session.
//
// The exec'd argv is `tmux attach-session -t =<name>`:
//   - `=` prefixes the target so tmux uses exact-match resolution rather
//     than prefix match — uniform with HasSession / SelectWindow /
//     SelectPane / SwitchClient.
func (ac *AttachConnector) Connect(name string) error {
	tmuxPath := ac.tmuxPath
	if tmuxPath == "" {
		p, err := exec.LookPath("tmux")
		if err != nil {
			return fmt.Errorf("tmux not found: %w", err)
		}
		tmuxPath = p
	}
	ex := ac.execer
	if ex == nil {
		ex = &realExecer{}
	}

	// argv[0] is the "tmux" program name; argv[1:] is the tmux subcommand+flags.
	// We pass the full argv to both logExecHandoff (which strips argv[0] so args
	// renders "attach-session -t =<name>") and Exec.
	argv := []string{"tmux", "attach-session", "-t", "=" + name}

	// Exec-handoff marker, single-sourced via logExecHandoff and emitted
	// IMMEDIATELY before syscall.Exec (the terminal marker before the process
	// image is replaced).
	logExecHandoff(argv)

	return ex.Exec(tmuxPath, argv, os.Environ())
}

// buildSessionConnector returns the appropriate SessionConnector based on
// whether Portal is running inside an existing tmux session.
func buildSessionConnector(client *tmux.Client) SessionConnector {
	if tmux.InsideTmux() {
		return &SwitchConnector{client: client}
	}
	return &AttachConnector{}
}

// openSession connects this terminal to an existing named session — the
// session-domain outcome of resolution (Axiom 2: a session-domain hit attaches).
// buildSessionConnector selects switch-client (inside tmux) or exec attach
// (outside tmux); the "=" exact-match target is applied by the connector.
func openSession(cmd *cobra.Command, name string) error {
	return buildSessionConnector(tmuxClient(cmd)).Connect(name)
}

var openCmd = &cobra.Command{
	Use:   "open [targets…] [-- cmd args…]",
	Short: "Open portals to one or more targets, or launch the interactive picker",
	Long: `Open one or more portals, or launch the interactive session picker.

With no arguments, open launches the TUI picker so you can choose a destination.

A bare target is resolved through the precedence chain — exact session name → path
→ alias → zoxide query, first match wins. An exact session name attaches that
existing session; a path, alias, or zoxide match mints a new session there.

Domain pins skip the precedence chain and force a single domain:
  -s, --session   attach the named session or session glob (never mints)
  -p, --path      mint a new session at the given directory (must exist)
  -a, --alias     mint a new session at the given alias key or key glob
  -z, --zoxide    mint a new session at zoxide's best match

  -f, --filter    skip resolution and open the picker pre-filtered by <text>
                  (mutually exclusive with a target and every domain pin)

A command to run in a freshly minted session is scoped with -e/--exec or after a
-- separator; command scoping applies to mint outcomes only, never to an attach.

Passing two or more targets (or one glob that expands to several sessions) opens a
portal to each: this terminal becomes the first surface and the remaining N−1 open
in host-terminal windows.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		command, destination, err := parseCommandArgs(cmd, args)
		if err != nil {
			return err
		}

		// --ack <batch>:<token> is the hidden internal receipt flag: a spawned
		// host window writes the @portal-spawn-<batch>-<token>
		// server option as its last act before the attach/mint handoff. Validate the
		// value fast, at the very TOP of RunE — before the -f branch and the pin
		// blocks (which touch tmux via ListSessionNames) — so a malformed value is a
		// usage error (exit 2) before any tmux call. The best-effort write itself
		// rides both receivers via writeAckMarker in openResolved.
		if ackVal, _ := cmd.Flags().GetString("ack"); ackVal != "" {
			if _, _, ok := spawn.ParseSpawnAckFlag(ackVal); !ok {
				return NewUsageError("open: --ack must be <batch>:<token>")
			}
		}

		// -f/--filter is the sole non-composing flag: not a target, but a "skip
		// resolution, open the picker pre-filtered" redirect. Handled BEFORE
		// resolution AND before the pin dispatch blocks — it never routes through the
		// query resolver, so `open -f x -s y` is rejected rather than resolving the
		// pin. It is mutually exclusive with a positional target AND with every domain
		// pin (-s/-p/-z/-a); only the command (-e/--) may accompany it, specialising
		// the picker to filtered-Projects mode. An empty value is rejected,
		// mirroring the empty -e guard.
		if cmd.Flags().Changed("filter") {
			filterVal, _ := cmd.Flags().GetString("filter")
			if destination != "" || anyOpenDomainPin(cmd) {
				return NewUsageError("cannot use -f/--filter with a target or a domain pin (-s/-p/-z/-a)")
			}
			if filterVal == "" {
				return NewUsageError("-f/--filter value must not be empty")
			}
			return openTUIFunc(cmd, filterVal, command, serverWasStarted(cmd))
		}

		// Multi-target routing gate. Recover the ordered target set from
		// the RAW argv — cobra collapses repeated same-flag values (`open -s a -s b`)
		// and splits positionals from flags, losing the interleaved order and repeats
		// the burst needs. Positioned AFTER the --ack guard and the -f branch but
		// BEFORE the single-pin blocks, so a 2-pin set (`-s a -s b`) bursts rather
		// than hitting the single -s block. A set of 2+ targets bursts; a single
		// glob-expandable target also bursts because it may expand to K≥2 (overriding
		// the single-glob first-match). Everything else (zero targets, a single
		// non-glob / -p / -z target) falls through to the UNCHANGED single-target path
		// below.
		ordered := orderedOpenTargets(openOwnArgs())
		if isMultiTarget(ordered) {
			return dispatchOpenBurst(cmd, ordered, command)
		}

		// Domain-pinning flags: each pin resolves its
		// value in ONE domain only and dispatches the hit through the shared outcome
		// switch (openResolved) via resolvePinAndOpen. Checked in a FIXED precedence
		// order — session → path → alias → zoxide, the order of openDomainPinFlags —
		// and the first changed pin short-circuits. A pin never mints-to-picker: a
		// miss hard-fails and no resolve line is emitted
		// (pins are deterministic, not guesses). The block sits BEFORE the no-target
		// early-return so `open -s <name>` with an empty positional resolves the pin
		// rather than launching the picker. A future pin is added in ONE place: it is
		// declared in openDomainPinFlags (which the exclusivity guard anyOpenDomainPin
		// also iterates) and paired with its resolver in pinResolvers, so the guard
		// and the dispatch table cannot drift.
		for _, flag := range openDomainPinFlags {
			if cmd.Flags().Changed(flag) {
				return resolvePinAndOpen(cmd, flag, pinResolvers[flag], command)
			}
		}

		if destination == "" {
			return openTUIFunc(cmd, "", command, serverWasStarted(cmd))
		}

		query := destination

		qr, err := buildQueryResolver(cmd)
		if err != nil {
			return err
		}

		result, err := qr.Resolve(query)
		if err != nil {
			return err
		}

		// Resolution-decision receipt — tmux is the receipt: single-sourced via
		// emitResolveDecision, which owns the
		// !HasGlobMeta gate and the locked attr set. A mid-chain hard error
		// (DirNotFoundError) returned above never reaches here: classification did
		// not complete, so no decision line fires.
		emitResolveDecision(query, result)

		if miss, ok := result.(*resolver.MissResult); ok {
			// Total miss: hard-fail with the escape-hatch message. Handled inline
			// in the bare-positional path — pins never
			// yield a MissResult. A plain (non-usage) error → exit code 1 via
			// main.classify; the TUI picker is never launched on a miss. The em-dash
			// is U+2014. The wording is single-sourced in singleMissError.
			return singleMissError(miss.Target)
		}
		return openResolved(cmd, result, command)
	},
}

// openDomainPinFlags is the canonical, precedence-ordered set of open
// domain-pin flag names — the SINGLE source of the pin set, consumed by BOTH the
// exclusivity guard (anyOpenDomainPin, cmd/root.go) and openCmd.RunE's dispatch
// loop. Declaring the names once here is what closes the drift the guard
// previously invited: a future pin added to this list is automatically covered
// by anyOpenDomainPin (which consults this slice), so it can never be silently
// omitted from the exclusivity guard. Order is load-bearing — it is the fixed
// session → path → alias → zoxide precedence the RunE dispatch loop short-
// circuits in.
var openDomainPinFlags = []string{"session", "path", "alias", "zoxide"}

// pinResolvers maps each open domain-pin flag to the QueryResolver method that
// resolves its value in that ONE domain. It is keyed by the same flag names
// enumerated in openDomainPinFlags (the RunE loop iterates that slice for
// precedence order and looks the resolver up here); the two are locked in
// lockstep by the drift guards in open_pin_source_guard_test.go, so a listed
// flag always has a resolver and a resolver is never orphaned outside the guard.
//
//	-s/--session  attach the named session / glob; never mints.
//	-p/--path     mint at the LITERAL dir via ResolvePathPin (reuses ResolvePath
//	              for tilde/relative expansion + existence + is-directory
//	              validation). Because it stats the literal path, a glob-named dir
//	              (~/tmp/foo[1]) is reachable here, bypassing the bare-positional
//	              glob pre-check.
//	-a/--alias    look the key up directly in the alias store, bypassing the
//	              session→path→alias precedence — the ONLY way to reach an alias
//	              key shadowed by a same-named session (glob expands against the
//	              finite alias-key namespace).
//	-z/--zoxide   query zoxide and make its outcome EXPLICIT — zoxide-not-installed
//	              surfaces ErrZoxideNotInstalled verbatim — unlike the bare chain,
//	              which swallows the error and falls through to the miss tail.
var pinResolvers = map[string]func(*resolver.QueryResolver, string) (resolver.QueryResult, error){
	"session": (*resolver.QueryResolver).ResolveSessionPin,
	"path":    (*resolver.QueryResolver).ResolvePathPin,
	"alias":   (*resolver.QueryResolver).ResolveAliasPin,
	"zoxide":  (*resolver.QueryResolver).ResolveZoxidePin,
}

// resolvePinAndOpen is the single body behind all four -s/-p/-a/-z domain-pin
// arms in openCmd.RunE: read the flag value, build the query resolver, resolve
// the value in the pinned domain, and hand the result to the shared outcome
// switch (openResolved). The resolve parameter is a method value
// ((*resolver.QueryResolver).ResolveSessionPin, etc.) — all four pins share the
// uniform (string) (QueryResult, error) signature, so a new pin is one table row
// in RunE, not a fifth copy of this arm. Error propagation and the openResolved
// handoff are identical for every pin.
func resolvePinAndOpen(cmd *cobra.Command, flag string, resolve func(*resolver.QueryResolver, string) (resolver.QueryResult, error), command []string) error {
	val, _ := cmd.Flags().GetString(flag)
	qr, err := buildQueryResolver(cmd)
	if err != nil {
		return err
	}
	result, err := resolve(qr, val)
	if err != nil {
		return err
	}
	return openResolved(cmd, result, command)
}

// openResolved dispatches a resolved query result to its outcome: a session-
// domain hit attaches (openSessionFunc); a directory-domain hit mints
// (openPathFunc), threading the mint-scoped command. It is the single shared
// dispatch point for the -s/--session pin, the mint pins and the bare-positional
// path, so every entry point routes a resolved result
// through the identical outcome switch. A MissResult is deliberately NOT handled
// here — the bare path renders its own escape-hatch message inline, and pins
// never yield a miss — so any unexpected result type is a defensive error.
//
// A command (-e/--) is mint-scoped: an existing (attach) session has no safe
// command-injection channel (send-keys
// corrupts a busy pane; respawn-pane -k destroys running work), so a command can
// never run in an attach target. The guard lives here, in the *SessionResult
// arm, so it covers BOTH the bare-positional session hit and the -s pin (both
// route through this switch) with a single check.
func openResolved(cmd *cobra.Command, result resolver.QueryResult, command []string) error {
	switch r := result.(type) {
	case *resolver.SessionResult:
		if len(command) > 0 {
			return NewUsageError(commandAttachOnlyMessage)
		}
		// The --ack marker write is the LAST act before the attach handoff, and
		// strictly AFTER the command guard above — a command+attach usage error
		// must fire without writing a marker.
		writeAckMarker(cmd)
		return openSessionFunc(cmd, r.Name)
	case *resolver.PathResult:
		// The --ack marker write is the LAST act before the mint handoff.
		writeAckMarker(cmd)
		return openPathFunc(cmd, r.Path, command)
	default:
		return fmt.Errorf("unexpected resolution result: %T", result)
	}
}

// writeAckMarker writes the @portal-spawn-<batch>-<token> confirmation marker as
// the last act before the attach/mint handoff, when the hidden --ack flag is
// present (a no-op when absent). The value was already validated at the top of
// RunE, so the re-parse here is guaranteed to succeed. The write is best-effort
// a failure logs at DEBUG under the spawn component
// and falls through to the connector — the spawned window still attaches, a
// false negative the parent's poll classifies as failed, never an orphan.
func writeAckMarker(cmd *cobra.Command) {
	ackVal, _ := cmd.Flags().GetString("ack")
	if ackVal == "" {
		return
	}
	batch, token, ok := spawn.ParseSpawnAckFlag(ackVal)
	if !ok {
		return // unreachable: RunE rejects a malformed --ack before dispatch
	}
	if err := buildAckWriter(cmd).Write(batch, token); err != nil {
		spawnLogger.Debug("spawn-ack marker write failed",
			"batch", batch,
			"detail", err.Error(),
		)
	}
}

// buildAckWriter returns the ack writer for the open command's --ack receiver.
// When openDeps carries a non-nil AckWriter (testing), it is used; otherwise a
// @portal-spawn- server-option channel over the shared tmux client (which
// satisfies both the writer and lister seams).
func buildAckWriter(cmd *cobra.Command) spawn.AckWriter {
	if openDeps != nil && openDeps.AckWriter != nil {
		return openDeps.AckWriter
	}
	client := tmuxClient(cmd)
	return spawn.NewServerOptionAckChannel(client, client)
}

// emitResolveDecision writes the single resolve-decision INFO line
// for a bare positional resolved through the guessing chain — the SINGLE source
// shared by both bare paths (the single-target openCmd.RunE arm and the
// resolveOpenSurfaces burst arm), so the locked attr set (target/domain/
// resolved_path) and the emission gate cannot drift between them.
//
// The !HasGlobMeta gate lives INSIDE the helper so both call sites gate
// identically: glob (and pinned) targets are deterministic, not guesses, so they
// emit no line. One INFO line per guessing-chain target — emitted on a miss too
// (domain=miss, empty resolved_path) — so a confusing guess is reconstructable
// from portal.log — tmux is the receipt.
func emitResolveDecision(target string, result resolver.QueryResult) {
	if resolver.HasGlobMeta(target) {
		return
	}
	domain, resolvedPath := resolveDecision(result)
	// domain.String() yields the closed-taxonomy attr value
	// (session/path/alias/zoxide/miss) as a plain string, so the emitted log line
	// is byte-identical to the pre-typed-Domain wording.
	resolveLogger.Info("resolved", "target", target, "domain", domain.String(), "resolved_path", resolvedPath)
}

// resolveDecision derives the (domain, resolved_path) attrs for the resolve
// decision log line from a completed classification result. resolved_path is
// overloaded: the resolved directory for a path/alias/zoxide hit,
// the session name for a session hit, and empty for a miss. It reads the domain
// off the already-obtained result (r.Domain / resolver.DomainMiss) — it does not
// re-run the classification.
func resolveDecision(result resolver.QueryResult) (domain resolver.Domain, resolvedPath string) {
	switch r := result.(type) {
	case *resolver.SessionResult:
		return r.Domain, r.Name
	case *resolver.PathResult:
		return r.Domain, r.Path
	case *resolver.MissResult:
		return resolver.DomainMiss, ""
	default:
		return "", ""
	}
}

// logExecHandoff writes the process:exec handoff marker — the SINGLE source
// shared by both bare-shell exec paths (AttachConnector.Connect and
// PathOpener.Open), so this load-bearing forensic tripwire reads byte-identically
// on both.
//
// It takes the FULL argv and defensively strips argv[0] (the "tmux" program name)
// so args renders the tmux subcommand chain only. The len guard is a no-op for
// AttachConnector's fixed 4-element argv (→ argv[1:]) and guards PathOpener's
// always-populated ExecArgs; both sites therefore emit args=argv[1:].
//
// It MUST be called IMMEDIATELY BEFORE syscall.Exec: that call replaces the
// process image and never returns, so log.Close never fires and no process:exit
// line is emitted — this marker is the terminal log line for the handoff. The
// log writer is unbuffered, so the bytes are in the kernel before the image is
// replaced — no Sync needed.
func logExecHandoff(argv []string) {
	args := argv
	if len(args) > 0 {
		args = args[1:]
	}
	log.For("process").Info("exec", "target", "tmux", "args", strings.Join(args, " "))
}

// parseCommandArgs extracts the command slice and destination from cobra args and flags.
// It validates mutual exclusivity of -e/--exec and --, and rejects empty commands.
func parseCommandArgs(cmd *cobra.Command, args []string) ([]string, string, error) {
	execFlag, _ := cmd.Flags().GetString("exec")
	dashIdx := cmd.ArgsLenAtDash()

	hasExec := cmd.Flags().Changed("exec")
	hasDash := dashIdx >= 0

	if hasExec && hasDash {
		return nil, "", NewUsageError("cannot use both -e/--exec and -- to specify a command")
	}

	if hasExec {
		if execFlag == "" {
			return nil, "", NewUsageError("-e/--exec value must not be empty")
		}
		var dest string
		if len(args) > 0 {
			dest = args[0]
		}
		return []string{execFlag}, dest, nil
	}

	if hasDash {
		dashArgs := args[dashIdx:]
		if len(dashArgs) == 0 {
			return nil, "", NewUsageError("no command specified after --")
		}
		var dest string
		if dashIdx > 0 {
			dest = args[0]
		}
		return dashArgs, dest, nil
	}

	// No command specified
	var dest string
	if len(args) > 0 {
		dest = args[0]
	}
	return nil, dest, nil
}

// sessionCreatorIface creates a tmux session from a directory and returns the session name.
type sessionCreatorIface interface {
	CreateFromDir(dir string, command []string) (string, error)
}

// quickStarter runs the quick-start pipeline and returns exec args.
type quickStarter interface {
	Run(path string, command []string) (*session.QuickStartResult, error)
}

// execer abstracts process replacement for testability.
type execer interface {
	Exec(argv0 string, argv []string, envv []string) error
}

// realExecer replaces the current process via syscall.Exec.
type realExecer struct{}

// Exec replaces the current process.
func (r *realExecer) Exec(argv0 string, argv []string, envv []string) error {
	return syscall.Exec(argv0, argv, envv)
}

// quickStartAdapter adapts session.QuickStart to the quickStarter interface.
type quickStartAdapter struct {
	qs *session.QuickStart
}

// Run delegates to the underlying QuickStart pipeline.
func (a *quickStartAdapter) Run(path string, command []string) (*session.QuickStartResult, error) {
	return a.qs.Run(path, command)
}

// PathOpener handles creating a new tmux session from a resolved path.
// It branches on insideTmux: inside tmux creates detached then switches;
// outside tmux uses exec handoff with -A flag.
type PathOpener struct {
	insideTmux bool
	creator    sessionCreatorIface
	switcher   SwitchClienter
	qs         quickStarter
	execer     execer
	tmuxPath   string
}

// Open creates a session at the given path and connects to it.
// When command is non-nil, it is passed through to session creation
// for execution as a tmux shell-command.
func (po *PathOpener) Open(resolvedPath string, command []string) error {
	if po.insideTmux {
		sessionName, err := po.creator.CreateFromDir(resolvedPath, command)
		if err != nil {
			return err
		}
		return po.switcher.SwitchClient(sessionName)
	}

	result, err := po.qs.Run(resolvedPath, command)
	if err != nil {
		return err
	}

	// session.QuickStart builds ExecArgs as the chained create-stamp-attach
	// invocation {"tmux", "new-session", "-d", …, ";", "set-option", …, ";",
	// "attach-session", …}. ExecArgs[0] is always the "tmux" program name.
	// logExecHandoff drops it (single-sourced with AttachConnector so both emit
	// args=argv[1:]) and is emitted IMMEDIATELY before syscall.Exec (the terminal
	// marker before the process image is replaced).
	logExecHandoff(result.ExecArgs)

	return po.execer.Exec(po.tmuxPath, result.ExecArgs, os.Environ())
}

// openPath creates a new tmux session at the given resolved directory path.
// When inside tmux, it creates the session detached and switches to it.
// When outside tmux, it execs into tmux with the -A flag for atomic create-or-attach.
func openPath(cmd *cobra.Command, resolvedPath string, command []string) error {
	client := tmuxClient(cmd)
	gitResolver := &resolverAdapter{}
	projectsPath, err := projectsFilePath()
	if err != nil {
		return err
	}
	store := project.NewStore(projectsPath)
	gen := session.NewNanoIDGenerator()

	insideTmux := tmux.InsideTmux()

	opener := &PathOpener{
		insideTmux: insideTmux,
		creator:    session.NewSessionCreator(gitResolver, store, client, gen),
		switcher:   client,
		qs:         &quickStartAdapter{qs: session.NewQuickStart(gitResolver, store, client, gen)},
		execer:     &realExecer{},
	}

	if !insideTmux {
		tmuxPath, err := exec.LookPath("tmux")
		if err != nil {
			return fmt.Errorf("tmux not found: %w", err)
		}
		opener.tmuxPath = tmuxPath
	}

	return opener.Open(resolvedPath, command)
}

// resolverAdapter adapts resolver.ResolveGitRoot to the session.GitResolver interface.
type resolverAdapter struct{}

// Resolve resolves a directory to its git repository root.
func (r *resolverAdapter) Resolve(dir string) (string, error) {
	return resolver.ResolveGitRoot(dir, &resolver.RealCommandRunner{})
}

// tuiConfig holds injectable dependencies for building the TUI model.
type tuiConfig struct {
	lister          tui.SessionLister
	killer          tui.SessionKiller
	renamer         tui.SessionRenamer
	projectStore    tui.ProjectStore
	projectEditor   tui.ProjectEditor
	aliasEditor     tui.AliasEditor
	sessionCreator  tui.SessionCreator
	enumerator      tui.TmuxEnumerator
	reader          tui.ScrollbackReader
	previewAttacher tui.PreviewAttacher
	dirReader       session.PaneCurrentPathReader
	dirRunner       resolver.CommandRunner
	initialMode     prefs.SessionListMode
	// theme is the LOADED theme setting the model renders from — one palette
	// under a constant, both under an adaptive pair. It replaces the light/dark
	// appearance this struct used to carry: a theme IS the mode, so there is no
	// mode left to pin. Its zero value is neither state,
	// which leaves a model on tui.New's dark built-in seed — the shape every
	// test config that does not care about theming carries.
	theme theme.Nomination
	// themeKeys / themeSource are the panel's constructor slots. The keys are
	// prefs.json's three theme keys as read (control-stripped, post-translation) —
	// the SNAPSHOT the panel lists and marks from and never re-reads; the
	// source is the seam the `t` keypress reads the themes directory through,
	// and through which the panel re-resolves the setting against that read for its
	// badges. Their zero values are what a test config that
	// does not care about theming carries: no keys is the shipped adaptive pair,
	// and a nil source makes `t` a silent no-op.
	themeKeys     theme.RawKeys
	themeSource   tui.ThemeSource
	modePersister tui.ModePersister
	// themePersister is the theme-commit seam — the cmd-owned persister, never
	// the store itself (whose savers deliberately do not satisfy the interface, so
	// the single `theme: commit failed` emission site cannot be bypassed). Wired
	// only when the prefs store loaded; nil leaves the panel's commit unwired,
	// which is what a fixture / capturetool model carries.
	themePersister tui.ThemePersister
	// detector + resolve are the async host-terminal detection seams. Built
	// once at TUI construction (detector over the shared *tmux.Client; resolve from
	// the config-aware buildResolver, terminals.json loaded once) and threaded into
	// tui.Deps. This is the SINGLE injection site — the picker burst reuses the
	// model's cached resolution and never re-injects.
	detector tui.TerminalDetector
	resolve  spawn.AdapterResolver
	// N≥2 picker-burst seams. Built once here (defaults from the shared
	// productionSpawnSeams bundle, cmd/spawn_seams.go: client.HasSession / a shared
	// server-option ack channel / os.Executable / os.Getenv — the same bundle the
	// multi-target open burst wires from) and threaded into tui.Deps. The burst REUSES
	// the resolve seam above (the cached resolution + a re-resolve for the adapter).
	sessionExists func(string) bool
	ackChannel    spawn.AckChannelFull
	spawnExe      spawn.ExecutableResolver
	spawnGetenv   func(string) string
	// spawnLogger is the spawn-component logger the picker burst's completion
	// chokepoint emits its batch summary + per-window detail through (log.For("spawn")
	// in production; the parallel to cmd/spawn_seams.go's package-level spawnLogger).
	spawnLogger    *slog.Logger
	cwd            string
	insideTmux     bool
	currentSession string
	serverStarted  bool
	// progressReceiver is the concurrent cold-boot route's channel-receive
	// tea.Cmd. Set only on the cold + TUI path (where bootstrap was deferred to a
	// goroutine); nil on every synchronous path, leaving the model's today
	// behaviour intact.
	progressReceiver tea.Cmd
	// noColor is the NO_COLOR carve-out decision, read ONCE here in the cmd
	// layer (os.Getenv) so internal/tui stays env-free. It is the single inheritable
	// colourless flag passed into tui.Deps.NoColor.
	noColor bool
}

// noColorEnabled reports whether the NO_COLOR carve-out is active, per the
// no-color.org convention: the NO_COLOR env var must be PRESENT and NON-EMPTY. A
// set-but-empty NO_COLOR ("") does NOT enable it (an empty value is treated as
// unset by the convention). This is the SINGLE place NO_COLOR is read in the
// cmd/open path; the decision flows as a boolean into tui.Deps so internal/tui
// stays env-free and every canvas-dependent surface inherits one flag.
func noColorEnabled() bool {
	v, ok := os.LookupEnv("NO_COLOR")
	return ok && v != ""
}

// newThemeLoader returns the loader every theme read on the TUI-construction
// path goes through, carrying the package's `theme` component logger.
//
// Constructed per call rather than held package-level, because a Loader owns the
// event logger's per-process dedup state: one loader per TUI construction is one
// dedup scope per launch, which is what stops the construction-time read and the
// panel's later enumeration reporting the same directory condition twice.
func newThemeLoader() theme.Loader {
	return theme.NewLoader(theme.NewEventLogger(themeLogger))
}

// buildThemeLoader returns the loader TUI construction resolves the persisted
// theme setting through: openDeps.ThemeLoader when injected (staging the
// broken-binary state), otherwise the production one. It is the buildAckWriter shape,
// for the same reason — the seam is a testing concern, so the production path
// stays a plain constructor call.
func buildThemeLoader() theme.Loader {
	if openDeps != nil && openDeps.ThemeLoader != nil {
		return *openDeps.ThemeLoader
	}
	return newThemeLoader()
}

// themeResolution resolves the theme setting into everything TUI construction
// takes from it: prefs' three raw keys → the two-state setting → one loaded
// palette under a constant, two under an adaptive pair, with the mode-matched
// default standing in for any slot that will not load.
//
// IT RETURNS THREE THINGS FROM ONE EVALUATION, and that is the point rather than
// a convenience. The NOMINATION says what will be painted. The per-slot RECORD
// (on the Resolution) says what each slot asked for and what happened to it,
// which is what the panel's `●` marks — a Nomination holds palettes and cannot
// express the slug a fallback replaced. The control-stripped RAW KEYS are what the
// panel lists a slug that never loaded from, and what the confirm renders. A surface
// deriving any of them separately is how the picker and its badges would come to
// disagree about which theme is live.
//
// It takes the keys AS READ rather than the store, because "as read" means the
// POST-TRANSLATION in-memory value, not the on-disk bytes: loadPrefsStore applies
// the appearance translation, and resolving from a second read here would render
// the shipped pair for a migrated user on the very launch their pin was
// translated. The keys it hands back are the construction-time snapshot the
// panel keeps for its whole life, never re-read on open.
//
// ZERO KEYS ARE THE SHIPPED ADAPTIVE PAIR, which is what an unconfigured install
// renders anyway — so they are also what openTUI degrades to when the prefs path
// could not be resolved at all, exactly as the grouping mode degrades to Flat.
//
// THE TWO FAILURE SHAPES ARE DIFFERENT AND ARE HANDLED DIFFERENTLY. A themes
// directory that will not RESOLVE degrades to the empty string, which the resolver
// reads as "there is no directory to look in" — built-ins still resolve, a drop-in
// slug takes the mode-matched fallback, and the picker opens.
// ResolveNomination's error is the broken-builtin fatal (the FALLBACK itself did
// not resolve), which is returned so the caller constructs nothing.
func themeResolution(keys prefs.ThemeKeys, loader theme.Loader) (theme.Resolution, theme.RawKeys, error) {
	setting, raw := theme.ResolveSetting(theme.NewRawKeys(keys.Theme, keys.Light, keys.Dark))

	// A themes-directory path that cannot be resolved AT ALL degrades to the empty
	// directory rather than blocking the launch. themesDirPath already answers with
	// "" on failure, so the discarded error is the degradation: the embedded set is
	// reachable with no path at all (the embedded set is consulted first), so a
	// built-in nomination is
	// unaffected and a drop-in one falls back and reports.
	themesDir, _ := themesDirPath()

	resolution, err := loader.ResolveNomination(setting, themesDir)
	if err != nil {
		return theme.Resolution{}, theme.RawKeys{}, err
	}
	return resolution, raw, nil
}

// buildTUIModel constructs a tui.Model from the given config and parameters by
// mapping the cmd-local tuiConfig onto the shared tui.Deps seam set and
// delegating to tui.Build — the single model-construction chokepoint shared with
// the offline capture harness (cmd/capturetool). The mapping is field-for-field
// (no nil-guards or option assembly here — Build owns that), so production and
// the harness assemble the identical model.
func buildTUIModel(cfg tuiConfig, initialFilter string, command []string) tui.Model {
	return tui.Build(tui.Deps{
		Lister:           cfg.lister,
		Killer:           cfg.killer,
		Renamer:          cfg.renamer,
		Creator:          cfg.sessionCreator,
		ProjectStore:     cfg.projectStore,
		ProjectEditor:    cfg.projectEditor,
		AliasEditor:      cfg.aliasEditor,
		Enumerator:       cfg.enumerator,
		Reader:           cfg.reader,
		PreviewAttacher:  cfg.previewAttacher,
		DirReader:        cfg.dirReader,
		DirRunner:        cfg.dirRunner,
		ModePersister:    cfg.modePersister,
		ThemePersister:   cfg.themePersister,
		CWD:              cfg.cwd,
		InitialMode:      cfg.initialMode,
		Theme:            cfg.theme,
		ThemeKeys:        cfg.themeKeys,
		ThemeSource:      cfg.themeSource,
		InitialFilter:    initialFilter,
		Command:          command,
		ServerStarted:    cfg.serverStarted,
		InsideTmux:       cfg.insideTmux,
		CurrentSession:   cfg.currentSession,
		NoColor:          cfg.noColor,
		ProgressReceiver: cfg.progressReceiver,
		Detector:         cfg.detector,
		Resolve:          cfg.resolve,
		SessionExists:    cfg.sessionExists,
		AckChannel:       cfg.ackChannel,
		SpawnExe:         cfg.spawnExe,
		SpawnGetenv:      cfg.spawnGetenv,
		SpawnLogger:      cfg.spawnLogger,
	})
}

// processTUIResult handles the result of a TUI run.
//
// Fatal cold-boot: if the model carries a fatal (a fatal bootstrap step
// aborted the boot on the concurrent cold/TUI route, and q/Esc quit the error
// frame), return that fatal — the underlying *bootstrap.FatalError — BEFORE any
// connect. Execute writes its single UserMessage line and main.classify maps it
// to code 1 with no double-print, exactly as the synchronous warm/CLI path does;
// returning the SAME *bootstrap.FatalError instance keeps the exit byte-for-byte
// identical. A fatal model has no selection, so this also guarantees no connect.
//
// Otherwise: if the user selected a session, connect via the given connector; if
// the user quit without selecting, return nil (unchanged).
func processTUIResult(model tui.Model, connector SessionConnector) error {
	if fatal := model.FatalError(); fatal != nil {
		return fatal
	}
	selected := model.Selected()
	if selected == "" {
		return nil
	}
	return connector.Connect(selected)
}

// openTUI launches the interactive session picker with an optional initial filter.
func openTUI(cmd *cobra.Command, initialFilter string, command []string, serverStarted bool) error {
	client := tmuxClient(cmd)
	gitResolver := &resolverAdapter{}
	gen := session.NewNanoIDGenerator()

	// Concurrent full-bootstrap route: PersistentPreRunE deferred the
	// orchestrator on the (latch-not-satisfied) TUI path. Build the progress
	// pipe, launch the orchestrator in a goroutine, and stream live per-step
	// progress to the loading page over the channel. The model renders the
	// loading page from frame one — serverStarted is forced true just below
	// because a full bootstrap is in progress on this route, NOT because the
	// server was necessarily cold: a warm-unlatched server (hand-started tmux hit
	// by `x`) reaches this route too, so "the server was not running" is not a
	// safe assumption. On every synchronous path pipe is nil and openTUI keeps
	// today's behaviour (serverStarted carried via the serverStartedKey context
	// that the caller already read).
	var pipe *bootstrapProgressPipe
	if deferred := deferredBootstrapFromContext(cmd); deferred != nil {
		pipe = newBootstrapProgressPipe()
		pipe.start(cmd.Context(), deferred.runner)
		// Full bootstrap in progress: the loading page must show, so force
		// serverStarted regardless of the caller's (false, deferred) flag.
		// serverStarted's sole effect is parking the model on the loading page
		// (WithServerStarted(true) -> activePage = PageLoading); on the deferred
		// route the server may or may not have pre-existed (warm-unlatched), but a
		// full bootstrap is running either way, which is exactly when the loading
		// page should show.
		serverStarted = true
	}

	store, err := loadProjectStore()
	if err != nil {
		return err
	}

	aliasStore, err := loadAliasStore()
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine working directory: %w", err)
	}

	// Resolve stateDir once per Portal process — captured into the
	// scrollback reader adapter at TUI construction so the preview page
	// reads from the same directory the daemon and bootstrap orchestrator
	// write to. state.Dir() is the single source of truth for state-path
	// policy; preview never resolves stateDir on its own.
	stateDir, err := state.Dir()
	if err != nil {
		return fmt.Errorf("failed to resolve state directory: %w", err)
	}
	previewReader := tui.NewProductionScrollbackReader(stateDir)

	// Load the prefs store once at TUI construction; the same *prefs.Store
	// instance serves the initial-mode read here and per-toggle writes via the
	// tui.ModePersister seam. The load also computes the one-shot appearance
	// translation in memory — it runs where a TUI is constructed, because its
	// result is what the picker is built from. A prefs path-resolution failure
	// must NOT block opening the TUI: swallow it
	// and proceed with a nil persister, zero keys (the shipped pair) + the Flat
	// default. prefs.json is deliberately outside the closed state-mutation
	// audit-trail (see internal/prefs), so there is no breadcrumb component to
	// log under here.
	loadedPrefs, err := loadPrefsStore()
	if err != nil {
		loadedPrefs = prefsLoad{}
	}
	prefsStore := loadedPrefs.Store
	// Read the persisted initial mode tolerantly. Store.Load collapses every
	// degenerate case (missing / empty / corrupt / unrecognised value) to
	// ModeFlat, so the discarded error is acceptable: a read failure can only
	// yield Flat, which is the first-launch default anyway.
	initialMode := prefs.ModeFlat
	if prefsStore != nil {
		initialMode, _ = prefsStore.Load()
	}
	// Resolve the theme setting persisted in prefs.json — the keys AS READ from
	// the load above, which is the post-translation in-memory value, so a
	// migrated user renders their pin on the launch that translates it — into the
	// nomination the model renders from. A CONSTANT paints from frame one with no
	// detection and no first-paint wait; a PAIR holds both palettes and the
	// light/dark gate selects a member before anything is painted.
	//
	// The loader is built ONCE and shared with the panel's theme source below,
	// because a Loader owns the `theme` component's per-process dedup state: the
	// construction-time by-name read and the panel's enumeration hit the same
	// directory conditions, so one loader per launch is one dedup scope per launch.
	//
	// The error is the broken-builtin fatal — the theme a slot fell back to is not in the
	// embedded set, so there is nothing honest left to paint. It is returned
	// UNTOUCHED and NO TUI IS CONSTRUCTED: Execute hands it to main's single
	// os.Exit owner, which prints the one pinned line and exits non-zero.
	themeLoader := buildThemeLoader()
	resolution, themeKeys, err := themeResolution(loadedPrefs.Keys, themeLoader)
	if err != nil {
		return err
	}

	// Resolve the connector once. It is used post-TUI by processTUIResult
	// for both Sessions-page Enter and Preview-page Enter. Both
	// *AttachConnector and *SwitchConnector are safe to reuse across
	// calls — neither holds per-attach state — so a single instance per
	// openTUI invocation is sufficient. The preview-page pipeline no
	// longer holds a reference to the connector: it emits a
	// previewAttachSelectedMsg, the model records the selected session +
	// tea.Quit, and the connector handoff happens in processTUIResult
	// after the TUI program shuts down. This prevents the inside-tmux
	// orphan-portal-process regression where switch-client would move
	// the surrounding tmux client while portal kept event-looping with
	// no UI.
	connector := buildSessionConnector(client)

	// The pre-select pipeline WARNs on select-window / select-pane failures
	// under the preview component. The handler is configured once by
	// main -> log.Init; there is
	// no per-process log open here.
	previewAttacher := tui.NewPreviewAttachPipeline(client, previewLogger)

	// Build the shared production spawn seams ONCE from the resolved client. This
	// is the same bundle the multi-target open burst (buildOpenBurstDeps) reads, so
	// the picker's detection + burst seams cannot silently diverge from the
	// open burst's.
	spawnSeams := buildProductionSpawnSeams(client)

	cfg := tuiConfig{
		lister:          client,
		killer:          client,
		renamer:         client,
		projectStore:    store,
		projectEditor:   store,
		aliasEditor:     aliasStore,
		sessionCreator:  session.NewSessionCreator(gitResolver, store, client, gen),
		enumerator:      client,
		reader:          previewReader,
		previewAttacher: previewAttacher,
		// Render-layer lazy directory-resolution fallback: client
		// (*tmux.Client) satisfies session.PaneCurrentPathReader via
		// ActivePaneCurrentPath; RealCommandRunner resolves the active pane's
		// cwd to a git-root. The result is cached in-memory only, never stamped.
		dirReader:   client,
		dirRunner:   &resolver.RealCommandRunner{},
		initialMode: initialMode,
		theme:       resolution.Nomination,
		// The panel's constructor slots. The keys ride along because the nomination
		// alone cannot answer for them: a slug that never loaded is not in it, and
		// under a fallback the `●` belongs on the slug the user SET rather than on the
		// palette that rendered. The construction-time per-slot record deliberately
		// does NOT — the panel re-resolves it against its own enumeration on every
		// open, through the source's Resolve, so an injected copy would be a
		// second and staler answer to which slug carries the `●`. The source
		// shares this resolution's loader (one dedup scope per launch) and reads
		// nothing until `t` is pressed.
		themeKeys:     themeKeys,
		themeSource:   newThemeSource(themeLoader),
		cwd:           cwd,
		serverStarted: serverStarted,
		// The async host-terminal detection seams, from the shared builder: the
		// detector over the shared *tmux.Client, and the config-aware resolver's
		// Resolve (buildResolver loads terminals.json once, degrading to an empty
		// native-only config on a configFilePath error). Detection runs off the
		// Update path as a tea.Cmd; the resolution is cached on the model and reused
		// by the later picker burst.
		detector: spawnSeams.Detector,
		resolve:  spawnSeams.Resolve,
		// The N≥2 picker-burst seams — the same shared productionSpawnSeams bundle
		// the multi-target open burst defaults from: the pre-flight has-session probe
		// folds a probe fault to gone (conservative), the shared server-option ack channel
		// confirms/cleans spawned windows, and the exe/PATH seams compose each
		// spawned attach argv.
		sessionExists: spawnSeams.Exists,
		ackChannel:    spawnSeams.Ack,
		spawnExe:      spawnSeams.Exe,
		spawnGetenv:   spawnSeams.Getenv,
		// The picker burst's spawn-component logger — the TUI parallel to
		// cmd/spawn_seams.go's package-level spawnLogger = log.For("spawn").
		spawnLogger: spawnSeams.Logger,
		// NO_COLOR carve-out: read the env ONCE here (cmd layer) so
		// internal/tui stays env-free. The single colourless flag flows through
		// tui.Deps.NoColor and is inherited by every canvas-dependent surface.
		noColor: noColorEnabled(),
	}
	// Concurrent cold-boot route: wire the channel-receive tea.Cmd so the
	// loading-page model streams live per-step progress and the channel owns the
	// terminal BootstrapCompleteMsg (Init does NOT synthesize it on this route).
	if pipe != nil {
		cfg.progressReceiver = pipe.receiver()
	}
	// Guard the persister assignments: a typed-nil *prefs.Store boxed into the
	// tui.ModePersister interface would be non-nil, defeating buildTUIModel's nil
	// check — and the theme persister, which WRAPS that store, would be a
	// live-looking seam whose every write panics on the nil store inside it. Only
	// wire them when the store actually loaded; both ride the one store instance
	// read once per process.
	if prefsStore != nil {
		cfg.modePersister = prefsStore
		cfg.themePersister = newThemePersister(prefsStore)
	}

	if tmux.InsideTmux() {
		sessionName, err := client.CurrentSessionName()
		if err == nil && sessionName != "" {
			cfg.insideTmux = true
			cfg.currentSession = sessionName
		}
	}

	m := buildTUIModel(cfg, initialFilter, command)
	// Drain any soft bootstrap warnings accumulated during PersistentPreRunE
	// and stage them on the model. Init folds them into BootstrapCompleteMsg
	// so they ride the loading-page dismissal gate; the model emits them to
	// stderr (with alt-screen toggle) only after the loading page has been
	// dismissed — direct writes during loading would corrupt the rendered UI.
	stageBootstrapWarningsOnModel(&m)
	// Bootstrap ordering differs by route:
	//   - Synchronous (warm/CLI): PersistentPreRunE already ran the orchestrator,
	//     so the model's Init emits BootstrapCompleteMsg from its first event-loop
	//     tick (carrying staged warnings), paired with the 1.2s LoadingMinElapsedMsg
	//     tick. Dismissal gates on both.
	//   - Concurrent (cold/TUI): the orchestrator runs in the pipe's goroutine; the
	//     channel streams BootstrapProgressMsg per step and the terminal
	//     BootstrapCompleteMsg, so Init wires the receiver instead of synthesizing
	//     the terminal event. Dismissal still gates on the 1.2s tick + the terminal
	//     channel event.
	// Bubble Tea v2 removed the tea.WithAltScreen() program option — the
	// alternate screen is now declared via the tea.View.AltScreen field, set
	// in tui.Model.View(). The launch is otherwise unchanged.
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	model, ok := finalModel.(tui.Model)
	if !ok {
		return fmt.Errorf("unexpected model type: %T", finalModel)
	}

	// Restore the terminal's original background on exit, BEFORE any session
	// attach/exec handoff. The owned canvas paint
	// sets the terminal background via OSC 11 so it extends into the gutter;
	// terminals that ignore the OSC 111 reset (mosh/Blink) keep the canvas
	// colour after Portal quits, so SET the captured original back. It MUST run
	// before processTUIResult: on the quit-to-shell path (no selection) this is
	// the restore the user sees; on the attach-handoff path it is harmless (tmux
	// takes over the screen). No-op when no OSC 11 response was captured (best-
	// effort fallback to Bubble Tea's own OSC 111 reset). Shared helper with
	// cmd/capturetool so both restore identically; writes to os.Stdout (the
	// program's output).
	tui.RestoreTerminalBackground(os.Stdout, model)

	return processTUIResult(model, connector)
}

// buildQueryResolver creates a QueryResolver with appropriate dependencies.
// The session lister is the user-visible (leading-underscore-filtered) session
// set: openDeps.SessionLister when injected, otherwise the shared *tmux.Client
// (which satisfies resolver.SessionLister via ListSessionNames).
func buildQueryResolver(cmd *cobra.Command) (*resolver.QueryResolver, error) {
	if openDeps != nil {
		return resolver.NewQueryResolver(openDeps.SessionLister, openDeps.AliasLookup, openDeps.Zoxide, openDeps.DirValidator), nil
	}

	store, err := loadAliasStore()
	if err != nil {
		return nil, err
	}

	zoxide := resolver.NewZoxideResolver(&resolver.RealCommandRunner{}, exec.LookPath)
	dirValidator := &resolver.OSDirValidator{}

	return resolver.NewQueryResolver(tmuxClient(cmd), store, zoxide, dirValidator), nil
}

func init() {
	openCmd.Flags().StringP("exec", "e", "", "command to execute in the new session")
	openCmd.Flags().StringP("filter", "f", "", "open the picker pre-filtered by <text> (skips resolution)")
	openCmd.Flags().StringP("session", "s", "", "attach the named session or session glob (session-domain; never mints)")
	openCmd.Flags().StringP("path", "p", "", "mint a new session at the given directory (path-domain; dir must exist)")
	openCmd.Flags().StringP("alias", "a", "", "mint a new session at the given alias key or key glob (alias-domain)")
	openCmd.Flags().StringP("zoxide", "z", "", "mint a new session at zoxide's best match (zoxide-domain; explicit error if zoxide is not installed)")
	openCmd.Flags().String("ack", "", "internal: <batch>:<token> — write the @portal-spawn-<batch>-<token> ack marker before the attach/mint handoff")
	_ = openCmd.Flags().MarkHidden("ack")

	// Tab completion: the bare positional and the
	// -s/--session pin complete session names via the shared completer; -a/--alias
	// completes the finite Portal-owned alias-key namespace via completeAliasKeys.
	// -p/--path and -z/--zoxide register NO completer, so cobra emits
	// ShellCompDirectiveDefault and the shell (and zoxide's own, for -z) provides
	// file/default completion — the deliberate ABSENCE is the intended behaviour.
	// The hidden --ack flag and the hidden state namespace are auto-excluded by
	// cobra — no work here beyond not un-hiding them.
	openCmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeSessionNames(toComplete)
	}
	_ = openCmd.RegisterFlagCompletionFunc("session", func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeSessionNames(toComplete)
	})
	_ = openCmd.RegisterFlagCompletionFunc("alias", func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeAliasKeys(toComplete)
	})

	rootCmd.AddCommand(openCmd)
}
