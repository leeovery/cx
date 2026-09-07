package sourceguardtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/leeovery/portal/internal/harnesstest"
)

// ParseMode is the mode every guard's sources are parsed under. Comments are
// retained because a guard reading a build tag reads a comment, and object
// resolution is skipped because these rules are read off the AST's shape alone;
// both together are a superset of what any one guard needs, which is what keeps
// the mode a single stated value rather than a per-guard decision.
const ParseMode = parser.SkipObjectResolution | parser.ParseComments

// ParsedSource is one parsed .go file: the path it was read from, the file set
// its positions resolve against, and the syntax tree a guard walks.
type ParsedSource struct {
	Path string
	Fset *token.FileSet
	File *ast.File
}

// Position resolves pos within the source, naming the file as Path does rather
// than as the file set parsed it, so a finding built from it reads the way the
// scan names its sources.
func (s ParsedSource) Position(pos token.Pos) token.Position {
	position := s.Fset.Position(pos)
	position.Filename = s.Path
	return position
}

// ParsePackageSources parses the .go files held directly by dir, test sources
// included only when includeTests is set, in the filename order PackageGoFiles
// returns. Enumerating nothing, parsing nothing and failing to parse a file are
// all fatal: a guard that scanned nothing reports a safety it is not providing.
func ParsePackageSources(t harnesstest.TestingT, dir string, includeTests bool) []ParsedSource {
	t.Helper()

	paths, err := PackageGoFiles(dir, includeTests)
	if err != nil {
		t.Fatalf("enumerate package sources: %v", err)
		return nil
	}
	return ParseSources(t, paths)
}

// PackageSource returns the source named by base name among the .go files held
// directly by dir. Test sources are searched only when the name asks for one.
// Finding no such source is fatal: a guard reading a file that has been renamed
// away must fail rather than judge nothing.
func PackageSource(t harnesstest.TestingT, dir, name string) ParsedSource {
	t.Helper()

	for _, source := range ParsePackageSources(t, dir, strings.HasSuffix(name, "_test.go")) {
		if filepath.Base(source.Path) == name {
			return source
		}
	}
	t.Fatalf("no %s among the sources of %s", name, dir)
	return ParsedSource{}
}

// ParseSources parses each of paths in order. An unparseable file is fatal, as
// is an empty result: a guard handed no source would otherwise pass by having
// stopped looking.
func ParseSources(t harnesstest.TestingT, paths []string) []ParsedSource {
	t.Helper()

	sources := make([]ParsedSource, 0, len(paths))
	for _, path := range paths {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, ParseMode)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
			return nil
		}
		sources = append(sources, ParsedSource{Path: path, Fset: fset, File: file})
	}
	if len(sources) == 0 {
		t.Fatalf("parsed no sources, so a guard over them would pass by having stopped looking")
		return nil
	}
	return sources
}
