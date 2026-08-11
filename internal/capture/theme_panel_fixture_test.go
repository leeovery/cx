package capture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
	"github.com/leeovery/portal/internal/tui"
)

const panelLeftBorder = "│"

func panelFixtureNames() []string {
	var names []string
	for _, name := range FixtureNames() {
		if strings.HasPrefix(name, panelFixturePrefix) {
			names = append(names, name)
		}
	}
	return names
}

const panelFixturePrefix = "theme-panel-"

func TestPanelFixture_FourInputs(t *testing.T) {
	for _, name := range panelFixtureNames() {
		t.Run(name, func(t *testing.T) {
			fx, err := FixtureByName(name)
			if err != nil {
				t.Fatalf("FixtureByName(%s): %v", name, err)
			}

			if (fx.themeKeys == theme.RawKeys{}) {
				t.Error("input 2: the fixture declares no raw persisted theme keys, so its badges have nothing to derive from")
			}
			if len(fx.themeUnion.Rows) == 0 {
				t.Error("input 3a: the fixture declares no union rows, so the panel would list nothing")
			}
			if len(fx.themeSlots) == 0 {
				t.Error("input 3b: the fixture declares no slot resolutions, so no row carries a ● at all")
			}
			if fx.initialThemeCursor == "" {
				t.Error("input 4: the fixture declares no cursor row; §13.3 makes the cursor position a declared input precisely because a one-shot render cannot arrow to it")
			}

			deps := fx.Deps(theme.Theme{})
			if deps.ThemeKeys != fx.themeKeys {
				t.Errorf("Deps().ThemeKeys = %+v, want the declared %+v", deps.ThemeKeys, fx.themeKeys)
			}
			if deps.Capture.ThemeCursor != fx.initialThemeCursor {
				t.Errorf("Deps().Capture.ThemeCursor = %q, want the declared %q", deps.Capture.ThemeCursor, fx.initialThemeCursor)
			}
			if deps.ThemeSource == nil {
				t.Fatal("Deps() wires no ThemeSource, so `t` is a silent no-op and the fixture renders no panel at all")
			}

			enumeration, union := deps.ThemeSource.Open(fx.themeKeys)
			if got, want := rowIdentities(union.Rows), rowIdentities(fx.themeUnion.Rows); !slices.Equal(got, want) {
				t.Errorf("the seam's union lists %v, want the declared %v", got, want)
			}
			if got, want := enumeration.DirPath, fx.themeEnumeration.DirPath; got != want {
				t.Errorf("the seam's enumeration is of %q, want the declared %q", got, want)
			}
		})
	}
}

func rowIdentities(rows []theme.Row) []string {
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.Identity())
	}
	return keys
}

func TestFakeThemeSource_ResolveReportsTheInjectedPalette(t *testing.T) {
	injected := themetest.Builtin(t, theme.DefaultDarkSlug)
	declared := []theme.SlotResolution{
		{Slot: theme.SlotLight, Requested: "tokyo-night-day", Resolved: "tokyo-night-day", Theme: themetest.Builtin(t, theme.DefaultLightSlug)},
		{Slot: theme.SlotDark, Requested: "nord", Resolved: "nord", Theme: themetest.Builtin(t, "nord")},
	}
	fake := newFakeThemeSource(injected, theme.Enumeration{}, theme.Union{}, declared)

	resolution, err := fake.Resolve(theme.Enumeration{}, theme.RawKeys{})
	if err != nil {
		t.Fatalf("Resolve returned %v; the fake has nothing that can fail", err)
	}

	t.Run("every slot reports the injected palette", func(t *testing.T) {
		if len(resolution.Slots) != len(declared) {
			t.Fatalf("Resolve returned %d slot(s), want the declared %d", len(resolution.Slots), len(declared))
		}
		for i, slot := range resolution.Slots {
			if slot.Slot != declared[i].Slot || slot.Requested != declared[i].Requested || slot.Resolved != declared[i].Resolved {
				t.Errorf("slot %d = %+v, want the declared identity %+v", i, slot, declared[i])
			}
			if slot.Theme != injected {
				t.Errorf("slot %d reports canvas %q, want the injected %q — the open-time ApplyTheme would repaint the frame off `--theme`", i, slot.Theme.Canvas.Value, injected.Canvas.Value)
			}
		}
	})

	t.Run("the nomination reports the injected palette", func(t *testing.T) {
		if !resolution.Nomination.IsConstant() {
			t.Fatal("Resolve's nomination is not the constant shape capturetool pins (§13.3)")
		}
		if active := resolution.Nomination.Constant(); active != injected {
			t.Errorf("the nomination carries canvas %q, want the injected %q", active.Canvas.Value, injected.Canvas.Value)
		}
	})

	t.Run("a fake reporting another palette repaints the frame", func(t *testing.T) {
		pinned := themetest.Builtin(t, "nord")
		other := themetest.Builtin(t, theme.DefaultLightSlug)

		frame := driveToPanel(t, panelDepsReporting(t, "theme-panel-adaptive-pair", pinned, other))
		if !strings.Contains(frame, backgroundSGR(t, other.Canvas)) {
			t.Error("a fake reporting another palette did NOT repaint the frame, so the injected-palette requirement asserts nothing")
		}
	})

	t.Run("a fake reporting a zero palette renders colourless", func(t *testing.T) {
		pinned := themetest.Builtin(t, "nord")

		frame := driveToPanel(t, panelDepsReporting(t, "theme-panel-adaptive-pair", pinned, theme.Theme{}))
		if strings.Contains(frame, backgroundSGR(t, pinned.Canvas)) {
			t.Error("the frame still carries `--theme`'s canvas after a zero-palette apply; the hazard is not being driven")
		}
		if strings.Contains(frame, truecolorBackground) {
			t.Errorf("the frame carries a 24-bit background after a zero-palette apply, so the colourless outcome this warns about is not what happens; re-derive the warning:\n%s", ansi.Strip(frame))
		}
	})
}

func TestFakeThemeSource_RowsCarryTheInjectedPalette(t *testing.T) {
	t.Run("arrowing to the next row keeps the frame on the injected palette", func(t *testing.T) {
		pinned := themetest.Builtin(t, "nord")

		fx, err := FixtureByName("theme-panel-adaptive-pair")
		if err != nil {
			t.Fatalf("FixtureByName: %v", err)
		}

		model := panelModel(t, fx.Deps(pinned))
		opened := frameOf(t, model)

		if !strings.Contains(ansi.Strip(opened), panelLeftBorder) {
			t.Fatalf("the frame carries no %q, so the panel never opened and the `↓` below cannot reach it:\n%s", panelLeftBorder, ansi.Strip(opened))
		}

		model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		previewed := frameOf(t, model)

		if opened == previewed {
			t.Fatal("the `↓` changed nothing on the frame; the panel cursor never moved, so this asserts nothing about what a preview applies")
		}
		if !strings.Contains(previewed, backgroundSGR(t, pinned.Canvas)) {
			t.Errorf("one `↓` left the frame without `--theme`'s canvas %s; the previewed row's palette is not the injected one, and the live view is colourless from the first arrow key:\n%s", pinned.Canvas.Value, ansi.Strip(previewed))
		}
	})

	t.Run("a rejected row keeps its rejection and takes no palette", func(t *testing.T) {
		injected := themetest.Builtin(t, "nord")
		rejection := &theme.Rejection{Reason: theme.ReasonBadSyntax, Detail: "line 4: quoted value"}
		declared := theme.Union{Rows: []theme.Row{
			{Slug: "valid-drop-in", Filename: "valid-drop-in.theme", Source: theme.SourceFile},
			{Filename: "broken.theme", Source: theme.SourceFile, Rejection: rejection},
		}}

		_, union := newFakeThemeSource(injected, theme.Enumeration{}, declared, nil).Open(theme.RawKeys{})
		if len(union.Rows) != len(declared.Rows) {
			t.Fatalf("the fake lists %d row(s), want the declared %d", len(union.Rows), len(declared.Rows))
		}

		if got := union.Rows[0].Theme; got != injected {
			t.Errorf("the selectable row carries canvas %q, want the injected %q", got.Canvas.Value, injected.Canvas.Value)
		}
		if got := union.Rows[1].Theme; got != (theme.Theme{}) {
			t.Errorf("the rejected row carries canvas %q, want none — Theme is populated IFF Rejection is nil, and a half-populated rejected row is a shape the real assembly never returns", got.Canvas.Value)
		}
		if union.Rows[1].Rejection != rejection {
			t.Error("the rejected row's rejection was rewritten; the fake repaints palettes and touches nothing else")
		}
	})
}

func panelDepsReporting(t *testing.T, fixture string, pinned, reported theme.Theme) tui.Deps {
	t.Helper()

	fx, err := FixtureByName(fixture)
	if err != nil {
		t.Fatalf("FixtureByName(%s): %v", fixture, err)
	}
	deps := fx.Deps(pinned)
	deps.ThemeSource = newFakeThemeSource(reported, fx.themeEnumeration, fx.themeUnion, fx.themeSlots)
	return deps
}

const truecolorBackground = "48;2;"

func TestFakeThemeSource_ResolveIsTheOnlyBadgeSource(t *testing.T) {
	pinned := themetest.Builtin(t, "nord")

	fx, err := FixtureByName("theme-panel-adaptive-pair")
	if err != nil {
		t.Fatalf("FixtureByName: %v", err)
	}

	t.Run("the declared slots put a badge on their rows", func(t *testing.T) {
		for _, badge := range panelBadges(t, driveToPanel(t, fx.Deps(pinned))) {
			if badge != "" {
				return
			}
		}
		t.Error("no panel row carries a badge although the fixture declares two slots; the badge source is not the seam's Resolve return")
	})

	t.Run("declaring no slots renders no badge on any row", func(t *testing.T) {
		deps := fx.Deps(pinned)
		deps.ThemeSource = newFakeThemeSource(pinned, fx.themeEnumeration, fx.themeUnion, nil)

		for i, badge := range panelBadges(t, driveToPanel(t, deps)) {
			if badge != "" {
				t.Errorf("panel row %d carries badge %q although no slot was declared; something other than Resolve is marking rows", i, badge)
			}
		}
	})
}

func panelBadges(t *testing.T, frame string) []string {
	t.Helper()

	var badges []string
	for line := range strings.SplitSeq(ansi.Strip(frame), "\n") {
		_, text, onPanel := strings.Cut(line, panelLeftBorder)
		if !onPanel || strings.TrimSpace(text) == "" {
			continue
		}
		badges = append(badges, badgeIn(text))
	}
	if len(badges) == 0 {
		t.Fatalf("no panel rows were found in the frame; the slide-over did not render:\n%s", ansi.Strip(frame))
	}
	return badges
}

func badgeIn(text string) string {
	at := strings.Index(text, "●")
	if at < 0 {
		return ""
	}
	return strings.TrimSpace(text[at:])
}

func TestFakeThemeSource_NoIO(t *testing.T) {
	const source = "theme_fake.go"
	file := parseFakeSource(t, source)

	fake, ok := fakeThemeSourceStruct(file)
	if !ok {
		t.Fatalf("%s declares no fakeThemeSource struct, so scanning it proves nothing", source)
	}

	t.Run("the fake's source imports nothing that can touch the filesystem", func(t *testing.T) {
		banned := []string{"os", "io", "io/fs", "io/ioutil", "path/filepath", "embed", "os/exec", "net"}
		for _, spec := range file.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			if slices.Contains(banned, path) {
				t.Errorf("%s imports %q; the fake must answer from declared values with no file or directory access", source, path)
			}
		}
	})

	t.Run("the fake holds no theme.Loader", func(t *testing.T) {
		if field, held := loaderFieldIn(fake, themePackageName(file)); held {
			t.Errorf("%s's fakeThemeSource declares field %q, a theme.Loader — the one I/O-capable type in internal/theme, and the one the import scan above cannot ban. The fake must hold no loader, resolve no path and read no file", source, field)
		}
	})

	t.Run("the loader scan reports one that is there", func(t *testing.T) {
		control := parseControlSource(t, loaderHoldingFakeSource)
		fake, ok := fakeThemeSourceStruct(control)
		if !ok {
			t.Fatal("the control source declares no fakeThemeSource struct, so it controls nothing")
		}
		field, held := loaderFieldIn(fake, themePackageName(control))
		if !held {
			t.Fatal("the scan reports no loader on a fake that holds one behind a pointer and an import alias; it would pass over theme_fake.go whatever that file held")
		}
		if want := "loader"; field != want {
			t.Errorf("the scan names field %q, want %q — it is matching some other field", field, want)
		}
	})

	t.Run("every method answers from declared values with the config paths poisoned", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "no-such-themes-dir")
		t.Setenv("PORTAL_THEMES_DIR", missing)
		t.Setenv("XDG_CONFIG_HOME", missing)
		t.Setenv("PORTAL_PREFS_FILE", filepath.Join(missing, "prefs.json"))

		fx, err := FixtureByName("theme-panel-adaptive-pair")
		if err != nil {
			t.Fatalf("FixtureByName: %v", err)
		}
		palette := themetest.Builtin(t, "nord")
		seam := fx.Deps(palette).ThemeSource

		enumeration, union := seam.Open(fx.themeKeys)
		if got, want := rowIdentities(union.Rows), rowIdentities(fx.themeUnion.Rows); !slices.Equal(got, want) {
			t.Errorf("Open listed %v, want the declared %v", got, want)
		}
		if got, want := rowIdentities(seam.Reassemble(enumeration, fx.themeKeys).Rows), rowIdentities(fx.themeUnion.Rows); !slices.Equal(got, want) {
			t.Errorf("Reassemble listed %v, want the declared %v", got, want)
		}
		resolution, err := seam.Resolve(enumeration, theme.RawKeys{})
		if err != nil {
			t.Fatalf("Resolve returned %v", err)
		}
		if len(resolution.Slots) != len(fx.themeSlots) {
			t.Errorf("Resolve returned %d slot(s), want the declared %d", len(resolution.Slots), len(fx.themeSlots))
		}
		res, err := seam.ResolveSlot(enumeration, theme.SlotLight, theme.RawKeys{Light: "nord"})
		if err != nil {
			t.Fatalf("ResolveSlot returned %v", err)
		}
		if res.Slot != theme.SlotLight || res.Requested != "nord" || res.Resolved != "nord" {
			t.Errorf("ResolveSlot answered slot=%v requested=%q resolved=%q, want the light slot and %q on both slugs", res.Slot, res.Requested, res.Resolved, "nord")
		}
		if res.Theme != palette {
			t.Errorf("ResolveSlot answered canvas %q, want the injected palette's %q — the fourth method must report the same palette as the other three", res.Theme.Canvas.Value, palette.Canvas.Value)
		}
		if _, err := os.Stat(missing); !os.IsNotExist(err) {
			t.Errorf("the poisoned themes directory %s now exists (%v); something on the render path created it", missing, err)
		}
	})
}

const loaderHoldingFakeSource = `package capture

import palette "github.com/leeovery/portal/internal/theme"

type fakeThemeSource struct {
	slots  []palette.SlotResolution
	loader *palette.Loader
}
`

func parseFakeSource(t *testing.T, source string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), source, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", source, err)
	}
	return file
}

func parseControlSource(t *testing.T, src string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "control_fake.go", src, 0)
	if err != nil {
		t.Fatalf("parse the control source: %v", err)
	}
	return file
}

func fakeThemeSourceStruct(file *ast.File) (*ast.StructType, bool) {
	var found *ast.StructType
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok || spec.Name.Name != "fakeThemeSource" {
			return true
		}
		if declared, ok := spec.Type.(*ast.StructType); ok {
			found = declared
		}
		return false
	})
	return found, found != nil
}

func themePackageName(file *ast.File) string {
	const path = `"github.com/leeovery/portal/internal/theme"`
	for _, spec := range file.Imports {
		if spec.Path.Value != path {
			continue
		}
		if spec.Name != nil {
			return spec.Name.Name
		}
		return "theme"
	}
	return ""
}

func loaderFieldIn(fake *ast.StructType, pkg string) (string, bool) {
	if pkg == "" {
		return "", false
	}
	for _, field := range fake.Fields.List {
		if !mentionsLoader(field.Type, pkg) {
			continue
		}
		if len(field.Names) == 0 {
			return "Loader", true
		}
		return field.Names[0].Name, true
	}
	return "", false
}

func mentionsLoader(expr ast.Expr, pkg string) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		selector, ok := n.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Loader" {
			return true
		}
		if ident, isIdent := selector.X.(*ast.Ident); isIdent && ident.Name == pkg {
			found = true
		}
		return !found
	})
	return found
}

func panelModel(t *testing.T, deps tui.Deps) tea.Model {
	t.Helper()

	sessions, err := deps.Lister.ListSessions()
	if err != nil {
		t.Fatalf("the fixture's lister failed: %v", err)
	}
	projects, err := deps.ProjectStore.List()
	if err != nil {
		t.Fatalf("the fixture's project store failed: %v", err)
	}

	var model tea.Model = tui.Build(deps)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model, _ = model.Update(tui.SessionsMsg{Sessions: sessions})
	model, _ = model.Update(tui.ProjectsLoadedMsg{Projects: projects})
	model, _ = model.Update(keyRune('t'))
	return model
}

func driveToPanel(t *testing.T, deps tui.Deps) string {
	t.Helper()
	return frameOf(t, panelModel(t, deps))
}

func frameOf(t *testing.T, model tea.Model) string {
	t.Helper()
	painted, ok := model.(tui.Model)
	if !ok {
		t.Fatalf("the drive returned a %T, not a tui.Model", model)
	}
	return painted.View().Content
}

func backgroundSGR(t *testing.T, tok theme.Token) string {
	t.Helper()
	probe := lipgloss.NewStyle().Background(tok.Color()).Render("x")
	start := strings.IndexByte(probe, '[')
	end := strings.IndexByte(probe, 'm')
	if start < 0 || end <= start {
		t.Fatalf("could not derive the background SGR from %q", probe)
	}
	return probe[start+1 : end]
}

func TestPanelPaginatedUnion_DerivesFromBase(t *testing.T) {
	base := themePanelUnion()
	paginated := themePanelPaginatedUnion()

	if len(paginated.Rows) <= len(base.Rows) {
		t.Fatalf("the paginating union carries %d rows against the base's %d; it must carry the base set and then the synthetics", len(paginated.Rows), len(base.Rows))
	}

	t.Run("the base rows lead the paginating union in order", func(t *testing.T) {
		if got := paginated.Rows[:len(base.Rows)]; !slices.Equal(got, base.Rows) {
			t.Errorf("the paginating union leads with rows %v, want the base set %v", rowSlugs(got), rowSlugs(base.Rows))
		}
	})

	t.Run("the synthetics follow the base rows", func(t *testing.T) {
		for i, row := range paginated.Rows[len(base.Rows):] {
			if want := themePanelSyntheticSlug(i); row.Slug != want {
				t.Errorf("synthetic row %d is %q, want %q — the synthetics sort after every built-in, which is what keeps the badged rows on page 1", i, row.Slug, want)
			}
		}
	})

	t.Run("the count is re-derived from the rows", func(t *testing.T) {
		if got, want := paginated.Count, len(paginated.Rows); got != want {
			t.Errorf("the paginating union counts %d rows and carries %d", got, want)
		}
	})

	t.Run("the base builders mint a fresh row set per call", func(t *testing.T) {
		first := themePanelUnion()
		first.Rows[0] = theme.Row{Slug: "mutated"}
		if themePanelUnion().Rows[0].Slug == "mutated" {
			t.Error("themePanelUnion hands back a shared row set, so every fixture derived from it would alias the others")
		}
	})

	t.Run("the base enumeration mints fresh entries per call", func(t *testing.T) {
		first := themePanelEnumeration()
		first.Entries[0] = theme.Entry{Filename: "mutated"}
		if themePanelEnumeration().Entries[0].Filename == "mutated" {
			t.Error("themePanelEnumeration hands back shared entries, so every fixture derived from it would alias the others")
		}
	})

	t.Run("the retained parse leads with the base enumeration's entries", func(t *testing.T) {
		baseEntries := themePanelEnumeration().Entries
		entries := themePanelPaginatedEntries()
		if len(entries) <= len(baseEntries) {
			t.Fatalf("the paginating parse carries %d entries against the base's %d", len(entries), len(baseEntries))
		}
		if got := entries[:len(baseEntries)]; !slices.Equal(got, baseEntries) {
			t.Errorf("the paginating parse leads with entries %+v, want the base entries %+v", got, baseEntries)
		}
	})
}

func rowSlugs(rows []theme.Row) []string {
	slugs := make([]string, 0, len(rows))
	for _, row := range rows {
		slugs = append(slugs, row.Slug)
	}
	return slugs
}
