package theme_test

import (
	"go/ast"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
	"github.com/leeovery/portal/internal/theme"
)

func TestSlotDefault_IsTheSlotsOwnShippedDefaultOnEveryPath(t *testing.T) {
	requireDistinctDefaults(t)

	tests := []struct {
		name       string
		slot       theme.Slot
		otherSlot  theme.RawKeys
		nominated  string
		slotsIndex int
		want       string
	}{
		{
			name:       "the light slot",
			slot:       theme.SlotLight,
			otherSlot:  theme.RawKeys{Dark: "nord"},
			nominated:  "gone-light",
			slotsIndex: 0,
			want:       theme.DefaultLightSlug,
		},
		{
			name:       "the dark slot",
			slot:       theme.SlotDark,
			otherSlot:  theme.RawKeys{Light: "nord"},
			nominated:  "gone-dark",
			slotsIndex: 1,
			want:       theme.DefaultDarkSlug,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setting, _ := theme.ResolveSetting(tt.otherSlot)

			if got := setting.Slug(tt.slot); got != tt.want {
				t.Errorf("ResolveSetting substituted %q into the unset slot, want %q", got, tt.want)
			}
			if got := (theme.Setting{}).Slug(tt.slot); got != tt.want {
				t.Errorf("Slug(%v) on an unset slot = %q, want %q", tt.slot, got, tt.want)
			}

			broken := theme.Setting{Light: "gone-light", Dark: "gone-dark"}
			resolution, err := nominationLoader().ResolveNomination(broken, t.TempDir())
			if err != nil {
				t.Fatalf("ResolveNomination(%+v) = %v, want the fallback applied", broken, err)
			}
			fallen := resolution.Slots[tt.slotsIndex]
			if fallen.Slot != tt.slot || fallen.Requested != tt.nominated {
				t.Fatalf("Slots[%d] = %+v, want the %v slot's resolution of %q", tt.slotsIndex, fallen, tt.slot, tt.nominated)
			}
			if fallen.Resolved != tt.want {
				t.Errorf("the %v slot fell back to %q, want %q", tt.slot, fallen.Resolved, tt.want)
			}
		})
	}
}

func TestSlotDefault_IsPairedWithASlotInOneFunction(t *testing.T) {
	var pairing []string

	for _, source := range sourceguardtest.ParsePackageSources(t, ".", false) {
		for _, decl := range source.File.Decls {
			fn, isFn := decl.(*ast.FuncDecl)
			if !isFn || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				ident, isIdent := n.(*ast.Ident)
				if !isIdent || (ident.Name != "DefaultLightSlug" && ident.Name != "DefaultDarkSlug") {
					return true
				}
				if !slices.Contains(pairing, fn.Name.Name) {
					pairing = append(pairing, fn.Name.Name)
				}
				return true
			})
		}
	}

	if len(pairing) != 1 {
		slices.Sort(pairing)
		t.Errorf("the shipped defaults are named in %v, want exactly one function — a slot's default is paired with its slot in one place so the pairing cannot be half-changed", pairing)
	}
}
