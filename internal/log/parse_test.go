package log

import (
	"testing"
	"time"
)

func TestParseLogLine_WellFormed(t *testing.T) {
	line := "2026-06-09T10:15:30.123456789Z WARN daemon: tick complete took=12ms pid=4242 version=1.2.3 process_role=daemon"

	parsed, ok := ParseLogLine(line)
	if !ok {
		t.Fatalf("ParseLogLine ok = false, want true for %q", line)
	}

	wantTime, err := time.Parse(time.RFC3339Nano, "2026-06-09T10:15:30.123456789Z")
	if err != nil {
		t.Fatalf("fixture timestamp unparseable: %v", err)
	}
	if !parsed.Time.Equal(wantTime) {
		t.Errorf("Time = %v, want %v", parsed.Time, wantTime)
	}
	if parsed.Level != "WARN" {
		t.Errorf("Level = %q, want %q", parsed.Level, "WARN")
	}
	if parsed.Component != "daemon" {
		t.Errorf("Component = %q, want %q", parsed.Component, "daemon")
	}
	if parsed.Message != "tick complete" {
		t.Errorf("Message = %q, want %q", parsed.Message, "tick complete")
	}
}

func TestParseLogLine_StripsAttrsAndBaselines(t *testing.T) {
	line := "2026-06-09T10:15:30Z INFO bootstrap: orchestration complete warnings=2 took=1.2s pid=9 version=1.0.0 process_role=cli"

	parsed, ok := ParseLogLine(line)
	if !ok {
		t.Fatalf("ParseLogLine ok = false, want true")
	}
	if parsed.Message != "orchestration complete" {
		t.Errorf("Message = %q, want %q", parsed.Message, "orchestration complete")
	}
}

func TestParseLogLine_NoAttrsPreservedWhole(t *testing.T) {
	line := "2026-06-09T10:15:30Z WARN restore: skeleton reconstruction failed pid=9 version=1.0.0 process_role=cli"

	parsed, ok := ParseLogLine(line)
	if !ok {
		t.Fatalf("ParseLogLine ok = false, want true")
	}
	if parsed.Message != "skeleton reconstruction failed" {
		t.Errorf("Message = %q, want %q", parsed.Message, "skeleton reconstruction failed")
	}
}

func TestParseLogLine_PreservesLaterColons(t *testing.T) {
	line := "2026-06-09T10:15:30Z WARN daemon: flush failed: disk full pid=9 version=1.0.0 process_role=daemon"

	parsed, ok := ParseLogLine(line)
	if !ok {
		t.Fatalf("ParseLogLine ok = false, want true")
	}
	if parsed.Component != "daemon" {
		t.Errorf("Component = %q, want %q", parsed.Component, "daemon")
	}
	if parsed.Message != "flush failed: disk full" {
		t.Errorf("Message = %q, want %q", parsed.Message, "flush failed: disk full")
	}
}

func TestParseLogLine_QuotedMultiWordAttrValue(t *testing.T) {
	line := `2026-06-09T10:15:30Z INFO process: start pid=9 version="3.6 beta" process_role=cli`

	parsed, ok := ParseLogLine(line)
	if !ok {
		t.Fatalf("ParseLogLine ok = false, want true")
	}
	if parsed.Message != "start" {
		t.Errorf("Message = %q, want %q", parsed.Message, "start")
	}
}

func TestParseLogLine_EmptyComponent(t *testing.T) {
	line := "2026-06-09T10:15:30Z WARN : tick complete pid=9 version=1.0.0 process_role=cli"

	parsed, ok := ParseLogLine(line)
	if !ok {
		t.Fatalf("ParseLogLine ok = false, want true")
	}
	if parsed.Component != "" {
		t.Errorf("Component = %q, want %q", parsed.Component, "")
	}
	if parsed.Message != "tick complete" {
		t.Errorf("Message = %q, want %q", parsed.Message, "tick complete")
	}
}

func TestParseLogLine_EmptyMessage(t *testing.T) {
	line := "2026-06-09T10:15:30Z WARN daemon: pid=9 version=1.0.0 process_role=daemon"

	parsed, ok := ParseLogLine(line)
	if !ok {
		t.Fatalf("ParseLogLine ok = false, want true")
	}
	if parsed.Component != "daemon" {
		t.Errorf("Component = %q, want %q", parsed.Component, "daemon")
	}
	if parsed.Message != "" {
		t.Errorf("Message = %q, want %q", parsed.Message, "")
	}
}

func TestParseLogLine_UnparseableTimestamp(t *testing.T) {
	line := "not-a-timestamp WARN daemon: tick complete pid=9 version=1.0.0 process_role=daemon"

	if _, ok := ParseLogLine(line); ok {
		t.Fatalf("ParseLogLine ok = true, want false for unparseable timestamp")
	}
}

func TestParseLogLine_NoColon(t *testing.T) {
	line := "2026-06-09T10:15:30Z WARN daemon tick complete pid=9"

	if _, ok := ParseLogLine(line); ok {
		t.Fatalf("ParseLogLine ok = true, want false for line with no colon")
	}
}

func TestParseLogLine_FewerThanTwoTokens(t *testing.T) {
	line := "2026-06-09T10:15:30Z"

	if _, ok := ParseLogLine(line); ok {
		t.Fatalf("ParseLogLine ok = true, want false for single-token line")
	}
}

func TestParseLogLine_EmptyLine(t *testing.T) {
	if _, ok := ParseLogLine(""); ok {
		t.Fatalf("ParseLogLine ok = true, want false for empty line")
	}
}

func TestParseLogLine_WholeAndFractionalSecondTimestamps(t *testing.T) {
	cases := []struct {
		name string
		ts   string
	}{
		{name: "whole second", ts: "2026-06-09T10:15:30Z"},
		{name: "fractional second", ts: "2026-06-09T10:15:30.987654321Z"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			line := tc.ts + " WARN daemon: tick complete pid=9 version=1.0.0 process_role=daemon"

			parsed, ok := ParseLogLine(line)
			if !ok {
				t.Fatalf("ParseLogLine ok = false, want true for %q", tc.ts)
			}
			want, err := time.Parse(time.RFC3339Nano, tc.ts)
			if err != nil {
				t.Fatalf("fixture timestamp unparseable: %v", err)
			}
			if !parsed.Time.Equal(want) {
				t.Errorf("Time = %v, want %v", parsed.Time, want)
			}
		})
	}
}
