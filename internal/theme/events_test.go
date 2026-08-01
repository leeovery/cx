package theme_test

import (
	"log/slog"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/theme"
)

// themeComponent is the log component §12.3 adds to the closed vocabulary. The
// loader never binds it — `cmd` does, on the paths where a theme is USED — so
// every test that asserts on the component binds it here exactly as production
// will.
const themeComponent = "theme"

// closedAttrKeys is §12.3's closed attr-key vocabulary for the `theme`
// component, restated here so a call site that invents a key fails the suite.
// Only four are reachable in this phase; `slot`, `count` and `rejected` belong
// to events Phases 5 and 8 add.
var closedAttrKeys = []string{"slug", "slot", "reason", "path", "token", "count", "rejected"}

// TestEventLogger_RejectionsAreWarn pins the level and the component of both of
// this phase's events, and their messages.
//
// WARN rather than INFO is a decision, not a default (§12.3): "doctor treats
// them as advisory for exit-code purposes, but 'your config did not work' is a
// warning in a log."
//
// The component is asserted through a REAL log.For binding rather than a bare
// capture logger, because the binding is the whole mechanism §12.3 describes:
// the loader emits and the caller decides, so `theme` reaches the line from the
// injected logger and from nowhere in this package.
func TestEventLogger_RejectionsAreWarn(t *testing.T) {
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)
	events := theme.NewEventLogger(log.For(themeComponent))

	events.Rejected("nord-lee", "/themes/nord-lee.theme", &theme.Rejection{Reason: theme.ReasonBadSyntax, Detail: "line 4: quoted value", Line: 4})
	events.DirectoryUnusable("/themes", &theme.Rejection{Reason: theme.ReasonUnreadable, Detail: "open /themes: permission denied"})

	records := sink.Records()
	if len(records) != 2 {
		t.Fatalf("emitted %d records, want 2: %+v", len(records), records)
	}
	if got, want := records[0].Msg, "rejected"; got != want {
		t.Errorf("rejection message = %q, want %q", got, want)
	}
	if got, want := records[1].Msg, "directory unusable"; got != want {
		t.Errorf("directory message = %q, want %q", got, want)
	}
	for _, record := range records {
		if record.Level != slog.LevelWarn {
			t.Errorf("record %q emitted at %v, want %v", record.Msg, record.Level, slog.LevelWarn)
		}
		if got := record.AttrString(t, "component"); got != themeComponent {
			t.Errorf("record %q component = %q, want %q", record.Msg, got, themeComponent)
		}
	}
}

// TestEventLogger_TokenAttrOnlyWhereReasonNamesOne pins the `token` attr against
// every one of §6.2's seven reasons: present for exactly the two that NAME a
// token — `missing tokens` and `bad colour` — and absent for the other five.
// This event is that attr's only consumer in the whole vocabulary, so the
// biconditional is the contract.
//
// The value is §14A's comma-separated list, the same one doctor prints, so a
// grep of the log and a doctor run name the same tokens. `missing tokens` is the
// one detail carrying a lead-in ("missing …"), and the list is what rides the
// attr — the reason is already its own attr, so repeating it inside the value
// would be noise.
func TestEventLogger_TokenAttrOnlyWhereReasonNamesOne(t *testing.T) {
	tests := []struct {
		reason    theme.Reason
		detail    string
		wantToken string
	}{
		{reason: theme.ReasonMissingTokens, detail: "missing text.primary, bg.subtle", wantToken: "text.primary, bg.subtle"},
		{reason: theme.ReasonBadColour, detail: "text.primary = #GGGGGG, canvas = blue", wantToken: "text.primary = #GGGGGG, canvas = blue"},
		{reason: theme.ReasonBadName},
		{reason: theme.ReasonReservedName},
		{reason: theme.ReasonUnreadable, detail: "open /themes/nord-lee.theme: permission denied"},
		{reason: theme.ReasonBadSyntax, detail: "line 12: duplicate key text.primary"},
		{reason: theme.ReasonNotFound},
	}

	for _, tt := range tests {
		t.Run(string(tt.reason), func(t *testing.T) {
			logger, sink := logtest.NewCaptureLogger(t)
			events := theme.NewEventLogger(logger)

			events.Rejected("nord-lee", "/themes/nord-lee.theme", &theme.Rejection{Reason: tt.reason, Detail: tt.detail})

			record := sink.OnlyRecord(t)
			if tt.wantToken == "" {
				if record.HasAttr("token") {
					t.Errorf("reason %q carried token=%q, want no token attr — the reason names none", tt.reason, record.AttrString(t, "token"))
				}
				return
			}
			if got := record.AttrString(t, "token"); got != tt.wantToken {
				t.Errorf("reason %q token = %q, want %q", tt.reason, got, tt.wantToken)
			}
		})
	}
}

// TestEventLogger_AttrKeysAreInTheClosedSet pins the exact attr keys of every
// shape this phase emits, and that each is drawn from §12.3's closed seven —
// the component's vocabulary is spec-governed and never extended at a call site.
//
// The logger is deliberately UNBOUND (no component), so the captured keys are
// exactly what this package passed rather than what a caller's binding added.
func TestEventLogger_AttrKeysAreInTheClosedSet(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	events := theme.NewEventLogger(logger)

	events.Rejected("nord-lee", "/themes/nord-lee.theme", &theme.Rejection{Reason: theme.ReasonBadColour, Detail: "canvas = blue"})
	events.Rejected("", "/themes/Nord.THEME", &theme.Rejection{Reason: theme.ReasonBadName})
	events.DirectoryUnusable("/themes", &theme.Rejection{Reason: theme.ReasonUnreadable, Detail: "open /themes: permission denied"})

	records := sink.Records()
	if len(records) != 3 {
		t.Fatalf("emitted %d records, want 3: %+v", len(records), records)
	}

	wantKeys := [][]string{
		{"slug", "reason", "token"},
		{"path", "reason"},
		{"path", "reason"},
	}
	for i, record := range records {
		if !slices.Equal(record.Keys, wantKeys[i]) {
			t.Errorf("record %d (%q) keys = %v, want %v", i, record.Msg, record.Keys, wantKeys[i])
		}
		for _, key := range record.Keys {
			if !slices.Contains(closedAttrKeys, key) {
				t.Errorf("record %d (%q) carries the attr key %q, which is not one of §12.3's closed keys %v", i, record.Msg, key, closedAttrKeys)
			}
		}
	}
}

// TestEventLogger_DiscardSilencesEverything pins the diagnose-shaped callers'
// contract: a seam constructed with log.Discard() writes NOTHING, for any
// sequence of calls including a full reject set (§12.3).
//
// That is what `portal doctor`, `portal theme export` and capturetool are
// constructed with, and doctor is the run most likely to hit every reason at
// once. The process handler is capturing everything for the duration of the
// test, so a record reaching the sink would mean the discard logger had been
// bypassed rather than merely being quiet.
func TestEventLogger_DiscardSilencesEverything(t *testing.T) {
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)
	events := theme.NewEventLogger(log.Discard())

	emitFullRejectSet(events)

	// The doctor/export/capturetool construction itself, driven over a directory
	// holding one broken file of every shape those callers actually meet.
	dir := t.TempDir()
	writeTheme(t, dir, "bad-colour.theme", withValue(themeLines(), "canvas", "blue"))
	writeTheme(t, dir, "missing.theme", withoutKey(themeLines(), "bg.subtle"))
	writeTheme(t, dir, "Nord.THEME", themeLines())
	loader := theme.NewLoader(theme.NewEventLogger(log.Discard()))
	for range 2 {
		entries, rejection := loader.Enumerate(dir)
		if rejection != nil {
			t.Fatalf("Enumerate(%q) reported the directory unusable: %v", dir, rejection)
		}
		// Asserted so the silence cannot be vacuous: every staged file really
		// did reject, and every rejection really did reach a silenced seam.
		if len(entries) != 3 {
			t.Fatalf("Enumerate(%q) returned %d entries, want 3", dir, len(entries))
		}
		for _, entry := range entries {
			if entry.Rejection == nil {
				t.Fatalf("entry %q loaded cleanly, want every staged file rejected", entry.Filename)
			}
		}
	}

	if lines := sink.Lines(); len(lines) != 0 {
		t.Errorf("a discard-backed event logger emitted %d records, want none:\n%s", len(lines), sink.Body())
	}
}

// TestEventLogger_NilLoggerIsSafe pins the nil-tolerant guard at the
// constructor (log.OrDiscard at entry, the internal/spawn precedent): a nil
// *slog.Logger neither panics — a nil logger panics on use — nor leaks records
// to the process handler.
func TestEventLogger_NilLoggerIsSafe(t *testing.T) {
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)
	events := theme.NewEventLogger(nil)

	emitFullRejectSet(events)

	if lines := sink.Lines(); len(lines) != 0 {
		t.Errorf("a nil-logger event logger emitted %d records, want none:\n%s", len(lines), sink.Body())
	}
}

// TestEventLogger_DedupsRejectedOnSlugAndReason pins the per-process dedup that
// keeps a forensic trail from turning into a running commentary: enumeration
// re-reads the directory on EVERY panel open (§5.8), so five opens over the same
// broken directory must produce one WARN per distinct slug+reason, not five sets
// of identical ones.
//
// It drives the whole wired path rather than the seam alone, so it also pins the
// task's other half: Enumerate emits exactly one `rejected` per REJECTED entry
// and nothing at all for the valid file beside them.
func TestEventLogger_DedupsRejectedOnSlugAndReason(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "bad-colour.theme", withValue(themeLines(), "canvas", "blue"))
	writeTheme(t, dir, "missing.theme", withoutKey(themeLines(), "bg.subtle"))
	writeTheme(t, dir, "valid.theme", themeLines())

	logger, sink := logtest.NewCaptureLogger(t)
	loader := theme.NewLoader(theme.NewEventLogger(logger))

	for open := range 5 {
		entries, rejection := loader.Enumerate(dir)
		if rejection != nil {
			t.Fatalf("open %d: Enumerate(%q) reported the directory unusable: %v", open, dir, rejection)
		}
		if len(entries) != 3 {
			t.Fatalf("open %d: Enumerate(%q) returned %d entries, want 3", open, dir, len(entries))
		}
	}

	records := sink.Records()
	if len(records) != 2 {
		t.Fatalf("five enumerations emitted %d records, want one per distinct slug+reason (2):\n%s", len(records), sink.Body())
	}

	wantSlugs := []string{"bad-colour", "missing"}
	wantReasons := []theme.Reason{theme.ReasonBadColour, theme.ReasonMissingTokens}
	for i, record := range records {
		if got, want := record.Msg, "rejected"; got != want {
			t.Errorf("record %d message = %q, want %q", i, got, want)
		}
		if got := record.AttrString(t, "slug"); got != wantSlugs[i] {
			t.Errorf("record %d slug = %q, want %q", i, got, wantSlugs[i])
		}
		if got, want := record.AttrString(t, "reason"), string(wantReasons[i]); got != want {
			t.Errorf("record %d reason = %q, want %q", i, got, want)
		}
	}
}

// TestEventLogger_DedupsOnPathWhenNoSlug pins the dedup key's other half: a
// `bad name` file yields no slug, so it is identified by `path` and dedups on
// path+reason.
//
// Without it the class most likely to recur across panel opens — a file whose
// name is wrong, which the user has no reason to have fixed between two opens —
// would be the one class with no dedup key at all. The `slug` attr must be
// absent rather than empty: an empty one would read as a theme called nothing.
func TestEventLogger_DedupsOnPathWhenNoSlug(t *testing.T) {
	dir := t.TempDir()
	path := writeTheme(t, dir, "Nord.THEME", themeLines())

	logger, sink := logtest.NewCaptureLogger(t)
	loader := theme.NewLoader(theme.NewEventLogger(logger))

	for range 5 {
		if _, rejection := loader.Enumerate(dir); rejection != nil {
			t.Fatalf("Enumerate(%q) reported the directory unusable: %v", dir, rejection)
		}
	}

	record := sink.OnlyRecord(t)
	if got, want := record.Msg, "rejected"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	if got := record.AttrString(t, "path"); got != path {
		t.Errorf("path = %q, want %q", got, path)
	}
	if record.HasAttr("slug") {
		t.Errorf("record carries slug=%q, want no slug attr — a bad name yields none", record.AttrString(t, "slug"))
	}
	if got, want := record.AttrString(t, "reason"), string(theme.ReasonBadName); got != want {
		t.Errorf("reason = %q, want %q", got, want)
	}
}

// TestEventLogger_DirectoryUnusableDedupsOnPathAndReason pins the same rule for
// §5.5's misconfigured directory: enumeration runs on every panel open, so
// without dedup a user with a bad directory would collect an identical WARN per
// open.
func TestEventLogger_DirectoryUnusableDedupsOnPathAndReason(t *testing.T) {
	dir := unreadableDir(t)

	logger, sink := logtest.NewCaptureLogger(t)
	loader := theme.NewLoader(theme.NewEventLogger(logger))

	for range 5 {
		if _, rejection := loader.Enumerate(dir); rejection == nil {
			t.Fatalf("Enumerate(%q) accepted the directory, the fixture is not unreadable", dir)
		}
	}

	record := sink.OnlyRecord(t)
	if got, want := record.Msg, "directory unusable"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	if got := record.AttrString(t, "path"); got != dir {
		t.Errorf("path = %q, want %q", got, dir)
	}
	if got, want := record.AttrString(t, "reason"), string(theme.ReasonUnreadable); got != want {
		t.Errorf("reason = %q, want %q", got, want)
	}
}

// TestEventLogger_SameSlugDifferentReasonEmitsTwice pins that dedup is per
// (identity, reason) PAIR rather than per file: the same theme reported for a
// different reason is a different problem and earns its own line.
//
// It is the mid-session shape §5.8 exists for — a user edits a file that was
// missing a token and introduces a bad colour instead — and a per-file key would
// silently swallow the second state.
func TestEventLogger_SameSlugDifferentReasonEmitsTwice(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	events := theme.NewEventLogger(logger)
	path := "/themes/nord-lee.theme"

	for range 2 {
		events.Rejected("nord-lee", path, &theme.Rejection{Reason: theme.ReasonMissingTokens, Detail: "missing canvas"})
		events.Rejected("nord-lee", path, &theme.Rejection{Reason: theme.ReasonBadColour, Detail: "canvas = blue"})
	}

	records := sink.Records()
	if len(records) != 2 {
		t.Fatalf("emitted %d records, want one per reason (2):\n%s", len(records), sink.Body())
	}
	wantReasons := []theme.Reason{theme.ReasonMissingTokens, theme.ReasonBadColour}
	for i, record := range records {
		if got, want := record.AttrString(t, "reason"), string(wantReasons[i]); got != want {
			t.Errorf("record %d reason = %q, want %q", i, got, want)
		}
	}
}

// TestEventLogger_FreshInstanceHasFreshDedupState pins where the dedup state
// lives: on the injected INSTANCE, not in package state in the leaf (§12.3).
//
// Both seams here write to the same logger, so only the instance boundary can
// separate them — and that is exactly how a test controls dedup, and how §8.9's
// concurrent Portal processes each keep their own.
func TestEventLogger_FreshInstanceHasFreshDedupState(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	rejection := &theme.Rejection{Reason: theme.ReasonMissingTokens, Detail: "missing canvas"}

	for range 2 {
		events := theme.NewEventLogger(logger)
		events.Rejected("nord-lee", "/themes/nord-lee.theme", rejection)
		events.Rejected("nord-lee", "/themes/nord-lee.theme", rejection)
	}

	if records := sink.Records(); len(records) != 2 {
		t.Fatalf("two separately constructed event loggers emitted %d records, want one each (2):\n%s", len(records), sink.Body())
	}
}

// TestEventLogger_ConcurrentEmissionIsRaceFree pins that the dedup set is safe
// under concurrent emission — several instances are normal (§8.9) and one TUI
// process may drive enumeration from more than one path (§5.5), so the
// check-and-record must be atomic rather than merely fast.
//
// Under -race this fails on an unsynchronised map; without it, the exactly-once
// counts still fail on a lost update between the check and the record.
func TestEventLogger_ConcurrentEmissionIsRaceFree(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	events := theme.NewEventLogger(logger)
	rejection := &theme.Rejection{Reason: theme.ReasonMissingTokens, Detail: "missing canvas"}

	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			for range 50 {
				events.Rejected("nord-lee", "/themes/nord-lee.theme", rejection)
				events.Rejected("", "/themes/Nord.THEME", &theme.Rejection{Reason: theme.ReasonBadName})
				events.DirectoryUnusable("/themes", &theme.Rejection{Reason: theme.ReasonUnreadable, Detail: "open /themes: permission denied"})
			}
		})
	}
	wg.Wait()

	if records := sink.Records(); len(records) != 3 {
		t.Fatalf("concurrent emission produced %d records, want one per distinct key (3):\n%s", len(records), sink.Body())
	}
}

// TestEnumerate_AbsentDirectoryEmitsNothing pins the one directory state that
// owes the log nothing: an absent themes directory is §5.5's common case and is
// silent — no doctor line and NO log entry, because zero drop-ins is not an
// error.
//
// It is asserted against a REAL component logger, so the silence is the loader's
// decision not to emit rather than a discarded record: this is the one place the
// loader chooses, and it chooses by reaching neither call site.
func TestEnumerate_AbsentDirectoryEmitsNothing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "themes")
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)
	loader := theme.NewLoader(theme.NewEventLogger(log.For(themeComponent)))

	for range 5 {
		entries, rejection := loader.Enumerate(dir)
		if rejection != nil {
			t.Fatalf("Enumerate(%q) = %v, want no rejection — an absent directory is silent", dir, rejection)
		}
		if len(entries) != 0 {
			t.Fatalf("Enumerate(%q) returned %d entries, want none", dir, len(entries))
		}
	}

	if lines := sink.Lines(); len(lines) != 0 {
		t.Errorf("an absent themes directory emitted %d records, want none:\n%s", len(lines), sink.Body())
	}
}

// emitFullRejectSet drives every event this phase emits over every §6.2 reason,
// under both identities and twice over — the sequence a silenced seam must
// produce nothing for.
func emitFullRejectSet(events *theme.EventLogger) {
	reasons := []theme.Reason{
		theme.ReasonBadName,
		theme.ReasonReservedName,
		theme.ReasonUnreadable,
		theme.ReasonBadSyntax,
		theme.ReasonBadColour,
		theme.ReasonMissingTokens,
		theme.ReasonNotFound,
	}

	for range 2 {
		for _, reason := range reasons {
			events.Rejected("nord-lee", "/themes/nord-lee.theme", &theme.Rejection{Reason: reason, Detail: "missing canvas"})
			events.Rejected("", "/themes/Nord.THEME", &theme.Rejection{Reason: reason, Detail: "missing canvas"})
			events.DirectoryUnusable("/themes", &theme.Rejection{Reason: reason, Detail: "open /themes: permission denied"})
		}
	}
}
