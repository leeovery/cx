# Plan: Theming System

## Phases

### Phase 1: Theme file format and loader

**Goal**: A new `internal/theme` leaf package owning the 19-token single-palette vocabulary, turning a `.theme` file into a validated `Theme` or exactly one rejection reason.

**Why this order**: Everything downstream consumes it, it declares the token names that become the public contract every later phase writes against, and it is the branch-heaviest component in the feature with no dependency on the render layer.

**Acceptance**:
- [ ] `internal/theme` declares `Token{Name, Value}` with `Color()`, a 19-field `Theme`, and `All()` in the §2.4 table order 1–19; a guard test pins the count at 19 and the token-name set
- [ ] The parser implements §4.2's lexical rules and §4.3's `#RRGGBB`-only value domain, canonicalising hex to uppercase; every row of §4.2's branch table is a passing case
- [ ] Validity is all-19-present plus every known value well-formed; unknown keys are ignored, key and value alike
- [ ] §6.2's seven reasons evaluate in fixed order and short-circuit, so any input yields exactly one reason (a duplicate-keyed file also missing tokens reports `bad syntax`)
- [ ] Slug charset `^[a-z0-9][a-z0-9-]*$` and the exactly-lowercase `.theme` extension are enforced by rejection, never normalisation; other extension casings enumerate then fail `bad name`
- [ ] `themesDirPath` resolves `PORTAL_THEMES_DIR` → `XDG_CONFIG_HOME/portal/themes/` → `~/.config/portal/themes/` in `cmd/config.go` and is injected; the loader never resolves, creates or seeds the directory
- [ ] Enumeration is top-level only, follows symlinked files and the resolved directory root, skips directory-valued entries silently; an absent directory is silent, an unusable one reports `unreadable`
- [ ] A new `theme` log component emits through an injected logger seam with per-process dedup state on that logger; `log.Discard` silences it entirely

### Phase 2: The three built-in themes and `portal theme export`

**Goal**: Ship Tokyo Night Dark, Tokyo Night Light and Nord as embedded `.theme` files parsed by the Phase 1 loader, floor-checked automatically, with `portal theme export <slug>` printing any theme's bytes.

**Why this order**: Porting one genuinely external palette is the first real test of whether the 19 roles map cleanly, and that test must happen before the names become a public contract; it also gives the loader real content and the feature its first user-facing surface.

**Acceptance**:
- [ ] `tokyo-night`, `tokyo-night-day` and `nord` ship as `.theme` files embedded via `go:embed` and parsed by the same loader as a drop-in; no palette values remain in Go
- [ ] MV's seven light corrections are re-derived in Oklab with chroma loss recorded against the original; anything at or over ΔE 0.05 is replaced and visually gated, and every figure lands as a `#` comment beside its value in `tokyo-night-day.theme`
- [ ] `nord.theme` ships §7.4's values and clears every §13.5 rule including both pairing legs it was corrected for; its eyeball-pinned tints and inventions pass a human gate through the `--theme`-re-pointed swatch surface
- [ ] The floor tests auto-enumerate the embedded set and measure against each theme's own `canvas`, with a light-theme enrolment table asserting every embedded theme appears in it
- [ ] Built-in slugs are reserved: a drop-in whose slug collides is rejected `reserved name` from the slug alone, before any read
- [ ] A build-time test parses and validates every embedded theme AND asserts every fallback slug and the shipped default pair resolves within that set
- [ ] `portal theme export <slug>` writes the file's bytes verbatim (comments included) to stdout, is bootstrap-exempt, takes exactly one slug, and refuses unknown / invalid / unreadable / bad-name slugs with §14A's copy on stderr and exit 1

### Phase 3: The render layer on a single threaded palette

**Goal**: Rename the vocabulary at every call site, collapse `Token` to one value, delete `theme.Mode` threading and `internal/tui/theme`, and have the model hold and thread the active `Theme` loaded from the embedded set.

**Why this order**: The loader and the built-ins now exist for the render layer to consume, and split plus rename is a single edit per call site — sequencing them apart would touch all ~182 sites twice.

**Acceptance**:
- [ ] Every renderer takes the active `Theme` where it takes `theme.Mode` today; `theme.MV`, `Token.ColorFor`, `theme.Mode` and `internal/tui/theme` are gone, with no package-level mutable theme state replacing them
- [ ] The model holds the active `Theme`; `tui.Build` takes the loaded nomination (constant or pair) in place of `WithAppearance`, and the first-paint gate selects the pair's member before anything is painted
- [ ] Rendering is unchanged apart from the consolidated `border`: an existing `appearance` pin still selects its built-in, `NO_COLOR` still renders colourless and glyph-backed, and every existing fixture renders as before
- [ ] `canvasHexFor` references no built-in and `RestoreTerminalBackground` compares against a startup canvas hex retained on the model, captured when the gate resolved
- [ ] `capturetool` takes `--theme <slug|path>` (default `tokyo-night`) in place of `--appearance`, with §13.3's slug/path discrimination, a hard error on invalid content and stderr warnings for the filename reasons
- [ ] The colour-literal guard's `theme`-subpackage exemption is deleted and the guard still passes over `internal/tui`; the MV-shaped variant tests are retired
- [ ] Today's committed reference PNGs and VHS tapes are deleted, and a grouped Nord capture is produced that passes the outstanding `text.subtle` visual gate
- [ ] CLAUDE.md's `tui/theme` and `tui` rows are corrected in this phase, re-anchoring the standing "do not drop this guard" warning to the retained startup canvas hex

### Phase 4: Live-swap completeness and the swap-and-diff guard

**Goal**: Make a mid-session theme change reach every rendered surface, and prove it behaviourally rather than by inspection.

**Why this order**: It protects the threading just landed while it is fresh, and the panel's live preview — three phases later — rests entirely on the swap being complete.

**Acceptance**:
- [ ] The init-time derived-style sweep is run and its residue recorded; `pagepreview.go`'s package-scope `Token` copy is gone
- [ ] `tui.Build` / `internal/capture` expose a seam that renders a fixture, swaps the theme through the production restyle path, and renders again — a live mutation of one already-rendered model, not two separately-built ones
- [ ] The guard enumerates the harness's fixture set without naming fixtures, uses two synthetic themes with all 38 values unique, forces a truecolor profile, and compares each token's rendered SGR form rather than its hex
- [ ] Its three assertions hold: no theme-A value survives the swap, every theme-B value appears across the fixture union, and every token is exercised by at least one fixture
- [ ] Colourless fixtures are excluded, and the guard runs in the unit lane
- [ ] A named `RestoreTerminalBackground` test covers both divergence cases — a theme changed mid-session, and quit with an uncommitted preview active

### Phase 5: The theme setting — resolution, fallback and detection

**Goal**: `prefs.json`'s `theme` / `theme_light` / `theme_dark` decide which theme(s) load at construction, with per-slot mode-matched fallback and terminal-background detection choosing between a nominated pair.

**Why this order**: With rendering on a threaded palette, the only remaining question is where the nomination comes from — and the panel, two phases later, writes exactly these keys.

**Acceptance**:
- [ ] `prefs.json` decodes `theme` / `theme_light` / `theme_dark` tolerantly per field, retains raw `appearance` as a preserved string, defaults unset slots to the shipped pair, and lets a non-empty `theme` win over the slots
- [ ] Construction loads only nominated themes — one read for a constant, two for a pair, no enumeration — resolving the embedded set before the themes directory
- [ ] A persisted slug is charset-validated before use as a path component; an unloadable nomination falls back per slot (`theme_light` → `tokyo-night-day`, otherwise `tokyo-night`), never overwrites the persisted name, and logs `theme: fallback applied`
- [ ] The gate is skipped entirely under a constant and resolves exactly once under a pair; a late OSC 11 reply is still consumed for the original-background capture but never re-themes
- [ ] A fallback that cannot resolve within the embedded set is a fatal one-line message per §14A rather than a panic, and nothing walks the embedded set at startup
- [ ] `theme: loaded` fires once per nomination carrying `slug` and `slot`, and additionally for the fallback whenever a nomination is unloadable
- [ ] `portal open <target>` still constructs no TUI, reads no prefs and does no theme work at all

### Phase 6: Prefs write path and the `appearance` upgrade

**Goal**: Every `prefs.json` write is a read-modify-write that cannot lose another instance's field, and an existing `appearance` pin translates exactly once into the equivalent constant theme.

**Why this order**: Writes must be safe before the panel starts making them, the upgrade must land before users meet the new setting, and this is the one path in the feature whose failure silently and permanently destroys a user's config.

**Acceptance**:
- [ ] `prefs` gains `SaveTheme`, `SaveThemeSlot`, `SaveMigrationMarker` and `SaveTranslation`, each performing its own RMW with a strict write-path decode; the existing `s`-key persister comes under the same rule
- [ ] An absent `prefs.json` is created by a write; a syntactically malformed one aborts the write rather than overwriting it; unrecognised values in valid JSON are absorbed tolerantly as today
- [ ] Mutual exclusion holds on write — a constant clears both slots, a slot clears the constant — in one atomic write, and `theme_migrated` never participates in it
- [ ] The translation is marker-gated, runs only where a TUI is constructed, writes theme key and marker in one combined save, is best-effort and non-blocking, applies its computed value in memory the same launch, and writes no theme key when any is already set — evaluated at load for the in-memory half and again at the RMW re-read for the write
- [ ] Raw `appearance` round-trips untouched through every subsequent write, and `theme: appearance migrated` fires only on a successful theme-key persist
- [ ] A `WithThemePersister` seam mirrors `WithModePersister`, is owned by `cmd`, and is the single emission site for `theme: commit failed`; `cmd/config.go` exposes a non-migrating prefs read

### Phase 7: `portal doctor` theme advisories

**Goal**: Doctor reports every unusable theme file, unresolvable persisted slug and unusable themes directory at full width, as advisories that do not drive the exit code.

**Why this order**: It is the only surface carrying §6.2's per-file detail, it depends on the loader and the resolved setting (both now present), and shipping it before the panel gives a broken drop-in a diagnosis route.

**Acceptance**:
- [ ] Doctor scans the themes directory and emits one `⚠` advisory line per finding using §14A's exact frames and per-reason detail formats (missing token names, offending `key = value` pairs, line number and content, verbatim OS error, and the reserved-name line naming both conflict and fix)
- [ ] One slug produces one advisory line: the unresolvable-persisted line wins over the file line, renders `light` / `dark` / `both` or omits the parenthetical under a constant, and reports only the keys in force
- [ ] Advisories render as a trailing block after the ordered check catalog and before the summary, never interleaved, on both the plain and `--fix` paths
- [ ] Theme lines never affect the exit code; the closing summary distinguishes `<N>`/`<T>` checks from `· <M> advisory|advisories`, suppressed entirely at M=0
- [ ] Doctor reads prefs through the non-migrating variant, emits no `theme` log events, and performs no repair or write; an absent themes directory produces no line at all

### Phase 8: The slide-over panel — surface, preview and navigation

**Goal**: `t` opens a full-height, right-edge, non-blanking panel listing every theme, arrowing re-themes the app and the panel live behind it, and `Esc` closes onto the resolved persisted state.

**Why this order**: The render layer now swaps completely and the setting resolves, so the panel is the surface built over them; commits are deliberately held back so that browsing and previewing land as their own checkpoint.

**Acceptance**:
- [ ] `internal/theme` assembles the §9.4 union behind a `ThemeEnumerator` seam — every file, every built-in, every persisted slug resolving to neither — deduped one-slug-one-row with each row's reason, re-read on every open and retained for the panel's lifetime; `theme: enumerated` carries `count` and `rejected`
- [ ] Rows render per §9.1's surface-token table and §9.5's rules: sort key separate from display label, case-insensitive comparison with byte-wise tie-break and the built-in ahead of its colliding file, the four-element composition priority with its three-character truncation floor, invalid rows in `text.subtle` with a glyph-backed `⚠` and terse reason, badges per the three-row derivation table including the shipped-default case, and the pinned `⚠ dir unreadable` chrome row outside the ordering
- [ ] Opening lands the cursor on the theme actually rendering (constant row, in-force slot row, `● both` row, or the fallback's row), previews nothing, and picks up a mid-session file edit — including one that invalidates the active theme, which flips on open
- [ ] Arrowing re-themes the app and the panel's own chrome, including the panel's `bubbles/list`-owned styles, through the restyle path with no file read per keystroke; `Esc` discards the preview and renders the resolved persisted state
- [ ] `t` is bound on Sessions and Projects with the filter carve-out, blocked with §14A's pinned flashes under `NO_COLOR` and below either render-floor dimension, swallowed during a pending burst, and unbound on Preview, Loading and modals; the panel is key-exclusive with `Ctrl-C` live and nests over multi-select without disturbing the marked set
- [ ] Width and height degrade between preferred and minimum and refuse only below the floor (header + footer + one row + one message row, plus the directory row when present); a resize below the floor takes the `Esc` path with its pinned flash
- [ ] The panel's six keys live in the keymap descriptor as a panel scope driving the vertical footer from its `Core` entries; §14's footer revision ships on both pages with lockstep help filtering and right-to-left degradation that never drops `? help`; Projects gains a transient flash slot
- [ ] Panel fixtures render: adaptive pair, constant-while-previewing, invalid row, `⚠ dir unreadable`, narrow degraded, a paginating panel, and the panel over Projects — each declaring its palette, raw persisted keys, faked union and cursor coherently

### Phase 9: Panel commits — assignment, confirmation and failure reporting

**Goal**: `Enter` / `d` / `l` persist a constant or a slot, with the slot-from-constant confirm and a failed write that is reported rather than silent.

**Why this order**: These are the only panel actions that write; the surface, the live preview and the safe write path all exist beneath them, so this phase adds persistence and nothing else.

**Acceptance**:
- [ ] `Enter` commits a constant clearing both slots, `d`/`l` commit a slot clearing the constant, the panel stays open in every case, and a commit never changes what is previewed
- [ ] A successful commit recomputes the full row set from the construction-time snapshot plus this instance's own mutation (rows appear and disappear), re-sorts, re-anchors the cursor to the previewed theme's identity rather than its index, performs no directory re-read, and loads a newly-live opposite slot with `theme: loaded` emitted at commit
- [ ] `d`/`l` on a constant raises the pinned inline confirm in the message slot, key-exclusive within the panel and resolving only on `y`/`Y`, `n`/`N`/`Esc` or `Ctrl-C`, swapping the footer to its own keys from a nested descriptor scope, and committing constant-clear plus slot in one atomic write; a forced close cancels it silently
- [ ] A failed write keeps the theme applied in memory, does not move the `●`, shows `⚠ couldn't save theme` until the next keypress, and leaves the failure outstanding until a later commit succeeds
- [ ] Closing with a failure outstanding raises `⚠ theme not saved — see portal.log` and discharges the state; on a forced close it wins over the geometry flash; `Ctrl-C` is an accepted undelivered report with the log as the record
- [ ] Theme flashes take precedence over the filter line in the notice band, on Sessions and Projects alike
- [ ] `keymap_dispatch_guard_test` covers the panel and confirm scopes; the panel behaviour test covers the union, ordering, composition, badge derivation, commit recompute, the confirm's three-input resolution and the outstanding-failure state machine
- [ ] The confirm, failed-commit and minimum-height-with-message fixtures render

### Phase 10: Documentation, spec amendments and capture cleanup

**Goal**: Publish the public contract and reconcile every document the feature invalidates.

**Why this order**: The contract is only fully settled once every surface exists, and the doc guard compares the published token table against the final `Theme.All()`.

**Acceptance**:
- [ ] `docs/theming.md` carries the 19 roles and meanings, the ramp's weight ordering, the file format, the discovery rules and resolution chain, the two-line `mkdir -p` drop-in workflow, a complete copy-pasteable example theme, the two-slot config including the `theme`-wins rule, the reserved built-in slugs, and attribution for the ported palettes with Nord's corrections
- [ ] A guard test parses the doc's token table against `Theme.All()` and parses its example theme as a valid 19-key file
- [ ] README's four `appearance` sites are replaced per §12.5, the `prefs.json` row lists the theme keys, and a themes-directory row carries `PORTAL_THEMES_DIR`; the retained `appearance` key is not documented as live
- [ ] CHANGELOG carries the upgrade note — the new setting and three built-ins, the automatic translation requiring no user action, and the old key left in place and not kept in sync
- [ ] CLAUDE.md's remaining stale entries are corrected: config path resolution, the bootstrap-exempt set, the `prefs` row, the log component count, and the capture harness section
- [ ] The Modern Vivid specification is amended per §15.1 and §15.2 through the completed-unit correction protocol (in-place edit, corrigendum, re-index, scoped commit)
- [ ] This feature's captures and tapes are cleared out at sign-off; the Go fixture definitions in `internal/capture` and the harness itself remain

## Planning Notes

Two ordering decisions made at phase-design time, both planning-level (not spec deviations):

1. **Phase 3 keeps the legacy `appearance` pref working** by mapping it onto the new nomination shape (§10.2's mapping applied in memory: `auto` → the built-in pair, `light`/`dark` → the equivalent constant). Without it, the render-layer phase would silently break existing pins for the several phases before the migration lands in Phase 6. Every phase boundary therefore leaves a shippable binary.
2. **Nord's outstanding `text.subtle` visual gate (§7.4) lands in Phase 3, not Phase 2**, because a grouped Nord capture needs the render layer to take a `Theme` first. Phase 2 still gates Nord's tints and the re-derived light values through the standalone swatch surface, which does not route through `tui.Build`.
