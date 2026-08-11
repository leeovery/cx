package tui

import "fmt"

// Must not import cmd/bootstrap (wrong import direction), so step identity is
// keyed off BootstrapProgressMsg.Index alone.

const (
	LabelStartedTmuxServer     = "Started tmux server"
	LabelRegisteredHooks       = "Registered hooks"
	LabelRestoringSessions     = "Restoring sessions"
	LabelReplayingScrollback   = "Replaying scrollback"
	LabelRunningResumeCommands = "Running resume commands"
)

// One increment per distinct completed step, not per label. Keep in lockstep
// with stepLabelTable.
const totalBootstrapSteps = 10

type LabelState int

const (
	LabelPending LabelState = iota
	LabelActive
	LabelDone
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
// events (RestoreM > 0) belong to "Restoring sessions", its completion tick here.
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

// LabelForStep returns "" for an out-of-range index.
func LabelForStep(e BootstrapProgressMsg) string {
	if e.Index == restoreStep && e.RestoreM > 0 {
		return LabelRestoringSessions
	}
	return stepLabelTable[e.Index]
}

// Counter is populated only for LabelRestoringSessions.
type LoadingLabel struct {
	Text    string
	State   LabelState
	Counter string
}

// Message is empty on the normal view; when FailedView populates it, one label
// carries LabelFailed.
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

// Apply is idempotent per step index, so a duplicate or out-of-order event
// never double-advances the bar. It assumes the producer emits every skeleton
// event (RestoreM > 0) before step 6's trailing completion tick.
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

// FailedView leaves no label failed when failedStep is out of range.
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

// A fatal step is never the dual-mapped restore step, so the static table is
// exact here. An out-of-range index returns "".
func LabelForStepIndex(index int) string {
	return stepLabelTable[index]
}

// A label is done once every constituent step completed; the active label is the
// first not-yet-done one, so a multi-step label stays active until its last step.
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
