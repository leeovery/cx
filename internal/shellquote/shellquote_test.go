package shellquote_test

import (
	"testing"

	"github.com/leeovery/portal/internal/shellquote"
)

func TestSingle(t *testing.T) {
	t.Run("it wraps a plain string in single quotes", func(t *testing.T) {
		got := shellquote.Single("/usr/local/bin/portal")
		const want = `'/usr/local/bin/portal'`
		if got != want {
			t.Errorf("Single = %q, want %q", got, want)
		}
	})

	t.Run("it re-quotes an embedded single quote", func(t *testing.T) {
		got := shellquote.Single("it's")
		const want = `'it'\''s'`
		if got != want {
			t.Errorf("Single = %q, want %q", got, want)
		}
	})

	t.Run("it quotes an empty string", func(t *testing.T) {
		got := shellquote.Single("")
		const want = `''`
		if got != want {
			t.Errorf("Single = %q, want %q (an empty argument must survive as one word)", got, want)
		}
	})
}
