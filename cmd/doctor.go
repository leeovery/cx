package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/spawn"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/spf13/cobra"
)

// ErrDoctorUnhealthy drives a non-zero exit and is deliberately silent on
// stderr — the rendered report is already on stdout.
var ErrDoctorUnhealthy = errors.New("doctor unhealthy")

const doctorRuntimeNotRunning = "Portal runtime not running — run portal open to start"

func runtimeDownResult(name string) checkResult {
	return checkResult{name: name, status: checkFail, detail: doctorRuntimeNotRunning}
}

type checkStatus int

const (
	// Zero value: an unset status counts as unhealthy, never as a passing check.
	checkUnknown checkStatus = iota
	checkPass
	checkFail
	// checkInfo and checkNotEvaluable never drive the exit code.
	checkInfo
	checkNotEvaluable
)

type checkResult struct {
	name   string
	status checkStatus
	detail string
}

// advisory sits outside the pass/fail catalog, and so outside the exit code.
// line is the whole rendered string, glyph included — the renderer only indents.
type advisory struct {
	line string
}

// DoctorDeps fields are all optional: an unset one falls through to the
// production default in resolveDoctorDeps, and nothing left nil aborts
// diagnosis.
type DoctorDeps struct {
	StateDir      string
	ThemesDir     string
	ServerRunning func() bool
	SaverPresent  func() (present bool, err error)
	HookCounts    func() (map[string]int, error)
	HookLister    AllPaneLister
	HookStore     *hooks.Store
	ProjectStore  *project.Store
	PrefsStore    *prefs.Store
	Detector      TerminalDetector
	Resolve       spawn.AdapterResolver
}

var doctorDeps *DoctorDeps

func resolveDoctorDeps() *DoctorDeps {
	client := tmux.DefaultClient()
	seams := buildProductionSpawnSeams(client)
	deps := &DoctorDeps{
		ServerRunning: client.ServerRunning,
		SaverPresent: func() (bool, error) {
			_, present, err := tmux.SaverPanePIDOrAbsent(client, tmux.PortalSaverName)
			return present, err
		},
		HookCounts: func() (map[string]int, error) {
			return tmux.PortalHookCountsByEvent(client)
		},
		HookLister: client,
		Detector:   seams.Detector,
		Resolve:    seams.Resolve,
	}
	if hookStore, err := loadHookStore(); err == nil {
		deps.HookStore = hookStore
	}
	if projectStore, err := loadProjectStore(); err == nil {
		deps.ProjectStore = projectStore
	}
	// Never the migrating loadPrefsStore: doctor heals nothing on the read-only
	// path, and that one dispatches the one-shot appearance translation.
	if prefsStore, err := loadPrefsStoreNoMigrate(); err == nil {
		deps.PrefsStore = prefsStore
	}
	if themesDir, err := themesDirPath(); err == nil {
		deps.ThemesDir = themesDir
	}
	if doctorDeps == nil {
		return deps
	}
	if doctorDeps.StateDir != "" {
		deps.StateDir = doctorDeps.StateDir
	}
	if doctorDeps.ThemesDir != "" {
		deps.ThemesDir = doctorDeps.ThemesDir
	}
	if doctorDeps.ServerRunning != nil {
		deps.ServerRunning = doctorDeps.ServerRunning
	}
	if doctorDeps.SaverPresent != nil {
		deps.SaverPresent = doctorDeps.SaverPresent
	}
	if doctorDeps.HookCounts != nil {
		deps.HookCounts = doctorDeps.HookCounts
	}
	if doctorDeps.HookLister != nil {
		deps.HookLister = doctorDeps.HookLister
	}
	if doctorDeps.HookStore != nil {
		deps.HookStore = doctorDeps.HookStore
	}
	if doctorDeps.ProjectStore != nil {
		deps.ProjectStore = doctorDeps.ProjectStore
	}
	if doctorDeps.PrefsStore != nil {
		deps.PrefsStore = doctorDeps.PrefsStore
	}
	if doctorDeps.Detector != nil {
		deps.Detector = doctorDeps.Detector
	}
	if doctorDeps.Resolve != nil {
		deps.Resolve = doctorDeps.Resolve
	}
	return deps
}

var doctorCmd = &cobra.Command{
	Use:           "doctor",
	Short:         "Diagnose Portal's health across the resurrection machinery",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := resolveDoctorDeps()
		results, err := runDoctorDiagnosis(deps)
		if err != nil {
			return err
		}
		renderDoctorReport(cmd.OutOrStdout(), results, collectThemeAdvisories(deps))

		fix, _ := cmd.Flags().GetBool("fix")
		if !fix {
			if doctorUnhealthy(results) {
				return ErrDoctorUnhealthy
			}
			return nil
		}

		// The exit is driven solely by the post-repair re-diagnosis below; the
		// repairs never touch it directly.
		if err := runDoctorFix(cmd, deps); err != nil {
			return err
		}
		postResults, err := runDoctorDiagnosis(deps)
		if err != nil {
			return err
		}
		// Re-collected, not carried down from the first pass: the whole second
		// report must describe one moment.
		renderDoctorReport(cmd.OutOrStdout(), postResults, collectThemeAdvisories(deps))
		if doctorUnhealthy(postResults) {
			return ErrDoctorUnhealthy
		}
		return nil
	},
}

// runDoctorFix applies only repairs that are reversible by reconstruction, each
// best-effort so a failure is left for the re-diagnosis to report. Themes get no
// repair step: every available one would destroy user-authored content.
func runDoctorFix(cmd *cobra.Command, deps *DoctorDeps) error {
	w := cmd.OutOrStdout()
	pruneDoctorStaleHooks(w, deps)
	pruneDoctorStaleProjects(w, deps)
	sweepDoctorLogs(deps)
	return nil
}

// runHookStaleCleanup's own mass-deletion hazard guard is the down-server
// protection here — do not add a second guard.
func pruneDoctorStaleHooks(w io.Writer, deps *DoctorDeps) {
	if deps.HookStore == nil {
		return
	}
	_ = runHookStaleCleanup(deps.HookLister, deps.HookStore, bootstrapLogger, func(key string) {
		_, _ = fmt.Fprintf(w, "Pruned stale hook: %s\n", key)
	})
}

func pruneDoctorStaleProjects(w io.Writer, deps *DoctorDeps) {
	if deps.ProjectStore == nil {
		return
	}
	removed, err := deps.ProjectStore.CleanStale()
	if err != nil {
		bootstrapLogger.Warn("doctor --fix: stale-project prune failed", "error", err)
		return
	}
	for _, p := range removed {
		_, _ = fmt.Fprintf(w, "Pruned stale project: %s (%s)\n", p.Name, p.Path)
	}
}

// sweepDoctorLogs is unconditional maintenance, not the repair of a diagnosed
// condition — there is no stale-logs check.
func sweepDoctorLogs(deps *DoctorDeps) {
	stateDir := deps.StateDir
	if stateDir == "" {
		dir, err := state.Dir()
		if err != nil {
			bootstrapLogger.Warn("doctor --fix: state dir unresolvable, skipping log sweep", "error", err)
			return
		}
		stateDir = dir
	}
	if err := log.SweepLogsForClean(stateDir); err != nil {
		bootstrapLogger.Warn("doctor --fix: log sweep failed", "error", err)
	}
}

// runDoctorDiagnosis resolves the state directory read-only — never EnsureDir;
// doctor creates nothing.
func runDoctorDiagnosis(deps *DoctorDeps) ([]checkResult, error) {
	dir := deps.StateDir
	var dirErr error
	if dir == "" {
		dir, dirErr = state.Dir()
	}

	serverUp := deps.ServerRunning()

	results := []checkResult{
		checkDaemonAlive(serverUp, dir, dirErr),
		checkSaverUp(serverUp, deps.SaverPresent),
		checkHooksRegistered(serverUp, deps.HookCounts),
		checkStateDirSane(dir, dirErr),
		checkSessionsJSON(dir, dirErr),
		checkStaleHooks(deps.HookLister, deps.HookStore),
		checkStaleProjects(deps.ProjectStore),
	}
	if deps.Detector != nil && deps.Resolve != nil {
		results = append(results, checkHostTerminal(deps.Detector, deps.Resolve))
	}
	return results, nil
}

// An unsupported or remote host is an environmental state, not a Portal-health
// defect, so this line never drives the exit code. A NULL identity short-circuits
// before Resolve, so a config `*` catch-all cannot reclassify a remote client.
func checkHostTerminal(detector TerminalDetector, resolve spawn.AdapterResolver) checkResult {
	const name = "host terminal"
	id := detector.Detect()
	if id.IsNull() {
		return checkResult{name: name, status: checkInfo, detail: "unsupported (remote session)"}
	}
	if _, resolution := resolve(id); resolution == spawn.ResolutionUnsupported {
		return checkResult{name: name, status: checkInfo, detail: fmt.Sprintf("%s (unsupported)", id.Name)}
	}
	return checkResult{name: name, status: checkInfo, detail: fmt.Sprintf("%s (supported)", id.Name)}
}

// The guards below must precede the stale count: an unreadable or empty live
// set would otherwise report every entry stale and mislead a --fix into
// mass-deleting user-authored on-resume commands.
func checkStaleHooks(lister AllPaneLister, store *hooks.Store) checkResult {
	const name = "stale hooks"
	if store == nil {
		return checkResult{name: name, status: checkNotEvaluable, detail: "could not read hooks.json"}
	}
	persisted, err := store.Load()
	if err != nil {
		return checkResult{name: name, status: checkNotEvaluable, detail: "could not read hooks.json"}
	}
	live, err := lister.ListAllPaneHookKeys()
	if err != nil {
		return checkResult{name: name, status: checkNotEvaluable, detail: "could not enumerate live panes"}
	}
	if len(live) == 0 {
		if len(persisted) == 0 {
			return checkResult{name: name, status: checkPass, detail: "no hooks"}
		}
		return checkResult{name: name, status: checkNotEvaluable, detail: "zero live panes with hooks present (not evaluable)"}
	}
	stale := len(hooks.StaleKeys(persisted, live))
	if stale > 0 {
		return checkResult{name: name, status: checkFail, detail: pluralCount(stale, "stale hook entry", "stale hook entries")}
	}
	return checkResult{name: name, status: checkPass, detail: "no stale hooks"}
}

func checkStaleProjects(store *project.Store) checkResult {
	const name = "stale projects"
	if store == nil {
		return checkResult{name: name, status: checkNotEvaluable, detail: "could not read projects.json"}
	}
	stale, err := store.StaleEntries()
	if err != nil {
		return checkResult{name: name, status: checkNotEvaluable, detail: "could not read projects.json"}
	}
	if len(stale) > 0 {
		return checkResult{name: name, status: checkFail, detail: pluralCount(len(stale), "stale project", "stale projects")}
	}
	return checkResult{name: name, status: checkPass, detail: "no stale projects"}
}

// checkDaemonAlive is a narrow state-file probe: it deliberately does not walk
// the state-dir tree or scan portal.log, so a routine doctor run stays cheap.
func checkDaemonAlive(serverUp bool, dir string, dirErr error) checkResult {
	const name = "daemon"
	if !serverUp {
		return runtimeDownResult(name)
	}
	if dirErr != nil {
		return checkResult{name: name, status: checkFail, detail: "not running"}
	}
	pid, err := state.ReadPIDFile(dir)
	if err != nil || !state.IsProcessAlive(pid) {
		return checkResult{name: name, status: checkFail, detail: "not running"}
	}
	version, _ := state.ReadVersionFile(dir)
	return checkResult{
		name:   name,
		status: checkPass,
		detail: fmt.Sprintf("running (pid %d, version %s)", pid, doctorDaemonVersion(version)),
	}
}

func checkSaverUp(serverUp bool, saverPresent func() (bool, error)) checkResult {
	const name = "saver"
	if !serverUp {
		return runtimeDownResult(name)
	}
	present, err := saverPresent()
	switch {
	case err != nil:
		return checkResult{name: name, status: checkNotEvaluable, detail: "could not read saver (transient tmux error)"}
	case present:
		return checkResult{name: name, status: checkPass, detail: "_portal-saver up"}
	default:
		return checkResult{name: name, status: checkFail, detail: "_portal-saver not running"}
	}
}

// Events are reported in sorted order so the message is deterministic, and
// duplicates ahead of missing entries: a stacked duplicate is the runaway-append
// failure this check exists to catch.
func checkHooksRegistered(serverUp bool, hookCounts func() (map[string]int, error)) checkResult {
	const name = "hooks"
	if !serverUp {
		return runtimeDownResult(name)
	}
	counts, err := hookCounts()
	if err != nil {
		return checkResult{name: name, status: checkNotEvaluable, detail: "could not read hooks (transient tmux error)"}
	}

	events := make([]string, 0, len(counts))
	for ev := range counts {
		events = append(events, ev)
	}
	sort.Strings(events)

	for _, ev := range events {
		if counts[ev] >= 2 {
			return checkResult{name: name, status: checkFail, detail: fmt.Sprintf("duplicate hook entries on %s (%d)", ev, counts[ev])}
		}
	}
	for _, ev := range events {
		if counts[ev] == 0 {
			return checkResult{name: name, status: checkFail, detail: fmt.Sprintf("hooks not registered on %s", ev)}
		}
	}
	return checkResult{name: name, status: checkPass, detail: "hooks registered (one per event)"}
}

func doctorDaemonVersion(v string) string {
	if v == "" {
		return "unknown"
	}
	return v
}

func pluralCount(n int, singular, plural string) string {
	unit := plural
	if n == 1 {
		unit = singular
	}
	return fmt.Sprintf("%d %s", n, unit)
}

// A not-yet-created state directory passes: a fresh install is healthy.
func checkStateDirSane(dir string, dirErr error) checkResult {
	const name = "state dir"
	if dirErr != nil {
		return checkResult{name: name, status: checkFail, detail: "unresolvable"}
	}
	info, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return checkResult{name: name, status: checkPass, detail: "not created yet"}
	case err != nil:
		return checkResult{name: name, status: checkFail, detail: "unreadable"}
	case info.IsDir():
		return checkResult{name: name, status: checkPass, detail: dir}
	default:
		return checkResult{name: name, status: checkFail, detail: "not a directory"}
	}
}

// state.ReadIndex, not the lossy HasLastSave: absent and corrupt must report
// differently.
func checkSessionsJSON(dir string, dirErr error) checkResult {
	const name = "sessions.json"
	if dirErr != nil {
		return checkResult{name: name, status: checkFail, detail: "unresolvable"}
	}
	idx, skip, err := state.ReadIndex(dir)
	switch {
	case err != nil:
		return checkResult{name: name, status: checkFail, detail: "sessions.json corrupt"}
	case skip:
		return checkResult{name: name, status: checkPass, detail: "no sessions saved yet"}
	default:
		return checkResult{
			name:   name,
			status: checkPass,
			detail: fmt.Sprintf("%s, %s", pluralCount(len(idx.Sessions), "session", "sessions"), pluralCount(state.CountPanes(idx), "pane", "panes")),
		}
	}
}

func renderDoctorReport(w io.Writer, results []checkResult, advisories []advisory) {
	_, _ = fmt.Fprintln(w, "Portal doctor:")
	for _, r := range results {
		_, _ = fmt.Fprintf(w, "  %s %s: %s\n", checkMarker(r.status), r.name, r.detail)
	}
	// The variable-length advisory block trails the catalog and never interleaves,
	// so a reader can expect a given check at a given place.
	for _, a := range advisories {
		_, _ = fmt.Fprintf(w, "  %s\n", a.line)
	}
	_, _ = fmt.Fprintf(w, "  %s\n", doctorSummaryLine(results, advisories))
}

func checkMarker(s checkStatus) string {
	switch s {
	case checkPass:
		return "✓"
	case checkFail:
		return "✗"
	case checkNotEvaluable:
		return "·"
	case checkInfo:
		return " "
	default:
		return " "
	}
}

func doctorUnhealthy(results []checkResult) bool {
	for _, r := range results {
		if r.status == checkFail || r.status == checkUnknown {
			return true
		}
	}
	return false
}

// doctorCheckCounts explains the exit code and never computes it, so its arms
// must agree with doctorUnhealthy.
func doctorCheckCounts(results []checkResult) (passed, total int) {
	for _, r := range results {
		switch r.status {
		case checkPass:
			passed++
			total++
		case checkFail, checkUnknown:
			total++
		case checkInfo, checkNotEvaluable:
			// Outside the exit-code class: counted by neither, deliberately.
		}
	}
	return passed, total
}

// The checks count deliberately skips pluralCount — doctor's catalog never has
// a single member — while the advisory count, which routinely does, takes it.
func doctorSummaryLine(results []checkResult, advisories []advisory) string {
	passed, total := doctorCheckCounts(results)
	summary := fmt.Sprintf("%d checks passed", passed)
	if passed != total {
		summary = fmt.Sprintf("%d of %d checks passed", passed, total)
	}
	if m := len(advisories); m > 0 {
		summary += " · " + pluralCount(m, "advisory", "advisories")
	}
	return summary
}

func init() {
	doctorCmd.Flags().Bool("fix", false, "apply low-stakes reversible repairs, then re-diagnose")
	rootCmd.AddCommand(doctorCmd)
}
