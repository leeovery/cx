package hookstest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/hookstest"
	"github.com/leeovery/portal/internal/nanoid"
)

func TestHookKeySeedVocabulary(t *testing.T) {
	t.Run("a reapable key is token-shaped", func(t *testing.T) {
		for n := range 4 {
			if got := hookstest.ReapableHookKey(n); !nanoid.IsTokenShaped(got) {
				t.Errorf("ReapableHookKey(%d) = %q, want a token-shaped key", n, got)
			}
		}
	})

	t.Run("it authors a reapable key at the pane-token width", func(t *testing.T) {
		token, err := nanoid.NewPaneTokenGenerator()()
		if err != nil {
			t.Fatalf("mint a pane token: %v", err)
		}
		for n := range 4 {
			if got := hookstest.ReapableHookKey(n); len(got) != len(token) {
				t.Errorf("ReapableHookKey(%d) = %q (%d bytes), want the pane-token width of %d", n, got, len(got), len(token))
			}
		}
	})

	t.Run("an unjudgeable key is not token-shaped", func(t *testing.T) {
		for n := range 4 {
			if got := hookstest.UnjudgeableHookKey(n); nanoid.IsTokenShaped(got) {
				t.Errorf("UnjudgeableHookKey(%d) = %q, want a key the staleness rule cannot judge", n, got)
			}
		}
	})

	t.Run("successive n give distinct keys", func(t *testing.T) {
		seen := map[string]int{}
		for n := range 8 {
			for _, got := range []string{hookstest.ReapableHookKey(n), hookstest.UnjudgeableHookKey(n)} {
				if prev, dup := seen[got]; dup {
					t.Errorf("key %q produced for both n=%d and n=%d", got, prev, n)
				}
				seen[got] = n
			}
		}
	})
}

// recordingT stands in for *testing.T so the assertion's own failing paths can
// be exercised without failing the harness running them.
type recordingT struct {
	errors []string
	fatals []string
}

func (r *recordingT) Helper() {}

func (r *recordingT) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func (r *recordingT) Fatalf(format string, args ...any) {
	r.fatals = append(r.fatals, fmt.Sprintf(format, args...))
	panic(recordedFatal{})
}

type recordedFatal struct{}

// captureAssert runs fn against the stand-in and reports what it said,
// absorbing the panic a Fatalf raises so an unexpected fatal is a returned
// message rather than an abort.
func captureAssert(fn func(*recordingT)) (rec *recordingT) {
	rec = &recordingT{}
	defer func() { _ = recover() }()
	fn(rec)
	return rec
}

func stageHooksFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hooks.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("seed hooks.json: %v", err)
	}
	return path
}

func TestAssertHooksFileUnchanged(t *testing.T) {
	t.Run("it passes when the file is byte-identical before and after", func(t *testing.T) {
		path := stageHooksFile(t, `{"tok01":{"on-resume":"npm start"}}`)
		before := hookstest.HooksFileBytes(t, path)

		rec := captureAssert(func(rt *recordingT) {
			hookstest.AssertHooksFileUnchanged(rt, path, before)
		})

		if len(rec.errors) != 0 || len(rec.fatals) != 0 {
			t.Errorf("an unchanged file reported errors %v and fatals %v, want neither", rec.errors, rec.fatals)
		}
	})

	t.Run("it fails when a single byte changed", func(t *testing.T) {
		path := stageHooksFile(t, `{"tok01":{"on-resume":"npm start"}}`)
		before := hookstest.HooksFileBytes(t, path)
		if err := os.WriteFile(path, []byte(`{"tok01":{"on-resume":"npm starT"}}`), 0o600); err != nil {
			t.Fatalf("rewrite hooks.json: %v", err)
		}

		rec := captureAssert(func(rt *recordingT) {
			hookstest.AssertHooksFileUnchanged(rt, path, before)
		})

		if len(rec.errors) != 1 {
			t.Fatalf("got %d errors, want exactly 1: %v", len(rec.errors), rec.errors)
		}
		if want := "changed on a failing route"; !strings.Contains(rec.errors[0], want) {
			t.Errorf("failure message %q does not carry the default wording %q", rec.errors[0], want)
		}
		if !strings.Contains(rec.errors[0], "npm starT") {
			t.Errorf("failure message %q does not carry the after bytes", rec.errors[0])
		}
	})

	t.Run("it uses the caller's context in the failure message", func(t *testing.T) {
		path := stageHooksFile(t, `{"tok01":{"on-resume":"npm start"}}`)
		before := hookstest.HooksFileBytes(t, path)
		if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
			t.Fatalf("rewrite hooks.json: %v", err)
		}

		rec := captureAssert(func(rt *recordingT) {
			hookstest.AssertHooksFileUnchanged(rt, path, before, "rewritten on a stand-down")
		})

		if len(rec.errors) != 1 {
			t.Fatalf("got %d errors, want exactly 1: %v", len(rec.errors), rec.errors)
		}
		if want := "rewritten on a stand-down"; !strings.Contains(rec.errors[0], want) {
			t.Errorf("failure message %q does not carry the caller's context %q", rec.errors[0], want)
		}
		if unwanted := "changed on a failing route"; strings.Contains(rec.errors[0], unwanted) {
			t.Errorf("failure message %q still carries the default wording %q", rec.errors[0], unwanted)
		}
	})

	t.Run("it fatals when the read fails for a reason other than absence", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hooks.json")
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatalf("stage a directory at the hooks.json path: %v", err)
		}

		rec := captureAssert(func(rt *recordingT) {
			hookstest.HooksFileBytes(rt, path)
		})

		if len(rec.fatals) != 1 {
			t.Fatalf("got %d fatals, want exactly 1: %v", len(rec.fatals), rec.fatals)
		}
		if !strings.Contains(rec.fatals[0], path) {
			t.Errorf("fatal message %q does not name the path it could not read", rec.fatals[0])
		}
	})

	t.Run("it treats an absent file as absent rather than fatalling", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hooks.json")

		before := hookstest.HooksFileBytes(t, path)
		if before != nil {
			t.Fatalf("HooksFileBytes of an absent file = %q, want nil", before)
		}

		rec := captureAssert(func(rt *recordingT) {
			hookstest.AssertHooksFileUnchanged(rt, path, before)
		})

		if len(rec.fatals) != 0 {
			t.Errorf("an absent file fatalled: %v", rec.fatals)
		}
		if len(rec.errors) != 0 {
			t.Errorf("an absent file before and after compared unequal: %v", rec.errors)
		}
	})
}
