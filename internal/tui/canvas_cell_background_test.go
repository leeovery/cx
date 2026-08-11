package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

type bgState struct {
	set    bool
	params string
}

func applySGR(st bgState, params []string) bgState {
	if len(params) == 0 {
		return bgState{}
	}
	for i := 0; i < len(params); i++ {
		p := params[i]
		switch p {
		case "", "0", "49":
			st = bgState{}
		case "48":
			run := []string{p}
			if i+1 < len(params) {
				switch params[i+1] {
				case "2":
					run = append(run, params[i+1:min(i+5, len(params))]...)
					i = min(i+4, len(params)-1)
				case "5":
					run = append(run, params[i+1:min(i+3, len(params))]...)
					i = min(i+2, len(params)-1)
				}
			}
			st = bgState{set: true, params: strings.Join(run, ";")}
		case "38":
			if i+1 < len(params) {
				switch params[i+1] {
				case "2":
					i = min(i+4, len(params)-1)
				case "5":
					i = min(i+2, len(params)-1)
				}
			}
		default:
			if isNamedBg(p) {
				st = bgState{set: true, params: p}
			}
		}
	}
	return st
}

func isNamedBg(p string) bool {
	switch p {
	case "40", "41", "42", "43", "44", "45", "46", "47",
		"100", "101", "102", "103", "104", "105", "106", "107":
		return true
	}
	return false
}

func wantCanvasBgParams(t *testing.T, th theme.Theme) string {
	t.Helper()
	seq := canvasSeq(t, th)
	inner := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m")
	if inner == "" {
		t.Fatalf("could not derive canvas bg params from %q", seq)
	}
	return inner
}

func scanCellBackgrounds(line string) []bgState {
	var cells []bgState
	st := bgState{}
	p := ansi.NewParser()
	b := []byte(line)
	state := byte(0)
	for len(b) > 0 {
		seq, width, n, newState := ansi.DecodeSequence(b, state, p)
		if n == 0 {
			break
		}
		if ansi.HasCsiPrefix(seq) && len(seq) > 0 && seq[len(seq)-1] == 'm' {
			st = applySGR(st, splitSGRParams(string(seq)))
		} else if width > 0 {
			for range width {
				cells = append(cells, st)
			}
		}
		b = b[n:]
		state = newState
	}
	return cells
}

func splitSGRParams(seq string) []string {
	inner := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m")
	if inner == "" {
		return nil
	}
	return strings.Split(inner, ";")
}

func TestCanvasCellBackground_EveryInGridCellIsCanvas(t *testing.T) {
	forEachCanvasMode(t, func(t *testing.T, mode theme.Member) {
		const w, h = 90, 24
		m := newCanvasTestModel(t, w, h, mode)
		th := themeForAppearance(t, mode)
		canvasParams := wantCanvasBgParams(t, th)

		view := m.View().Content
		lines := strings.Split(view, "\n")

		sawCanvas := false
		for li, line := range lines {
			cells := scanCellBackgrounds(line)
			for col, c := range cells {
				if col >= w {
					break
				}
				if !c.set {
					t.Fatalf("mode %s line %d col %d: cell has NO explicit background (falls back to terminal default) — mid-line bleed. line=%q",
						themeLabel(th), li, col, strings.ReplaceAll(line, "\x1b", "\\e"))
				}
				if c.params == canvasParams {
					sawCanvas = true
				}
			}
		}
		if !sawCanvas {
			t.Errorf("mode %s: no cell carried the canvas background %q — the canvas paint is absent", themeLabel(th), canvasParams)
		}
	})
}

func TestCanvasCellBackground_TitleAndFooterGaps(t *testing.T) {
	const w, h = 90, 24
	m := newCanvasTestModel(t, w, h, theme.MemberDark)

	view := m.View().Content
	lines := strings.Split(view, "\n")

	check := func(li int, label string) {
		if li < 0 || li >= len(lines) {
			t.Fatalf("expected a line for %s", label)
		}
		for col, c := range scanCellBackgrounds(lines[li]) {
			if col >= w {
				break
			}
			if !c.set {
				t.Fatalf("%s (line %d) col %d unpainted (terminal-default cell): %q",
					label, li, col, strings.ReplaceAll(lines[li], "\x1b", "\\e"))
			}
		}
	}

	check(0, "title row")
	for li, line := range lines {
		if strings.Contains(ansi.Strip(line), "switch view") ||
			strings.Contains(ansi.Strip(line), "go to start") ||
			strings.Contains(ansi.Strip(line), "go to end") {
			check(li, "footer row")
		}
	}
}
