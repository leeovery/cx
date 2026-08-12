package capture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
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
			if len(fx.themeSlots) == 0 {
				t.Error("input 3: the fixture declares no slot resolutions, so no row carries a ● at all")
			}
			if fx.initialThemeCursor == "" {
				t.Error("input 4: the fixture declares no cursor row; the cursor position is a declared input precisely because a one-shot render cannot arrow to it")
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
			if got, want := enumeration.DirPath, fx.themeEnumeration.DirPath; got != want {
				t.Errorf("the seam's enumeration is of %q, want the declared %q", got, want)
			}

			cursor, found := rowFor(union, fx.initialThemeCursor)
			if !found {
				t.Fatalf("the union lists %v, none of them the declared cursor row %q — the cursor has nothing to rest on", rowIdentities(union.Rows), fx.initialThemeCursor)
			}
			if !cursor.Selectable() {
				t.Errorf("the cursor row %q is rejected; the arrow skip keeps the cursor off invalid rows, so a frame parked on one is a state production cannot reach", fx.initialThemeCursor)
			}
			for _, slot := range fx.themeSlots {
				if _, badged := rowFor(union, slot.Requested); !badged {
					t.Errorf("the %v slot requests %q, which the union does not list — the badge marking it has no row to sit on", slot.Slot, slot.Requested)
				}
			}
		})
	}
}

func rowFor(union theme.Union, identity string) (theme.Row, bool) {
	for _, row := range union.Rows {
		if row.Identity() == identity {
			return row, true
		}
	}
	return theme.Row{}, false
}

// The fixtures state their inputs and production assembles the list, so a change
// to membership, dedup or ordering reaches every captured frame.
func TestPanelFixture_UnionIsProductionAssembled(t *testing.T) {
	for _, name := range panelFixtureNames() {
		t.Run(name, func(t *testing.T) {
			fx, err := FixtureByName(name)
			if err != nil {
				t.Fatalf("FixtureByName(%s): %v", name, err)
			}

			assembled := theme.Assembler{Loader: theme.NewSilentLoader()}.Reassemble(fx.themeEnumeration, fx.themeKeys)
			if !reflect.DeepEqual(fx.themeUnion, assembled) {
				t.Errorf("the fixture's union is not what the assembler derives from its declared entries and keys:\n got %v (count %d, rejected %d)\nwant %v (count %d, rejected %d)",
					rowIdentities(fx.themeUnion.Rows), fx.themeUnion.Count, fx.themeUnion.Rejected,
					rowIdentities(assembled.Rows), assembled.Count, assembled.Rejected)
			}
		})
	}
}

func TestPanelFixture_InvalidRowsCarryTheirReasons(t *testing.T) {
	union := unionOf(t, "theme-panel-invalid-row")

	for identity, want := range map[string]theme.Reason{
		"aurora-glow": theme.ReasonBadSyntax,
		"My Gorgeous Midnight Palette" + theme.FileExtension: theme.ReasonBadName,
		themePanelBrokenSlug: theme.ReasonBadColour,
	} {
		row, found := rowFor(union, identity)
		if !found {
			t.Errorf("the union does not list %q; the frame this fixture exists for renders no such row", identity)
			continue
		}
		if row.Rejection == nil {
			t.Errorf("the %q row carries no rejection, so it renders as a valid theme", identity)
			continue
		}
		if row.Rejection.Reason != want {
			t.Errorf("the %q row is rejected %q, want %q", identity, row.Rejection.Reason, want)
		}
	}

	if got, want := union.Rejected, 3; got != want {
		t.Errorf("the union counts %d rejected rows, want %d", got, want)
	}
}

func TestPanelFixture_DirUnreadablePersistedRows(t *testing.T) {
	union := unionOf(t, "theme-panel-dir-unreadable")

	if !union.DirUnusable {
		t.Error("the union does not report an unusable directory, so the pinned `⚠ dir unreadable` chrome row never renders")
	}
	for _, slug := range []string{themePanelBrokenSlug, themePanelUnreachableLightSlug} {
		row, found := rowFor(union, slug)
		if !found {
			t.Errorf("the union does not list the persisted slug %q, so its badge has no row", slug)
			continue
		}
		if row.Source != theme.SourcePersisted {
			t.Errorf("the %q row is sourced %v, want a persisted row — nothing in an unreadable directory can answer for it", slug, row.Source)
		}
		if row.Rejection == nil || row.Rejection.Reason != theme.ReasonUnreadable {
			t.Errorf("the %q row carries rejection %+v, want %q", slug, row.Rejection, theme.ReasonUnreadable)
		}
	}
}

func TestPanelFixture_PaginatedOverflowsOnePage(t *testing.T) {
	fx, err := FixtureByName("theme-panel-paginated")
	if err != nil {
		t.Fatalf("FixtureByName: %v", err)
	}

	last := themePanelSyntheticSlug(themePanelSyntheticDropIns - 1)
	if _, found := rowFor(fx.themeUnion, last); !found {
		t.Fatalf("the union does not list the last synthetic row %q, so it cannot be the one pushed off the page", last)
	}

	frame := ansi.Strip(driveToPanel(t, fx.Deps(themetest.Builtin(t, "nord"))))
	if strings.Contains(frame, last) {
		t.Errorf("the panel renders every one of its %d rows, so the pagination the fixture exists to capture never happens:\n%s", fx.themeUnion.Count, frame)
	}
}

func unionOf(t *testing.T, name string) theme.Union {
	t.Helper()

	fx, err := FixtureByName(name)
	if err != nil {
		t.Fatalf("FixtureByName(%s): %v", name, err)
	}
	return fx.themeUnion
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
			t.Fatal("Resolve's nomination is not the constant shape capturetool pins")
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
		keys := theme.RawKeys{Light: "nord"}
		if err := seam.LoadSlot(enumeration, theme.SlotLight, keys); err != nil {
			t.Errorf("LoadSlot returned %v, want no error — a fixture's load reads nothing and cannot fail", err)
		}
		if got := theme.SlugForSlot(keys, theme.SlotLight); got != "nord" {
			t.Errorf("the light slot collapses to %q, want %q — the slug the load names", got, "nord")
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

func TestPanelPaginatedEntries_DeriveFromBase(t *testing.T) {
	baseEntries := themePanelEnumeration().Entries
	entries := themePanelPaginatedEntries()

	if len(entries) <= len(baseEntries) {
		t.Fatalf("the paginating parse carries %d entries against the base's %d; it must carry the base set and then the synthetics", len(entries), len(baseEntries))
	}

	t.Run("the base entries lead the paginating parse in order", func(t *testing.T) {
		if got := entries[:len(baseEntries)]; !slices.Equal(got, baseEntries) {
			t.Errorf("the paginating parse leads with entries %+v, want the base entries %+v", got, baseEntries)
		}
	})

	t.Run("the synthetics follow the base entries", func(t *testing.T) {
		for i, entry := range entries[len(baseEntries):] {
			if want := themePanelSyntheticSlug(i); entry.Slug != want {
				t.Errorf("synthetic entry %d is %q, want %q — the synthetics sort after every built-in, which is what keeps the badged rows on page 1", i, entry.Slug, want)
			}
		}
	})

	t.Run("the base enumeration mints fresh entries per call", func(t *testing.T) {
		first := themePanelEnumeration()
		first.Entries[0] = theme.Entry{Filename: "mutated"}
		if themePanelEnumeration().Entries[0].Filename == "mutated" {
			t.Error("themePanelEnumeration hands back shared entries, so every fixture derived from it would alias the others")
		}
	})
}
