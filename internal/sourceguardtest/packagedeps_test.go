package sourceguardtest_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

const selfPkg = "github.com/leeovery/portal/internal/sourceguardtest"

func TestPackageDeps_EnumeratesTransitiveDependencies(t *testing.T) {
	deps := sourceguardtest.PackageDeps(t, selfPkg)

	if !slices.Contains(deps, selfPkg) {
		t.Errorf("PackageDeps(%s) omits the package itself: %v", selfPkg, deps)
	}
	// go/parser is imported directly; go/scanner arrives only through it, so
	// the pair distinguishes a transitive list from an immediate one — the
	// depth the guards behind this primitive police.
	for _, want := range []string{"go/parser", "go/scanner"} {
		if !slices.Contains(deps, want) {
			t.Errorf("PackageDeps(%s) omits %s: %v", selfPkg, want, deps)
		}
	}
}

func TestPackageDeps_FatalsWhenGoListCannotResolveThePackage(t *testing.T) {
	stub := &recordingT{}

	deps := sourceguardtest.PackageDeps(stub, selfPkg+"/no-such-package")

	if !stub.fataled {
		t.Fatalf("PackageDeps did not fatal on an unresolvable package — a leaf guard would pass over an empty dependency set; errors %v", stub.errors)
	}
	if deps != nil {
		t.Errorf("PackageDeps returned %v after fatalling, want nil", deps)
	}
	if !strings.Contains(stub.msg, "go list -deps") {
		t.Errorf("fatal message %q does not name the failing command", stub.msg)
	}
}

// recordingT stands in for *testing.T so the fatal path is observable. A real
// Fatalf ends the goroutine, which the caller cannot rely on here, so the
// recorder returns and PackageDeps' own explicit return covers it.
type recordingT struct {
	fataled bool
	msg     string
	errors  []string
}

func (r *recordingT) Helper() {}

func (r *recordingT) Fatalf(format string, args ...any) {
	r.fataled = true
	r.msg = fmt.Sprintf(format, args...)
}
func (r *recordingT) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}
