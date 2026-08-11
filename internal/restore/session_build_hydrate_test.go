package restore

import (
	"strings"
	"testing"
)

func TestBuildHydrateCommand(t *testing.T) {
	t.Run("typical inputs produce single-quoted invocation without sh -c envelope", func(t *testing.T) {
		got := buildHydrateCommand("/x.fifo", "/y.bin", "work:0.0")
		want := "portal state hydrate --fifo '/x.fifo' --file '/y.bin' --hook-key 'work:0.0'"
		if got != want {
			t.Errorf("buildHydrateCommand:\n got %q\nwant %q", got, want)
		}
	})

	t.Run("empty hookKey produces well-formed quoted invocation", func(t *testing.T) {
		got := buildHydrateCommand("/x.fifo", "/y.bin", "")
		want := "portal state hydrate --fifo '/x.fifo' --file '/y.bin' --hook-key ''"
		if got != want {
			t.Errorf("buildHydrateCommand empty hook-key:\n got %q\nwant %q", got, want)
		}
	})

	t.Run("output contains no sh -c envelope or exec $SHELL trailer", func(t *testing.T) {
		got := buildHydrateCommand("/x.fifo", "/y.bin", "work:0.0")
		if strings.Contains(got, "sh -c") {
			t.Errorf("buildHydrateCommand output %q must not contain `sh -c`", got)
		}
		if strings.Contains(got, "exec $SHELL") {
			t.Errorf("buildHydrateCommand output %q must not contain `exec $SHELL`", got)
		}
	})

	t.Run("canonical fifo/file/hookKey shape is single-quoted", func(t *testing.T) {
		got := buildHydrateCommand(
			"/tmp/portal/hydrate-work__0.0.fifo",
			"/tmp/portal/dump-portal-session-XYZ__0.1.bin",
			"work:0.0",
		)
		want := "portal state hydrate --fifo '/tmp/portal/hydrate-work__0.0.fifo' --file '/tmp/portal/dump-portal-session-XYZ__0.1.bin' --hook-key 'work:0.0'"
		if got != want {
			t.Errorf("buildHydrateCommand canonical:\n got %q\nwant %q", got, want)
		}
	})

	t.Run("hookKey with whitespace is single-quoted as one token", func(t *testing.T) {
		got := buildHydrateCommand("/x.fifo", "/y.bin", "evvi webhooks and watchers:0.0")
		want := "portal state hydrate --fifo '/x.fifo' --file '/y.bin' --hook-key 'evvi webhooks and watchers:0.0'"
		if got != want {
			t.Errorf("buildHydrateCommand whitespace hookKey:\n got %q\nwant %q", got, want)
		}
	})

	t.Run("hookKey with embedded single quote uses close-escape-reopen idiom", func(t *testing.T) {
		got := buildHydrateCommand("/x.fifo", "/y.bin", "it's:0.0")
		want := `portal state hydrate --fifo '/x.fifo' --file '/y.bin' --hook-key 'it'\''s:0.0'`
		if got != want {
			t.Errorf("buildHydrateCommand embedded-quote hookKey:\n got %q\nwant %q", got, want)
		}
	})

	t.Run("hookKey with shell-meta bytes is single-quoted", func(t *testing.T) {
		got := buildHydrateCommand("/x.fifo", "/y.bin", "a$b`c;d:0.0")
		want := "portal state hydrate --fifo '/x.fifo' --file '/y.bin' --hook-key 'a$b`c;d:0.0'"
		if got != want {
			t.Errorf("buildHydrateCommand shell-meta hookKey:\n got %q\nwant %q", got, want)
		}
	})
}
