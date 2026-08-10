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
	"github.com/leeovery/portal/internal/themetest"
)

// themeComponent is the log component this feature adds to the closed vocabulary. The
// loader never binds it — `cmd` does, on the paths where a theme is USED — so
// every test that asserts on the component binds it here exactly as production
// will.
const themeComponent = "theme"

// closedAttrKeys is the closed attr-key vocabulary for the `theme`
// component, restated here so a call site that invents a key fails the suite.
// Five of the seven are reachable from the per-theme events below; `count` and
// `rejected` belong to `theme: enumerated`, the one event about a SET of themes,
// and are pinned with it in union_test.go.
var closedAttrKeys = []string{"slug", "slot", "reason", "path", "token", "count", "rejected"}

// TestEventLogger_RejectionsAreWarn pins the level and the component of both of
// this phase's events, and their messages.
//
// WARN rather than INFO is a decision, not a default: "doctor treats
// them as advisory for exit-code purposes, but 'your config did not work' is a
// warning in a log."
//
// The component is asserted through a REAL log.For binding rather than a bare
// capture logger, because the binding is the whole mechanism the `theme` log component
// describes: the loader emits and the caller decides, so `theme` reaches the line from the
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
// every one of the reason vocabulary's seven reasons: present for exactly the two that NAME a
// token — `missing tokens` and `bad colour` — and absent for the other five.
// This event is that attr's only consumer in the whole vocabulary, so the
// biconditional is the contract.
//
// The value is the comma-separated list doctor prints, so a grep of the log and
// a doctor run name the same tokens. `missing tokens` is the one detail carrying
// a lead-in ("missing …"), and the bare list is what rides the attr — the reason
// is already its own attr, so repeating it inside the value would be noise.
func TestEventLogger_TokenAttrOnlyWhereReasonNamesOne(t *testing.T) {
	tests := []struct {
		reason    theme.Reason
		detail    string
		tokens    []string
		wantToken string
	}{
		{
			reason:    theme.ReasonMissingTokens,
			detail:    "missing text.primary, bg.subtle",
			tokens:    []string{"text.primary", "bg.subtle"},
			wantToken: "text.primary, bg.subtle",
		},
		{
			reason:    theme.ReasonBadColour,
			detail:    "text.primary = #GGGGGG, canvas = blue",
			tokens:    []string{"text.primary = #GGGGGG", "canvas = blue"},
			wantToken: "text.primary = #GGGGGG, canvas = blue",
		},
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

			events.Rejected("nord-lee", "/themes/nord-lee.theme", &theme.Rejection{Reason: tt.reason, Detail: tt.detail, Tokens: tt.tokens})

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

// TestEventLogger_TokenAttrRendersFromTokensNotDetail pins where the attr's
// value comes from: the structured token list, never the rendered detail. Both
// details below are deliberately worded away from the copy they carry today —
// the lead-in reworded, the pairs rewrapped — and the attr is unmoved by either,
// so a copy edit cannot reach a log attr.
func TestEventLogger_TokenAttrRendersFromTokensNotDetail(t *testing.T) {
	tests := []struct {
		name      string
		rejection theme.Rejection
		wantToken string
	}{
		{
			name: "missing tokens behind reworded copy",
			rejection: theme.Rejection{
				Reason: theme.ReasonMissingTokens,
				Detail: "absent: text.primary, bg.subtle",
				Tokens: []string{"text.primary", "bg.subtle"},
			},
			wantToken: "text.primary, bg.subtle",
		},
		{
			name: "bad colour behind reworded copy",
			rejection: theme.Rejection{
				Reason: theme.ReasonBadColour,
				Detail: "canvas (blue) and text.primary (#GGGGGG) are not hex",
				Tokens: []string{"canvas = blue", "text.primary = #GGGGGG"},
			},
			wantToken: "canvas = blue, text.primary = #GGGGGG",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, sink := logtest.NewCaptureLogger(t)
			events := theme.NewEventLogger(logger)

			events.Rejected("nord-lee", "/themes/nord-lee.theme", &tt.rejection)

			if got := sink.OnlyRecord(t).AttrString(t, "token"); got != tt.wantToken {
				t.Errorf("token = %q, want %q", got, tt.wantToken)
			}
		})
	}
}

// TestEventLogger_AttrKeysAreInTheClosedSet pins the exact attr keys of every
// shape this phase emits, and that each is drawn from the `theme` log component's closed seven
// — the component's vocabulary is spec-governed and never extended at a call site.
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
// sequence of calls including a full reject set.
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
	themetest.Write(t, dir, "bad-colour.theme", themetest.WithValue(themetest.Lines(), "canvas", "blue"))
	themetest.Write(t, dir, "missing.theme", themetest.WithoutKey(themetest.Lines(), "bg.subtle"))
	themetest.Write(t, dir, "Nord.THEME", themetest.Lines())
	loader := theme.NewSilentLoader()
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
// re-reads the directory on EVERY panel open, so five opens over the same
// broken directory must produce one WARN per distinct slug+reason, not five sets
// of identical ones.
//
// It drives the whole wired path rather than the seam alone, so it also pins the
// task's other half: Enumerate emits exactly one `rejected` per REJECTED entry
// and nothing at all for the valid file beside them.
func TestEventLogger_DedupsRejectedOnSlugAndReason(t *testing.T) {
	dir := t.TempDir()
	themetest.Write(t, dir, "bad-colour.theme", themetest.WithValue(themetest.Lines(), "canvas", "blue"))
	themetest.Write(t, dir, "missing.theme", themetest.WithoutKey(themetest.Lines(), "bg.subtle"))
	themetest.Write(t, dir, "valid.theme", themetest.Lines())

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
	path := themetest.Write(t, dir, "Nord.THEME", themetest.Lines())

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
// the directory-resolution rule's misconfigured directory: enumeration runs on every panel
// open, so without dedup a user with a bad directory would collect an identical WARN per
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
// It is the mid-session shape the re-read-on-open rule exists for — a user edits a file that
// was missing a token and introduces a bad colour instead — and a per-file key would
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
// lives: on the injected INSTANCE, not in package state in the leaf.
//
// Both seams here write to the same logger, so only the instance boundary can
// separate them — and that is exactly how a test controls dedup, and how the concurrent-write
// rule's concurrent Portal processes each keep their own.
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
// under concurrent emission — several instances are normal and one TUI
// process may drive enumeration from more than one path, so the
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
// owes the log nothing: an absent themes directory is the directory-resolution rule's common
// case and is silent — no doctor line and NO log entry, because zero drop-ins is not an
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

// TestEvents_LoadedOncePerNomination pins the `theme` log component's cadence for `theme:
// loaded`: ONE line per NOMINATED theme — one under a constant, two under an adaptive
// pair — never one combined line carrying both.
//
// One line per nomination is what keeps `slug` and `slot` SINGLE-VALUED, which
// is the whole of the component's greppability claim: a combined line would make
// `grep "theme:"` answer with a pair a reader has to take apart, per theme, on
// the one surface that exists precisely so the user does not have to go looking.
//
// The count is asserted over EVERY record rather than over the `loaded` ones
// alone, so a clean resolution emitting a spurious warning beside them fails
// here rather than passing quietly.
func TestEvents_LoadedOncePerNomination(t *testing.T) {
	tests := []struct {
		name    string
		setting theme.Setting
		want    int
	}{
		{
			name:    "a constant nominates one theme",
			setting: theme.Setting{IsConstant: true, Constant: theme.DefaultDarkSlug},
			want:    1,
		},
		{
			name:    "a pair nominates two",
			setting: theme.Setting{Light: theme.DefaultLightSlug, Dark: theme.DefaultDarkSlug},
			want:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, sink := logtest.NewCaptureLogger(t)
			loader := theme.NewLoader(theme.NewEventLogger(logger))

			if _, err := loader.ResolveNomination(tt.setting, t.TempDir()); err != nil {
				t.Fatalf("ResolveNomination(%+v) = %v, want the nomination loaded", tt.setting, err)
			}

			records := sink.Records()
			if len(records) != tt.want {
				t.Fatalf("emitted %d records, want %d `loaded` and nothing else:\n%s", len(records), tt.want, sink.Body())
			}
			for i, record := range records {
				if got, want := record.Msg, "loaded"; got != want {
					t.Errorf("record %d message = %q, want %q", i, got, want)
				}
			}
		})
	}
}

// TestEvents_SlotAttrOnlyUnderAPair pins the `slot` attr to the setting state it
// describes: `light` or `dark` under an adaptive pair, and ABSENT under a
// constant.
//
// Absent rather than empty, and absent rather than the word "constant". The attr
// says WHICH HALF of a pair a line is about; a constant has no halves, so a value
// there would be a distinction the setting does not have — and an empty one would
// grep as a slot named nothing.
func TestEvents_SlotAttrOnlyUnderAPair(t *testing.T) {
	t.Run("a constant carries no slot attr", func(t *testing.T) {
		logger, sink := logtest.NewCaptureLogger(t)
		loader := theme.NewLoader(theme.NewEventLogger(logger))
		setting := theme.Setting{IsConstant: true, Constant: theme.DefaultDarkSlug}

		if _, err := loader.ResolveNomination(setting, t.TempDir()); err != nil {
			t.Fatalf("ResolveNomination(%+v) = %v, want the nomination loaded", setting, err)
		}

		record := sink.OnlyRecord(t)
		if got := record.AttrString(t, "slug"); got != theme.DefaultDarkSlug {
			t.Errorf("slug = %q, want %q", got, theme.DefaultDarkSlug)
		}
		if record.HasAttr("slot") {
			t.Errorf("a constant's `loaded` carries slot=%q, want no slot attr at all", record.AttrString(t, "slot"))
		}
	})

	t.Run("a pair carries one slot each", func(t *testing.T) {
		logger, sink := logtest.NewCaptureLogger(t)
		loader := theme.NewLoader(theme.NewEventLogger(logger))
		setting := theme.Setting{Light: theme.DefaultLightSlug, Dark: theme.DefaultDarkSlug}

		if _, err := loader.ResolveNomination(setting, t.TempDir()); err != nil {
			t.Fatalf("ResolveNomination(%+v) = %v, want both nominations loaded", setting, err)
		}

		records := sink.Records()
		if len(records) != 2 {
			t.Fatalf("emitted %d records, want one per slot (2):\n%s", len(records), sink.Body())
		}
		wantSlots := []string{"light", "dark"}
		wantSlugs := []string{theme.DefaultLightSlug, theme.DefaultDarkSlug}
		for i, record := range records {
			if got := record.AttrString(t, "slot"); got != wantSlots[i] {
				t.Errorf("record %d slot = %q, want %q", i, got, wantSlots[i])
			}
			if got := record.AttrString(t, "slug"); got != wantSlugs[i] {
				t.Errorf("record %d slug = %q, want %q", i, got, wantSlugs[i])
			}
		}
	})
}

// TestEvents_LoadedNamesTheFallbackSlug pins the rule the whole event exists to
// satisfy: when a nomination is unloadable, `theme: loaded` fires FOR THE
// FALLBACK, carrying the FALLBACK's slug.
//
// Without it, `theme: fallback applied` and `theme: loaded` both name the slug
// that FAILED, and a `grep "theme:"` on a broken install cannot answer which
// palette is actually rendering — which is the greppability the rejection-surface split
// justifies the whole component on.
//
// The surviving dark slot is asserted alongside it, so the light slot's fallback
// is shown not to have disturbed what the other slot reports.
func TestEvents_LoadedNamesTheFallbackSlug(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	loader := theme.NewLoader(theme.NewEventLogger(logger))
	setting := theme.Setting{Light: "gone-light", Dark: theme.DefaultDarkSlug}

	resolution, err := loader.ResolveNomination(setting, t.TempDir())
	if err != nil {
		t.Fatalf("ResolveNomination(%+v) = %v, want the fallback applied", setting, err)
	}
	// Non-vacuity: the assertions below say nothing unless the light slot really
	// did fall back rather than resolving `gone-light` somehow.
	if !resolution.Slots[0].FellBack {
		t.Fatalf("light slot = %+v, want the fallback applied — the fixture resolves the nomination", resolution.Slots[0])
	}

	loaded := recordsNamed(sink, "loaded")
	if len(loaded) != 2 {
		t.Fatalf("emitted %d `loaded` records, want one per slot (2):\n%s", len(loaded), sink.Body())
	}
	if got := loaded[0].AttrString(t, "slug"); got != theme.DefaultLightSlug {
		t.Errorf("the light slot's `loaded` names %q, want the fallback %q — the log must say which palette is rendering", got, theme.DefaultLightSlug)
	}
	if got, want := loaded[0].AttrString(t, "slot"), "light"; got != want {
		t.Errorf("the light slot's `loaded` slot = %q, want %q", got, want)
	}
	if got := loaded[1].AttrString(t, "slug"); got != theme.DefaultDarkSlug {
		t.Errorf("the dark slot's `loaded` names %q, want the surviving nomination %q", got, theme.DefaultDarkSlug)
	}
}

// TestEvents_FallbackAppliedNamesTheFailedSlug pins the other half of that pair:
// `theme: fallback applied` carries the slug that FAILED, the slot it failed in,
// and the terse reason it failed for.
//
// Without all three the line is not greppable, which is the whole reason the
// component earns its place. And the two events naming DIFFERENT slugs is
// asserted explicitly rather than left implied by the two slug assertions: naming
// the same slug twice is precisely the failure the rule exists to prevent, so a
// future change that made both name the nomination has to fail HERE rather than
// merely change what one of them says.
func TestEvents_FallbackAppliedNamesTheFailedSlug(t *testing.T) {
	dir := t.TempDir()
	themetest.Write(t, dir, "broken-light.theme", themetest.WithoutKey(themetest.Lines(), "bg.subtle"))
	logger, sink := logtest.NewCaptureLogger(t)
	loader := theme.NewLoader(theme.NewEventLogger(logger))
	setting := theme.Setting{Light: "broken-light", Dark: theme.DefaultDarkSlug}

	resolution, err := loader.ResolveNomination(setting, dir)
	if err != nil {
		t.Fatalf("ResolveNomination(%+v) = %v, want the fallback applied", setting, err)
	}
	if !resolution.Slots[0].FellBack {
		t.Fatalf("light slot = %+v, want the fallback applied — the fixture resolves the nomination", resolution.Slots[0])
	}

	applied := recordsNamed(sink, "fallback applied")
	if len(applied) != 1 {
		t.Fatalf("emitted %d `fallback applied` records, want one for the broken light slot:\n%s", len(applied), sink.Body())
	}
	if got, want := applied[0].AttrString(t, "slug"), "broken-light"; got != want {
		t.Errorf("slug = %q, want the nomination that failed, %q", got, want)
	}
	if got, want := applied[0].AttrString(t, "slot"), "light"; got != want {
		t.Errorf("slot = %q, want %q", got, want)
	}
	if got, want := applied[0].AttrString(t, "reason"), string(theme.ReasonMissingTokens); got != want {
		t.Errorf("reason = %q, want %q", got, want)
	}

	loaded := recordsNamed(sink, "loaded")
	if len(loaded) != 2 {
		t.Fatalf("emitted %d `loaded` records, want one per slot (2):\n%s", len(loaded), sink.Body())
	}
	if applied[0].AttrString(t, "slug") == loaded[0].AttrString(t, "slug") {
		t.Errorf("`fallback applied` and `loaded` both name %q; the failed nomination and the palette that rendered are different themes, and a grep on a broken install must tell them apart", loaded[0].AttrString(t, "slug"))
	}
}

// TestEvents_FallbackAppliedDedupsOnSlugAndReason pins the asymmetry between the
// two events over a REPEATED resolution: five resolutions of the same broken slug
// are ONE `fallback applied` and FIVE `loaded`.
//
// Five is the shape a live process actually produces — construction resolves once,
// every panel open resolves again and so does every `Esc` —
// so a per-fallback WARN would turn the passive forensic trail into running
// commentary about a condition the user was already told about once. The INFO
// does not dedup because it is per LOAD, not per condition: five loads happened,
// and the log says so.
func TestEvents_FallbackAppliedDedupsOnSlugAndReason(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	loader := theme.NewLoader(theme.NewEventLogger(logger))
	setting := theme.Setting{IsConstant: true, Constant: "gone"}

	for open := range 5 {
		resolution, err := loader.ResolveNomination(setting, t.TempDir())
		if err != nil {
			t.Fatalf("resolution %d: ResolveNomination(%+v) = %v, want the fallback applied", open, setting, err)
		}
		if !resolution.Slots[0].FellBack {
			t.Fatalf("resolution %d: slot = %+v, want the fallback applied", open, resolution.Slots[0])
		}
	}

	if applied := recordsNamed(sink, "fallback applied"); len(applied) != 1 {
		t.Errorf("five resolutions emitted %d `fallback applied` records, want one per slug+reason (1):\n%s", len(applied), sink.Body())
	}
	if loaded := recordsNamed(sink, "loaded"); len(loaded) != 5 {
		t.Errorf("five resolutions emitted %d `loaded` records, want one per load (5):\n%s", len(loaded), sink.Body())
	}
}

// TestEvents_FallbackDifferentReasonEmitsTwice pins that the WARN's dedup key is
// the (slug, reason) PAIR rather than the slug alone: the same theme failing for
// a DIFFERENT reason is a different problem and earns its own line.
//
// The fixture is the mid-session shape that makes it matter — the user, having
// seen the fallback, creates the file they were missing and gets a token wrong —
// which a per-slug key would swallow entirely, leaving the log asserting a
// condition that had already been fixed.
func TestEvents_FallbackDifferentReasonEmitsTwice(t *testing.T) {
	dir := t.TempDir()
	logger, sink := logtest.NewCaptureLogger(t)
	loader := theme.NewLoader(theme.NewEventLogger(logger))
	setting := theme.Setting{IsConstant: true, Constant: "nord-lee"}

	if _, err := loader.ResolveNomination(setting, dir); err != nil {
		t.Fatalf("ResolveNomination(%+v) = %v, want the fallback applied", setting, err)
	}
	themetest.Write(t, dir, "nord-lee.theme", themetest.WithoutKey(themetest.Lines(), "bg.subtle"))
	if _, err := loader.ResolveNomination(setting, dir); err != nil {
		t.Fatalf("ResolveNomination(%+v) after the file appeared = %v, want the fallback applied", setting, err)
	}

	applied := recordsNamed(sink, "fallback applied")
	if len(applied) != 2 {
		t.Fatalf("emitted %d `fallback applied` records, want one per reason (2):\n%s", len(applied), sink.Body())
	}
	wantReasons := []theme.Reason{theme.ReasonNotFound, theme.ReasonMissingTokens}
	for i, record := range applied {
		if got, want := record.AttrString(t, "slug"), "nord-lee"; got != want {
			t.Errorf("record %d slug = %q, want %q", i, got, want)
		}
		if got, want := record.AttrString(t, "reason"), string(wantReasons[i]); got != want {
			t.Errorf("record %d reason = %q, want %q", i, got, want)
		}
	}
}

// TestEvents_LoadedIsNotDeduplicated pins the INFO's cadence on a path with no
// failure anywhere near it: five resolutions of a perfectly good constant are
// five `loaded` records.
//
// It is per LOAD, not per condition — the commit-time load is the same
// event at a different cadence — so deduping it would make the log unable to say
// that a theme was loaded again at all.
func TestEvents_LoadedIsNotDeduplicated(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	loader := theme.NewLoader(theme.NewEventLogger(logger))
	setting := theme.Setting{IsConstant: true, Constant: theme.DefaultDarkSlug}

	for open := range 5 {
		if _, err := loader.ResolveNomination(setting, t.TempDir()); err != nil {
			t.Fatalf("resolution %d: ResolveNomination(%+v) = %v, want the nomination loaded", open, setting, err)
		}
	}

	records := sink.Records()
	if len(records) != 5 {
		t.Fatalf("five resolutions emitted %d records, want one `loaded` each (5):\n%s", len(records), sink.Body())
	}
	for i, record := range records {
		if got, want := record.Msg, "loaded"; got != want {
			t.Errorf("record %d message = %q, want %q", i, got, want)
		}
	}
}

// TestEvents_LevelsAreLoadedInfoFallbackWarn pins each event's level, its
// component, and the fixed ORDER the two are emitted in.
//
// WARN for the fallback and INFO for the load is the same judgement the `theme` log component
// makes for its neighbours: "your config did not work" is a warning in a log, while a
// theme loading is a fact.
//
// The order is part of the contract, not an accident of the call site: the
// failure is stated first and the palette that replaced it second, so a reader
// scanning the log meets the cause before the consequence.
//
// The component is asserted through a REAL log.For binding, because the binding
// is the whole mechanism — `theme` reaches the line from the injected logger and
// from nowhere in this package.
func TestEvents_LevelsAreLoadedInfoFallbackWarn(t *testing.T) {
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)
	loader := theme.NewLoader(theme.NewEventLogger(log.For(themeComponent)))
	setting := theme.Setting{IsConstant: true, Constant: "gone"}

	if _, err := loader.ResolveNomination(setting, t.TempDir()); err != nil {
		t.Fatalf("ResolveNomination(%+v) = %v, want the fallback applied", setting, err)
	}

	records := sink.Records()
	if len(records) != 2 {
		t.Fatalf("emitted %d records, want the fallback and the load it caused (2):\n%s", len(records), sink.Body())
	}
	wantMsgs := []string{"fallback applied", "loaded"}
	wantLevels := []slog.Level{slog.LevelWarn, slog.LevelInfo}
	for i, record := range records {
		if got := record.Msg; got != wantMsgs[i] {
			t.Errorf("record %d message = %q, want %q — the failure is stated before the palette that replaced it", i, got, wantMsgs[i])
		}
		if record.Level != wantLevels[i] {
			t.Errorf("record %q emitted at %v, want %v", record.Msg, record.Level, wantLevels[i])
		}
		if got := record.AttrString(t, "component"); got != themeComponent {
			t.Errorf("record %q component = %q, want %q", record.Msg, got, themeComponent)
		}
	}
}

// TestEvents_AttrKeysAreInTheClosedSet pins the exact attr keys of all FOUR
// shapes a resolution emits, and that each is drawn from the `theme` log component's closed
// seven — the component's vocabulary is spec-governed and never extended at a call site.
//
// Both events are driven under BOTH setting states, so the exact key set is what
// pins `slot` to the pair: a constant's `fallback applied` is {slug, reason} and
// its `loaded` is {slug}, with no `slot` on either. The attr says which half of a
// pair a line is about, and a constant has no halves — asserting that here, where
// the key set is exact, is what keeps the rule from being enforced on one of the
// two events and merely observed on the other.
//
// `count` and `rejected` are asserted absent by name as well as by the exact key
// sets: NOTHING IS ENUMERATED at construction, and those two keys belong to
// the panel's `theme: enumerated`, which counts its rows.
//
// The logger is deliberately UNBOUND (no component), so the captured keys are
// exactly what this package passed rather than what a caller's binding added.
func TestEvents_AttrKeysAreInTheClosedSet(t *testing.T) {
	dir := t.TempDir()
	themetest.Write(t, dir, "broken-light.theme", themetest.WithoutKey(themetest.Lines(), "bg.subtle"))
	logger, sink := logtest.NewCaptureLogger(t)
	loader := theme.NewLoader(theme.NewEventLogger(logger))

	pair := theme.Setting{Light: "broken-light", Dark: theme.DefaultDarkSlug}
	if _, err := loader.ResolveNomination(pair, dir); err != nil {
		t.Fatalf("ResolveNomination(%+v) = %v, want the fallback applied", pair, err)
	}
	constant := theme.Setting{IsConstant: true, Constant: theme.DefaultDarkSlug}
	if _, err := loader.ResolveNomination(constant, dir); err != nil {
		t.Fatalf("ResolveNomination(%+v) = %v, want the nomination loaded", constant, err)
	}
	brokenConstant := theme.Setting{IsConstant: true, Constant: "gone"}
	resolution, err := loader.ResolveNomination(brokenConstant, dir)
	if err != nil {
		t.Fatalf("ResolveNomination(%+v) = %v, want the fallback applied", brokenConstant, err)
	}
	// Non-vacuity: the last two key sets say nothing about a constant's fallback
	// unless the constant really did fall back.
	if !resolution.Slots[0].FellBack {
		t.Fatalf("constant slot = %+v, want the fallback applied — the fixture resolves the nomination", resolution.Slots[0])
	}

	records := sink.Records()
	wantKeys := [][]string{
		{"slug", "slot", "reason"},
		{"slug", "slot"},
		{"slug", "slot"},
		{"slug"},
		{"slug", "reason"},
		{"slug"},
	}
	if len(records) != len(wantKeys) {
		t.Fatalf("emitted %d records, want %d:\n%s", len(records), len(wantKeys), sink.Body())
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
		for _, enumerated := range []string{"count", "rejected"} {
			if record.HasAttr(enumerated) {
				t.Errorf("record %d (%q) carries %q; nothing is enumerated at construction", i, record.Msg, enumerated)
			}
		}
	}
}

// TestEvents_DiscardSilencesResolution pins the diagnose-shaped callers' contract
// over a WHOLE resolution — every setting state, fallbacks included: a seam
// constructed with log.Discard() writes nothing, and neither does a nil one.
//
// That is what `portal doctor`, `portal theme export` and capturetool are
// constructed with, and it is what leaves their per-process dedup state owned
// rather than dangling. The nil cases are the same silence reached two other
// ways: a nil *slog.Logger (log.OrDiscard at the constructor) and a nil
// *EventLogger (the zero-value Loader's own seam), neither of which may panic on
// a path that runs at every TUI construction.
//
// The process handler captures everything for the duration, so a record reaching
// the sink would mean the silence had been bypassed rather than merely quiet.
func TestEvents_DiscardSilencesResolution(t *testing.T) {
	tests := []struct {
		name   string
		events *theme.EventLogger
	}{
		{name: "a discard-backed seam", events: theme.NewEventLogger(log.Discard())},
		{name: "a nil logger", events: theme.NewEventLogger(nil)},
		{name: "a nil seam", events: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := &logtest.Sink{}
			log.SetTestHandler(t, sink)

			// Asserted so the silence cannot be vacuous: the sequence really did
			// include the fallbacks a silenced seam must stay quiet through.
			if fellBack := resolveEverySettingState(t, theme.NewLoader(tt.events)); fellBack != wantFallbacks {
				t.Fatalf("the resolution set produced %d fallbacks, want %d", fellBack, wantFallbacks)
			}

			if lines := sink.Lines(); len(lines) != 0 {
				t.Errorf("emitted %d records, want none:\n%s", len(lines), sink.Body())
			}
		})
	}
}

// TestEvents_FreshInstanceHasFreshDedupState pins where the fallback WARN's dedup
// state lives: on the injected INSTANCE, not in package state in the leaf.
//
// Both loaders here write to the same logger and resolve the same broken slug, so
// only the instance boundary can separate them — and that is exactly how the concurrent-write
// rule's concurrent Portal processes each keep their own, and how a test controls dedup
// at all.
func TestEvents_FreshInstanceHasFreshDedupState(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	setting := theme.Setting{IsConstant: true, Constant: "gone"}

	for process := range 2 {
		loader := theme.NewLoader(theme.NewEventLogger(logger))
		for open := range 2 {
			if _, err := loader.ResolveNomination(setting, t.TempDir()); err != nil {
				t.Fatalf("instance %d, resolution %d: ResolveNomination(%+v) = %v", process, open, setting, err)
			}
		}
	}

	if applied := recordsNamed(sink, "fallback applied"); len(applied) != 2 {
		t.Errorf("two separately constructed event loggers emitted %d `fallback applied` records, want one each (2):\n%s", len(applied), sink.Body())
	}
}

// wantFallbacks is how many slots resolveEverySettingState falls back in: both
// slots of the broken pair, plus the broken constant.
const wantFallbacks = 3

// resolveEverySettingState runs every outcome a resolution can have through
// loader — a clean pair, a pair whose slots both fall back, and a broken constant
// — and returns how many slots fell back, so a silence assertion can be shown to
// have covered them.
func resolveEverySettingState(t *testing.T, loader theme.Loader) int {
	t.Helper()

	settings := []theme.Setting{
		{Light: theme.DefaultLightSlug, Dark: theme.DefaultDarkSlug},
		{Light: "gone-light", Dark: "gone-dark"},
		{IsConstant: true, Constant: "gone"},
	}

	fellBack := 0
	for _, setting := range settings {
		resolution, err := loader.ResolveNomination(setting, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveNomination(%+v) = %v, want a resolution", setting, err)
		}
		for _, slot := range resolution.Slots {
			if slot.FellBack {
				fellBack++
			}
		}
	}
	return fellBack
}

// recordsNamed returns the captured records carrying the given message, in
// emission order.
//
// The events of one resolution interleave by design — a fallback's WARN is
// followed by the load it caused — so a per-slot assertion needs one catalogue at
// a time. The ORDER between the two is asserted where it belongs, in
// TestEvents_LevelsAreLoadedInfoFallbackWarn.
func recordsNamed(sink *logtest.Sink, msg string) []logtest.Record {
	named := []logtest.Record{}
	for _, record := range sink.Records() {
		if record.Msg == msg {
			named = append(named, record)
		}
	}
	return named
}

// emitFullRejectSet drives every event this phase emits over every terse reason,
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
