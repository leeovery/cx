package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func referenceFitCluster(count, budget int, render func(n int) (string, int), sep, ellipsis string) (string, int) {
	if full, fullWidth := render(count); fullWidth <= budget {
		return full, fullWidth
	}
	sepWidth := lipgloss.Width(sep)
	ellipsisWidth := lipgloss.Width(ellipsis)
	best := ""
	bestWidth := 0
	for n := 1; n <= count; n++ {
		cluster, clusterWidth := render(n)
		candidateWidth := clusterWidth + sepWidth + ellipsisWidth
		if candidateWidth > budget {
			break
		}
		best = lipgloss.JoinHorizontal(lipgloss.Top, cluster, sep, ellipsis)
		bestWidth = candidateWidth
	}
	if best != "" {
		return best, bestWidth
	}
	if ellipsisWidth <= budget {
		return ellipsis, ellipsisWidth
	}
	return "", 0
}

func TestFitClusterToWidth_AlgorithmAcrossWidthRegimes(t *testing.T) {
	const sep = " · "
	const ellipsis = "…"
	const count = 4
	render := func(n int) (string, int) {
		s := strings.Repeat("X", n*5)
		return s, lipgloss.Width(s)
	}

	for _, tc := range []struct {
		name    string
		budget  int
		wantStr string
		wantWid int
	}{
		{
			name:    "wide budget returns the full cluster",
			budget:  100,
			wantStr: strings.Repeat("X", 20),
			wantWid: 20,
		},
		{
			name:    "narrow budget returns leading prefix plus separator and ellipsis",
			budget:  15,
			wantStr: strings.Repeat("X", 10) + sep + ellipsis,
			wantWid: 14,
		},
		{
			name:    "ellipsis-only budget returns the bare ellipsis",
			budget:  1,
			wantStr: ellipsis,
			wantWid: 1,
		},
		{
			name:    "sub-ellipsis budget returns the empty cluster",
			budget:  0,
			wantStr: "",
			wantWid: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotStr, gotWid := fitClusterToWidth(count, tc.budget, render, sep, ellipsis)
			if gotStr != tc.wantStr {
				t.Errorf("fitClusterToWidth string = %q, want %q", gotStr, tc.wantStr)
			}
			if gotWid != tc.wantWid {
				t.Errorf("fitClusterToWidth width = %d, want %d", gotWid, tc.wantWid)
			}
			if gotWid > tc.budget {
				t.Errorf("fitClusterToWidth width = %d exceeds budget %d", gotWid, tc.budget)
			}
		})
	}
}

func TestFitClusterToWidth_EmptyClusterCount(t *testing.T) {
	render := func(n int) (string, int) { return "", 0 }
	got, gotWid := fitClusterToWidth(0, 10, render, " · ", "…")
	if got != "" || gotWid != 0 {
		t.Errorf("fitClusterToWidth(0, ...) = (%q, %d), want (%q, 0)", got, gotWid, "")
	}
}

func TestFitFilterCluster_MatchesSharedHelperAcrossWidths(t *testing.T) {
	th := testDarkTheme(t)
	const colourless = false
	entries := multiSelectFooterEntries(th)

	sep := renderFooterDetail(footerEntrySeparator, th, colourless)
	ellipsis := renderFooterDetail(footerEllipsis, th, colourless)
	render := func(n int) (string, int) {
		cluster := renderFilterCluster(entries[:n], th, colourless)
		return cluster, lipgloss.Width(cluster)
	}
	fullWidth := lipgloss.Width(renderFilterCluster(entries, th, colourless))

	for _, tc := range []struct {
		name string
		w    int
	}{
		{"wide full cluster", fullWidth + 20},
		{"narrow degrade prefix plus ellipsis", 30},
		{"ellipsis only", 1},
		{"sub-ellipsis empty", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantStr, wantWid := referenceFitCluster(len(entries), tc.w, render, sep, ellipsis)
			gotStr, gotWid := fitFilterCluster(entries, tc.w, th, colourless)
			if gotStr != wantStr {
				t.Errorf("fitFilterCluster string diverged from pre-refactor output:\ngot  %q\nwant %q", gotStr, wantStr)
			}
			if gotWid != wantWid {
				t.Errorf("fitFilterCluster width = %d, want %d (pre-refactor)", gotWid, wantWid)
			}
			if gotWid > tc.w {
				t.Errorf("fitFilterCluster width = %d exceeds budget %d", gotWid, tc.w)
			}
		})
	}
}

func TestFitLeftCluster_MatchesSharedHelperAcrossWidths(t *testing.T) {
	th := testDarkTheme(t)
	const colourless = false
	core, _ := splitFooterEntries(sessionsKeymap())

	sep := renderFooterDetail(footerEntrySeparator, th, colourless)
	ellipsis := renderFooterDetail(footerEllipsis, th, colourless)
	render := func(n int) (string, int) {
		cluster := renderFooterCluster(core[:n], th, colourless)
		return cluster, lipgloss.Width(cluster)
	}
	fullWidth := lipgloss.Width(renderFooterCluster(core, th, colourless))

	for _, tc := range []struct {
		name       string
		w          int
		rightWidth int
	}{
		{"wide full cluster no right anchor", fullWidth + 20, 0},
		{"narrow degrade with reserved right anchor", 60, 6},
		{"ellipsis only", 1, 0},
		{"sub-ellipsis empty", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			budget := tc.w
			if tc.rightWidth > 0 {
				budget = tc.w - tc.rightWidth - 1
			}
			if budget < 0 {
				budget = 0
			}
			wantStr, wantWid := referenceFitCluster(len(core), budget, render, sep, ellipsis)
			gotStr, gotWid := fitLeftCluster(core, tc.w, tc.rightWidth, th, colourless)
			if gotStr != wantStr {
				t.Errorf("fitLeftCluster string diverged from pre-refactor output:\ngot  %q\nwant %q", gotStr, wantStr)
			}
			if gotWid != wantWid {
				t.Errorf("fitLeftCluster width = %d, want %d (pre-refactor)", gotWid, wantWid)
			}
			if gotWid > budget {
				t.Errorf("fitLeftCluster width = %d exceeds budget %d", gotWid, budget)
			}
		})
	}
}
