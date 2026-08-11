package tui

import "fmt"

// Maps the ten real bootstrap steps to the five friendly loading labels. Pure
// accumulator, deliberately free of the channel transport and render code; it
// must not import cmd/bootstrap (wrong import direction), so it keys off
// BootstrapProgressMsg.Index alone.

const (
	LabelStartedTmuxServer     = "Started tmux server"
	LabelRegisteredHooks       = "Registered hooks"
	LabelRestoringSessions     = "Restoring sessions"
	LabelReplayingScrollback   = "Replaying scrollback"
	LabelRunningResumeCommands = "Running resume commands"
)

// The bar advances 1/totalBootstrapSteps per distinct completed step — ten
// increments, not five. Keep in lockstep with stepLabelTable.
const totalBootstrapSteps = 10

// LabelState is the tick state of a friendly label: done, active, or pending.
type LabelState int

const (
	// LabelPending is the zero value: the label's steps have not started.
	LabelPending LabelState = iota
	// LabelActive: the current step falls in this label's group.
	LabelActive
	// LabelDone: every constituent step of this label has completed.
	LabelDone
	// LabelFailed: a fatal step in this label's group aborted the boot. Only
	// FailedView sets it; the normal View projection never does.
	LabelFailed
)

var labelOrder = []string{
	LabelStartedTmuxServer,
	LabelRegisteredHooks,
	LabelRestoringSessions,
	LabelReplayingScrollback,
	LabelRunningResumeCommands,
}

// Step 6 (Restore) dual-maps at runtime, not here: its per-session skeleton
// events (RestoreM > 0) belong to "Restoring sessions", its completion tick to
// the label below.
var stepLabelTable = map[int]string{
	1:  LabelStartedTmuxServer,
	2:  LabelRegisteredHooks,
	3:  LabelRegisteredHooks,
	4:  LabelRegisteredHooks,
	5:  LabelRegisteredHooks,
	6:  LabelReplayingScrollback,
	7:  LabelReplayingScrollback,
	8:  LabelRunningResumeCommands,
	9:  LabelRunningResumeCommands,
	10: LabelRunningResumeCommands,
}

const restoreStep = 6

// LabelForStep returns the friendly label a step event maps to; a skeleton
// per-session event maps to "Restoring sessions". An out-of-range index
// returns "".
func LabelForStep(e BootstrapProgressMsg) string {
	if e.Index == restoreStep && e.RestoreM > 0 {
		return LabelRestoringSessions
	}
	return stepLabelTable[e.Index]
}

// LoadingLabel is one row of the tick-list. Counter is only ever populated for
// LabelRestoringSessions.
type LoadingLabel struct {
	Text    string
	State   LabelState
	Counter string
}

// LoadingProgressView is the render input. Message is empty on the normal
// view; when FailedView populates it, exactly one label carries LabelFailed.
type LoadingProgressView struct {
	BarFraction float64
	Labels      []LoadingLabel
	Message     string
}

// LoadingProgress accumulates bootstrap progress: fold each message through
// Apply, then call View. The zero value is ready to use.
type LoadingProgress struct {
	completedSteps map[int]bool
	restoreN       int
	restoreM       int
}

// Apply folds one message in and returns the updated value, leaving the
// receiver untouched. Unmapped indices are ignored.
//
// Completion tracks distinct step indices, so a duplicate or out-of-order
// event never double-advances the bar. The producer emits every skeleton event
// (RestoreM > 0) before step 6's single trailing completion tick, so a
// skeleton event reliably means step 6 is unfinished: it advances the counter
// only, and the trailing tick is what marks the step done.
func (p LoadingProgress) Apply(e BootstrapProgressMsg) LoadingProgress {
	if _, mapped := stepLabelTable[e.Index]; !mapped {
		return p
	}

	next := p.clone()
	if e.Index == restoreStep && e.RestoreM > 0 {
		next.restoreN = e.RestoreN
		next.restoreM = e.RestoreM
	} else {
		next.completedSteps[e.Index] = true
	}
	return next
}

func (p LoadingProgress) clone() LoadingProgress {
	steps := make(map[int]bool, len(p.completedSteps)+1)
	for idx := range p.completedSteps {
		steps[idx] = true
	}
	return LoadingProgress{
		completedSteps: steps,
		restoreN:       p.restoreN,
		restoreM:       p.restoreM,
	}
}

// View projects the render inputs from the accumulated state.
func (p LoadingProgress) View() LoadingProgressView {
	v := LoadingProgressView{
		BarFraction: float64(len(p.completedSteps)) / float64(totalBootstrapSteps),
		Labels:      make([]LoadingLabel, 0, len(labelOrder)),
	}
	for _, text := range labelOrder {
		v.Labels = append(v.Labels, LoadingLabel{
			Text:    text,
			State:   p.labelState(text),
			Counter: p.counterFor(text),
		})
	}
	return v
}

// FailedView projects the fatal error frame: steps completed before the fatal
// stay done, the failed step's label flips to LabelFailed, and the bar freezes
// at the fraction reached. An out-of-range failedStep leaves no label failed.
func (p LoadingProgress) FailedView(failedStep int, message string) LoadingProgressView {
	failedLabel := LabelForStepIndex(failedStep)
	v := LoadingProgressView{
		BarFraction: float64(len(p.completedSteps)) / float64(totalBootstrapSteps),
		Labels:      make([]LoadingLabel, 0, len(labelOrder)),
		Message:     message,
	}
	for _, text := range labelOrder {
		state := p.labelState(text)
		if text == failedLabel {
			state = LabelFailed
		}
		v.Labels = append(v.Labels, LoadingLabel{
			Text:    text,
			State:   state,
			Counter: p.counterFor(text),
		})
	}
	return v
}

// LabelForStepIndex maps a 1-based step index to its label. The fatal steps
// are never the dual-mapped restore step, so the static mapping is exact. An
// out-of-range index returns "".
func LabelForStepIndex(index int) string {
	return stepLabelTable[index]
}

// Events signal step completion, so a label is done once every constituent
// step completed and the active label is the first not-yet-done one. This
// keeps a multi-step label active until its last step, and ticks a zero-item
// label done rather than stalled.
func (p LoadingProgress) labelState(text string) LabelState {
	if p.labelDone(text) {
		return LabelDone
	}
	if text == p.activeLabel() {
		return LabelActive
	}
	return LabelPending
}

func (p LoadingProgress) activeLabel() string {
	if len(p.completedSteps) == 0 {
		return ""
	}
	for _, text := range labelOrder {
		if !p.labelDone(text) {
			return text
		}
	}
	return ""
}

// "Restoring sessions" has no static table entry (step 6's slot is the
// scrollback label), so its completion keys off step 6 directly.
func (p LoadingProgress) labelDone(text string) bool {
	if text == LabelRestoringSessions {
		return p.completedSteps[restoreStep]
	}
	for idx := 1; idx <= totalBootstrapSteps; idx++ {
		if stepLabelTable[idx] != text {
			continue
		}
		if !p.completedSteps[idx] {
			return false
		}
	}
	return true
}

func (p LoadingProgress) counterFor(text string) string {
	if text != LabelRestoringSessions || p.restoreM == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d", p.restoreN, p.restoreM)
}
