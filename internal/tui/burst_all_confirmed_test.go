package tui

import (
	"errors"
	"testing"

	"github.com/leeovery/portal/internal/spawn"
)

func confirmedResult(sess string) spawn.WindowResult {
	return spawn.WindowResult{Session: sess, Ack: spawn.AckConfirmed, Result: spawn.Success("")}
}

func timeoutResult(sess string) spawn.WindowResult {
	return spawn.WindowResult{Session: sess, Ack: spawn.AckTimeout, Result: spawn.Success("")}
}

func failedResult(sess string) spawn.WindowResult {
	return spawn.WindowResult{Session: sess, Ack: spawn.AckFailed, Result: spawn.SpawnFailed("boom")}
}

func permissionResult(sess string) spawn.WindowResult {
	return spawn.WindowResult{Session: sess, Ack: spawn.AckFailed, Result: spawn.PermissionRequired("evt -1743", "grant access")}
}

func sessionsOf(results []spawn.WindowResult) []string {
	names := make([]string, len(results))
	for i, r := range results {
		names[i] = r.Session
	}
	return names
}

func TestBurstAllConfirmed_TruthTable(t *testing.T) {
	external := []string{"alpha", "bravo"}
	confirmedPair := []spawn.WindowResult{confirmedResult("alpha"), confirmedResult("bravo")}

	tests := []struct {
		name string
		msg  spawnCompleteMsg
		want bool
	}{
		{
			name: "error-free full-length all-confirmed → true",
			msg:  spawnCompleteMsg{Results: confirmedPair},
			want: true,
		},
		{
			name: "one ack-timeout → false",
			msg:  spawnCompleteMsg{Results: []spawn.WindowResult{confirmedResult("alpha"), timeoutResult("bravo")}},
			want: false,
		},
		{
			name: "one ack-failed → false",
			msg:  spawnCompleteMsg{Results: []spawn.WindowResult{confirmedResult("alpha"), failedResult("bravo")}},
			want: false,
		},
		{
			name: "permission result → false",
			msg:  spawnCompleteMsg{Results: []spawn.WindowResult{confirmedResult("alpha"), permissionResult("bravo")}},
			want: false,
		},
		{
			name: "msg.Err set (results otherwise all-confirmed) → false",
			msg:  spawnCompleteMsg{Err: errors.New("os.Executable: boom"), Results: confirmedPair},
			want: false,
		},
		{
			name: "length mismatch — too few results → false",
			msg:  spawnCompleteMsg{Results: []spawn.WindowResult{confirmedResult("alpha")}},
			want: false,
		},
		{
			name: "length mismatch — too many results → false",
			msg:  spawnCompleteMsg{Results: []spawn.WindowResult{confirmedResult("alpha"), confirmedResult("bravo"), confirmedResult("charlie")}},
			want: false,
		},
		{
			name: "empty results vs non-empty external → false (the length guard covers the vacuous case)",
			msg:  spawnCompleteMsg{Results: nil},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{burstExternal: external}
			if got := m.burstAllConfirmed(tt.msg); got != tt.want {
				t.Errorf("burstAllConfirmed() = %v, want %v", got, tt.want)
			}
		})
	}
}

type burstClass int

const (
	classAllConfirmed burstClass = iota
	classPermission
	classPartial
)

func (c burstClass) String() string {
	switch c {
	case classAllConfirmed:
		return "all-confirmed"
	case classPermission:
		return "permission"
	default:
		return "partial"
	}
}

func canonicalBurstClass(results []spawn.WindowResult) burstClass {
	if _, failed := spawn.PartitionResults(results); len(failed) == 0 {
		return classAllConfirmed
	}
	if _, ok := spawn.FirstPermission(results); ok {
		return classPermission
	}
	return classPartial
}

func TestBurstAllConfirmed_ClassificationParityWithChokepoint(t *testing.T) {
	tests := []struct {
		name    string
		results []spawn.WindowResult
		want    burstClass
	}{
		{"all confirmed", []spawn.WindowResult{confirmedResult("s1"), confirmedResult("s2")}, classAllConfirmed},
		{"single confirmed", []spawn.WindowResult{confirmedResult("s1")}, classAllConfirmed},
		{"one timeout is partial", []spawn.WindowResult{confirmedResult("s1"), timeoutResult("s2")}, classPartial},
		{"one spawn-failed is partial", []spawn.WindowResult{confirmedResult("s1"), failedResult("s2")}, classPartial},
		{"timeout and failed is partial", []spawn.WindowResult{timeoutResult("s1"), failedResult("s2")}, classPartial},
		{"permission takes precedence over partial", []spawn.WindowResult{confirmedResult("s1"), permissionResult("s2")}, classPermission},
		{"permission with a trailing failed is still permission", []spawn.WindowResult{permissionResult("s1"), failedResult("s2")}, classPermission},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canonicalBurstClass(tt.results); got != tt.want {
				t.Fatalf("canonicalBurstClass = %s, want %s (the shared spawn.PartitionResults/FirstPermission derivation)", got, tt.want)
			}

			m := Model{burstExternal: sessionsOf(tt.results)}
			gotConfirmed := m.burstAllConfirmed(spawnCompleteMsg{Results: tt.results})
			wantConfirmed := tt.want == classAllConfirmed
			if gotConfirmed != wantConfirmed {
				t.Errorf("burstAllConfirmed = %v, want %v (picker gate must equal the chokepoint's all-confirmed class)", gotConfirmed, wantConfirmed)
			}

			_, failed := spawn.PartitionResults(tt.results)
			if gotConfirmed != (len(failed) == 0) {
				t.Errorf("burstAllConfirmed = %v, but spawn.PartitionResults failed==empty is %v — the picker gate must rest on the same relationship the CLI's does", gotConfirmed, len(failed) == 0)
			}
		})
	}
}
