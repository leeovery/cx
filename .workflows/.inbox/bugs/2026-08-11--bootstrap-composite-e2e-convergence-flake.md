# The bootstrap composite e2e convergence assertion flakes

The composite end-to-end tests in `cmd/bootstrap` fail intermittently on their
daemon-convergence assertion, and they do so often enough to make the
integration lane unreliable as a gate. Across a run of five invocations of
`go test -tags integration -count=1 ./cmd/bootstrap/`, four failed. The failure
is not deterministic and not tied to one test: `TestCompositeBootstrap_
ConvergesPgrepToOneWithin6s` and `TestCompositeBootstrap_
ExternalSaverKillTriggersSelfEject` have each been seen failing on separate
runs, both on the same shared assertion.

The message is of the form "post-bootstrap: pgrep -fx did not converge to 1
within 6s of bootstrap-slice entry", with the elapsed time printed alongside the
budget. The observed overshoots are small: 6.076s and 6.379s against a 6s
budget, so it misses by tens to a few hundred milliseconds. The diagnostic block
the assertion prints is itself informative — in both observed failures every
harness PID is reported `alive=false` and the current pgrep snapshot is empty,
meaning convergence did in fact happen and the daemons did die; the measurement
simply landed outside the window. The tests are in `cmd/bootstrap/composition_
e2e_convergence_integration_test.go` and `composition_e2e_self_eject_
integration_test.go`.

It is worse under load. Both failures first appeared while the machine was busy
with other test suites, and a related timing assertion —
`TestTailScrollback_PerformanceBudget` in `internal/state`, which allows 5ms for
a warmed tail-N read — failed in the same window at 14.9ms and then passed three
times consecutively once the machine was quiet. But contention is not the whole
story, since the composite suite still failed four times out of five when run on
its own with nothing else running.

This is pre-existing and not introduced by any recent change. It was checked
against the `chore/comment-strip` branch and then against the base commit
`2eb64f04` in a separate git worktree; the base tree failed the same assertion
too. The only difference in those test files on the branch is failure-message
string text, which renders only after a test has already failed.

CLAUDE.md already notes that these suites are timing-sensitive and that `-p 1`
on the integration lane is load-bearing because the daemon-timing tests flake
under mutual contention. The observation here is that the flaking persists even
serially and even in isolation, so the current budget does not appear to hold on
this machine under ordinary conditions.
