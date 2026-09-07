package sourceguardtest_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/harnesstest"
	"github.com/leeovery/portal/internal/sourceguardtest"
)

func TestPackageSource(t *testing.T) {
	t.Run("it returns the named source of a package directory", func(t *testing.T) {
		dir := stageScanTree(t, map[string]string{
			"wanted.go": "package fixture\n\nvar Wanted = 1\n",
			"other.go":  "package fixture\n",
		})

		source := sourceguardtest.PackageSource(t, dir, "wanted.go")

		if filepath.Base(source.Path) != "wanted.go" {
			t.Errorf("Path = %q, want the wanted.go of %s", source.Path, dir)
		}
		if source.File == nil || len(source.File.Decls) != 1 {
			t.Errorf("wanted.go came back unparsed")
		}
	})

	t.Run("it fatals when the directory holds no source of that name", func(t *testing.T) {
		dir := stageScanTree(t, map[string]string{"other.go": "package fixture\n"})

		recorder := &harnesstest.Recorder{}
		recorder.Run(func() { sourceguardtest.PackageSource(recorder, dir, "wanted.go") })

		if len(recorder.Fatals) != 1 || !strings.Contains(recorder.Fatals[0], "wanted.go") {
			t.Fatalf("fatals = %v, want one naming wanted.go", recorder.Fatals)
		}
	})
}
