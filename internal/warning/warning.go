// Package warning provides the canonical shape and stderr emission helper for
// soft bootstrap warnings. It sits below the cmd→tui import boundary so both
// sides can share one definition and emit identical output.
package warning

import (
	"fmt"
	"io"
)

// Warning is a soft bootstrap failure that must not terminate Portal. Lines are
// emitted in order, one per line, with no banners, colours or prefixes.
type Warning struct {
	Lines []string
}

// WriteLines emits every warning's lines to w in slice order. Write errors are
// ignored: a diagnostic must not itself fail the program.
func WriteLines(w io.Writer, ws []Warning) {
	for _, warn := range ws {
		for _, line := range warn.Lines {
			_, _ = fmt.Fprintln(w, line)
		}
	}
}
