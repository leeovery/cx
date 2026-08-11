package tui

import "testing"

func TestModel_FlashState_ZeroValue(t *testing.T) {
	var m Model
	if m.flashText != "" {
		t.Fatalf("zero-value flashText: want %q, got %q", "", m.flashText)
	}
	if m.flashGen != 0 {
		t.Fatalf("zero-value flashGen: want 0, got %d", m.flashGen)
	}
}

func TestModel_SetFlash_SetsTextAndBumpsGen(t *testing.T) {
	var m Model
	m.setFlash("hello")
	if m.flashText != "hello" {
		t.Fatalf("flashText after setFlash: want %q, got %q", "hello", m.flashText)
	}
	if m.flashGen != 1 {
		t.Fatalf("flashGen after first setFlash: want 1, got %d", m.flashGen)
	}
}

func TestModel_SetFlash_GenIncrementsMonotonically(t *testing.T) {
	var m Model
	m.setFlash("a")
	m.setFlash("b")
	m.setFlash("c")
	if m.flashGen != 3 {
		t.Fatalf("flashGen after three setFlash calls: want 3, got %d", m.flashGen)
	}
	if m.flashText != "c" {
		t.Fatalf("flashText after three setFlash calls: want %q, got %q", "c", m.flashText)
	}
}

func TestModel_ClearFlash_ZerosTextLeavesGen(t *testing.T) {
	var m Model
	m.setFlash("x")
	if m.flashGen != 1 || m.flashText != "x" {
		t.Fatalf("setup invariant: want gen=1 text=%q, got gen=%d text=%q", "x", m.flashGen, m.flashText)
	}
	m.clearFlash()
	if m.flashText != "" {
		t.Fatalf("flashText after clearFlash: want %q, got %q", "", m.flashText)
	}
	if m.flashGen != 1 {
		t.Fatalf("flashGen after clearFlash: want 1 (unchanged), got %d", m.flashGen)
	}
}

func TestModel_ClearFlash_IdempotentOnAlreadyCleared(t *testing.T) {
	var m Model
	m.clearFlash()
	if m.flashText != "" {
		t.Fatalf("flashText after clearFlash on zero value: want %q, got %q", "", m.flashText)
	}
	if m.flashGen != 0 {
		t.Fatalf("flashGen after clearFlash on zero value: want 0, got %d", m.flashGen)
	}

	m.setFlash("once")
	m.clearFlash()
	m.clearFlash()
	if m.flashText != "" {
		t.Fatalf("flashText after repeated clearFlash: want %q, got %q", "", m.flashText)
	}
	if m.flashGen != 1 {
		t.Fatalf("flashGen after repeated clearFlash: want 1, got %d", m.flashGen)
	}
}

func TestModel_FlashState_SetClearSet(t *testing.T) {
	var m Model
	m.setFlash("a")
	m.clearFlash()
	m.setFlash("b")
	if m.flashText != "b" {
		t.Fatalf("flashText after set→clear→set: want %q, got %q", "b", m.flashText)
	}
	if m.flashGen != 2 {
		t.Fatalf("flashGen after set→clear→set: want 2, got %d", m.flashGen)
	}
}

func TestModel_SetSuccessFlash_InheritsTheWholeSetFlashSequence(t *testing.T) {
	const text = "__SUCCESS_PARITY__"

	warning := noticeBandModel("alpha-row")
	success := noticeBandModel("alpha-row")

	warning.setFlash(text)
	success.setSuccessFlash(text)

	if got, want := success.flashText, warning.flashText; got != want {
		t.Errorf("flashText after setSuccessFlash = %q, want setFlash's %q", got, want)
	}
	if got, want := success.flashGen, warning.flashGen; got != want {
		t.Errorf("flashGen after setSuccessFlash = %d, want setFlash's %d", got, want)
	}
	if got, want := success.flashOrigin, warning.flashOrigin; got != want {
		t.Errorf("flash origin after setSuccessFlash = %v, want setFlash's %v", got, want)
	}
	if warning.flashKind != flashWarning || success.flashKind != flashSuccess {
		t.Errorf("flash kinds are warning=%v success=%v, want the kind to be the ONE difference", warning.flashKind, success.flashKind)
	}

	if slot := success.sessionBandHeight(); slot == 0 {
		t.Fatal("a success flash reserved no band rows; the slot must be measured like every other flash's")
	}
	_, baseH := noticeBandModel("alpha-row").SessionListSize()
	_, warningH := warning.SessionListSize()
	_, successH := success.SessionListSize()
	if warningH == baseH {
		t.Fatalf("fixture: setFlash left the session list at its %d-row unflashed height, so the comparison below proves nothing", baseH)
	}
	if successH != warningH {
		t.Errorf("the session list is %d rows after setSuccessFlash and %d after setFlash — the success variant skipped the layout re-sync", successH, warningH)
	}
}

func TestModel_SetFlash_EmptyStringStillBumpsGen(t *testing.T) {
	var m Model
	m.setFlash("")
	if m.flashText != "" {
		t.Fatalf("flashText after setFlash(\"\"): want %q, got %q", "", m.flashText)
	}
	if m.flashGen != 1 {
		t.Fatalf("flashGen after setFlash(\"\"): want 1, got %d", m.flashGen)
	}
}
