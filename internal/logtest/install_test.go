package logtest_test

import (
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
)

func TestInstall_RoutesComponentLoggersToTheReturnedSink(t *testing.T) {
	sink := logtest.Install(t)

	log.For("hooks").Info("set", "op", "set")

	rec := sink.Records().Only(t, "log record")
	if rec.Msg != "set" {
		t.Errorf("Msg = %q, want %q", rec.Msg, "set")
	}
	if got := rec.AttrString(t, "component"); got != "hooks" {
		t.Errorf("component = %q, want %q", got, "hooks")
	}
}

func TestInstall_RestoresThePriorHandlerOnCleanup(t *testing.T) {
	outer := logtest.Install(t)

	t.Run("inner", func(t *testing.T) {
		inner := logtest.Install(t)
		log.For("hooks").Info("inner line")
		if len(inner.Records()) != 1 {
			t.Fatalf("inner sink captured %d records, want 1", len(inner.Records()))
		}
	})

	log.For("hooks").Info("outer line")

	rec := outer.Records().Only(t, "log record")
	if rec.Msg != "outer line" {
		t.Errorf("Msg = %q, want %q (inner install must not leak past its subtest)", rec.Msg, "outer line")
	}
}
