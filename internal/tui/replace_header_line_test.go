package tui

import "testing"

func TestReplaceHeaderLine(t *testing.T) {
	for _, tc := range []struct {
		name     string
		listView string
		header   string
		want     string
	}{
		{
			name:     "multi-line splices header onto the tail from the first newline",
			listView: "old title\nrow1\nrow2",
			header:   "NEW HEADER",
			want:     "NEW HEADER\nrow1\nrow2",
		},
		{
			name:     "single-line no-newline listView returns header bare",
			listView: "old title only",
			header:   "NEW HEADER",
			want:     "NEW HEADER",
		},
		{
			name:     "empty listView returns header bare",
			listView: "",
			header:   "NEW HEADER",
			want:     "NEW HEADER",
		},
		{
			name:     "listView that is just a trailing newline keeps the empty tail row",
			listView: "old title\n",
			header:   "NEW HEADER",
			want:     "NEW HEADER\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := replaceHeaderLine(tc.listView, tc.header); got != tc.want {
				t.Errorf("replaceHeaderLine(%q, %q) = %q, want %q", tc.listView, tc.header, got, tc.want)
			}
		})
	}
}
