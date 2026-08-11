//go:build integration

package restoretest_test

import (
	"reflect"
	"testing"

	"github.com/leeovery/portal/internal/restoretest"
)

func TestSortedKeySet(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]struct{}
		want []string
	}{
		{
			name: "empty",
			in:   map[string]struct{}{},
			want: []string{},
		},
		{
			name: "single",
			in:   map[string]struct{}{"alpha": {}},
			want: []string{"alpha"},
		},
		{
			name: "already sorted",
			in:   map[string]struct{}{"alpha": {}, "beta": {}, "gamma": {}},
			want: []string{"alpha", "beta", "gamma"},
		},
		{
			name: "reverse",
			in:   map[string]struct{}{"gamma": {}, "beta": {}, "alpha": {}},
			want: []string{"alpha", "beta", "gamma"},
		},
		{
			name: "unsorted",
			in:   map[string]struct{}{"beta": {}, "gamma": {}, "alpha": {}},
			want: []string{"alpha", "beta", "gamma"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := restoretest.SortedKeySet(tt.in)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SortedKeySet(%v) = %v; want %v", tt.in, got, tt.want)
			}
		})
	}
}
