package cmd

import (
	"reflect"
	"testing"

	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

func TestDoctorAndPanelReadOneThemesDirectoryIdentically(t *testing.T) {
	dir := t.TempDir()
	themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())
	themetest.Write(t, dir, "zed-lee.theme", themetest.WithoutKey(themetest.Lines(), "bg.subtle"))
	loader := theme.NewSilentLoader()

	doctorEnumeration := loader.OpenEnumeration(dir)
	panelEnumeration, _ := theme.Assembler{Loader: loader}.Open(dir, theme.RawKeys{})

	if !reflect.DeepEqual(doctorEnumeration, panelEnumeration) {
		t.Errorf("doctor's enumeration %+v differs from the panel's %+v", doctorEnumeration, panelEnumeration)
	}
}
