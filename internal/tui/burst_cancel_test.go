package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/spawn"
	"github.com/leeovery/portal/internal/spawntest"
)

func cancellablePendingModel(t *testing.T, names ...string) (Model, *bool) {
	t.Helper()
	m := newPendingBurstModel(t, names)
	m.termWidth = 80
	m.termHeight = 24
	cancelled := false
	pipe := newBurstProgressPipe()
	pipe.ch <- burstProgress{Done: true}
	m.burstPipe = pipe
	m.burstCancel = func() { cancelled = true }
	return m, &cancelled
}

func driveCancelToTerminal(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	for range 200 {
		if cmd == nil {
			t.Fatal("cancelBurst must return the receiver so the terminal event is drained")
		}
		msg := cmd()
		updated, follow := m.Update(msg)
		m = updated.(Model)
		switch msg.(type) {
		case spawnProgressMsg:
			cmd = follow
		case spawnCompleteMsg, spawnAbortMsg:
			return m
		case burstChannelClosedMsg:
			t.Fatal("burst channel closed WITHOUT delivering the terminal event — the terminal send was dropped on cancel (the naked-terminal-send regression)")
		default:
			t.Fatalf("unexpected burst message %T", msg)
		}
	}
	t.Fatal("burst did not terminate within the step budget")
	return m
}

func realCancellableBurst(t *testing.T, names ...string) (Model, tea.Cmd, *spawntest.FakeAckChannel) {
	t.Helper()
	ack := &spawntest.FakeAckChannel{}
	confirm := make([]bool, len(names))
	adapter := &spawntest.FakeAdapter{Ack: ack, Confirm: confirm}
	m := NewModelWithSessions(sessionsFromNames(names))
	m.termWidth = 80
	m.termHeight = 24
	wireBurstSeams(&m, adapter, spawn.ResolutionNative, allPresent, ack)
	m = resolveDetection(t, m, ghosttyIdentity())
	m = enterMultiSelectEmpty(t, m)
	for i := range names {
		m = markRow(t, m, i)
	}
	m, cmd := pressEnter(t, m)
	if !m.BurstPending() {
		t.Fatal("precondition: the burst must be pending after dispatch")
	}
	return m, cmd, ack
}

func TestBurstCancel_CtrlCReturnsToMultiSelectNotQuit(t *testing.T) {
	m, cancelled := cancellablePendingModel(t, "alpha", "bravo", "charlie")

	updated, cmd := m.updateSessionList(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	m = updated.(Model)

	if !*cancelled {
		t.Error("Ctrl-C mid-burst must invoke burstCancel (cancel the goroutine ctx)")
	}
	if isQuitCmd(cmd) {
		t.Error("Ctrl-C mid-burst must NOT tea.Quit — the picker returns to multi-select mode")
	}
	if !m.multiSelectMode {
		t.Error("Ctrl-C mid-burst must stay in multi-select mode")
	}
	if !m.burstCancelled {
		t.Error("cancelBurst must set burstCancelled so the completion handler suppresses the flash + quit")
	}
	if !m.BurstPending() {
		t.Error("burstPending must stay true until the goroutine's terminal event lands")
	}
	if cmd == nil {
		t.Error("cancelBurst must return the receiver cmd so the terminal event is still drained")
	}
}

func TestBurstCancel_EscReturnsToMultiSelectNotQuit(t *testing.T) {
	m, cancelled := cancellablePendingModel(t, "alpha", "bravo")

	updated, cmd := m.updateSessionList(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated.(Model)

	if !*cancelled {
		t.Error("Esc mid-burst must invoke burstCancel")
	}
	if isQuitCmd(cmd) {
		t.Error("Esc mid-burst must not quit")
	}
	if !m.multiSelectMode {
		t.Error("Esc mid-burst must route to cancelBurst, NOT exit multi-select mode")
	}
	if !m.burstCancelled {
		t.Error("cancelBurst must set burstCancelled")
	}
	if !m.BurstPending() {
		t.Error("burstPending must stay true until the terminal event")
	}
	if cmd == nil {
		t.Error("cancelBurst must return the receiver cmd")
	}
}

func TestBurstCancel_BeforeFirstSpawnKeepsAllMarkedSilent(t *testing.T) {
	m := newPendingBurstModel(t, []string{"alpha", "bravo", "charlie"})
	m.burstCancelled = true
	before := m.SelectedSessionCount()

	rm, follow := injectComplete(t, m, spawnCompleteMsg{Batch: "b1", Results: nil})

	for _, n := range []string{"alpha", "bravo", "charlie"} {
		if !rm.IsSessionSelected(n) {
			t.Errorf("%q must stay marked after a cancel before the first spawn (nothing opened)", n)
		}
	}
	if got := rm.SelectedSessionCount(); got != before {
		t.Errorf("cancel before first spawn must leave the selection unchanged; count %d → %d", before, got)
	}
	if rm.flashText != "" {
		t.Errorf("cancel is user-initiated → silent; flashText = %q, want empty", rm.flashText)
	}
	if follow != nil {
		t.Error("cancel must not return a cmd (no self-attach, no tea.Quit)")
	}
	if rm.Selected() != "" {
		t.Errorf("cancel must not self-attach; Selected() = %q", rm.Selected())
	}
	if rm.BurstPending() {
		t.Error("the terminal event must clear burstPending (no permanent input-lock)")
	}
	if !rm.MultiSelectActive() {
		t.Error("cancel stays in multi-select mode")
	}
	if rm.burstCancelled {
		t.Error("burstCancelled must reset after the terminal event")
	}
}

func TestBurstCancel_AfterSomeOpenedUnmarksConfirmedKeepsRest(t *testing.T) {
	m := newPendingBurstModel(t, []string{"alpha", "bravo", "charlie", "delta"})
	m.burstCancelled = true
	msg := spawnCompleteMsg{
		Batch: "b1",
		Results: []spawn.WindowResult{
			{Session: "alpha", Ack: spawn.AckConfirmed, Result: spawn.Success("")},
			{Session: "bravo", Ack: spawn.AckTimeout, Result: spawn.Success("")},
		},
	}

	rm, follow := injectComplete(t, m, msg)

	if rm.IsSessionSelected("alpha") {
		t.Error("the confirmed alpha must be unmarked (its window opened; a retry must not re-open it)")
	}
	if !rm.IsSessionSelected("bravo") {
		t.Error("the ack-abandoned bravo must stay marked (a retry re-opens it)")
	}
	if !rm.IsSessionSelected("charlie") {
		t.Error("the un-attempted charlie must stay marked")
	}
	if !rm.IsSessionSelected("delta") {
		t.Error("the trigger delta must stay marked (no self-attach)")
	}
	if rm.flashText != "" {
		t.Errorf("cancel is silent; flashText = %q, want empty", rm.flashText)
	}
	if follow != nil {
		t.Error("cancel must not self-attach / quit")
	}
	if rm.BurstPending() {
		t.Error("the terminal event must clear burstPending")
	}
}

func TestBurstCancel_AllConfirmedRaceDoesNotSelfAttach(t *testing.T) {
	m := newPendingBurstModel(t, []string{"alpha", "bravo"})
	m.burstCancelled = true
	msg := spawnCompleteMsg{
		Batch: "b1",
		Results: []spawn.WindowResult{
			{Session: "alpha", Ack: spawn.AckConfirmed, Result: spawn.Success("")},
		},
	}

	rm, follow := injectComplete(t, m, msg)

	if follow != nil {
		t.Error("a cancel that races an all-confirmed terminal must NOT self-attach / quit")
	}
	if rm.Selected() != "" {
		t.Errorf("must not self-attach; Selected() = %q", rm.Selected())
	}
	if rm.IsSessionSelected("alpha") {
		t.Error("the confirmed alpha must be unmarked (leave-what-opened)")
	}
	if !rm.IsSessionSelected("bravo") {
		t.Error("the trigger bravo must stay marked")
	}
	if rm.flashText != "" {
		t.Errorf("cancel is silent; flashText = %q", rm.flashText)
	}
	if rm.BurstPending() {
		t.Error("burstPending must be cleared")
	}
}

func TestBurstCancel_SelfCleansBatchMarkersOnCancelPath(t *testing.T) {
	m, cmd, ack := realCancellableBurst(t, "alpha", "bravo", "charlie")

	updated, drainCmd := m.cancelBurst()
	m = updated.(Model)
	_ = cmd
	m = driveCancelToTerminal(t, m, drainCmd)

	if len(ack.Cleaned) != 1 {
		t.Errorf("the cancel path must self-clean the batch exactly once; ack.Cleaned = %v", ack.Cleaned)
	}
	if m.BurstPending() {
		t.Error("the delivered terminal event must clear burstPending on the cancel path")
	}
}

func TestBurstCancel_TerminalEventAlwaysDeliveredAfterCancel(t *testing.T) {
	m, cmd, _ := realCancellableBurst(t, "alpha", "bravo", "charlie", "delta")
	_ = cmd

	updated, drainCmd := m.cancelBurst()
	m = updated.(Model)
	if drainCmd == nil {
		t.Fatal("cancelBurst must return the receiver so the terminal event is drained")
	}

	m = driveCancelToTerminal(t, m, drainCmd)

	if m.BurstPending() {
		t.Error("no permanent input-lock: the reliably-delivered terminal event must clear burstPending")
	}
	if m.burstCancelled {
		t.Error("burstCancelled must reset once the terminal event lands")
	}
	if !m.MultiSelectActive() {
		t.Error("cancellation returns to multi-select mode")
	}
}

func TestBurstCancel_CtrlCLiveWhileInputLockedCancelsNotQuits(t *testing.T) {
	m, cancelled := cancellablePendingModel(t, "alpha", "bravo")

	updated, enterCmd := m.updateSessionList(tea.KeyPressMsg{Code: tea.KeyEnter})
	locked := updated.(Model)
	if enterCmd != nil {
		t.Error("Enter must be swallowed while input-locked (nil cmd)")
	}
	if *cancelled {
		t.Error("Enter must not cancel the burst")
	}
	if !locked.BurstPending() {
		t.Error("Enter must leave the burst pending")
	}

	updated, ctrlCmd := locked.updateSessionList(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	m = updated.(Model)
	if !*cancelled {
		t.Error("Ctrl-C must stay live while input-locked and invoke burstCancel")
	}
	if isQuitCmd(ctrlCmd) {
		t.Error("Ctrl-C while input-locked must cancel, NOT quit")
	}
	if !m.multiSelectMode {
		t.Error("Ctrl-C while input-locked must stay in multi-select mode")
	}
	if !m.burstCancelled {
		t.Error("Ctrl-C while input-locked must flag burstCancelled")
	}
}

func TestBurstPartialFailureFlash_DegenerateEmptyFailedNoFlash(t *testing.T) {
	got := burstPartialFailureFlash(
		[]spawn.WindowResult{{Session: "alpha", Ack: spawn.AckConfirmed, Result: spawn.Success("")}},
	)
	if got != "" {
		t.Errorf("degenerate (no failed windows, no permission wall) must yield no flash; got %q", got)
	}
}

func TestBurstPartialFailure_DegenerateEmptyFailedRendersNoBand(t *testing.T) {
	m := newPendingBurstModel(t, []string{"alpha", "bravo", "charlie"})
	msg := spawnCompleteMsg{
		Batch:   "b1",
		Results: []spawn.WindowResult{{Session: "alpha", Ack: spawn.AckConfirmed, Result: spawn.Success("")}},
	}

	rm, _ := injectComplete(t, m, msg)

	if rm.flashText != "" {
		t.Errorf("a degenerate empty-failed partial must render no flash band; flashText = %q", rm.flashText)
	}
	if rm.IsSessionSelected("alpha") {
		t.Error("the confirmed alpha must be unmarked")
	}
	if !rm.IsSessionSelected("bravo") {
		t.Error("the un-attempted bravo must stay marked")
	}
}
