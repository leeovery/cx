package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/spawn"
	"github.com/leeovery/portal/internal/theme"
)

func unsupportedResolvedModel(t *testing.T, identity spawn.Identity) Model {
	t.Helper()
	return warmResolvedModel(t, &fakeDetector{identity: identity}, nativeResolve())
}

func TestUnsupportedHeader_NamedIdentityAmberDimSeeDocs(t *testing.T) {
	forEachBuiltinTheme(t, func(t *testing.T, th theme.Theme) {
		header := renderUnsupportedHeader("Apple Terminal", "com.apple.Terminal", sectionHeaderWidth, th, false)

		const wantVisible = "⚠ unsupported terminal — Apple Terminal · com.apple.Terminal"
		if !strings.Contains(ansi.Strip(header), wantVisible) {
			t.Errorf("banner missing the exact copy %q:\n%s", wantVisible, ansi.Strip(header))
		}
		if !strings.Contains(ansi.Strip(header), "see docs") {
			t.Errorf("banner missing the %q hint:\n%s", "see docs", ansi.Strip(header))
		}
		if strings.Contains(ansi.Strip(header), noticeBarGlyph) {
			t.Errorf("banner must not carry the %q notice-bar glyph:\n%s", noticeBarGlyph, ansi.Strip(header))
		}

		amberRun := headerStyle(th.AccentAttention, th, false).Render(flashWarningGlyph + " " + "unsupported terminal")
		if !strings.Contains(header, amberRun) {
			t.Errorf("banner missing the accent.orange label run:\n%s", header)
		}
		dimRun := headerStyle(th.TextMuted, th, false).Render(" — Apple Terminal · com.apple.Terminal")
		if !strings.Contains(header, dimRun) {
			t.Errorf("banner missing the text.detail identity run:\n%s", header)
		}
		blueRun := headerStyle(th.AccentKey, th, false).Hyperlink(unsupportedDocsURL).Render("see docs")
		if !strings.Contains(header, blueRun) {
			t.Errorf("banner missing the hyperlinked accent.blue %q run:\n%s", "see docs", header)
		}
	})
}

func TestUnsupportedHeader_RightAlignedSeeDocs(t *testing.T) {
	header := renderUnsupportedHeader("Apple Terminal", "com.apple.Terminal", sectionHeaderWidth, testDarkTheme(t), false)

	labelIdx := strings.Index(header, "unsupported terminal")
	hintIdx := strings.LastIndex(header, "see docs")
	if labelIdx < 0 || hintIdx < 0 {
		t.Fatalf("banner missing a cluster: labelIdx=%d hintIdx=%d\n%s", labelIdx, hintIdx, header)
	}
	if hintIdx < labelIdx {
		t.Errorf("hint (idx %d) appears before the label (idx %d); must be right-aligned", hintIdx, labelIdx)
	}
	if got := lipgloss.Width(header); got != sectionHeaderWidth {
		t.Errorf("banner width = %d, want exactly %d (flex spacer to content width)", got, sectionHeaderWidth)
	}
}

func TestUnsupportedHeader_ExactlyOneRow(t *testing.T) {
	for _, tc := range []struct {
		name     string
		bundleID string
	}{
		{"named", "com.apple.Terminal"},
	} {
		header := renderUnsupportedHeader("Apple Terminal", tc.bundleID, sectionHeaderWidth, testDarkTheme(t), false)
		if got := lipgloss.Height(header); got != 1 {
			t.Errorf("%s banner height = %d, want exactly 1 row:\n%s", tc.name, got, header)
		}
	}
}

func TestUnsupportedHeader_NarrowDegradeDropsHint(t *testing.T) {
	wide := renderUnsupportedHeader("Apple Terminal", "com.apple.Terminal", sectionHeaderWidth, testDarkTheme(t), false)
	if !strings.Contains(wide, "see docs") {
		t.Fatalf("wide banner missing the hint:\n%s", wide)
	}
	clusterWidth := lipgloss.Width("⚠ unsupported terminal — Apple Terminal · com.apple.Terminal")

	narrow := clusterWidth + 4
	header := renderUnsupportedHeader("Apple Terminal", "com.apple.Terminal", narrow, testDarkTheme(t), false)
	if strings.Contains(header, "see docs") {
		t.Errorf("narrow banner at width %d still shows the %q hint (degrade failed):\n%s", narrow, "see docs", header)
	}
	if !strings.Contains(ansi.Strip(header), "unsupported terminal — Apple Terminal · com.apple.Terminal") {
		t.Errorf("narrow banner dropped the identity cluster:\n%s", ansi.Strip(header))
	}
	for i, line := range strings.Split(header, "\n") {
		if lw := lipgloss.Width(line); lw > narrow {
			t.Errorf("narrow banner line %d width = %d (overflow, want <= %d)", i, lw, narrow)
		}
	}
}

func TestUnsupportedHeader_ColourlessGlyphBacked(t *testing.T) {
	header := renderUnsupportedHeader("Apple Terminal", "com.apple.Terminal", sectionHeaderWidth, testDarkTheme(t), true)

	stripped := ansi.Strip(header)
	for _, want := range []string{"⚠", "unsupported terminal", "Apple Terminal", "com.apple.Terminal", "see docs"} {
		if !strings.Contains(stripped, want) {
			t.Errorf("colourless banner dropped %q:\n%s", want, stripped)
		}
	}
	if seq := canvasSeq(t, testDarkTheme(t)); strings.Contains(header, seq) {
		t.Errorf("colourless banner still paints the canvas background sequence %q", seq)
	}
	for _, tok := range []theme.Token{testDarkTheme(t).AccentAttention, testDarkTheme(t).TextMuted, testDarkTheme(t).AccentKey} {
		if seq := tokenFgSeq(t, tok); strings.Contains(header, seq) {
			t.Errorf("colourless banner still emits a foreground role sequence %q", seq)
		}
	}
}

func TestUnsupportedHeader_PaintsCanvasNoEdgeBleed(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		header := renderUnsupportedHeader("Apple Terminal", "com.apple.Terminal", sectionHeaderWidth, th, false)
		if seq := canvasSeq(t, th); !strings.Contains(header, seq) {
			t.Errorf("banner does not paint the canvas background sequence %q:\n%s", seq, header)
		}
	}
}

func TestApplySectionHeader_UnsupportedShowsBanner(t *testing.T) {
	m := unsupportedResolvedModel(t, appleTerminalIdentity())
	if !m.DetectUnsupported() {
		t.Fatalf("precondition: com.apple.Terminal must resolve unsupported")
	}

	first := ansi.Strip(bannerFirstLine(m))
	for _, want := range []string{"unsupported terminal", "Apple Terminal", "com.apple.Terminal", "see docs"} {
		if !strings.Contains(first, want) {
			t.Errorf("unsupported section-header row missing %q:\n%s", want, first)
		}
	}
	if strings.Contains(first, "Sessions") {
		t.Errorf("unsupported section-header row must NOT show the standard %q header:\n%s", "Sessions", first)
	}
	if seq := tokenFgSeq(t, m.themeState.active.AccentAttention); !strings.Contains(bannerFirstLine(m), seq) {
		t.Errorf("unsupported banner missing the accent.orange fg sequence %q:\n%s", seq, bannerFirstLine(m))
	}
}

func TestApplySectionHeader_UnsupportedNullShowsStandardHeader(t *testing.T) {
	m := unsupportedResolvedModel(t, spawn.Identity{})
	if !m.DetectUnsupported() {
		t.Fatalf("precondition: a NULL identity must resolve unsupported")
	}

	first := ansi.Strip(bannerFirstLine(m))
	if !strings.Contains(first, "Sessions") {
		t.Errorf("NULL section-header row must show the standard %q header:\n%s", "Sessions", first)
	}
	for _, absent := range []string{"no host-local terminal", "unsupported terminal", "see docs"} {
		if strings.Contains(first, absent) {
			t.Errorf("NULL section-header row must NOT show %q (banner is named-only):\n%s", absent, first)
		}
	}
}

func TestApplySectionHeader_InFlightShowsStandardHeader(t *testing.T) {
	m, _ := dispatchWarmDetection(t, &fakeDetector{identity: appleTerminalIdentity()}, nativeResolve())
	if !m.DetectDispatched() || m.DetectResolved() {
		t.Fatalf("precondition: in-flight must be dispatched && !resolved; dispatched=%v resolved=%v", m.DetectDispatched(), m.DetectResolved())
	}

	first := ansi.Strip(bannerFirstLine(m))
	if !strings.Contains(first, "Sessions") {
		t.Errorf("in-flight section-header row must show the standard %q header:\n%s", "Sessions", first)
	}
	if strings.Contains(first, "unsupported terminal") || strings.Contains(first, "no host-local terminal") {
		t.Errorf("in-flight section-header row must NOT show the unsupported banner:\n%s", first)
	}
}

func TestApplySectionHeader_SupportedShowsStandardHeader(t *testing.T) {
	m := unsupportedResolvedModel(t, ghosttyIdentity())
	if m.DetectUnsupported() {
		t.Fatalf("precondition: ghostty must resolve native (supported)")
	}

	first := ansi.Strip(bannerFirstLine(m))
	if !strings.Contains(first, "Sessions") {
		t.Errorf("supported section-header row must show the standard %q header:\n%s", "Sessions", first)
	}
	if strings.Contains(first, "unsupported terminal") {
		t.Errorf("supported section-header row must NOT show the unsupported banner:\n%s", first)
	}
}

func TestApplySectionHeader_MultiSelectStepsUnsupportedAside(t *testing.T) {
	m := unsupportedResolvedModel(t, appleTerminalIdentity())
	if !m.DetectUnsupported() {
		t.Fatalf("precondition: com.apple.Terminal must resolve unsupported")
	}
	m.multiSelectMode = true

	first := ansi.Strip(bannerFirstLine(m))
	if !strings.Contains(first, "selected") {
		t.Errorf("multi-select must own the section-header row with the %q banner:\n%s", "N selected", first)
	}
	if strings.Contains(first, "unsupported terminal") {
		t.Errorf("multi-select mode must step the unsupported banner aside:\n%s", first)
	}
	if m.unsupportedBannerActive() {
		t.Errorf("unsupportedBannerActive() must be false in multi-select mode")
	}
}

func TestActiveNoticeBand_SuppressesSignpostWhenUnsupported(t *testing.T) {
	m := signpostModel(t)
	if _, _, ok := m.activeNoticeBand(); !ok {
		t.Fatalf("precondition: the signpost must own the slot before the unsupported banner activates")
	}

	m.detectIdentity = appleTerminalIdentity()
	m.detectResolution = spawn.ResolutionUnsupported
	m.detectResolved = true
	if !m.unsupportedBannerActive() {
		t.Fatalf("precondition: the unsupported banner must be active")
	}

	if _, _, ok := m.activeNoticeBand(); ok {
		t.Errorf("the unsupported banner must suppress the By-Tag signpost notice band")
	}
}

func TestActiveNoticeBand_NullReturnsSignpost(t *testing.T) {
	m := signpostModel(t)
	if _, _, ok := m.activeNoticeBand(); !ok {
		t.Fatalf("precondition: the signpost must own the slot before detection resolves")
	}

	m.detectIdentity = spawn.Identity{}
	m.detectResolution = spawn.ResolutionUnsupported
	m.detectResolved = true
	if m.unsupportedBannerActive() {
		t.Fatalf("unsupportedBannerActive() must be false for a resolved-unsupported NULL identity")
	}

	if _, _, ok := m.activeNoticeBand(); !ok {
		t.Errorf("a NULL client with no tags must show the By-Tag signpost (no banner competing for the slot)")
	}
}

func TestActiveNoticeBand_FlashOutranksUnsupported(t *testing.T) {
	m := signpostModel(t)
	m.detectIdentity = appleTerminalIdentity()
	m.detectResolution = spawn.ResolutionUnsupported
	m.detectResolved = true
	const flash = "session \"alpha\" no longer exists"
	m.setFlash(flash)

	role, message, ok := m.activeNoticeBand()
	if !ok {
		t.Fatalf("a transient flash must own the notice slot even when the unsupported banner is active")
	}
	if message != flash {
		t.Errorf("flash message = %q, want %q", message, flash)
	}
	if role != bandWarning {
		t.Errorf("default flash role = %v, want bandWarning", role)
	}
}
