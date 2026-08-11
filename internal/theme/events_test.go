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

const themeComponent = "theme"

var closedAttrKeys = []string{"slug", "slot", "reason", "path", "token", "count", "rejected"}

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

func TestEventLogger_TokenAttrOnlyWhereReasonNamesOne(t *testing.T) {
	tests := []struct {
		reason    theme.Reason
		detail    string
		tokens    []string
		values    []string
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
			tokens:    []string{"text.primary", "canvas"},
			values:    []string{"#GGGGGG", "blue"},
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

			events.Rejected("nord-lee", "/themes/nord-lee.theme", &theme.Rejection{Reason: tt.reason, Detail: tt.detail, Tokens: tt.tokens, Values: tt.values})

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
				Tokens: []string{"canvas", "text.primary"},
				Values: []string{"blue", "#GGGGGG"},
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
				t.Errorf("record %d (%q) carries the attr key %q, which is not one of the closed keys %v", i, record.Msg, key, closedAttrKeys)
			}
		}
	}
}

func TestEventLogger_DiscardSilencesEverything(t *testing.T) {
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)
	events := theme.NewEventLogger(log.Discard())

	emitFullRejectSet(events)

	dir := t.TempDir()
	themetest.WriteWithCanvas(t, dir, "bad-colour.theme", "blue")
	themetest.Write(t, dir, "missing.theme", themetest.WithoutKey(themetest.Lines(), "bg.subtle"))
	themetest.Write(t, dir, "Nord.THEME", themetest.Lines())
	loader := theme.NewSilentLoader()
	for range 2 {
		entries, rejection := loader.Enumerate(dir)
		if rejection != nil {
			t.Fatalf("Enumerate(%q) reported the directory unusable: %v", dir, rejection)
		}
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

func TestEventLogger_NilLoggerIsSafe(t *testing.T) {
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)
	events := theme.NewEventLogger(nil)

	emitFullRejectSet(events)

	if lines := sink.Lines(); len(lines) != 0 {
		t.Errorf("a nil-logger event logger emitted %d records, want none:\n%s", len(lines), sink.Body())
	}
}

func TestEventLogger_DedupsRejectedOnSlugAndReason(t *testing.T) {
	dir := t.TempDir()
	themetest.WriteWithCanvas(t, dir, "bad-colour.theme", "blue")
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

func TestEventLogger_DirectoryUnusableDedupsOnPathAndReason(t *testing.T) {
	dir := themesDirWithOneTheme(t)
	_ = themetest.DenyDir(t, dir)

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

func TestEvents_LoadedNamesTheFallbackSlug(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	loader := theme.NewLoader(theme.NewEventLogger(logger))
	setting := theme.Setting{Light: "gone-light", Dark: theme.DefaultDarkSlug}

	resolution, err := loader.ResolveNomination(setting, t.TempDir())
	if err != nil {
		t.Fatalf("ResolveNomination(%+v) = %v, want the fallback applied", setting, err)
	}
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
				t.Errorf("record %d (%q) carries the attr key %q, which is not one of the closed keys %v", i, record.Msg, key, closedAttrKeys)
			}
		}
		for _, enumerated := range []string{"count", "rejected"} {
			if record.HasAttr(enumerated) {
				t.Errorf("record %d (%q) carries %q; nothing is enumerated at construction", i, record.Msg, enumerated)
			}
		}
	}
}

func TestEvents_DiscardSilencesResolution(t *testing.T) {
	tests := []struct {
		name   string
		loader theme.Loader
	}{
		{name: "the silent constructor", loader: theme.NewSilentLoader()},
		{name: "a discard-backed seam", loader: theme.NewLoader(theme.NewEventLogger(log.Discard()))},
		{name: "a nil logger", loader: theme.NewLoader(theme.NewEventLogger(nil))},
		{name: "a zero-value loader's nil seam", loader: theme.Loader{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := &logtest.Sink{}
			log.SetTestHandler(t, sink)

			if fellBack := resolveEverySettingState(t, tt.loader); fellBack != wantFallbacks {
				t.Fatalf("the resolution set produced %d fallbacks, want %d", fellBack, wantFallbacks)
			}

			if lines := sink.Lines(); len(lines) != 0 {
				t.Errorf("emitted %d records, want none:\n%s", len(lines), sink.Body())
			}
		})
	}
}

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

const wantFallbacks = 3

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

func recordsNamed(sink *logtest.Sink, msg string) []logtest.Record {
	named := []logtest.Record{}
	for _, record := range sink.Records() {
		if record.Msg == msg {
			named = append(named, record)
		}
	}
	return named
}

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
