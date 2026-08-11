package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/state"
)

type stepRecorder struct {
	calls []string

	EnsureServerErr       error
	RegisterErr           error
	SetErr                error
	SweepOrphanDaemonsErr error
	EnsureSaverErr        error
	RestoreCorrupt        bool
	RestoreErr            error
	EagerSignalHydrateErr error
	ClearErr              error
	CleanStaleMarkersErr  error
	SweepErr              error
	ServerStarted         bool
}

func (r *stepRecorder) EnsureServer() (bool, error) {
	r.calls = append(r.calls, "EnsureServer")
	return r.ServerStarted, r.EnsureServerErr
}

func (r *stepRecorder) RegisterPortalHooks() error {
	r.calls = append(r.calls, "RegisterPortalHooks")
	return r.RegisterErr
}

func (r *stepRecorder) Set() error {
	r.calls = append(r.calls, "Set")
	return r.SetErr
}

func (r *stepRecorder) Clear() error {
	r.calls = append(r.calls, "Clear")
	return r.ClearErr
}

func (r *stepRecorder) SweepOrphanDaemons() error {
	r.calls = append(r.calls, "SweepOrphanDaemons")
	return r.SweepOrphanDaemonsErr
}

func (r *stepRecorder) EnsureSaver() error {
	r.calls = append(r.calls, "EnsureSaver")
	return r.EnsureSaverErr
}

func (r *stepRecorder) Restore() (bool, error) {
	r.calls = append(r.calls, "Restore")
	return r.RestoreCorrupt, r.RestoreErr
}

func (r *stepRecorder) EagerSignalHydrate() error {
	r.calls = append(r.calls, "EagerSignalHydrate")
	return r.EagerSignalHydrateErr
}

func (r *stepRecorder) CleanStaleMarkers() error {
	r.calls = append(r.calls, "CleanStaleMarkers")
	return r.CleanStaleMarkersErr
}

func (r *stepRecorder) Sweep() error {
	r.calls = append(r.calls, "Sweep")
	return r.SweepErr
}

type RecordingLogger struct {
	debugs          []string
	debugComponents []string
	infos           []string
	infoComponents  []string
	warnings        []string
	warnComponents  []string
	errors          []string
	errorComponents []string

	shared *RecordingLogger
	bound  []slog.Attr
}

type recordingLoggerHandler struct {
	owner *RecordingLogger
	bound []slog.Attr
}

func (h *recordingLoggerHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *recordingLoggerHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Attr, 0, len(h.bound)+len(attrs))
	next = append(next, h.bound...)
	next = append(next, attrs...)
	return &recordingLoggerHandler{owner: h.owner, bound: next}
}

func (h *recordingLoggerHandler) WithGroup(_ string) slog.Handler {
	return &recordingLoggerHandler{owner: h.owner, bound: h.bound}
}

func (h *recordingLoggerHandler) Handle(_ context.Context, r slog.Record) error {
	return h.owner.record(h.bound, r)
}

func (l *RecordingLogger) Logger() *slog.Logger { return slog.New(l) }

func (l *RecordingLogger) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (l *RecordingLogger) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Attr, 0, len(l.bound)+len(attrs))
	next = append(next, l.bound...)
	next = append(next, attrs...)
	return &recordingLoggerHandler{owner: l.owner(), bound: next}
}

func (l *RecordingLogger) WithGroup(_ string) slog.Handler {
	return &recordingLoggerHandler{owner: l.owner(), bound: l.bound}
}

func (l *RecordingLogger) owner() *RecordingLogger {
	if l.shared != nil {
		return l.shared
	}
	return l
}

func (l *RecordingLogger) Handle(_ context.Context, r slog.Record) error {
	return l.record(l.bound, r)
}

func (l *RecordingLogger) record(bound []slog.Attr, r slog.Record) error {
	owner := l.owner()
	var component string
	var trailer strings.Builder
	emit := func(a slog.Attr) bool {
		if a.Key == "component" {
			component = a.Value.String()
			return true
		}
		trailer.WriteString(" ")
		trailer.WriteString(a.Key)
		trailer.WriteString("=")
		trailer.WriteString(a.Value.String())
		return true
	}
	for _, a := range bound {
		emit(a)
	}
	r.Attrs(func(a slog.Attr) bool { return emit(a) })
	msg := r.Message + trailer.String()
	switch r.Level {
	case slog.LevelDebug:
		owner.debugs = append(owner.debugs, msg)
		owner.debugComponents = append(owner.debugComponents, component)
	case slog.LevelInfo:
		owner.infos = append(owner.infos, msg)
		owner.infoComponents = append(owner.infoComponents, component)
	case slog.LevelWarn:
		owner.warnings = append(owner.warnings, msg)
		owner.warnComponents = append(owner.warnComponents, component)
	case slog.LevelError:
		owner.errors = append(owner.errors, msg)
		owner.errorComponents = append(owner.errorComponents, component)
	}
	return nil
}

func (l *RecordingLogger) AllEntries() []string {
	out := make([]string, 0, len(l.debugs)+len(l.infos)+len(l.warnings)+len(l.errors))
	for _, m := range l.debugs {
		out = append(out, "DEBUG: "+m)
	}
	for _, m := range l.infos {
		out = append(out, "INFO: "+m)
	}
	for _, m := range l.warnings {
		out = append(out, "WARN: "+m)
	}
	for _, m := range l.errors {
		out = append(out, "ERROR: "+m)
	}
	return out
}

func newOrchestrator(r *stepRecorder, logger *slog.Logger) *Orchestrator {
	return &Orchestrator{
		Server:        r,
		Hooks:         r,
		Restoring:     r,
		OrphanSweeper: r,
		Saver:         r,
		Restore:       r,
		EagerSignaler: r,
		StaleMarkers:  r,
		Sweeper:       r,
		Logger:        logger,
	}
}

func equalCalls(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestOrchestratorRun_executesStepsInSpecOrder(t *testing.T) {
	r := &stepRecorder{}
	o := newOrchestrator(r, nil)

	_, _, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	want := []string{
		"EnsureServer",
		"RegisterPortalHooks",
		"Set",
		"SweepOrphanDaemons",
		"EnsureSaver",
		"Restore",
		"EagerSignalHydrate",
		"Clear",
		"CleanStaleMarkers",
		"Sweep",
	}
	if !equalCalls(r.calls, want) {
		t.Errorf("call order = %v, want %v", r.calls, want)
	}
}

func TestOrchestratorRun_propagatesEnsureServerError(t *testing.T) {
	sentinel := errors.New("server boom")
	r := &stepRecorder{EnsureServerErr: sentinel}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	_, _, err := o.Run(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel, got %v", err)
	}
	var fatal *FatalError
	if !errors.As(err, &fatal) {
		t.Fatalf("expected *FatalError, got %T (%v)", err, err)
	}
	wantPrefix := "Portal failed to start tmux server: "
	if !strings.HasPrefix(fatal.UserMessage, wantPrefix) {
		t.Errorf("UserMessage = %q, want prefix %q", fatal.UserMessage, wantPrefix)
	}
	if !strings.Contains(fatal.UserMessage, "server boom") {
		t.Errorf("UserMessage = %q, want to contain underlying %q", fatal.UserMessage, "server boom")
	}
	if len(logger.errors) == 0 {
		t.Error("expected logger.Error to be called before fatal return")
	}
	want := []string{"EnsureServer"}
	if !equalCalls(r.calls, want) {
		t.Errorf("calls = %v, want %v", r.calls, want)
	}
}

func TestOrchestratorRun_propagatesRegisterHooksError(t *testing.T) {
	sentinel := errors.New("register boom")
	r := &stepRecorder{RegisterErr: sentinel}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	_, _, err := o.Run(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel, got %v", err)
	}
	var fatal *FatalError
	if !errors.As(err, &fatal) {
		t.Fatalf("expected *FatalError, got %T (%v)", err, err)
	}
	wantPrefix := "Portal failed to register tmux hooks: "
	if !strings.HasPrefix(fatal.UserMessage, wantPrefix) {
		t.Errorf("UserMessage = %q, want prefix %q", fatal.UserMessage, wantPrefix)
	}
	if !strings.Contains(fatal.UserMessage, "register boom") {
		t.Errorf("UserMessage = %q, want to contain underlying %q", fatal.UserMessage, "register boom")
	}
	if len(logger.errors) == 0 {
		t.Error("expected logger.Error to be called before fatal return")
	}
	want := []string{"EnsureServer", "RegisterPortalHooks"}
	if !equalCalls(r.calls, want) {
		t.Errorf("calls = %v, want %v", r.calls, want)
	}
}

func TestOrchestratorRun_propagatesSetRestoringErrorAndSkipsLaterSteps(t *testing.T) {
	sentinel := errors.New("set marker boom")
	r := &stepRecorder{SetErr: sentinel}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	_, _, err := o.Run(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel, got %v", err)
	}
	var fatal *FatalError
	if !errors.As(err, &fatal) {
		t.Fatalf("expected *FatalError, got %T (%v)", err, err)
	}
	wantPrefix := "Portal failed to set @portal-restoring marker: "
	if !strings.HasPrefix(fatal.UserMessage, wantPrefix) {
		t.Errorf("UserMessage = %q, want prefix %q", fatal.UserMessage, wantPrefix)
	}
	if !strings.Contains(fatal.UserMessage, "set marker boom") {
		t.Errorf("UserMessage = %q, want to contain underlying %q", fatal.UserMessage, "set marker boom")
	}
	if len(logger.errors) == 0 {
		t.Error("expected logger.Error to be called before fatal return")
	}
	for _, c := range r.calls {
		if c == "EnsureSaver" {
			t.Errorf("EnsureSaver must not run when Set fails; calls = %v", r.calls)
		}
		if c == "Restore" {
			t.Errorf("Restore must not run when Set fails; calls = %v", r.calls)
		}
	}
	found := slices.Contains(r.calls, "Set")
	if !found {
		t.Errorf("expected Set in calls, got %v", r.calls)
	}
}

func TestOrchestratorRun_continuesPastEnsureSaverFailureAndAppendsWarning(t *testing.T) {
	sentinel := errors.New("saver boom")
	r := &stepRecorder{EnsureSaverErr: sentinel}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	_, warnings, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("Run must not return saver failures; got %v", err)
	}
	wantWarning := SaverDownWarning()
	if len(warnings) != 1 {
		t.Fatalf("warnings len = %d, want 1; got %#v", len(warnings), warnings)
	}
	if len(warnings[0].Lines) != len(wantWarning.Lines) {
		t.Fatalf("warning Lines len = %d, want %d", len(warnings[0].Lines), len(wantWarning.Lines))
	}
	for i := range wantWarning.Lines {
		if warnings[0].Lines[i] != wantWarning.Lines[i] {
			t.Errorf("warning Lines[%d] = %q, want %q", i, warnings[0].Lines[i], wantWarning.Lines[i])
		}
	}
	want := []string{
		"EnsureServer",
		"RegisterPortalHooks",
		"Set",
		"SweepOrphanDaemons",
		"EnsureSaver",
		"Restore",
		"EagerSignalHydrate",
		"Clear",
		"CleanStaleMarkers",
		"Sweep",
	}
	if !equalCalls(r.calls, want) {
		t.Errorf("calls = %v, want %v", r.calls, want)
	}
	if len(logger.warnings) == 0 {
		t.Error("expected logger to record at least one warning")
	}
}

func TestOrchestratorRun_continuesPastEagerSignalHydrateFailure(t *testing.T) {
	sentinel := errors.New("eager-signal boom")
	r := &stepRecorder{EagerSignalHydrateErr: sentinel}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	_, _, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("EagerSignalHydrate failure must not propagate; got %v", err)
	}

	var fatal *FatalError
	if errors.As(err, &fatal) {
		t.Errorf("Run unexpectedly returned *FatalError on soft EagerSignalHydrate failure: %v", err)
	}
	if len(logger.warnings) == 0 {
		t.Fatal("expected logger.Warn to record the soft EagerSignalHydrate failure")
	}

	foundCause := false
	for _, msg := range logger.warnings {
		if strings.Contains(msg, "step failed") && strings.Contains(msg, "EagerSignalHydrate") && strings.Contains(msg, sentinel.Error()) {
			foundCause = true
			break
		}
	}
	if !foundCause {
		t.Errorf("expected a step-7 (EagerSignalHydrate) Warn message containing %q; got %v", sentinel.Error(), logger.warnings)
	}

	want := []string{
		"EnsureServer",
		"RegisterPortalHooks",
		"Set",
		"SweepOrphanDaemons",
		"EnsureSaver",
		"Restore",
		"EagerSignalHydrate",
		"Clear",
		"CleanStaleMarkers",
		"Sweep",
	}
	if !equalCalls(r.calls, want) {
		t.Errorf("calls = %v, want %v", r.calls, want)
	}
}

func TestOrchestratorRun_clearsRestoringEvenWhenRestoreFails(t *testing.T) {
	corruptErr := fmt.Errorf("restore: %w", state.ErrCorruptIndex)
	r := &stepRecorder{RestoreCorrupt: true, RestoreErr: corruptErr}
	o := newOrchestrator(r, nil)

	_, _, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("Run must treat corrupt-index restore as soft; got %v", err)
	}

	restoreIdx, clearIdx := -1, -1
	for i, c := range r.calls {
		if c == "Restore" {
			restoreIdx = i
		}
		if c == "Clear" {
			clearIdx = i
		}
	}
	if restoreIdx == -1 {
		t.Fatalf("Restore not called; calls = %v", r.calls)
	}
	if clearIdx == -1 {
		t.Fatalf("Clear not called; calls = %v", r.calls)
	}
	if clearIdx < restoreIdx {
		t.Errorf("Clear must run after Restore; calls = %v", r.calls)
	}
}

func TestOrchestratorRun_reportsClearRestoringFailureAsFatal(t *testing.T) {
	sentinel := errors.New("clear boom")
	r := &stepRecorder{ClearErr: sentinel}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	_, _, err := o.Run(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel, got %v", err)
	}
	var fatal *FatalError
	if !errors.As(err, &fatal) {
		t.Fatalf("expected *FatalError, got %T (%v)", err, err)
	}
	wantPrefix := "Portal failed to clear @portal-restoring marker: "
	if !strings.HasPrefix(fatal.UserMessage, wantPrefix) {
		t.Errorf("UserMessage = %q, want prefix %q", fatal.UserMessage, wantPrefix)
	}
	if !strings.Contains(fatal.UserMessage, "clear boom") {
		t.Errorf("UserMessage = %q, want to contain underlying %q", fatal.UserMessage, "clear boom")
	}
	if len(logger.errors) == 0 {
		t.Error("expected logger.Error to be called before fatal return")
	}
}

func TestOrchestratorRun_isIdempotentAcrossInvocations(t *testing.T) {
	r1 := &stepRecorder{}
	o := newOrchestrator(r1, nil)

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("first Run errored: %v", err)
	}
	want := []string{
		"EnsureServer",
		"RegisterPortalHooks",
		"Set",
		"SweepOrphanDaemons",
		"EnsureSaver",
		"Restore",
		"EagerSignalHydrate",
		"Clear",
		"CleanStaleMarkers",
		"Sweep",
	}
	if !equalCalls(r1.calls, want) {
		t.Errorf("first calls = %v, want %v", r1.calls, want)
	}

	r2 := &stepRecorder{}
	o.Server = r2
	o.Hooks = r2
	o.Restoring = r2
	o.OrphanSweeper = r2
	o.Saver = r2
	o.Restore = r2
	o.EagerSignaler = r2
	o.StaleMarkers = r2
	o.Sweeper = r2

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("second Run errored: %v", err)
	}
	if !equalCalls(r2.calls, want) {
		t.Errorf("second calls = %v, want %v", r2.calls, want)
	}
}

func TestOrchestratorRun_returnsServerStartedFlagFromEnsureServer(t *testing.T) {
	t.Run("true when EnsureServer reports started", func(t *testing.T) {
		r := &stepRecorder{ServerStarted: true}
		o := newOrchestrator(r, nil)

		started, _, err := o.Run(context.Background())
		if err != nil {
			t.Fatalf("Run errored: %v", err)
		}
		if !started {
			t.Error("expected serverStarted=true, got false")
		}
	})

	t.Run("false when EnsureServer reports not started", func(t *testing.T) {
		r := &stepRecorder{ServerStarted: false}
		o := newOrchestrator(r, nil)

		started, _, err := o.Run(context.Background())
		if err != nil {
			t.Fatalf("Run errored: %v", err)
		}
		if started {
			t.Error("expected serverStarted=false, got true")
		}
	})
}

func TestOrchestratorRun_doesNotCallEnsureSaverWhenSetFails(t *testing.T) {
	r := &stepRecorder{SetErr: errors.New("set boom")}
	o := newOrchestrator(r, nil)

	_, _, _ = o.Run(context.Background())

	for _, c := range r.calls {
		if c == "EnsureSaver" {
			t.Fatalf("EnsureSaver must not run when Set fails; calls = %v", r.calls)
		}
	}
}

func TestOrchestratorRun_ensureSaverFailureDoesNotProduceFatalError(t *testing.T) {
	sentinel := errors.New("saver boom")
	r := &stepRecorder{EnsureSaverErr: sentinel}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	_, warnings, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("Run must not surface saver failures; got %v", err)
	}

	var fatal *FatalError
	if errors.As(err, &fatal) {
		t.Errorf("Run unexpectedly returned *FatalError on soft saver failure: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings len = %d, want 1; got %#v", len(warnings), warnings)
	}
	if warnings[0].Lines[0] != SaverDownWarning().Lines[0] {
		t.Errorf("warnings[0] = %q, want SaverDownWarning", warnings[0].Lines[0])
	}
	if len(logger.errors) != 0 {
		t.Errorf("logger.Error must NOT be called for soft saver failure; got %v", logger.errors)
	}
	if len(logger.warnings) == 0 {
		t.Error("expected logger.Warn to be called for soft saver failure")
	}
}

func TestOrchestratorRun_appendsSaverDownWarningOnEnsureSaverFailure(t *testing.T) {
	r := &stepRecorder{EnsureSaverErr: errors.New("saver boom")}
	o := newOrchestrator(r, nil)

	_, warnings, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("Run must not return saver failures; got %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings len = %d, want 1; got %#v", len(warnings), warnings)
	}
	want := SaverDownWarning()
	if len(warnings[0].Lines) != len(want.Lines) {
		t.Fatalf("warning Lines len = %d, want %d", len(warnings[0].Lines), len(want.Lines))
	}
	for i := range want.Lines {
		if warnings[0].Lines[i] != want.Lines[i] {
			t.Errorf("warning Lines[%d] = %q, want %q", i, warnings[0].Lines[i], want.Lines[i])
		}
	}
}

func TestOrchestratorRun_appendsCorruptSessionsJSONWarningOnRestoreErrCorruptIndex(t *testing.T) {
	corruptErr := fmt.Errorf("restore: %w", state.ErrCorruptIndex)
	r := &stepRecorder{RestoreCorrupt: true, RestoreErr: corruptErr}
	o := newOrchestrator(r, nil)

	_, warnings, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("Run must treat ErrCorruptIndex as soft; got err=%v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings len = %d, want 1; got %#v", len(warnings), warnings)
	}
	want := CorruptSessionsJSONWarning()
	for i := range want.Lines {
		if warnings[0].Lines[i] != want.Lines[i] {
			t.Errorf("warning Lines[%d] = %q, want %q", i, warnings[0].Lines[i], want.Lines[i])
		}
	}
}

func TestOrchestratorRun_accumulatesMultipleSoftWarnings(t *testing.T) {
	r := &stepRecorder{
		EnsureSaverErr: errors.New("saver boom"),
		RestoreCorrupt: true,
		RestoreErr:     fmt.Errorf("restore: %w", state.ErrCorruptIndex),
	}
	o := newOrchestrator(r, nil)

	_, warnings, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("Run must treat both as soft; got err=%v", err)
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings len = %d, want 2; got %#v", len(warnings), warnings)
	}
	if warnings[0].Lines[0] != SaverDownWarning().Lines[0] {
		t.Errorf("warnings[0] = %q, want SaverDownWarning first", warnings[0].Lines[0])
	}
	if warnings[1].Lines[0] != CorruptSessionsJSONWarning().Lines[0] {
		t.Errorf("warnings[1] = %q, want CorruptSessionsJSONWarning second", warnings[1].Lines[0])
	}
}

func TestOrchestratorRun_doesNotReturnFatalErrorForCorruptIndex(t *testing.T) {
	r := &stepRecorder{
		RestoreCorrupt: true,
		RestoreErr:     fmt.Errorf("restore: %w", state.ErrCorruptIndex),
	}
	o := newOrchestrator(r, nil)

	_, _, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("expected nil err for soft corrupt-index path; got %v", err)
	}
}

func TestOrchestratorRun_doesNotEscalateNonCorruptRestoreError(t *testing.T) {
	sentinel := errors.New("restore boom")
	r := &stepRecorder{RestoreCorrupt: false, RestoreErr: sentinel}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	_, _, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("Run must NOT escalate a non-corrupt soft restore error; got %v", err)
	}
	if len(logger.warnings) == 0 {
		t.Error("expected logger.Warn to record the contract-violating soft failure")
	}
	if len(logger.errors) != 0 {
		t.Errorf("logger.Error must NOT be called for a soft restore failure; got %v", logger.errors)
	}
}

func TestOrchestratorRun_doesNotEmitCorruptWarningWhenCorruptFalse(t *testing.T) {
	r := &stepRecorder{RestoreCorrupt: false, RestoreErr: errors.New("soft boom")}
	o := newOrchestrator(r, nil)

	_, warnings, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("Run errored: %v", err)
	}
	for _, w := range warnings {
		if len(w.Lines) > 0 && w.Lines[0] == CorruptSessionsJSONWarning().Lines[0] {
			t.Errorf("must not emit CorruptSessionsJSONWarning when corrupt=false; got %#v", warnings)
		}
	}
}

func TestOrchestratorRun_emptyWarningsOnHappyPath(t *testing.T) {
	r := &stepRecorder{}
	o := newOrchestrator(r, nil)

	_, warnings, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("Run errored: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected zero warnings on happy path; got %#v", warnings)
	}
}

func TestOrchestratorRun_runsSweepAsFinalStepAfterClearAndCleanStaleMarkers(t *testing.T) {
	r := &stepRecorder{}
	o := newOrchestrator(r, nil)

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run errored: %v", err)
	}

	clearIdx, cleanMarkersIdx, sweepIdx := -1, -1, -1
	for i, c := range r.calls {
		switch c {
		case "Clear":
			clearIdx = i
		case "CleanStaleMarkers":
			cleanMarkersIdx = i
		case "Sweep":
			sweepIdx = i
		}
	}
	if clearIdx == -1 || cleanMarkersIdx == -1 || sweepIdx == -1 {
		t.Fatalf("expected Clear, CleanStaleMarkers, Sweep in calls; got %v", r.calls)
	}
	if clearIdx >= cleanMarkersIdx || cleanMarkersIdx >= sweepIdx {
		t.Errorf("expected ordering Clear < CleanStaleMarkers < Sweep; got Clear=%d CleanStaleMarkers=%d Sweep=%d (%v)",
			clearIdx, cleanMarkersIdx, sweepIdx, r.calls)
	}
	if got := r.calls[len(r.calls)-1]; got != "Sweep" {
		t.Errorf("expected Sweep to be the final recorded step; got %q (%v)", got, r.calls)
	}
}

func TestOrchestratorRun_continuesPastSweepFailure(t *testing.T) {
	sentinel := errors.New("sweep boom")
	r := &stepRecorder{SweepErr: sentinel}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	_, _, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("Sweep failure must not propagate; got %v", err)
	}

	var fatal *FatalError
	if errors.As(err, &fatal) {
		t.Errorf("Run unexpectedly returned *FatalError on soft sweep failure: %v", err)
	}
	if len(logger.warnings) == 0 {
		t.Fatal("expected logger.Warn to record the soft sweep failure")
	}

	foundCause := false
	for _, msg := range logger.warnings {
		if strings.Contains(msg, sentinel.Error()) && strings.Contains(msg, "SweepOrphanFIFOs") {
			foundCause = true
			break
		}
	}
	if !foundCause {
		t.Errorf("expected a step-10 Warn message containing %q; got %v", sentinel.Error(), logger.warnings)
	}

	if got := r.calls[len(r.calls)-1]; got != "Sweep" {
		t.Errorf("expected Sweep to be the final recorded step; got %q (%v)", got, r.calls)
	}
}

func TestOrchestratorRun_runsCleanStaleMarkersBetweenClearAndSweep(t *testing.T) {
	r := &stepRecorder{}
	o := newOrchestrator(r, nil)

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run errored: %v", err)
	}

	clearIdx, cleanMarkersIdx, sweepIdx := -1, -1, -1
	for i, c := range r.calls {
		switch c {
		case "Clear":
			clearIdx = i
		case "CleanStaleMarkers":
			cleanMarkersIdx = i
		case "Sweep":
			sweepIdx = i
		}
	}
	if clearIdx == -1 || cleanMarkersIdx == -1 || sweepIdx == -1 {
		t.Fatalf("expected Clear, CleanStaleMarkers, Sweep in calls; got %v", r.calls)
	}
	if clearIdx >= cleanMarkersIdx || cleanMarkersIdx >= sweepIdx {
		t.Errorf("expected ordering Clear < CleanStaleMarkers < Sweep; got Clear=%d CleanStaleMarkers=%d Sweep=%d (%v)",
			clearIdx, cleanMarkersIdx, sweepIdx, r.calls)
	}
}

func TestOrchestratorRun_continuesPastCleanStaleMarkersFailure(t *testing.T) {
	sentinel := errors.New("clean stale markers boom")
	r := &stepRecorder{CleanStaleMarkersErr: sentinel}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	_, _, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("CleanStaleMarkers failure must not propagate; got %v", err)
	}

	var fatal *FatalError
	if errors.As(err, &fatal) {
		t.Errorf("Run unexpectedly returned *FatalError on soft CleanStaleMarkers failure: %v", err)
	}
	if len(logger.warnings) == 0 {
		t.Fatal("expected logger.Warn to record the soft CleanStaleMarkers failure")
	}

	foundCause := false
	for _, msg := range logger.warnings {
		if strings.Contains(msg, sentinel.Error()) && strings.Contains(msg, "step failed") && strings.Contains(msg, "CleanStaleMarkers") {
			foundCause = true
			break
		}
	}
	if !foundCause {
		t.Errorf("expected a step-9 (CleanStaleMarkers) Warn message containing %q; got %v", sentinel.Error(), logger.warnings)
	}

	sweepRan := false
	for _, c := range r.calls {
		if c == "Sweep" {
			sweepRan = true
		}
	}
	if !sweepRan {
		t.Errorf("Sweep must run even when CleanStaleMarkers fails; calls = %v", r.calls)
	}
}

var _ Runner = (*Orchestrator)(nil)

func TestOrchestratorRun_emitsDebugLinePerExecutedStep(t *testing.T) {
	r := &stepRecorder{}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run errored: %v", err)
	}

	steps := []string{
		"EnsureServer",
		"RegisterPortalHooks",
		"Set",
		"SweepOrphanDaemons",
		"EnsureSaver",
		"Restore",
		"EagerSignalHydrate",
		"Clear",
		"CleanStaleMarkers",
		"Sweep",
	}
	for _, step := range steps {
		matches := 0
		for _, line := range logger.debugs {
			if strings.Contains(line, step) {
				matches++
			}
		}
		if matches < 1 {
			t.Errorf("step %q: expected ≥1 DEBUG line referencing it; got debugs=%v", step, logger.debugs)
		}
	}
}

func TestOrchestratorRun_stepEntryLinesEmitUnderBootstrapComponent(t *testing.T) {
	r := &stepRecorder{}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger().With("component", "bootstrap"))

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run errored: %v", err)
	}

	var stepEntries int
	for i, line := range logger.debugs {
		if !strings.Contains(line, "step entering") {
			continue
		}
		stepEntries++
		if logger.debugComponents[i] != "bootstrap" {
			t.Errorf("step-entry DEBUG[%d] component = %q, want %q (line=%q)",
				i, logger.debugComponents[i], "bootstrap", line)
		}
	}
	if stepEntries == 0 {
		t.Errorf("expected ≥1 'step entering' DEBUG line; got debugs=%v", logger.debugs)
	}
}

var closedStepNames = []string{
	"EnsureServer",
	"RegisterPortalHooks",
	"SetRestoring",
	"SweepOrphanDaemons",
	"EnsureSaver",
	"Restore",
	"EagerSignalHydrate",
	"ClearRestoring",
	"CleanStaleMarkers",
	"SweepOrphanFIFOs",
}

func stepCompleteNames(infos []string) []string {
	var out []string
	for _, line := range infos {
		if !strings.HasPrefix(line, "step complete ") {
			continue
		}
		const key = "step="
		_, after, ok := strings.Cut(line, key)
		if !ok {
			continue
		}
		rest := after
		if sp := strings.IndexByte(rest, ' '); sp != -1 {
			rest = rest[:sp]
		}
		out = append(out, rest)
	}
	return out
}

func TestOrchestratorRun_emitsStepCompletePerStepInOrder(t *testing.T) {
	r := &stepRecorder{}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run errored: %v", err)
	}

	got := stepCompleteNames(logger.infos)
	if !equalCalls(got, closedStepNames) {
		t.Errorf("step complete names = %v, want %v", got, closedStepNames)
	}
}

func TestOrchestratorRun_emitsStepCompleteUnderBootstrapComponent(t *testing.T) {
	r := &stepRecorder{}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger().With("component", "bootstrap"))

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run errored: %v", err)
	}

	var stepCompletes int
	for i, line := range logger.infos {
		if !strings.HasPrefix(line, "step complete ") {
			continue
		}
		stepCompletes++
		if logger.infoComponents[i] != "bootstrap" {
			t.Errorf("step-complete INFO[%d] component = %q, want %q (line=%q)",
				i, logger.infoComponents[i], "bootstrap", line)
		}
	}
	if stepCompletes != len(closedStepNames) {
		t.Errorf("step complete INFO count = %d, want %d; infos=%v",
			stepCompletes, len(closedStepNames), logger.infos)
	}
}

func TestOrchestratorRun_emitsOrchestrationCompleteOnCleanBootstrap(t *testing.T) {
	r := &stepRecorder{}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run errored: %v", err)
	}

	matches := 0
	for _, line := range logger.infos {
		if !strings.HasPrefix(line, "orchestration complete ") {
			continue
		}
		matches++
		if !strings.Contains(line, "steps=10") {
			t.Errorf("orchestration complete line missing steps=10: %q", line)
		}
		if !strings.Contains(line, "warnings=0") {
			t.Errorf("orchestration complete line missing warnings=0: %q", line)
		}
		if !strings.Contains(line, "took=") {
			t.Errorf("orchestration complete line missing took=: %q", line)
		}
	}
	if matches != 1 {
		t.Errorf("expected exactly one orchestration complete INFO; got %d (infos=%v)", matches, logger.infos)
	}
}

func TestOrchestratorRun_orchestrationCompleteReportsAccumulatedWarnings(t *testing.T) {
	r := &stepRecorder{
		EnsureSaverErr: errors.New("saver boom"),
		RestoreCorrupt: true,
		RestoreErr:     fmt.Errorf("restore: %w", state.ErrCorruptIndex),
	}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	_, warnings, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("Run must treat both as soft; got %v", err)
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings len = %d, want 2; got %#v", len(warnings), warnings)
	}

	matches := 0
	for _, line := range logger.infos {
		if !strings.HasPrefix(line, "orchestration complete ") {
			continue
		}
		matches++
		if !strings.Contains(line, "warnings=2") {
			t.Errorf("orchestration complete line missing warnings=2: %q", line)
		}
		if !strings.Contains(line, "steps=10") {
			t.Errorf("orchestration complete line missing steps=10: %q", line)
		}
	}
	if matches != 1 {
		t.Errorf("expected exactly one orchestration complete INFO; got %d (infos=%v)", matches, logger.infos)
	}
}

func TestOrchestratorRun_fatalStep1ShortCircuitsBeforeSummaries(t *testing.T) {
	r := &stepRecorder{EnsureServerErr: errors.New("server boom")}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	_, _, err := o.Run(context.Background())
	if err == nil {
		t.Fatal("expected fatal error, got nil")
	}

	for _, line := range logger.infos {
		if strings.HasPrefix(line, "step complete ") {
			t.Errorf("no step complete must fire on a fatal step-1 abort; got %q", line)
		}
		if strings.HasPrefix(line, "orchestration complete ") {
			t.Errorf("orchestration summary must not fire on a fatal step-1 abort; got %q", line)
		}
	}
	if len(logger.errors) == 0 {
		t.Error("expected the fatal ERROR line as the terminal record")
	}
}

func TestOrchestratorRun_fatalStep8ShortCircuitsBeforeSummary(t *testing.T) {
	r := &stepRecorder{ClearErr: errors.New("clear boom")}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	_, _, err := o.Run(context.Background())
	if err == nil {
		t.Fatal("expected fatal error, got nil")
	}

	for _, line := range logger.infos {
		if strings.HasPrefix(line, "orchestration complete ") {
			t.Errorf("orchestration summary must not fire on a fatal step-8 abort; got %q", line)
		}
	}

	got := stepCompleteNames(logger.infos)
	for _, name := range got {
		if name == "ClearRestoring" {
			t.Errorf("no step complete must fire for the aborting step 8 (ClearRestoring); got %v", got)
		}
	}
	wantBefore := []string{
		"EnsureServer",
		"RegisterPortalHooks",
		"SetRestoring",
		"SweepOrphanDaemons",
		"EnsureSaver",
		"Restore",
		"EagerSignalHydrate",
	}
	if !equalCalls(got, wantBefore) {
		t.Errorf("step complete names before step-8 abort = %v, want %v", got, wantBefore)
	}
	if len(logger.errors) == 0 {
		t.Error("expected the fatal ERROR line as the terminal record")
	}
}

func TestOrchestratorRun_retainsEnteringDebugWithNormalizedNames(t *testing.T) {
	r := &stepRecorder{}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run errored: %v", err)
	}

	var enteringNames []string
	for _, line := range logger.debugs {
		if !strings.HasPrefix(line, "step entering ") {
			continue
		}
		const key = "step="
		_, after, ok := strings.Cut(line, key)
		if !ok {
			continue
		}
		rest := after
		if sp := strings.IndexByte(rest, ' '); sp != -1 {
			rest = rest[:sp]
		}
		enteringNames = append(enteringNames, rest)
	}
	if !equalCalls(enteringNames, closedStepNames) {
		t.Errorf("entering DEBUG step names = %v, want %v", enteringNames, closedStepNames)
	}
	for _, line := range logger.debugs {
		if strings.Contains(line, "Set @portal-restoring") || strings.Contains(line, "Clear @portal-restoring") {
			t.Errorf("entering DEBUG must use normalized names, not legacy %q", line)
		}
	}
}

func countCalls(calls []string, name string) int {
	n := 0
	for _, c := range calls {
		if c == name {
			n++
		}
	}
	return n
}

func indexOf(calls []string, name string) int {
	for i, c := range calls {
		if c == name {
			return i
		}
	}
	return -1
}

func TestOrchestratorRun_invokesSweepOrphanDaemonsExactlyOnce(t *testing.T) {
	r := &stepRecorder{}
	o := newOrchestrator(r, nil)

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run errored: %v", err)
	}

	if got := countCalls(r.calls, "SweepOrphanDaemons"); got != 1 {
		t.Errorf("SweepOrphanDaemons call count = %d, want 1; calls=%v", got, r.calls)
	}
}

func TestOrchestratorRun_runsSweepOrphanDaemonsBetweenSetAndEnsureSaver(t *testing.T) {
	r := &stepRecorder{}
	o := newOrchestrator(r, nil)

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run errored: %v", err)
	}

	setIdx := indexOf(r.calls, "Set")
	sweepIdx := indexOf(r.calls, "SweepOrphanDaemons")
	saverIdx := indexOf(r.calls, "EnsureSaver")
	if setIdx == -1 || sweepIdx == -1 || saverIdx == -1 {
		t.Fatalf("expected Set, SweepOrphanDaemons, EnsureSaver in calls; got %v", r.calls)
	}
	if setIdx >= sweepIdx || sweepIdx >= saverIdx {
		t.Errorf("expected ordering Set < SweepOrphanDaemons < EnsureSaver; got Set=%d SweepOrphanDaemons=%d EnsureSaver=%d (%v)",
			setIdx, sweepIdx, saverIdx, r.calls)
	}
}

func TestOrchestratorRun_continuesPastSweepOrphanDaemonsFailure(t *testing.T) {
	sentinel := errors.New("orphan-sweep boom")
	r := &stepRecorder{SweepOrphanDaemonsErr: sentinel}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	_, _, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("SweepOrphanDaemons failure must not propagate; got %v", err)
	}

	var fatal *FatalError
	if errors.As(err, &fatal) {
		t.Errorf("Run unexpectedly returned *FatalError on soft SweepOrphanDaemons failure: %v", err)
	}
	if len(logger.warnings) == 0 {
		t.Fatal("expected logger.Warn to record the soft SweepOrphanDaemons failure")
	}

	foundCause := false
	for _, msg := range logger.warnings {
		if strings.Contains(msg, "step failed") && strings.Contains(msg, "SweepOrphanDaemons") && strings.Contains(msg, sentinel.Error()) {
			foundCause = true
			break
		}
	}
	if !foundCause {
		t.Errorf("expected a step-4 (SweepOrphanDaemons) Warn message containing %q; got %v", sentinel.Error(), logger.warnings)
	}

	want := []string{
		"EnsureServer",
		"RegisterPortalHooks",
		"Set",
		"SweepOrphanDaemons",
		"EnsureSaver",
		"Restore",
		"EagerSignalHydrate",
		"Clear",
		"CleanStaleMarkers",
		"Sweep",
	}
	if !equalCalls(r.calls, want) {
		t.Errorf("calls = %v, want %v", r.calls, want)
	}
}

func TestOrchestratorRun_sweepOrphanDaemonsHappyPathEmitsNoWarn(t *testing.T) {
	r := &stepRecorder{}
	logger := &RecordingLogger{}
	o := newOrchestrator(r, logger.Logger())

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run errored: %v", err)
	}

	for _, msg := range logger.warnings {
		if strings.Contains(msg, "SweepOrphanDaemons") {
			t.Errorf("nil-returning SweepOrphanDaemons must not emit a Warn entry; got %q", msg)
		}
	}
}
