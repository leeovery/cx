package sourceguardtest

import (
	"go/ast"
	"go/parser"
	"go/token"
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

// ParsePackageSources parses the .go files held directly by dir, test sources
// included only when includeTests is set, in the filename order PackageGoFiles
// returns. Enumerating nothing, parsing nothing and failing to parse a file are
// all fatal: a guard that scanned nothing reports a safety it is not providing.
func ParsePackageSources(t TestingT, dir string, includeTests bool) []ParsedSource {
	t.Helper()

	paths, err := PackageGoFiles(dir, includeTests)
	if err != nil {
		t.Fatalf("enumerate package sources: %v", err)
		return nil
	}
	return ParseSources(t, paths)
}

// ParseSources parses each of paths in order. An unparseable file is fatal, as
// is an empty result: a guard handed no source would otherwise pass by having
// stopped looking.
func ParseSources(t TestingT, paths []string) []ParsedSource {
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
