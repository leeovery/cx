# Analysis Tasks: theming-system (Cycle 7)

## Task 1: Schedule the forced-close geometry flash's auto-clear tick
severity: medium
sources: standards

**Problem**: §14A pins six theme signals and states that all six route through the **transient flash** slot. In Portal a transient flash is `flashAutoClearDuration` plus a scheduled `flashTickCmd` (`internal/tui/sessions_flash.go:27-39`); every other theme flash obeys that — `blockThemePanel` returns `flashTickCmd(m.flashGen)` for the three entry refusals, and `reportOutstandingCommitFailure` (`internal/tui/theme_panel.go:248-255`) returns one for `theme not saved — see portal.log`. The resize-below-floor arm does not. In `resizeThemePanel` (`internal/tui/theme_panel.go:260-277`), when no commit failure is outstanding, `closeThemePanel()` returns nil, `m.setThemeFlash(themePanelForcedCloseFlash(dim))` raises the band, and the function returns that nil — so `Update`'s `if closed := (&m).resizeThemePanel(); closed != nil` (`internal/tui/model.go:1478-1480`) batches nothing. `terminal too narrow — theme picker closed` / `terminal too short — theme picker closed` therefore persist indefinitely, clearing only on the next actionable keypress or when another flash supersedes them. This is the one arm of the theme flash surface that is not transient, and it is the arm an ordinary window resize reaches. The geometry tests (`internal/tui/theme_panel_geometry_test.go:230-243`, `internal/tui/theme_panel_entry_test.go:364`) assert the flash text only, so nothing covers the missing tick.

**Solution**: Return a flash tick from the refusal arm's non-reporting branch, captured after `setThemeFlash` bumps the generation, batched with whatever `closeThemePanel` returned.

**Outcome**: Both forced-close strings clear themselves after `flashAutoClearDuration` exactly as every other theme signal does, and the tests pin the tick rather than only the wording.

**Do**:
1. In `internal/tui/theme_panel.go`'s `resizeThemePanel`, inside the `!ok` (below-floor) arm: after `m.setThemeFlash(themePanelForcedCloseFlash(dim))` in the `!willReport` branch, capture the post-bump generation and batch the tick onto the returned command — `cmd = tea.Batch(cmd, flashTickCmd(m.flashGen))`. `setFlash` increments `m.flashGen` before the tick is scheduled, so the captured generation is the live one and a superseded tick still cannot early-clear a later flash.
2. Keep the `willReport` branch untouched: `closeThemePanel()` → `reportOutstandingCommitFailure()` already returns exactly one tick for `theme not saved — see portal.log`, and this arm must not schedule a second.
3. Batch rather than replace, so the function keeps returning a single command if `closeThemePanel` is ever changed to return one alongside the geometry flash. `tea.Batch` drops nils, so the current nil-`cmd` case is unaffected.
4. Leave the "A due commit-failure report wins over the geometry flash" comment's rule intact and extend it to say the geometry flash now carries its own tick.

**Acceptance Criteria**:
- `resizeThemePanel` returns a non-nil command on the below-floor path in both branches: the commit-failure report (unchanged) and the geometry flash (new).
- The forced-close flash clears when its tick fires with a matching generation, and does not clear when a superseded generation's tick fires.
- Exactly one tick is scheduled per forced close — the commit-failure branch does not gain a second.
- No change to the flash strings, to the close ordering (read `commitFailed` before the close), or to the `Update` deferral shape at `internal/tui/model.go:1478-1480`.

**Tests**:
- Extend the forced-close assertions in `internal/tui/theme_panel_geometry_test.go` (~230-243) and `internal/tui/theme_panel_entry_test.go:364` to assert the returned command yields a `flashTickMsg`, and that feeding that message back to the model clears the band.
- A test that a resize below the floor while a commit failure is outstanding returns exactly one tick and raises `theme not saved — see portal.log`, not the geometry string.
- A test that a flash raised after the forced close (bumping the generation) is not cleared by the forced close's in-flight tick.

## Task 2: Extract the one commit protocol behind commitConstant and commitSlot
severity: medium
sources: duplication

**Problem**: `internal/tui/theme_panel_commit.go` holds two parallel implementations of one commit protocol. `commitConstant` (line 30) and `commitSlot` (line 86) each run the identical five steps — nil-persister short-circuit, write, `applyCommitResult(err)`, bail on error, mutate `m.themeState.keys`, `recomputeThemePanel()` — differing only in which persister method is called (`CommitTheme` vs `CommitThemeSlot`) and which `RawKeys` mutator is applied (`WithConstant` vs `WithMember`). The source acknowledges the copy ("Carries commitConstant's rules"), which is exactly the shape that drifts: a sixth step added to one path is silently absent from the other, and the ordering constraints the comments defend (keys mirror only after a landed write; recompute last; on error nothing moves so the `●` cannot move) are stated twice. `commitSelectedConstant` (line 11) and `commitSelectedSlot` (line 76) repeat the same `committableThemeSlug` guard-and-delegate wrapper a second time.

**Solution**: One private protocol function parameterised by the write and the key mirror, and one `commitSelected` wrapper parameterised by the committer.

**Outcome**: The commit protocol and its ordering rules are stated once; the two commit shapes are two closures; a change to the protocol cannot reach one path and miss the other.

**Do**:
1. Add `func (m *Model) commit(write func() error, mirror func(theme.RawKeys) theme.RawKeys) error` to `internal/tui/theme_panel_commit.go` holding the whole protocol in the current order: return nil if `m.themeState.persister == nil`; `err := write()`; `m.applyCommitResult(err)`; `if err != nil { return err }`; `m.themeState.keys = mirror(m.themeState.keys)`; `m.recomputeThemePanel()`; `return nil`.
2. Move the existing doc comments onto it — a nil persister is inert not failed; the mirror applies prefs' rule to the keys in hand rather than re-reading, since the persister's read-modify-write holds other instances' writes; on error nothing moves so the `●` cannot move. The closure is only invoked after the nil check, so no closure dereferences a nil persister.
3. Reduce `commitConstant(slug)` to `m.commit(func() error { return m.themeState.persister.CommitTheme(slug) }, func(k theme.RawKeys) theme.RawKeys { return k.WithConstant(slug) })`.
4. Reduce `commitSlot(slug, member)` to the same call with `CommitThemeSlot(slug, member)` and `k.WithMember(member, slug)`. Delete the now-false "Carries commitConstant's rules" lead-in; keep the substantive note that the untouched other slot is what makes `● both` reachable via `d` then `l` on one row.
5. Collapse the two selected-row wrappers into `func (m *Model) commitSelected(commit func(slug string) error) error` running the `committableThemeSlug` guard and delegating. Update call sites: the constant key path passes `m.commitConstant`; `handleSlotCommitKey` passes `func(slug string) error { return m.commitSlot(slug, member) }`. Keep the "the target is the selected row, never `m.themeState.keys`" comment on the wrapper.
6. Leave `confirmSlotAssignment`'s separate persister-nil check (`internal/tui/theme_panel_confirm.go:60-62`) in place — its comment is still true: the nil-persister case cannot be inferred from the nil error.

**Acceptance Criteria**:
- The five-step protocol appears once in the package; `commitConstant` and `commitSlot` are each a single delegating call.
- One `committableThemeSlug` guard-and-delegate wrapper, not two.
- Behaviour is byte-identical: same write order, same failure handling, same recompute timing, same return values on the nil-persister and failed-write paths.
- No new exported symbols.

**Tests**:
- Existing commit, commit-failure and slot-confirm tests pass unchanged.
- Add a table-driven test running both commit shapes through one set of protocol assertions: a failed write leaves `m.themeState.keys` untouched, does not recompute, and raises the commit-failed state; a landed write mirrors the keys and then recomputes, in that order.
- A test that a nil persister returns nil and mutates nothing on both shapes.

## Task 3: Single-source the slot → shipped-default pairing
severity: medium
sources: duplication

**Problem**: Which shipped default belongs to which slot — light→`DefaultLightSlug`, dark→`DefaultDarkSlug` — is written out independently three times: in `ResolveSetting`'s `cmp.Or` pair (`internal/theme/setting.go:76-79`), in `Setting.Slug`'s per-slot switch (`internal/theme/setting.go:86-95`, documented as "the per-slot half of ResolveSetting's substitution"), and in `fallbackSlugFor` (`internal/theme/resolution.go:210-215`). All three encode the same mapping and jointly protect the invariant the code most cares about: a light slot must never resolve or fall back to a dark palette. Nothing structurally couples them, so one can be edited alone.

**Solution**: Keep exactly one mapping function, use-agnostically named, and express the other two sites through it.

**Outcome**: The slot→default pairing exists in one place; the light-never-gets-dark invariant cannot be half-changed.

**Do**:
1. Rename `fallbackSlugFor` to `defaultSlugFor(slot Slot) string` in `internal/theme/resolution.go` — use-agnostic, since it now serves substitution as well as fallback. Reword its comment to state the rule once: the slot's shipped default, mode-matched deliberately, because one fixed default would throw a light-terminal user with a typo in their light slot onto a dark palette.
2. `ResolveSetting` builds `Light: cmp.Or(raw.Light, defaultSlugFor(SlotLight))` and `Dark: cmp.Or(raw.Dark, defaultSlugFor(SlotDark))`.
3. `Setting.Slug` returns `cmp.Or(s.Light, defaultSlugFor(SlotLight))` and `cmp.Or(s.Dark, defaultSlugFor(SlotDark))` in its two slot arms. The `default` arm is unchanged — an unset constant answers the empty string; there is no constant default.
4. `resolveSlot`'s fallback call site reads `defaultSlugFor(slot)`.
5. Leave `DefaultLightSlug` / `DefaultDarkSlug` themselves exactly as they are — only the *pairing* moves.
6. If the package's exported-surface guard list (`internal/theme/theme_test.go`) names the renamed symbol, update it; `fallbackSlugFor`/`defaultSlugFor` is unexported, so no public surface changes.

**Acceptance Criteria**:
- `DefaultLightSlug` and `DefaultDarkSlug` are paired with a `Slot` in exactly one function in `internal/theme`.
- `ResolveSetting`, `Setting.Slug` and the fallback path all produce today's answers for every combination of set/unset slots and the constant.
- No behaviour change and no exported-surface change.

**Tests**:
- One table over `SlotLight`/`SlotDark` asserting all three paths (the `ResolveSetting` substitution, `Setting.Slug` on an unset slot, and the `resolveSlot` fallback for an unloadable nomination) return that slot's own shipped default and never the other slot's.
- Keep the existing constant-slot assertion that `Setting.Slug(SlotConstant)` on an unset constant is the empty string.

## Task 4: Make the theme slot seam state what it does and single-source the slot collapse
severity: medium
sources: architecture, duplication

**Problem**: Two related defects on one seam. (a) `ThemeSource.ResolveSlot` (`internal/tui/theme_seams.go:14`) advertises a resolution no production caller consumes: `commitSlot` → `recomputeThemePanel` → `applyCommittedSetting` already resolves *both* slots through `Resolve`/`ResolveNominationFrom` and installs the resulting `Nomination`, then `confirmSlotAssignment` calls `loadNewlyLiveSlot` (`internal/tui/theme_panel_confirm.go:70-77`), which resolves that same slot a second time and discards the `SlotResolution` entirely — the only observable difference is that `ResolveSlot` reports through `reportSlot` (emitting `theme: loaded`) while `Resolve` reports through `reportFallback` (which does not). The call site's body is `if _, err := …; err != nil { return }` — a guard with no body at the end of a void function — and a reader cannot tell from the seam's shape that the method exists for its emission cadence. (b) Every implementation re-derives the nominated slug the same way — `setting, _ := theme.ResolveSetting(keys)` then `setting.Slug(slot)` — in the production adapter (`internal/theme/dir_theme_source.go:34-37`), the capture fake (`internal/capture/theme_fake.go:51`) and two tui test fakes (`internal/tui/theme_seams_test.go:31`, `internal/tui/theme_source_fake_test.go:58`), each carrying a comment promising it runs "the same collapse the production adapter runs" — the contract that silently rots if the adapter's collapse changes, because the fakes keep compiling and keep answering the old way.

**Solution**: Expose the collapse once in `internal/theme`, and narrow the seam method to the error the call site actually consumes, named for the act it performs.

**Outcome**: The interface states "resolve this slot and record the load"; there is one collapse from raw keys to a slot's nominated slug; the vacuous error guard is gone.

**Do**:
1. Add to `internal/theme/setting.go`: `func SlugForSlot(keys RawKeys, slot Slot) string { setting, _ := ResolveSetting(keys); return setting.Slug(slot) }`, documented as the one collapse from persisted raw keys to the slug a slot nominates (tiebreak plus shipped-default substitution).
2. Call it from `DirThemeSource`, from `internal/capture/theme_fake.go`, and from both tui test fakes. Delete the "the same collapse the production adapter runs" comments — the shared call now is the contract.
3. Rename the seam method on `tui.ThemeSource` to `LoadSlot(e theme.Enumeration, slot theme.Slot, keys theme.RawKeys) error` and rewrite the interface doc at `internal/tui/theme_seams.go:5-9` so the contract matches: this method resolves the named slot and records the load — it is the commit-time `theme: loaded` emission `Resolve` deliberately does not make. Keep the existing statements that every method takes raw keys, that no method reads the filesystem, and that `Resolve`'s error is the broken-builtin fatal.
4. Rename `DirThemeSource.ResolveSlot` to `LoadSlot` with the narrowed signature, implemented as the error from `e.Loader.ResolveSlot(enumeration, slot, SlugForSlot(keys, slot))`. Keep `Loader.ResolveSlot` itself unchanged — it returns the record and is the rule body.
5. Implement `LoadSlot` on the capture fake and both tui fakes, preserving what each currently asserts (the capture fake still reports the injected `--theme` palette's answer, so a fixture's frame cannot repaint off `--theme`).
6. Replace the vacuous guard in `loadNewlyLiveSlot` with `_ = m.themeState.source.LoadSlot(m.themePanel.enumeration, newlyLive.Slot(), m.themeState.keys)` and a comment saying why the error is not actionable here: the classification is assigned first and unconditionally, and a failed palette must not decide which half is in force. Keep the `adoptRetainedReply()` ordering.
7. Update the tests that drive the old four-value method: `internal/capture/theme_panel_fixture_test.go:332-340` and `internal/tui/theme_seams_test.go:107` retarget to `LoadSlot` (asserting the call is made and reports no error) plus a direct assertion on `theme.Loader.ResolveSlot`/`theme.SlugForSlot` where the returned record is the subject; `internal/tui/theme_panel_commit_load_test.go:559-572` — which asserts the slot path and the badge path share one rule body — retargets to `theme.Loader.ResolveSlot(enumeration, slot, theme.SlugForSlot(keys, slot))`, which is that shared body.
8. Update `internal/theme/theme_test.go`'s exported-surface list: `DirThemeSource.ResolveSlot` → `DirThemeSource.LoadSlot`, and add `SlugForSlot`.
9. Do **not** take the alternative of folding the newly-live slot's `SlotResolution` into the nomination: `applyCommittedSetting` is the single owner of the nomination, and a second writer would reintroduce the two-sources-of-truth problem the panel avoids. Record that rejection in the comment on `loadNewlyLiveSlot`.

**Acceptance Criteria**:
- `ResolveSetting(keys)` followed by `Setting.Slug(slot)` appears in exactly one place outside `internal/theme`'s own tests: `SlugForSlot`.
- `tui.ThemeSource` has four methods, the fourth returning `error` only, named for the act.
- No call site in the tree discards a `SlotResolution`.
- The `theme: loaded` emission still fires exactly once per confirmed slot commit, on the same path and with the same slug and slot attrs.

**Tests**:
- A test that `LoadSlot` emits `theme: loaded` with the slot's collapsed slug for a slot the user never set (so the shipped default is the recorded slug), and that `Resolve` on the same inputs emits no `loaded` line.
- A test that a confirmed slot commit produces exactly one `theme: loaded` line.
- A test that all `ThemeSource` implementations (production adapter and every fake) answer the same slug for one set of raw keys and one slot.

## Task 5: Give internal/themetest ownership of the deny-read and canvas-valued fixtures
severity: medium
sources: duplication

**Problem**: The `unreadable` rung of the rejection ladder is exercised from four packages, and every consumer wrote its own staging fixture — `chmod 0o000` plus a `t.Cleanup` restore, optionally preceded by a root guard. `internal/theme/load_test.go:554` (`writeUnreadableTheme`) and `internal/theme/enumerate_test.go:379` (`unreadableDir`) are near-identical down to a verbatim-identical doc comment; `cmd/theme_test.go:568`/`:581` (`unreadableThemeFile`/`unreadableThemesDir`) are a second near-identical pair; `cmd/open_theme_nomination_test.go:72` (`poisonThemesDir`) and `cmd/doctor_persisted_theme_test.go`'s inline `deniedDir` closure are a third and fourth; and at least eight further sites inline the same five lines (`cmd/theme_test.go:411`, `cmd/doctor_theme_test.go:70,125,197,273,376`, `cmd/doctor_persisted_theme_test.go:306,476,686`, `internal/prefs/read_shared_test.go:121`). The copies have already drifted on the part that matters: `internal/theme` and `cmd/doctor_theme_test.go:skipUnlessModeBitsDeny` skip under root, `cmd/theme_test.go:requireDeniedRead` instead fatals if the read succeeds, and several inline sites apply neither guard — so they degrade to a vacuous assertion when run as root rather than skipping or failing loudly. Separately, "a theme file whose canvas is X" — the most-used derived fixture — is composed ad hoc at four packages' call sites (`cmd/open_theme_construction_test.go:497`, `cmd/capturetool/main_test.go:434` and `:493`, `cmd/capturetool/swatch_test.go:31`, `internal/tui/theme_panel_open_test.go:103`, the last via its own all-tokens-one-colour loop `writeThemeFileForTest`), even though `internal/themetest` is the declared owner of the fixture format.

**Solution**: Put both fixtures in `internal/themetest` — one deny-read pair with a single root policy, and one canvas-valued writer — and route every call site through them.

**Outcome**: One root policy for the whole tree, no test silently vacuous under root, and the derived theme-file fixtures live with the package that owns the format.

**Do**:
1. Add `internal/themetest/deny.go` with `func DenyRead(t *testing.T, path string) error` and `func DenyDir(t *testing.T, dir string) error`. Each: record the prior mode, `chmod 0o000`, register a `t.Cleanup` restoring the prior mode, then verify the read is actually denied and return the verified OS error.
2. Apply **one** root policy in both helpers: `t.Skip` when the mode bits do not deny (the process is root or the filesystem ignores the bits), matching today's `skipUnlessModeBitsDeny`. Document it on the helpers so no call site re-decides.
3. Fold `requireDeniedRead`'s absent-versus-denied distinction into the returned error, so a caller that needs to assert "denied, not missing" does so on the returned value rather than by re-reading the path.
4. Replace `writeUnreadableTheme`, `unreadableDir`, `unreadableThemeFile`, `unreadableThemesDir`, `requireDeniedRead`, `poisonThemesDir`, the `deniedDir` closure and every inline `os.Chmod(..., 0o000)` block at the sites listed above with calls to the two helpers.
5. Add `func WriteWithCanvas(t *testing.T, dir, base, canvas string) string` to `internal/themetest/theme_file.go` — `Write(t, dir, base, WithValue(Lines(), "canvas", canvas))` — and route the cmd, capturetool and tui call sites through it.
6. For `internal/tui`'s `writeThemeFileForTest` all-tokens-one-colour loop: if a test genuinely needs every token at one value, add `func MonochromeLines(value string) []string` to themetest built from `theme.TokenNames()` and use it; otherwise replace the call sites with `WriteWithCanvas`. Do not leave a second local composer behind.
7. Keep `internal/theme`'s tests in their external test package where they consume `themetest`, so the existing import direction (themetest → theme) is preserved.

**Acceptance Criteria**:
- `rg 'Chmod\(.*0o000' --glob '*_test.go'` returns only `internal/themetest`.
- No test-local helper named for unreadable/denied/poisoned paths survives in `internal/theme`, `cmd`, `cmd/capturetool`, `internal/tui` or `internal/prefs`.
- Exactly one root policy exists in the tree, and every former inline site now skips under root rather than asserting vacuously.
- `themetest.WithValue(themetest.Lines(), "canvas", …)` appears at no call site outside `internal/themetest`.
- The unit lane and the integration lane both stay green, and the `unreadable` rung's coverage is unchanged in substance at every former site.

**Tests**:
- Tests in `internal/themetest` for both helpers: the path is genuinely unreadable while staged, the returned error is the denial (not a not-exist error), and the prior mode is restored after cleanup.
- A test that `WriteWithCanvas` produces a file the loader accepts whose `canvas` token is the requested value and whose other tokens are unchanged from `Lines()`.
- Every migrated call site's existing assertions retained unchanged.

## Task 6: Derive the capture panel fixtures' unions from their declared entries
severity: medium
sources: architecture

**Problem**: Each theme-panel fixture declares its membership twice — once as `theme.Enumeration.Entries` via `themePanelDirEntry`, and again as a fully-formed `theme.Union` with rows written out in display order and `Count`/`Rejected` set by hand (`internal/capture/fixtures.go:428-436`, `:552-575`, `:607-616`, `:647-655`). The two declarations must agree and nothing ties them together: `TestPanelFixture_FourInputs` (`internal/capture/theme_panel_fixture_test.go:35-75`) only checks that the *fake* hands back the union that was declared, which is vacuous with respect to whether that union is a shape `theme.Assembler` could produce from those entries. So the fixture set holds a hand-maintained copy of three production rules — membership (built-ins ∪ files ∪ unresolved persisted slugs), one-slug-one-row dedup, and `rowBefore`'s fold-then-bytes-then-built-in-first ordering — in the one place the project designates as the sole pre-release visual-verification route. A change to `rowBefore` or to the dedup leaves the fixtures rendering the old shape and the swap-and-diff guard passes, because it diffs colours, not row sets. The in-source rationale at `internal/capture/fixtures.go:550` ("re-deriving the order would put a second copy of the union's sort comparison in the harness") reads backwards: calling `Assembler` *is* the single copy; writing the order out by hand is the second one.

**Solution**: Let each panel fixture declare its inputs — entries and keys — and derive the union through the production assembler, keeping the existing repaint step.

**Outcome**: Membership, dedup, ordering, `Count` and `Rejected` come from production assembly; each fixture states only what it is testing; a change to the union's rules reaches every fixture by construction.

**Do**:
1. Add a derivation helper in `internal/capture/fixtures.go`: `func themePanelUnionFrom(e theme.Enumeration, keys theme.RawKeys) theme.Union { return theme.Assembler{Loader: theme.NewSilentLoader()}.Reassemble(e, keys) }`. Use `Reassemble`, never `Open` — the harness must perform no directory read and emit no `theme: enumerated`.
2. Move every hand-written row's verdict onto its `theme.Entry`: extend `themePanelDirEntry` (or add a rejection-carrying variant) so an entry can declare `Rejection`, and keep the existing `Slug: ""` shape for the `bad name` candidate. The built-in rows then come from the embedded set exactly as production's do.
3. Delete `themePanelUnion`, `themePanelInvalidRowUnion`, `themePanelDirUnreadableUnion` and `themePanelPaginatedUnion`, and set `fx.themeUnion = themePanelUnionFrom(fx.themeEnumeration, fx.themeKeys)` in each panel fixture *after* its enumeration and keys are assigned.
4. Derive only inside the panel fixtures. `Fixture.themeSource` (`internal/capture/fixtures.go:107-112`) returns nil when `len(f.themeUnion.Rows) == 0`, which is what keeps `t` a silent no-op on a fixture that declares no panel — derivation always yields the built-in rows, so it must not move into `Deps`/`themeSource` or every fixture would gain a slide-over.
5. Leave `newFakeThemeSource`'s `repaintUnion` step exactly as it is: rows are still repainted onto the `--theme` palette, so `--theme` stays live and rejected rows keep their zero palette.
6. Replace the comment at `internal/capture/fixtures.go:550` with the corrected rationale — the fixture declares inputs and the assembler derives the display order, so the harness holds no copy of the sort comparison.
7. Retarget `TestPanelFixture_FourInputs` (`internal/capture/theme_panel_fixture_test.go:35-75`): the "declared union comes back" assertion is now vacuous by construction, so assert instead the properties each fixture exists for — the badged rows are present and selectable, the invalid-row fixture's rejections carry their reasons, the dir-unreadable fixture's persisted rows carry `unreadable` and `DirUnusable` is set, and the paginated fixture's row count overflows one page.

**Acceptance Criteria**:
- No `theme.Union` composite literal remains in `internal/capture`.
- Every panel fixture's `Count` and `Rejected` are computed, not typed.
- Rendering each `theme-panel-*` fixture through `capturetool` produces the same frame as before the change (row order, badges, chrome), verified by comparing output before and after.
- A fixture that declares no panel still yields a nil theme source, so `t` remains a no-op there.
- The swap-and-diff completeness guard stays green and enumerates the same fixture set.

**Tests**:
- The retargeted `internal/capture/theme_panel_fixture_test.go` assertions above.
- A test that a panel fixture's union row set and ordering equal `theme.Assembler.Reassemble` over its declared entries and keys (trivially true after derivation, and the pin that stops a future hand-authored union creeping back).
- Existing capture guard tests unchanged.

## Task 7: Single-source Result and Enumeration construction in internal/theme
severity: low
sources: duplication

**Problem**: Two small construction rules are each stated twice or three times. (a) `LoadFile` (`internal/theme/load.go:76-81`), `LoadPath` (`:93-98`) and `LoadBuiltin` (`internal/theme/builtins.go:77-82`) each end with the same block — `parseThemeBytes(data)`, return the zero `Result` with the rejection on failure, else build a `Result` carrying the `Theme` and the verbatim `Source` bytes — differing only in whether `Slug` is populated, so the "Source is verbatim, never a re-serialisation" rule (`load.go:51-53`) has to hold at three construction sites. (b) `Assembler.Open` (`internal/theme/union.go:117-119`) and `cmd/doctor_theme.go:58-65` (`enumerateThemesDir`) each call `Enumerate(dir)` and hand-assemble the `Enumeration`, both encoding that `DirUnusable` is `rejection != nil` and that `DirPath` carries the directory just read; doctor adds only an empty-path short-circuit. Two independent statements of "what a directory read means" is what lets the panel and doctor disagree about a directory — the one thing the topic single-sources everywhere else.

**Solution**: One `Result`-from-bytes constructor and one `Enumeration`-from-a-directory-read constructor, both in `internal/theme`.

**Outcome**: The verbatim-Source rule and the directory-read invariant each have exactly one home; the panel and doctor cannot classify a directory differently.

**Do**:
1. Add `func resultFromBytes(slug string, data []byte) (Result, *Rejection)` to `internal/theme/load.go`: parse via `parseThemeBytes`, return the zero `Result` with the rejection on failure, else `Result{Slug: slug, Theme: built, Source: data}`. Move the Source-is-verbatim comment onto it.
2. Have `LoadFile` return `resultFromBytes(slug, data)`, `LoadPath` return `resultFromBytes("", data)` (a path handed in by a caller yields no slug), and `LoadBuiltin` return `resultFromBytes(slug, data)` with its `found` third value unchanged. Keep each entry point's own earlier rungs (filename, reserved, read) exactly where they are — only the tail moves.
3. Add `func (l Loader) OpenEnumeration(dir string) Enumeration` to `internal/theme` (beside `Enumerate`): return the zero `Enumeration` for an empty dir, else `Enumeration{Entries: entries, DirUnusable: rejection != nil, DirPath: dir}`.
4. Before wiring it into `Assembler.Open`, confirm the empty-dir short-circuit is behaviour-preserving there: `statThemeDir("")` treats an empty path as absent (`os.IsNotExist`) and returns `(false, nil)`, so `Enumerate("")` yields `(nil, nil)` with no `DirectoryUnusable` event and today's `Assembler.Open("")` already produces `DirUnusable: false`, `DirPath: ""`. If that is what the code does, use `OpenEnumeration` in `Assembler.Open`; if it is not, keep `Assembler.Open` on the non-short-circuiting path and give doctor the short-circuit, rather than changing either caller's behaviour.
5. Rewrite `cmd/doctor_theme.go:enumerateThemesDir` to delegate to `loader.OpenEnumeration(dir)` (or delete it and call the constructor directly), leaving doctor's silent loader and the "the `theme` log component records use, never diagnosis" property untouched.
6. `Assembler.Open` keeps its own `events.Enumerated(union.Count, union.Rejected)` emission after `Reassemble` — the constructor emits nothing beyond whatever `Enumerate` already emits.
7. Update `internal/theme/theme_test.go`'s exported-surface list for `Loader.OpenEnumeration`.

**Acceptance Criteria**:
- `Result{…Source: data}` is constructed in exactly one place in `internal/theme`.
- `Enumeration{Entries: …, DirUnusable: …, DirPath: …}` is constructed in exactly one place, consumed by both `Assembler.Open` and doctor.
- `Assembler.Open` still emits exactly one `theme: enumerated` per call, including for an absent or unusable directory; doctor still emits nothing.
- No behaviour change for an absent, empty-path, unusable or ordinary themes directory on either caller.

**Tests**:
- A test that `LoadFile`, `LoadPath` and `LoadBuiltin` all return the exact input bytes as `Source` and a zero `Result` (nil `Source`) on rejection.
- A test that `OpenEnumeration` classifies absent, empty-path, unusable and populated directories identically to what the panel and doctor produce today, driven from one table.
- A test that the panel's and doctor's `Enumeration` for one staged directory are equal.

## Task 8: Render the panel's rule row through the header's shared renderer
severity: low
sources: duplication, standards

**Problem**: `themePanelHeaderBlock` (`internal/tui/theme_panel_render.go:31-38`) composes the panel's rule row by hand — `headerStyle(th.Border, …).Render(strings.Repeat(headerRuleGlyph, max(width, 0)))` — which is what `headerSeparatorRule` (`internal/tui/header.go:68-72`) already does, bar the zero-width fallback the panel must not take. The panel's rule exists specifically to sit in the page rule's lane, so a change to the page's glyph or token is meant to move both; as written it would move only one and the shared-lane property the surrounding comment asserts would break silently. Separately, §9.1 pins the panel header as the `Themes` label "followed by a one-row `border` rule", while the implementation renders rule-then-label — deliberately, because the rule shares the page's rule lane and `Themes` then aligns with the `Sessions ··· N` section-header row, which is what §9.1's own "matching the Sessions section-header idiom" clause describes. The two halves of the spec sentence disagree, the code followed the idiom clause, and nothing in-source records the resolution, so a later reviewer could "fix" the order back.

**Solution**: Split the width-resolving wrapper from the glyph-run renderer and have both callers use the renderer; record the header-order resolution where the sequence claim would be re-read.

**Outcome**: One rule renderer serves the page and the panel, so a glyph or token change moves both; the deliberate rule-then-label order is documented at the site that implements it.

**Do**:
1. In `internal/tui/header.go`, extract `func ruleOfWidth(w int, th theme.Theme, colourless bool) string` rendering `strings.Repeat(headerRuleGlyph, max(w, 0))` through `headerStyle(th.Border, th, colourless)`.
2. Reduce `headerSeparatorRule(width, th, colourless)` to `ruleOfWidth(headerWidthOrFallback(width), th, colourless)`, preserving the page's zero-width fallback exactly.
3. Have `themePanelHeaderBlock` call `ruleOfWidth(width, th, colourless)` — the clamp now lives in the renderer, and the panel still must not take the page's fallback width.
4. Extend the comment above `themePanelHeaderBlock` (`internal/tui/theme_panel_render.go:28-30`) to record the §9.1 resolution: the header is rule-then-label deliberately, because the rule shares the page's rule lane and `Themes` then aligns with the `Sessions ··· N` section-header row — the "matching the Sessions section-header idiom" reading — so the order must not be reverted to label-then-rule.

**Acceptance Criteria**:
- `strings.Repeat(headerRuleGlyph, …)` appears in exactly one function in `internal/tui`.
- The page header and the panel header render byte-identical rule rows for the same width, theme and colourless flag.
- The panel's rendered frame is unchanged (rule row above the label, spanning the border column, at both header shapes).
- The rule-then-label rationale is stated in-source at `themePanelHeaderBlock`.

**Tests**:
- A test that the panel's rule row equals `headerSeparatorRule` at the same explicit width under the same theme, so a glyph or token change moves both.
- A test that the panel's rule renders at exactly the panel width (no fallback) for a small width, including width 0.
- Existing panel header-shape and geometry tests pass unchanged.

## Task 9: Keep Rejection.Tokens one shape across both reasons
severity: low
sources: architecture

**Problem**: `Rejection.Tokens` carries two different element shapes depending on `Reason`. For `missing tokens` it holds bare token names (`"text.primary"`, `internal/theme/validate.go:70-86`); for `bad colour` it holds pre-rendered offender pairs (`"text.primary = #GGGGGG"`, `:41-65`). Both flow unchanged through `tokenAttr` (`internal/theme/events.go:150-159`) into a log attr named `token`, so half the time the `token` attr's value is not a token. The field is documented as "the structured source behind Detail", but for `bad colour` it is already rendered copy — the thing the surrounding code is careful to keep out of structured fields (`tokenAttr`'s own comment says it "never parses Detail" for exactly this reason). Any consumer added later that wants the offending token names — a panel highlight, a doctor grouping, a count of affected tokens — gets a formatted pair and has to re-split it, which is the parse the design is trying to prevent.

**Solution**: Make `Tokens` token names for both reasons, carry the offending values alongside, and compose the rendered pair at the two edges that need it.

**Outcome**: A structured field holds structured data; `Detail` and the emitted log line are unchanged.

**Do**:
1. In `internal/theme`, keep `Rejection.Tokens []string` as token *names* for both reasons and add an index-aligned `Values []string`, populated only for `ReasonBadColour` (empty for `ReasonMissingTokens`). Document the pairing on the fields.
2. Rewrite `applyPairs` to collect the offending `(name, value)` pairs once, then set `Tokens` to the names, `Values` to the values, and compose `Detail` from the pairs with the existing `detailBadColourPair` format and `", "` join — `Detail` must be byte-identical to today's.
3. Leave `requireEveryToken` as-is apart from the doc: it already stores names.
4. In `tokenAttr`, compose the `token` attr for `ReasonBadColour` from the paired name and value using the same `"%s = %s"` format, so the emitted `theme: rejected` line is byte-identical; `ReasonMissingTokens` still joins names. Update the comment to say the attr renders from the structured pair and still never parses `Detail`.
5. Do **not** change the emitted `token` attr's value to names-only: the `theme` component's attrs are a spec-governed vocabulary, and dropping the offending hex from the log would lose the only place it is recorded for a rejected file. Note that rejection in the comment.
6. Update any test or consumer asserting `Tokens` contains formatted pairs to assert names plus values.

**Acceptance Criteria**:
- `Rejection.Tokens` holds token names for every reason that populates it.
- `Rejection.Detail` and the `theme: rejected` log line are byte-identical to before for both reasons, including multi-offender files.
- No caller re-splits a `Tokens` element.
- `internal/theme/theme_test.go`'s exported-surface list updated for the new field.

**Tests**:
- A test that a file with two bad colours yields `Tokens` = the two token names, `Values` = the two offending values as the user wrote them (not canonicalised/upper-cased), and the unchanged `Detail`.
- A test that a file with missing tokens yields names in `Tokens` and an empty `Values`.
- A log test pinning the `theme: rejected` line's `token` attr value for both reasons.

## Task 10: Collapse AdaptivePair's tagged-palette machinery to a named constructor
severity: low
sources: architecture

**Problem**: `MemberPalette`, `Member.Palette` (`internal/theme/member.go:26-36`) and `AdaptivePair`'s asymmetric `(named MemberPalette, opposite Theme)` shape (`internal/theme/nomination.go:32-45`) exist to stop a positional light/dark swap at a call site. There is exactly one production call site — `internal/theme/resolution.go:162`, inside the same package as the type — and every other user is a test. The protection is also only one layer deep: `AdaptivePair` immediately delegates to `pairFor(light, dark)`, which is positional again, so the guarded boundary is the outer call while the inner one carries the same risk unguarded. The net effect is three exported API symbols and a branch, on a package whose types are a public contract, for a hazard the compiler cannot see and one line can state plainly.

**Solution**: One named positional constructor; drop the tagging types from the exported surface.

**Outcome**: A smaller public surface on the contract package, with the one call site reading legibly beside the two `resolveSlot` lines that name their slots.

**Do**:
1. Change `AdaptivePair` to `func AdaptivePair(light, dark Theme) Nomination` with `pairFor`'s body (delete `pairFor`, or keep it unexported and have `AdaptivePair` be its only caller — one of the two, not both).
2. Delete `MemberPalette` and `Member.Palette` from `internal/theme`. Keep `Member`, `Member.Opposite` and `Member.Slot` untouched — they carry their own load elsewhere, and `MemberDark`-is-the-zero-value must stay first.
3. Update the one production call site to `AdaptivePair(light.Theme, dark.Theme)` and rewrite `AdaptivePair`'s comment: the ordering is legible at the call site because the two `resolveSlot(SlotLight…)` / `resolveSlot(SlotDark…)` lines directly above name their slots, and the inversion tests pin it.
4. Update the test users — `internal/theme/nomination_test.go:50,51,56,69,112,149`, `internal/tui/nomination_test.go:19`, `internal/tui/theme_testing_test.go:218` — to the positional constructor, keeping every existing assertion (in particular the light/dark inversion coverage, which is now the whole protection).
5. Update `internal/theme/theme_test.go`'s exported-surface list (drop `MemberPalette` and `Member.Palette`) and `internal/tui/theme_panel_commit_slot_test.go:463`'s `Member`-prefixed name filter, which currently excludes `MemberPalette` by name.

**Acceptance Criteria**:
- `MemberPalette` and `Member.Palette` no longer exist.
- `AdaptivePair(light, dark)` is the single adaptive-nomination constructor and no second positional constructor sits beneath it.
- `Nomination.Select(MemberLight)` and `Select(MemberDark)` return the palettes their arguments name for every existing test case.

**Tests**:
- Keep the existing inversion tests, retargeted: a pair built from a light and a dark palette answers each member with its own palette, and a swapped construction is observably different.
- A test that a zero `Nomination` still reports `IsConstant() == false` and answers `Select` with the zero `Theme`.

## Task 11: Let a capture Fixture declare its render size
severity: low
sources: architecture

**Problem**: `theme-panel-narrow` (`internal/capture/fixtures.go:618-625`) and `theme-panel-min-height-message` (`:498-505`) return their base fixtures with only `name` changed. The state that makes them the frames the design calls for — the panel's minimum width, and the height floor with a message live — is terminal geometry, which lives in the caller (`ModelAt(th, w, h)`, `internal/capture/harness.go:22-42`) and, during implementation, in a `.tape` that is scaffolding and gets cleared after sign-off. Every Go-side consumer renders all fixtures at one fixed `harnessWidth`/`harnessHeight`, so these two produce byte-identical output to their bases: they add nothing to the swap-and-diff guard while being counted by its fixture enumeration (`internal/capture/theme_swap_guard_test.go:313-350`), which inverts the property the guard leans on — a fixture's presence reading as coverage. Once the tapes are gone the geometry survives only as a prose comment.

**Solution**: Give `Fixture` an optional declared render size that `ModelAt` honours, and declare it on the two geometry fixtures.

**Outcome**: The two fixtures differ in data rather than in an external instruction, render their intended frames from any Go-side consumer, and their geometry stops depending on an artifact the retention rule deletes.

**Do**:
1. Add unexported `width, height int` fields to `Fixture` (zero meaning "the caller's size"), documented as the render size the fixture requires.
2. Have `ModelAt(th, w, h)` substitute the declared values when non-zero before sending `tea.WindowSizeMsg`, so every consumer — the swap-and-diff guard and `capturetool` alike — honours them without its own branch.
3. Set `themePanelNarrowFixture`'s width to the terminal width that steps the panel to its minimum, and `themePanelMinHeightMessageFixture`'s width and height to the minimum width and the panel's height floor, both derived from the panel geometry constants rather than typed as bare numbers where a constant exists.
4. Replace the two prose comments ("Identical data to the adaptive pair — only the capture width differs", "A fixture cannot resize itself…") with a statement of the declared size and what it exercises.
5. Confirm the guard's fixture enumeration and `capturetool` both go through `ModelAt`; if either sizes a model itself, route it through `ModelAt` rather than duplicating the substitution.

**Acceptance Criteria**:
- `theme-panel-narrow` and `theme-panel-min-height-message` render differently from their base fixtures with no external instruction, from both the guard and `capturetool`.
- A fixture that declares no size renders exactly as it does today at the caller's size.
- The swap-and-diff guard renders each fixture at its declared size and stays green.
- No render size is stated only in a `.tape`.

**Tests**:
- A test that each of the two fixtures' rendered output differs from its base fixture's at the harness's default size.
- A test that `ModelAt` uses the declared size when set and the caller's when not.
- A test that the min-height fixture's frame is at the panel's height floor with the message row live, and the narrow fixture's at the panel's minimum width.

## Task 12: Make the capture fixture registry one authoritative list
severity: low
sources: duplication

**Problem**: `FixtureByName`'s switch (`internal/capture/fixtures.go:126-185`) and `FixtureNames`'s literal slice (`:189-193`) are two hand-maintained lists of the same names. The pattern predates this topic, but the topic grew it by ten `theme-panel-*` entries in each list — the largest single addition it has taken. A name added to the switch but not the slice is invisible to the swap-and-diff completeness guard, which enumerates whatever `FixtureNames` returns, so the failure mode is a quietly shrinking guard rather than a build error.

**Solution**: One ordered list of fixture builders, with both lookups derived from it and each builder's own `name` staying the single source of the name itself.

**Outcome**: A fixture cannot exist in one lookup and not the other, so the guard cannot silently shrink.

**Do**:
1. Add a package-level ordered list of builders — `func fixtureBuilders() []func() *Fixture` — holding every fixture constructor currently named in the switch, in the switch's order.
2. Rewrite `FixtureByName` to build from that list and return the first fixture whose `Name()` matches, preserving the existing unknown-fixture error text and its `available:` list built from `FixtureNames()`.
3. Rewrite `FixtureNames` to map the builders to their `Name()`s, append `ContrastValidationFixture` (still the standalone `tea.Model`, per its comment), and sort as today.
4. Leave each builder assigning its own `name` — that keeps a directly-constructed fixture named in tests, and makes the builder the single source the registry derives from.
5. Keep both functions' behaviour identical for an unknown name, for `ContrastValidationFixture`, and for sort order.

**Acceptance Criteria**:
- Fixture names are enumerated in exactly one place in `internal/capture`.
- `FixtureNames()` returns the same sorted set as today.
- `FixtureByName` returns the same fixture for every existing name and the same error text for an unknown one.
- Adding a builder to the list makes it reachable by name and counted by the guard in one edit.

**Tests**:
- A test that every name in `FixtureNames()` except `ContrastValidationFixture` resolves through `FixtureByName`, and that every builder's name is unique.
- A test pinning the unknown-fixture error text and its `available:` list.
- The swap-and-diff completeness guard stays green over the same fixture set.

## Task 13: Add the styled-blank helper to the contrast swatch
severity: low
sources: duplication

**Problem**: `internal/capture/swatch.go` writes out `lipgloss.NewStyle().Background(<tint>).Render(strings.Repeat(" ", n))` inline five times — `fillCanvas` twice (`:71-72` and `:81`), `subtleBand` twice (`:182-187`) and `padBand` once (`:198`). The file already owns `onTint` for the styled-*text* case; the styled-*blank* case has no equivalent and was written out at each site.

**Solution**: One `fill(tint, n)` helper beside `onTint`, used at all five sites.

**Outcome**: The styled-blank idiom is stated once alongside the styled-text one it mirrors.

**Do**:
1. Add `func fill(tint color.Color, n int) string` beside `onTint` in `internal/capture/swatch.go`, rendering `strings.Repeat(" ", max(n, 0))` on a background of `tint`. The clamp makes it safe for the pad sites, which currently guard with `if gap > 0`.
2. Route all five sites through it: `fillCanvas`'s `blank` and its gap pad, `subtleBand`'s bar and empty segments, and `padBand`'s gap.
3. Leave `fillCanvas`'s clamp-then-pad-then-backfill loop structure alone — its independence from `internal/tui` is deliberate and documented; only its blank and pad *segments* move to the helper.
4. Keep the `if gap > 0` guards or let the clamp absorb them, whichever leaves the rendered bytes identical.

**Acceptance Criteria**:
- `lipgloss.NewStyle().Background(...).Render(strings.Repeat(" ", ...))` appears once in `internal/capture/swatch.go`.
- The contrast-validation swatch renders byte-identically before and after at both a sized and a zero-sized model.
- `fillCanvas` still owns its own loop and does not call into `internal/tui`.

**Tests**:
- A test that the swatch's rendered output is unchanged for a fixed theme at an explicit size and at the 80x24 fallback.
- A test that `fill` returns an empty string for a non-positive width.

## Task 14: Rename internal/sourceguard to the test-only convention
severity: low
sources: architecture

**Problem**: Extracting the AST/file-walk primitives out of `portalbintest` into their own package is right — they are shared by guards in five packages now. But every other test-only helper package in the tree (`portaltest`, `logtest`, `themetest`, `spawntest`, `tmuxtest`, `restoretest`, `portalbintest`, `transienttest`) ends in `test`, which is how a reader tells at the import line that production must not use it, and several additionally take `*testing.T` first so the rule is structural. `internal/sourceguard` states the rule only in its doc comment (`internal/sourceguard/doc.go:1-6`) and its three exported functions take plain strings (`gosourcefiles.go:13`, `packagegofiles.go:15`, `foreachfunccall.go:11`), so nothing at the import site or in the signatures stops production code depending on it — in a package whose whole job is enforcing structural rules, that is the one convention worth matching.

**Solution**: Rename the package so the test-only boundary reads off the import path like every sibling helper's does.

**Outcome**: The import line itself says "test-only", matching every other helper package.

**Do**:
1. Rename the directory `internal/sourceguard` to `internal/sourceguardtest` and the package clause to match.
2. Update the doc comment to keep every existing statement (stdlib-only, no build tag so its guards run in the unit lane, test-only) and note that the name now carries the boundary.
3. Update every importer — the guard tests across the five consuming packages — with no signature changes.
4. Do not fold the functions back into `portalbintest`: five packages share them and `portalbintest` builds binaries, so the separate package earns its keep. Record that in the doc comment so the alternative is not re-litigated.
5. Confirm no production (non-`_test.go`) file imports it before and after.

**Acceptance Criteria**:
- The package is named `sourceguardtest` and every importer compiles.
- No non-test file in the tree imports it.
- The three exported functions' signatures and behaviour are unchanged.
- `go test ./...` and `go test -tags integration -p 1 ./...` are green, and every guard that used it still runs in the unit lane.

**Tests**:
- Existing guard tests in all five consuming packages pass unchanged.
- The package's own tests (if any) move with it and pass.

## Task 15: Correct the theme-flash precedence tier's claims and pin the conformance argument
severity: low
sources: standards

**Problem**: §14A decides that the filter line is the one contender above flash that can be live throughout a panel open/use/close and that theme flashes take precedence over it, naming it a change to the band's precedence. The implementation adds `flashOrigin` (`internal/tui/sessions_flash.go:18-25`), `setThemeFlash` (`internal/tui/model.go:1340-1345`), `themeFlashClaim` and `flashSlotClaim` (`internal/tui/notice_band.go:235-250`) to express that tier — but the model holds a single `flashText`, so `themeFlashClaim` and `flashClaim` return the same value whenever a theme flash is live and `flashSlotClaim`'s two arms are functionally identical. The filter line is also not a contender for this slot at all: it renders on the list's title row via `applySectionHeader` (`internal/tui/model.go:3326-3376`), a different physical row from `renderSessionBandSlot`. The required behaviour holds — a theme flash always reaches the band regardless of filter state — but it holds structurally, not because of the tier. The risk is that the code carries a "Do not re-order" warning defending an arbitration that cannot currently change any outcome, so a future change that genuinely makes the filter line a band contender will believe it is already protected.

**Solution**: Keep the tier as an explicit forward guard, correct the comments to say what it does and does not currently do, and add the test that pins *why* the required behaviour holds today.

**Outcome**: No comment claims an ordering the code cannot exercise, and the conformance argument (filter line ≠ band contender) is pinned by a test rather than asserted in prose.

**Do**:
1. Rewrite the comment at `internal/tui/notice_band.go:242-244`: state that the theme tier is a forward guard — with a single `flashText` its two arms currently return the same claim, so the ordering is unobservable today — and that the behaviour §14A requires holds because the filter line renders on the section-header row, not in the band slot. Drop the bare "Do not re-order" in favour of naming the invariant it protects: if the band ever gains a contender that can be live throughout a panel session, the theme tier must stay above it.
2. Adjust the comment at `internal/tui/sessions_flash.go:18-19` the same way — the origin exists so the tier is granted by the setter rather than inferred from wording, and so a copy edit cannot move a signal out of it.
3. Keep `flashOrigin`, `flashOriginTheme`, `setThemeFlash`, `themeFlashClaim` and `flashSlotClaim`. Record in one place why the alternative (deleting the tier and stating the conformance argument in a comment on `activeNoticeBand`) was rejected: `setThemeFlash` is the single chokepoint granting the tier, and without it a later band change could silently rank a theme signal beneath a longer-lived contender.
4. Add the missing pin as a test, not a comment (see Tests).
5. Change no behaviour: the band's rendered output must be identical for every existing case.

**Acceptance Criteria**:
- No comment in `internal/tui` claims an ordering effect the current band cannot produce.
- The forward-guard rationale and the rejected alternative are each stated once.
- Every theme signal still routes through `setThemeFlash`, and no theme signal is raised through `setFlash` directly.
- Band output is unchanged for all existing cases.

**Tests**:
- A test that with a filter applied (and unfocused) and a theme flash live, the flash occupies the band slot and the filter line renders on the section-header row — the conformance argument, pinned.
- A test that every theme signal (the three entry refusals, both forced-close strings, and `theme not saved — see portal.log`) carries `flashOriginTheme`, so a signal added later without `setThemeFlash` fails.
- Existing notice-band precedence tests pass unchanged.
