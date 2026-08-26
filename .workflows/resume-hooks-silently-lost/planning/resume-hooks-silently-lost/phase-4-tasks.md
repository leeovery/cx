---
phase: 4
phase_name: Removal and listing report the truth
total: 3
---

## resume-hooks-silently-lost-4-1

### Task 4-1: The store answers whether it removed anything

**Problem**: `hooks.Store.Remove` (`internal/hooks/store.go:126`) cannot tell its caller whether it removed anything. It loads, deletes if the key and event happen to be there, and then **rewrites the file unconditionally** — its own doc comment says so ("It rewrites the file even when the key or event is absent, so the breadcrumb is emitted either way") — returning only an error. Three consequences follow. `portal hook rm` has no way to exit non-zero when it removed nothing, which is the whole point of task 4-2. A removal that removed nothing still emits an INFO `op=rm` naming a removal that did not happen — the exact class of falsehood this work unit exists to remove. And the unconditional write has real effects: an absent `hooks.json` is created as `{}`, and a malformed file that `Load` tolerantly decoded to an empty map is flattened to `{}`, both on a call that changed nothing.

**Solution**: Change `Remove` to return whether it removed anything, derived from the map it loaded and mutated, and make the write, the INFO breadcrumb and every other side effect conditional on that answer — a removal that removed nothing writes no file and emits no record at all.

**Outcome**: `Remove(key, event, via)` returns `(true, nil)` exactly when it deleted the named event from the loaded map and persisted that, `(false, nil)` when there was nothing to delete, and `(false, err)` when the save failed; `hooks.json` is byte-identical across every no-removal call, an absent file stays absent, and the `hooks` log holds one INFO per genuine removal and nothing at all for the rest.

**Do**:
- Change the signature to `func (s *Store) Remove(key, event, via string) (bool, error)`. After `Load`, look the key up once in the loaded map: when the key is absent, or present with the named event absent, return `(false, nil)` immediately — before `Save`, before any log call. Otherwise delete the event, drop the outer key when its last event goes, and fall through to the existing save. Rewrite the doc comment, which currently states the file is rewritten either way.
- Derive the answer solely from the map this method loaded and mutated. Add no `Get`, no second `Load`, and no existence pre-check outside that map — a later phase wraps exactly this load-mutate-save region in one exclusive hold, and a second acquisition from the same process is not re-entrant.
- Leave the removal path otherwise untouched: `Save` failure keeps its WARN (`op=rm`, `hook_key`, `via`, `error`, `error_class`) and now returns `(false, err)`; success keeps its INFO (`op=rm`, `hook_key`, `via`) and returns `(true, nil)`.
- Update the single production caller, `cmd/hooks.go:153`, to the two-value signature: discard the boolean and return the error, so the CLI's observable behaviour is unchanged by this task. The exit rule that consumes the boolean is task 4-2's.
- Re-point `internal/hooks/store_test.go`: `TestRemove`'s two no-op subtests take the second return value and additionally assert the file is byte-identical (and that an absent file stays absent); `TestRemoveLogging`'s `"still emits INFO op=rm when removing an absent key"` inverts to assert the sink holds **no** record at all, renamed to say so; and the write-failure subtest at `:1384` gains a seeded matching entry — add a seeded variant of the `readOnlyDirPath` helper that creates the directory writable, writes `hooks.json`, then `chmod 0o500` — or it has no write left to fail and stops exercising `error_class=write-failed-temp-create`.

**Acceptance Criteria**:
- [ ] `Remove` returns `(true, nil)` when it deleted the named event and the save succeeded
- [ ] `Remove` returns `(false, nil)` when the key is absent, when the key is present but the named event is not, and when `hooks.json` does not exist
- [ ] No `Save` occurs on the no-removal path: an absent `hooks.json` stays absent, a malformed file keeps its bytes rather than being flattened to `{}`, and a key mapped to an empty event map is left in place rather than pruned as a side effect
- [ ] A no-removal call emits no log record of any kind — no INFO, no WARN, no DEBUG
- [ ] A key holding several events loses only the named one, keeps its outer key, and reports `true`; a key holding only that event has its outer key dropped, as today, and reports `true`
- [ ] A failed `Save` returns `(false, err)` alongside the existing WARN carrying `op=rm`, `hook_key`, `via`, `error` and `error_class`
- [ ] The answer is computed from the map `Remove` loaded and mutated: no `Get`, no second `Load`, no pre-read anywhere in the method
- [ ] A genuine removal still emits exactly one INFO with `op=rm`, `hook_key` and `via`, carrying no `value` attr
- [ ] `portal hook rm` behaves exactly as it does today at the CLI — the existing `cmd/hooks_test.go` `TestHooksRmCommand` subtests pass unchanged
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass

**Tests**:
- `"it reports a removal when the named event was deleted"` — seeded entry; assert `true`, nil error, and the entry gone from the file
- `"it reports no removal when the key is absent"` — assert `false`, nil error
- `"it reports no removal when the key is present but the event is not"` — seed `on-resume`, remove `on-start`
- `"it leaves an absent hooks.json absent when it removes nothing"` — assert the path still does not exist afterwards
- `"it leaves a malformed hooks.json byte-identical when it removes nothing"` — write `not json`, remove, assert the bytes are unchanged
- `"it leaves a key mapped to an empty event map in place"` — seed `{"abc123": {}}`; assert `false` and the bytes unchanged
- `"it keeps the other events of a key and reports a removal"` — a key with `on-resume` plus another event; assert `true`, the outer key retained, the sibling event intact
- `"it drops the outer key when its last event goes"` — the existing behaviour, now also asserting `true`
- `"it emits no record at all when it removes nothing"` — capture with `logtest.Sink`; assert zero records across the absent-key, absent-event and absent-file cases
- `"it still emits INFO op=rm for a real removal"` — the re-pointed logging subtest, unchanged in its assertions
- `"it reports no removal when the save fails"` — seeded read-only dir; assert `false`, a non-nil error classified `write-failed-temp-create`, and the WARN record
- `"it leaves hook rm's CLI behaviour unchanged"` — the existing `cmd` rm subtests still exit 0 on the paths they cover

**Edge Cases**:
- The answer must come from the loaded-and-mutated map, never from a separate pre-read: a later phase wraps this exact region in one exclusive hold, and a second read would sit outside it and decide the answer from a snapshot the mutation never saw
- A key present with the named event absent removes nothing — the same answer as an absent key, and the same silence
- An absent `hooks.json` must stay **absent**: today's unconditional rewrite creates `{}`, and that is the one case where "byte-identical" genuinely fails
- A malformed file decodes tolerantly to an empty map, so a removal that removes nothing must no longer flatten it to `{}`
- A hand-edited key mapped to an empty event map is left in place rather than pruned as a side effect — no write happens, so nothing prunes it
- A failed save reports **no** removal: the entry is still on disk, and reporting `true` would let the caller exit 0 on a removal that did not land
- The write-failure fixture at `store_test.go:1384` drives a read-only dir with an *absent* key, so after this change it has no write left to fail and must seed a matching entry to keep exercising the write-failure classification
- `Remove` has exactly one production caller (`cmd/hooks.go:153`), so the signature change is contained; the caller must not start acting on the boolean in this task

**Context**:
> **Settled decision — a removal that removed nothing emits no breadcrumb at all.** This work unit's amendment to the closed `hooks` `op` vocabulary is fixed at three values — `load-unlocked`, `touch-save-requested` and `clean-stale-skipped` — so an `rm-noop` verb is not available and must not be invented at the call site. The rationale is stronger than the vocabulary constraint anyway: an `op=rm` INFO naming a removal that did not happen is exactly the falsehood this work unit exists to remove, and 61 of the 63 degenerate lines found in a month of `portal.log` were `op=rm`. Silence is the correct record for a call that changed nothing.
>
> Whether anything was removed is reported by the locked removal itself, not by a read taken before it: the store's removal answers from the file it mutated, and that answer alone drives task 4-2's exit status. A check taken before the mutation would decide the exit status from a snapshot the mutation never saw — a concurrent sweep or another pane's `hook set` between the two would make the report wrong in either direction.
>
> This is also what lets the `--pane-key` path carry the same exit rule while reading nothing of its own.
>
> Ordering: this task lands before 4-2. The CLI contract cannot be tested until the store reports whether it removed anything, and the split keeps the store change (`internal/hooks`) and the CLI contract (`cmd`) on separate cycles.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §4.2 (whether anything was removed is reported by the locked removal itself, not by a read taken before it), §6.3 (the exclusive hold spans the whole read-modify-write; a lock is never nested), §6.5 (the three new `op` values are the whole of this work unit's amendment to the closed vocabulary), §9.2 (removing nothing is non-zero every way — the store half of it).

## resume-hooks-silently-lost-4-2

### Task 4-2: Removing nothing is never a success

**Problem**: `portal hook rm --on-resume` exits 0 whether or not it removed anything. `hooksRmCmd`'s `RunE` (`cmd/hooks.go:126`) resolves a key, hands it to the store and returns whatever the store returns — and until task 4-1 the store returned success for a removal that deleted nothing. That is the same silent-success shape as the `:.` bug on the write side: an exit code that says nothing about whether anything happened. Registration now refuses a `$TMUX_PANE` that names no live pane; removal shares `resolveCurrentPaneKey` and inherits that probe, but a live pane carrying no token still resolves to an empty key and sails into a mutation, and a live stamped pane whose token has no entry — the routine outcome of deregistering twice — still reports success. Both halves of the CLI surface have to be behind one rule, or the guarantee is only half a guarantee.

**Solution**: Make the rule one line — `hook rm` exits 0 **iff** it removed an entry — driven by the boolean task 4-1 added to `Store.Remove`, with the three no-removal cases each exiting non-zero under their own fixed words.

**Outcome**: A gone pane exits non-zero with tmux's own words, a live pane carrying no token exits non-zero with `no resume hook registered for this pane`, and any key naming no entry — resolved token or literal `--pane-key` — exits non-zero with `no resume hook registered for <key>`; every one of them leaves `hooks.json` byte-identical, with an absent file staying absent, and `--pane-key` still issues no tmux call of any kind.

**Do**:
- Keep `hooksRmCmd`'s two paths as they are: a non-empty `--pane-key` is used verbatim with no validation and no tmux call; otherwise the key comes from `resolveCurrentPaneKey()`, whose existence probe fails a gone pane and returns tmux's own words, ending the command there.
- Before the store is consulted at all, return `fmt.Errorf("no resume hook registered for this pane")` when the resolved key is empty — a live pane carrying no token. It is Portal's own message precisely because the probe has already separated this case from a gone pane, and deciding it here means no empty key ever reaches a mutation.
- Drive the exit status from the removal itself: `removed, err := store.Remove(hookKey, "on-resume", "cli")`; return `err` unchanged when non-nil; return `fmt.Errorf("no resume hook registered for %s", hookKey)` when `removed` is false; return nil only when it is true. Take no `store.Get` or `store.Load` before the removal, on either path.
- Add nothing else: no retry, no soft-success arm, no dirty-flag touch, and no new exit-code plumbing — the error returns through `RunE` by the route every other `hook` failure already takes (`rootCmd` carries `SilenceUsage`/`SilenceErrors`, and `main`'s `classify` prints the message to stderr and exits 1).
- Re-point `cmd/hooks_test.go`: the `"silent no-op when no hook exists for pane"` subtest at `:491` asserts exit 0 for exactly the case that must now fail — invert it and rename it; check the surrounding `--pane-key` subtests seed keys that the removal actually finds; then add coverage for the three routes and for both success paths. Add the exit-0-iff-removed rule to CLAUDE.md's Resume-hooks paragraph, beside the existing `--pane-key` sentence (which stays true), touching none of the key-scheme passages Phases 2 and 3 rewrote.

**Acceptance Criteria**:
- [ ] `hook rm --on-resume` exits 0 iff `Store.Remove` reported a removal, with the answer coming from that removal and never from a read taken before it
- [ ] A `$TMUX_PANE` that names no live pane exits non-zero and the error carries tmux's own words unaltered; no production code inspects that text
- [ ] A live pane carrying no token exits non-zero with `no resume hook registered for this pane`, decided before the store is consulted
- [ ] A live stamped pane whose token has no entry exits non-zero with `no resume hook registered for <token>`
- [ ] `hook rm --pane-key <key>` naming no entry exits non-zero with `no resume hook registered for <key>` carrying the literal key, and issues no tmux call
- [ ] `hook rm --pane-key <seeded key>` still removes it and exits 0, with no tmux call and no validation of the key
- [ ] Every non-zero route leaves `hooks.json` byte-identical, and an absent `hooks.json` stays absent
- [ ] No empty key reaches `Store.Remove` from either path
- [ ] `hook rm` writes no `save.requested` on any path
- [ ] The non-zero exit comes from `RunE` returning an error — no new exit-code plumbing and no usage dump
- [ ] CLAUDE.md's Resume-hooks paragraph states that `hook rm` exits 0 only when an entry was removed
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass

**Tests**:
- `"it exits non-zero for a pane no live pane answers to"` — `KeyResolver` returns a `*tmux.CommandError` carrying tmux's stderr; assert the error, that stderr text surviving in it, and `hooks.json` byte-identical
- `"it exits non-zero with its own words for a live pane carrying no token"` — resolver returns `("", nil)`; assert the exact message `no resume hook registered for this pane`
- `"it consults the store for nothing when the pane carries no token"` — same fixture; assert the file is byte-identical and an absent file stays absent
- `"it exits non-zero when the resolved token has no entry"` — resolver returns a token, `hooks.json` holds a different one; assert `no resume hook registered for <token>`
- `"it exits non-zero when --pane-key names no entry"` — with a resolver that fails loudly if consulted; assert the literal key in the message and zero tmux calls
- `"it exits 0 and removes on the resolved-token path"` — the ordinary success
- `"it exits 0 and removes on the --pane-key path"` — the retained pass-through success
- `"it leaves hooks.json byte-identical on every failing route"` — table over the four failures
- `"it touches no dirty flag on either path"` — assert no `save.requested` after both a success and a failure
- `"it reports removing nothing as a plain error, not a usage error"` — assert the message reaches stderr and no usage text is printed

**Edge Cases**:
- Three routes, each non-zero and each leaving `hooks.json` byte-identical with an absent file staying absent — a gone pane on the existence probe carrying tmux's own words, a live pane carrying no token, and a live stamped pane whose token has no entry
- The empty-key case is decided before the store is consulted, so no empty key ever reaches a mutation, and it carries Portal's own message precisely because the probe has already separated it from a gone pane
- `--pane-key` keeps its literal pass-through — no validation, no tmux call at all — while carrying the same exit rule with the literal key in the message; the empty-key guard must not be hoisted anywhere that reintroduces a resolve on that path
- An empty `--pane-key` value is indistinguishable from the flag being unset today and falls back to `$TMUX_PANE`; that is unchanged, so no empty key reaches the store from that path either
- The exit status comes from the store's removal and never from a `Get`/`Load` taken before it, which would decide the exit from a snapshot the mutation never saw and would not be re-entrant under the lock a later phase adds
- Removing nothing is now **routine** rather than exceptional — 61 of the 63 degenerate log lines were `op=rm` from a Claude Code SessionEnd firing *because* the pane had closed — so no retry or soft-success path may be added to soften it
- The idempotent reading — treating "no entry for this pane" as the requested end state and exiting 0 — is explicitly rejected: it reinstates the property this work unit exists to remove
- `--pane-key` is also the route by which an old-format entry is removed by hand after this change, since no live pane will ever resolve to one; that use must keep working and keep exiting 0 when it removes something
- Scope: the key-scheme passages of CLAUDE.md were rewritten in Phases 2 and 3 and are not re-touched here

**Context**:
> The rule is one line: `hook rm` exits 0 **iff** it removed an entry. It puts both halves of the CLI surface behind one guard rather than only the write side.
>
> Removal does not mint and does not unstamp. A pane whose entry is removed keeps its token — for the same reason registration does not roll back a stamp, and for one more: clearing it would add a tmux write that can fail after the entry is already gone.
>
> **This failure fires routinely, and that is expected.** Deregistration against an already-closed pane is the ordinary case, not an error case, and after this change the same sequence exits non-zero every time. That is the point — a caller can no longer read `rc == 0` as proof anything happened — but it means a caller treating non-zero as an error will begin seeing one as a matter of course. The user's own `~/.claude/hooks/portal-resume-hook.sh` is that caller; updating it is out of scope for this work unit and no task here covers it.
>
> The words are fixed by the specification and are not paraphrased: `no resume hook registered for this pane` for a live pane carrying no token, `no resume hook registered for <key>` for a key naming no entry, and tmux's own words for a gone pane.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §4.2 (removal verifies the same way; removing nothing is always non-zero; the fixed words; the answer comes from the locked removal), §4.3 (`--pane-key` stays a literal pass-through and carries the same exit rule), §4.1 (the probe and why tmux's text is never parsed), §9.2 (`hook rm` on an unresolvable `$TMUX_PANE`; removing nothing is non-zero every way).

## resume-hooks-silently-lost-4-3

### Task 4-3: `hook list` shows where each token lives

**Problem**: A hook key is now an opaque six-character token, and `hook list` (`cmd/hooks.go:66`) prints three tab-separated fields — key, event, command — so the file has become a list of tokens with no way to answer "which pane is this?" short of a manual `list-panes` diff. That is the same hand audit this defect already forced once. The readability the old `<session>:<window>.<pane>` key gave away for free has to be paid back somewhere, and the enumeration that can pay it already exists: `ListAllPaneHookKeys` returns one row per live pane carrying both the pane's token and its location, and nothing consumes the location field yet.

**Solution**: Append a fourth tab-separated column to `hook list` carrying the token's resolved `<session>:<window>.<pane>` location, built from a single `list-panes -a` enumeration taken through a narrow new seam on `HooksDeps`, mapping non-empty tokens only.

**Outcome**: Every `hook list` line carries where its token lives, resolved from one tmux read for the whole listing; a token no live pane carries — including every old-format key, and every key when no tmux server is running — renders an empty fourth field rather than failing the command; and the existing three field positions are undisturbed for any caller parsing the output.

**Do**:
- Add a one-method enumerator seam to `cmd/hooks.go` beside `HookKeyResolver` — an interface carrying `ListAllPaneHookKeys() ([]tmux.PaneHookRow, error)`, a `var _ ... = (*tmux.Client)(nil)` assertion, and a field on `HooksDeps` defaulted through `buildHooksTmuxClient()` exactly as `KeyResolver` is. Do **not** reuse the sweep's `AllPaneLister`: it also carries the restore-marker read, which `hook list` has no use for.
- In `hooksListCmd`'s `RunE`, after `store.List()`: return immediately with no output when the list is empty, taking **no** tmux read at all. Otherwise take exactly one enumeration for the whole listing.
- Build a `map[token]location` from the rows, **skipping every row whose `Token` is empty**, and keeping the first row for a token that appears more than once so a duplicate resolves to one location deterministically rather than rendering twice.
- On an enumeration error carry on with an empty map: every fourth field renders empty and the command still exits 0. Emit no log line for it — see Context for why.
- Print `%s\t%s\t%s\t%s\n` per hook, with the map lookup's zero value for an unresolved key, so the column is always emitted and every line carries the same three separators. Then re-point `cmd/hooks_test.go`'s `TestHooksListCommand` exact-line subtests to four fields and add the new coverage against the injected seam (`cmd`'s `TestMain` poisons `TMUX` package-wide, so no test drives a real server).

**Acceptance Criteria**:
- [ ] `hook list` emits a fourth tab-separated field carrying the resolved `<session>:<window>.<pane>` location for a token a live pane carries
- [ ] The `key` / `event` / `command` field positions are byte-identical to today for every line
- [ ] The column is always emitted: an unresolved token gives an empty fourth field, never a dropped one, so every line carries exactly three separators
- [ ] The token → location map is built from non-empty tokens only: an entry keyed by `""` never picks up an unstamped pane's location
- [ ] A token no live pane carries renders empty; an old-format key renders empty
- [ ] A failed enumeration — including no tmux server running — renders every fourth field empty and still exits 0; `hook` starts no tmux server
- [ ] Exactly one enumeration read is taken per invocation regardless of entry count, and **zero** reads when there are no entries
- [ ] A token carried by two rows renders one line whose location is the first row's
- [ ] A location whose session name carries `|` renders verbatim
- [ ] The existing sort by key then event is unchanged
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass

**Tests**:
- `"it appends the resolved location as a fourth column"` — one entry, one matching row; assert the exact four-field line
- `"it renders an empty fourth field for a token no row carries"` — rows present, none matching; assert the trailing separator and empty value
- `"it renders an empty fourth field for an old-format key"` — a `sess:0.0` key alongside a token-keyed entry; only the token resolves
- `"it does not let an unstamped pane's row lend its location"` — seed a `""` key entry and include an unstamped row; assert that line's fourth field is empty
- `"it renders every fourth field empty when the enumeration fails"` — seam returns an error; assert exit 0 and the full expected output
- `"it takes exactly one enumeration read for many entries"` — recording seam; assert a call count of 1 with several entries
- `"it takes no enumeration read when there are no entries"` — seam fails loudly if called; the existing empty-output subtests still pass
- `"it renders one line for a token carried by two rows"` — assert a single line and the first row's location
- `"it renders a location whose session name carries a pipe verbatim"` — a `Location` of `a|b:0.0`
- `"it keeps the sort by key then event"` — the re-pointed ordering subtest, now four fields
- `"it accepts no arguments"` — unchanged

**Edge Cases**:
- The column is **appended**, so existing field positions are undisturbed for any caller parsing the output, and it is **always** emitted — an empty value is an empty fourth field, never a dropped one
- The map is built from **non-empty** tokens only, or an unstamped pane's row lends its location to an entry that names no pane
- A token no live pane carries renders empty, as does any old-format key, since no live pane can answer to one
- With no tmux server running the enumeration simply fails; `hook` is bootstrap-exempt and must start none, so a failed read can never fail `list` or drop the column
- One `list-panes -a` read serves every row — never one resolve per entry — and with zero entries there is nothing to resolve, so no tmux read is taken at all and the existing empty-output subtests still pass without a seam
- The seam is a narrow one-method enumerator on `HooksDeps` defaulted through `buildHooksTmuxClient()`, **not** the sweep's `AllPaneLister`, which also carries the restore-marker read
- A duplicate token across two rows is a hand-stamped anomaly — a split inherits nothing — and must resolve deterministically to one location rather than rendering the entry twice
- A location whose session name carries `|` already survives the enumeration's first-separator row parse and is rendered verbatim
- The location is display-only and is never a key, so rendering the same shape as the positional structural siblings couples nothing to them
- A command containing a tab already breaks field parsing today; appending a column neither worsens nor fixes that, and no escaping is introduced

**Context**:
> This is what pays back the readability the token-only key costs. Without it the file is a list of opaque six-character tokens with no way to answer "which pane is this?" short of a manual `list-panes` diff.
>
> Resolution is one `list-panes -a` read over the enumeration whose rows already carry the token alongside its location; the mapping is built once from that read and reused across all rows.
>
> **Decision made in this task:** a failed enumeration emits no log line. The specification fixes this work unit's amendment to the closed `hooks` `op` vocabulary at three values — `load-unlocked`, `touch-save-requested` and `clean-stale-skipped` — none of which names a failed pane enumeration, and inventing an `op` at the call site is forbidden. The specification is silent on logging here, and the user-visible contract it does fix is that the command still succeeds with empty columns, so silence is the reading that adds nothing to a closed vocabulary.
>
> This column renders locations for entries that still exist. It is not what makes a reaped hook recoverable — the reaper's own INFO line carrying the removed command is, and that landed in Phase 1.
>
> Scope: no CLAUDE.md edit belongs to this task. The architecture description's hook passages were settled in Phases 2 and 3, and the location column is display-only CLI output.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §4.4 (`hook list` renders the resolved location; the appended column, the non-empty mapping, the empty-not-dropped rule), §3.3 (the enumeration returns one row per live pane carrying token and location; the location field is display only), §5.3 (why the reaper's line, not this column, answers "what did I lose?"), §9.2 (`hook list` fourth column).
