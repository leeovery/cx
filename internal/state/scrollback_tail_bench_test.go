package state_test

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/state"
)

// Warmed-cache ceiling for one TailScrollback(N=1000) against the 4 MB fixture.
// Exceeding it means the synchronous-read decision in Update needs revisiting.
const perfBudget = 5 * time.Millisecond

const (
	perfFixtureLines     = 50_000
	perfFixtureLineWidth = 80
)

func buildPerfFixture(tb testing.TB, dir string) string {
	tb.Helper()
	rng := rand.New(rand.NewPCG(42, 42))
	colours := []string{
		"\x1b[31m",
		"\x1b[32m",
		"\x1b[33m",
		"\x1b[34m",
		"\x1b[36m",
	}
	const reset = "\x1b[0m"

	var buf bytes.Buffer
	buf.Grow(perfFixtureLines * (perfFixtureLineWidth + 8))

	nextAnsiAt := 3 + rng.IntN(3)

	for i := range perfFixtureLines {
		jitter := rng.IntN(perfFixtureLineWidth/5*2+1) - (perfFixtureLineWidth / 5)
		width := max(perfFixtureLineWidth+jitter, 16)

		prefix := fmt.Sprintf("[%05d] ", i)
		payloadLen := max(width-len(prefix)-1, 0)
		payload := make([]byte, payloadLen)
		for j := range payload {
			payload[j] = byte('A' + rng.IntN(26))
		}

		buf.WriteString(prefix)
		if i == nextAnsiAt {
			colour := colours[rng.IntN(len(colours))]
			buf.WriteString(colour)
			buf.Write(payload)
			buf.WriteString(reset)
			nextAnsiAt = i + 3 + rng.IntN(3)
		} else {
			buf.Write(payload)
		}
		buf.WriteByte('\n')
	}

	path := filepath.Join(dir, "perf-fixture.bin")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		tb.Fatalf("write perf fixture: %v", err)
	}
	return path
}

func BenchmarkTailScrollback(b *testing.B) {
	path := buildPerfFixture(b, b.TempDir())

	for b.Loop() {
		if _, err := state.TailScrollback(path, 1000); err != nil {
			b.Fatalf("TailScrollback: %v", err)
		}
	}
}

func TestTailScrollback_PerformanceBudget(t *testing.T) {
	if os.Getenv("PORTAL_SKIP_PERF") != "" {
		t.Skip("PORTAL_SKIP_PERF set; skipping warmed-cache perf budget assertion")
	}

	path := buildPerfFixture(t, t.TempDir())

	// Warmup so the measured call reflects the warmed-cache budget.
	if _, err := state.TailScrollback(path, 1000); err != nil {
		t.Fatalf("warmup TailScrollback: %v", err)
	}

	start := time.Now()
	got, err := state.TailScrollback(path, 1000)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("measured TailScrollback: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("expected non-empty tail bytes, got 0")
	}
	if elapsed >= perfBudget {
		t.Fatalf("warmed tail-N read = %v, budget = %v (spec: tail-N p99 < 5ms on 4 MB .bin)", elapsed, perfBudget)
	}
}
