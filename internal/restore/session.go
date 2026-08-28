// Package restore recreates each saved session's topology detached from
// Portal's persisted sessions.json, arming every pane via `respawn-pane -k` with
// the blocking `portal state hydrate` helper.
//
// The create-then-arm split is required: FIFO paths and skeleton-marker keys are
// derived from the live (window, pane) indices tmux assigned during creation
// rather than predicted, which is what makes restore robust to base-index /
// pane-base-index drift between save and restore.
package restore

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

type SessionRestorer struct {
	Client   *tmux.Client
	StateDir string
	Logger   *slog.Logger
}

// savedPaneArmInfo's paneToken is the pane's durable identity token as read
// from saved state — one value serving as both the re-stamped pane option and
// the baked hook key, since a second field would let the two drift. The firing
// path must never read it from the live server: baking from the snapshot keeps
// firing correct whatever the restore re-stamp does.
type savedPaneArmInfo struct {
	scrollAbs string
	paneToken string
}

// Restore returns the live pane coords, for the caller to thread into
// ApplyWindowGeometry and ApplySkeletonMarkers.
func (r *SessionRestorer) Restore(sess state.Session) ([]tmux.PaneCoord, error) {
	if len(sess.Windows) == 0 || len(sess.Windows[0].Panes) == 0 {
		return nil, fmt.Errorf("session %q: no windows/panes", sess.Name)
	}

	armInfos := r.collectArmInfos(sess)

	if err := r.createSkeleton(sess); err != nil {
		return nil, err
	}

	return r.armPanes(sess, armInfos)
}

func (r *SessionRestorer) collectArmInfos(sess state.Session) []savedPaneArmInfo {
	var infos []savedPaneArmInfo
	for _, w := range sess.Windows {
		for _, p := range w.Panes {
			infos = append(infos, savedPaneArmInfo{
				scrollAbs: filepath.Join(r.StateDir, p.ScrollbackFile),
				paneToken: p.PortalPaneID,
			})
		}
	}
	return infos
}

func (r *SessionRestorer) createSkeleton(sess state.Session) error {
	rootCWD := sess.Windows[0].Panes[0].CWD
	if err := r.Client.NewSessionWithCommand(sess.Name, rootCWD, ""); err != nil {
		return err
	}

	r.applyEnvironment(sess)

	// `<session>:` is the session's active window, always the most recently
	// created one, so splits land correctly with no predicted window index.
	target := fmt.Sprintf("%s:", sess.Name)

	for pj := 1; pj < len(sess.Windows[0].Panes); pj++ {
		p := sess.Windows[0].Panes[pj]
		if err := r.Client.SplitWindow(target, p.CWD, ""); err != nil {
			return err
		}
	}

	for wi := 1; wi < len(sess.Windows); wi++ {
		w := sess.Windows[wi]
		firstPane := w.Panes[0]
		if err := r.Client.NewWindow(target, w.Name, firstPane.CWD, ""); err != nil {
			return err
		}
		for pj := 1; pj < len(w.Panes); pj++ {
			p := w.Panes[pj]
			if err := r.Client.SplitWindow(target, p.CWD, ""); err != nil {
				return err
			}
		}
	}

	return nil
}

// armPanes uses respawn-pane rather than send-keys so the helper becomes the
// pane's initial process in one atomic call; under send-keys the default shell
// would first render its rc output and prompt above the replayed scrollback.
//
// Panes pair by structural position: list-panes and collectArmInfos both walk in
// (window, pane) order. Unlike geometry and markers, a FIFO or respawn failure
// aborts — without them the scrollback can never be hydrated.
func (r *SessionRestorer) armPanes(sess state.Session, armInfos []savedPaneArmInfo) ([]tmux.PaneCoord, error) {
	livePanes, err := r.Client.ListPanesInSession(sess.Name)
	if err != nil {
		return nil, fmt.Errorf("session %q: list live panes: %w", sess.Name, err)
	}

	if len(livePanes) != len(armInfos) {
		r.logger().Warn("live pane count differs from saved count (pairing up to shorter list)", "session", sess.Name)
	}

	pairCount := min(len(livePanes), len(armInfos))

	for i := range pairCount {
		live := livePanes[i]
		info := armInfos[i]

		liveKey := state.SanitizePaneKey(sess.Name, live.Window, live.Pane)
		liveTarget := tmux.PaneTarget(sess.Name, live.Window, live.Pane)
		fifo := state.FIFOPath(r.StateDir, liveKey)
		if err := state.CreateFIFO(fifo); err != nil {
			return nil, fmt.Errorf("session %q: %w", sess.Name, err)
		}

		r.stampPaneToken(sess.Name, liveKey, liveTarget, info.paneToken)

		hydrateCmd := buildHydrateCommand(fifo, info.scrollAbs, info.paneToken)
		if err := r.Client.RespawnPane(liveTarget, hydrateCmd); err != nil {
			return nil, fmt.Errorf("session %q: arm pane %s: %w", sess.Name, liveTarget, err)
		}
	}

	return livePanes, nil
}

// stampPaneToken re-establishes a pane's durable identity on the live server,
// which a tmux pane option cannot carry across a reboot. An empty saved token
// is skipped rather than written: a stamped "" reads back indistinguishably
// from absence. A failure costs that pane its hook rather than its session, so
// it degrades to a WARN instead of aborting the restore.
func (r *SessionRestorer) stampPaneToken(sessionName, liveKey, liveTarget, token string) {
	if token == "" {
		return
	}
	if err := r.Client.SetPaneOption(liveTarget, state.PortalPaneIDOption, token); err != nil {
		r.logger().Warn("set pane token failed", "session", sessionName, "pane_key", liveKey, "error", err)
	}
}

// ApplyWindowGeometry degrades locally on failure. Order matters: zoom is a
// toggle whose effect depends on the freshly-applied layout, so resize-pane -Z
// must follow select-layout.
func (r *SessionRestorer) ApplyWindowGeometry(sess state.Session, livePanes []tmux.PaneCoord) {
	start := time.Now()
	panes := len(livePanes)
	anomalous := 0

	groups := groupLivePanesBySavedWindow(sess, livePanes)

	for wi, win := range sess.Windows {
		group := groups[wi]
		if len(group) == 0 {
			// Not anomalous: the arm phase already warned about the mismatch.
			continue
		}
		liveWin := group[0].Window
		liveActivePane := group[activePanePosition(win.Panes)%len(group)].Pane

		if !r.applyLayoutWithFallback(sess.Name, liveWin, win.Layout) {
			anomalous++
		}
		if !r.applyActivePane(sess.Name, liveWin, liveActivePane) {
			anomalous++
		}
		if win.Zoomed {
			if !r.applyZoom(sess.Name, liveWin, liveActivePane) {
				anomalous++
			}
		}
	}

	r.logger().Info("geometry complete",
		"panes", panes,
		log.Took(start),
		"anomalous", anomalous,
	)
}

// groupLivePanesBySavedWindow drops live panes beyond the saved sequence, and
// yields an empty slice for an uncovered saved window.
func groupLivePanesBySavedWindow(sess state.Session, livePanes []tmux.PaneCoord) [][]tmux.PaneCoord {
	out := make([][]tmux.PaneCoord, len(sess.Windows))
	cursor := 0
	for wi, w := range sess.Windows {
		end := min(cursor+len(w.Panes), len(livePanes))
		if cursor < end {
			out[wi] = livePanes[cursor:end]
		}
		cursor = end
	}
	return out
}

func activePanePosition(panes []state.Pane) int {
	for i, p := range panes {
		if p.Active {
			return i
		}
	}
	return 0
}

// applyLayoutWithFallback reports false whenever the saved layout failed, even
// if the tiled fallback then succeeded: the saved geometry was still not applied.
func (r *SessionRestorer) applyLayoutWithFallback(session string, window int, layout string) bool {
	err := r.Client.SelectLayout(session, window, layout)
	if err == nil {
		return true
	}
	r.logger().Warn("select-layout failed; falling back to tiled", "session", session, "error", err)
	if err := r.Client.SelectLayout(session, window, "tiled"); err != nil {
		r.logger().Warn("select-layout tiled fallback also failed", "session", session, "error", err)
	}
	return false
}

func (r *SessionRestorer) applyActivePane(session string, window, pane int) bool {
	if err := r.Client.SelectPane(session, window, pane); err != nil {
		r.logger().Warn("select-pane failed", "session", session, "error", err)
		return false
	}
	return true
}

func (r *SessionRestorer) applyZoom(session string, window, pane int) bool {
	if err := r.Client.ResizePaneZoom(session, window, pane); err != nil {
		r.logger().Warn("resize-pane -Z failed", "session", session, "error", err)
		return false
	}
	return true
}

// ApplySkeletonMarkers sets `@portal-skeleton-<paneKey>` on every live pane; a
// per-pane failure still leaves the rest marked. It only ever writes markers —
// the hydrate helper unsets them after a successful dump.
func (r *SessionRestorer) ApplySkeletonMarkers(sess state.Session, livePanes []tmux.PaneCoord) {
	savedCount := countSavedPanes(sess)
	r.warnOnPaneCountMismatch(sess.Name, len(livePanes), savedCount)

	for _, live := range livePanes {
		liveKey := state.SanitizePaneKey(sess.Name, live.Window, live.Pane)
		r.setSkeletonMarker(sess.Name, liveKey)
	}
}

func countSavedPanes(sess state.Session) int {
	n := 0
	for _, w := range sess.Windows {
		n += len(w.Panes)
	}
	return n
}

func (r *SessionRestorer) warnOnPaneCountMismatch(name string, liveCount, savedCount int) {
	if liveCount == savedCount {
		return
	}
	r.logger().Warn("live pane count differs from saved count", "session", name)
}

func (r *SessionRestorer) setSkeletonMarker(sessionName, liveKey string) {
	if err := state.SetSkeletonMarker(r.Client, liveKey); err != nil {
		r.logger().Warn("set skeleton marker failed", "session", sessionName, "pane_key", liveKey, "error", err)
	}
}

// applyEnvironment sorts by key so the call sequence is deterministic.
func (r *SessionRestorer) applyEnvironment(sess state.Session) {
	if len(sess.Environment) == 0 {
		return
	}
	keys := make([]string, 0, len(sess.Environment))
	for k := range sess.Environment {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := r.Client.SetSessionEnvironment(sess.Name, k, sess.Environment[k]); err != nil {
			r.logger().Warn("set-environment failed", "session", sess.Name, "error", err)
		}
	}
}

// buildHydrateCommand takes no `exec` prefix: respawn already replaces the
// pane's process rather than stacking one. Every interpolated value is
// single-quoted so any bytes reach the helper's flag parser as one token.
func buildHydrateCommand(fifo, file, hookKey string) string {
	return fmt.Sprintf(
		"portal state hydrate --fifo %s --file %s --hook-key %s",
		shellQuoteSingle(fifo), shellQuoteSingle(file), shellQuoteSingle(hookKey),
	)
}

// shellQuoteSingle wraps s as one shell token, escaping embedded single quotes
// with the close-escape-reopen idiom.
func shellQuoteSingle(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
