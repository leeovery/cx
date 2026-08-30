## Attempt 1

ISSUES:
- internal/tmux/portal_saver_lifecycle_events_test.go:16-45 (and :346) still declares a full member of
  families (a) and (b): `saverEventSink` wrapping `*logtest.Sink`, `saverEvents(msg)` (a hardcoded
  `component=="saver"` + `r.Msg` filter, :20-29), `onlySaverEvent` (the exactly-one accessor, :32-38),
  `installSaverEventSink` (the four-line install, :40-45), plus `firstSaverIndex` (:346, the same
  component/msg filter). This is the same wrapper shape as `cleanSummarySink` / `fifoSummarySink` /
  `captureSummarySink`, all three of which this task deleted — so criterion 1 ("no test file outside
  `internal/logtest` declares a sink-install helper or a `(component, msg)` filter") is not met, and
  the family is half-converted. The executor swept beyond the bank item's enumeration for spawn's
  `debugRecords`/`infoRecords` and tui's `recordsByLevel` on exactly this reasoning; `internal/tmux`
  was simply not grepped. All 43 occurrences are confined to this one file.
  FIX: delete the `saverEventSink` type and its three methods; replace the 15 `installSaverEventSink(t)`
  sites with `logtest.Install(t)`, the 10 `sink.onlySaverEvent(t, msg)` sites with
  `sink.OnlyRecordWith(t, "saver", msg)`, and the `sink.saverEvents(msg)` sites with
  `sink.RecordsWith("saver", msg)`. Keep `firstSaverIndex` as a free function taking `*logtest.Sink`,
  implemented over `sink.RecordsWith("saver", msg)` against `sink.Records()` — logtest has no index
  accessor and does not need one for a single caller. Verify the file's verdict count is unchanged.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- cmd/logging_capture_test.go:41-43 — the comment claims something about the whole package that is
  false (cmd asserts records under `capture`, `hydrate`, `theme`, `signal`, `clean`, `bootstrap`,
  `projects`…, not only `hooks`), and it is a claim about what the package's tests assert, which
  ordinary additive change falsifies.
  OLD: // hooksRecordWant is the hooks-component half of a logtest.RecordWant: every
       // record this package asserts is emitted under that one component, so its
       // callers name only the parts that vary.
  NEW: // hooksRecordWant is the hooks-component half of a logtest.RecordWant: its
       // callers name only the parts that vary.

NOTES:
- Five helpers still hand-roll the two-line install inside a larger body (`newSharedExecFailureCapture`
  cmd/state_hydrate_exec_failure_test.go:16, `newSharedSelfEjectCapture` cmd/state_daemon_self_eject_log_test.go:20,
  `assertNoThemeRecords` cmd/theme_test.go:384, `resolveLoader` internal/theme/resolve_test.go:21,
  `themeOpenTestLoader` internal/tui/theme_panel_open_test.go:90). None is a bare install helper — each
  builds a logger, loader or assertion around it — so they fall under the executor's already-banked
  inline-install item rather than criterion 1. Same for `assertNoWarn` (internal/spawn/detect_test.go:22),
  which fuses a level check into an assertion loop.
- cmd/config_migrate_logging_test.go:255-263: the three surviving assertions in
  `TestConfigFilePathThreadsComponent` were reordered (component moved below op) with no change in
  what they check. Harmless, but it is diff noise in a task whose value is mechanical fidelity.
- `AssertRecord`'s doc says it reports each mismatch separately, but a record missing the `component`,
  `op` or `via` attr aborts via `AttrString`'s `Fatalf` before the later checks run. That matches the
  behaviour of the inline blocks it replaced, so nothing regressed; worth knowing if the helper is
  ever pointed at a record shape that legitimately omits one of the three.
