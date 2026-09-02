package sourceguardtest_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

func TestParseSources_ReturnsOneParsedSourcePerFile(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"alpha.go": "package a\n\nfunc Alpha() {}\n",
		"beta.go":  "package a\n\nfunc Beta() {}\n",
	})
	paths := []string{filepath.Join(dir, "alpha.go"), filepath.Join(dir, "beta.go")}

	sources := sourceguardtest.ParseSources(t, paths)

	if len(sources) != len(paths) {
		t.Fatalf("ParseSources returned %d sources over %d paths", len(sources), len(paths))
	}
	for i, source := range sources {
		if source.Path != paths[i] {
			t.Errorf("source %d carries path %q, want %q", i, source.Path, paths[i])
		}
		if source.Fset == nil || source.File == nil {
			t.Fatalf("source %d carries fset=%v file=%v, want both", i, source.Fset, source.File)
		}
		if got := source.Fset.Position(source.File.Pos()).Filename; got != paths[i] {
			t.Errorf("source %d resolves positions against %q, want %q", i, got, paths[i])
		}
	}
}

// A guard reading comments — a build tag among them — must find them there: the
// stated mode is the one every guard in the tree parses under.
func TestParseSources_ParsesUnderTheStatedMode(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"tagged.go": "//go:build integration\n\npackage a\n",
	})

	source := sourceguardtest.ParseSources(t, []string{filepath.Join(dir, "tagged.go")})[0]

	if len(source.File.Comments) == 0 {
		t.Error("the parsed file carries no comments, so a guard reading a build tag would see none")
	}
}

func TestParseSources_FatalsOnAnUnparseableFile(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"broken.go": "package a\n\nfunc (\n"})
	path := filepath.Join(dir, "broken.go")
	stub := &recordingT{}

	sources := sourceguardtest.ParseSources(stub, []string{path})

	if !stub.fataled {
		t.Fatal("ParseSources did not fatal on an unparseable file — a guard would scan a file it could not read")
	}
	if sources != nil {
		t.Errorf("ParseSources returned %v after fatalling, want nil", sources)
	}
	if !strings.Contains(stub.msg, path) {
		t.Errorf("fatal message %q does not name the unparseable file", stub.msg)
	}
}

func TestParseSources_FatalsWhenGivenNoPaths(t *testing.T) {
	stub := &recordingT{}

	sources := sourceguardtest.ParseSources(stub, nil)

	if !stub.fataled {
		t.Fatal("ParseSources did not fatal over an empty path set — a guard driven by it would pass having scanned nothing")
	}
	if sources != nil {
		t.Errorf("ParseSources returned %v after fatalling, want nil", sources)
	}
	if !strings.Contains(stub.msg, "stopped looking") {
		t.Errorf("fatal message %q does not say the guard would pass having stopped looking", stub.msg)
	}
}

func TestParsePackageSources_FatalsWhenThePackageYieldsNoSource(t *testing.T) {
	dir := t.TempDir()
	stub := &recordingT{}

	sources := sourceguardtest.ParsePackageSources(stub, dir, false)

	if !stub.fataled {
		t.Fatal("ParsePackageSources did not fatal over a directory holding no sources — a guard driven by it would pass having scanned nothing")
	}
	if sources != nil {
		t.Errorf("ParsePackageSources returned %v after fatalling, want nil", sources)
	}
	if !strings.Contains(stub.msg, dir) {
		t.Errorf("fatal message %q does not name the directory it enumerated", stub.msg)
	}
}

func TestParsePackageSources_IncludesTestSourcesOnlyWhenAsked(t *testing.T) {
	dir := packageFixture(t)

	production := parsedBaseNames(sourceguardtest.ParsePackageSources(t, dir, false))
	if want := []string{"alpha.go", "beta.go"}; !slices.Equal(production, want) {
		t.Errorf("ParsePackageSources(includeTests=false) parsed %v, want %v", production, want)
	}

	withTests := parsedBaseNames(sourceguardtest.ParsePackageSources(t, dir, true))
	if want := []string{"alpha.go", "alpha_test.go", "beta.go"}; !slices.Equal(withTests, want) {
		t.Errorf("ParsePackageSources(includeTests=true) parsed %v, want %v", withTests, want)
	}
}

func parsedBaseNames(sources []sourceguardtest.ParsedSource) []string {
	names := make([]string, 0, len(sources))
	for _, source := range sources {
		names = append(names, filepath.Base(source.Path))
	}
	return names
}
