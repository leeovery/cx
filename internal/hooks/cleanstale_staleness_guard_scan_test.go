package hooks_test

import (
	"fmt"
	"go/ast"
	"go/token"
	"testing"
)

// A guard whose scan silently found nothing reports a safety it is not
// providing, so the shared skeleton must fail rather than fall through.
func TestScanPackageCalls_FatalsWhenItEnumeratesNoFiles(t *testing.T) {
	stub := &scanRecorderT{}
	visited := 0

	scanPackageCalls(stub, t.TempDir(), func(string, *token.FileSet, string, *ast.CallExpr) {
		visited++
	})

	if !stub.fataled {
		t.Fatal("scanPackageCalls did not fatal over a directory holding no sources — a guard driven by it would pass having scanned nothing")
	}
	if visited != 0 {
		t.Errorf("scanPackageCalls visited %d calls over an empty directory, want 0", visited)
	}
}

// scanRecorderT stands in for *testing.T so the scan's own fatal is
// observable. A real Fatalf ends the goroutine; the recorder returns instead,
// which the scan's explicit returns after each Fatalf accommodate.
type scanRecorderT struct {
	fataled bool
	msg     string
}

func (r *scanRecorderT) Helper() {}

func (r *scanRecorderT) Fatalf(format string, args ...any) {
	r.fataled = true
	r.msg = fmt.Sprintf(format, args...)
}
