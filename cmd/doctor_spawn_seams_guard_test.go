package cmd

import (
	"go/ast"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

// resolveDoctorDeps must source its Detector/Resolve from the shared
// buildProductionSpawnSeams bundle rather than hand-rebuilding them: the
// compiler cannot catch a seam added or swapped on only one side.
func TestResolveDoctorDepsUsesSharedSpawnSeams(t *testing.T) {
	source := sourceguardtest.PackageSource(t, ".", "doctor.go")

	fn := findFuncDeclInFile(t, source.File, "resolveDoctorDeps")

	sawBundle := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			switch fun.Name {
			case "buildResolver":
				pos := source.Position(call.Pos())
				t.Errorf("doctor.go:%d resolveDoctorDeps calls buildResolver() directly; route the host-terminal Resolve seam through buildProductionSpawnSeams instead", pos.Line)
			case "buildProductionSpawnSeams":
				sawBundle = true
			}
		case *ast.SelectorExpr:
			if pkg, ok := fun.X.(*ast.Ident); ok && pkg.Name == "spawn" && fun.Sel.Name == "NewDetector" {
				pos := source.Position(call.Pos())
				t.Errorf("doctor.go:%d resolveDoctorDeps calls spawn.NewDetector directly; route the host-terminal Detector seam through buildProductionSpawnSeams instead", pos.Line)
			}
		}
		return true
	})

	if !sawBundle {
		t.Error("resolveDoctorDeps does not call buildProductionSpawnSeams; its Detector/Resolve seams must originate from the shared bundle")
	}
}

func findFuncDeclInFile(t *testing.T, file *ast.File, name string) *ast.FuncDecl {
	t.Helper()
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name {
			return fn
		}
	}
	t.Fatalf("function %s not found in parsed file", name)
	return nil
}
