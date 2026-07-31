# Phase 1: Theme file format and loader — 8 tasks

## theming-system-1-1

### Task 1.1: Declare the 19-token vocabulary in a new `internal/theme` leaf package

**Problem**: Portal's token layer today is `internal/tui/theme` — a ~20-token vocabulary where each `Token` carries a `Light` and a `Dark` hex resolved through `ColorFor(mode)`, exported as the single package-level `theme.MV`. The theming feature needs a **single-palette 19-token** vocabulary living in a package that `cmd/doctor.go`, `portal theme export`, the panel and the TUI can all import without reaching into a TUI subpackage. More urgently, these token names become the **public contract** every `.theme` file is written against (§1.3, §2.3) — renaming one later breaks every file in a user's themes directory — so the names, the count and the ordering must be settled and guarded *before* anything parses a file.

**Solution**: A new leaf package `internal/theme` declaring `Token{Name, Value}` with a no-argument `Color()`, a 19-field `Theme` struct carrying the §2.4 Go field names, and an `All()` accessor returning the tokens in the §2.4 table order 1–19 — all three driven by one canonical ordered name↔field table so they cannot drift. A guard test pins the count at 19, the exact token-name set, and the order.

**Outcome**: `go test ./internal/theme` proves the vocabulary is exactly 19 tokens with the §2.4 names in the §2.4 order, that every struct field is reachable through `All()`, and that a zero-value `Token.Color()` does not panic. `internal/tui/theme` is untouched and still in use by the render layer.

**Do**:
- Create `internal/theme/theme.go` with a package doc stating: this is the closed single-palette token vocabulary; a `Theme` is the parse result of a `.theme` file, not a package-level built-in; the package holds the vocabulary, the parser, the validator and the §6.2 ladder (§3.2).
- Declare `type Token struct { Name, Value string }` and `func (t Token) Color() color.Color { return lipgloss.Color(t.Value) }`. Document why it is an accessor rather than an inline conversion at each of the ~182 call sites: it keeps call sites reading as they do today, gives the Phase 4 swap-and-diff guard one place to derive a token's rendered form from, and leaves a single seam if the value domain ever widens (§3.2, §4.1's deferred `transparent` keyword).
- Declare `type Theme struct` with exactly 19 `Token` fields, in §2.4 row order, using the Go field names from that table: `TextPrimary`, `TextSecondary`, `TextTertiary`, `TextMuted`, `TextSubtle`, `TextFaint`, `TextOnSelection`, `AccentPrimary`, `AccentKey`, `AccentMode`, `AccentAttention`, `StatePositive`, `StateDestructive`, `Canvas`, `BgSelection`, `BgAttention`, `BgSubtle`, `Border`, `TextOnAttention`. Comment each field with its §2.5 role sentence.
- Add one unexported canonical table — e.g. `func (t *Theme) fields() []fieldRef` where `fieldRef{Name string; Field *Token}` — listing the 19 pairs in §2.4 order 1–19. This is the single source that `All()`, the exported `TokenNames() []string`, and (later tasks) the parser's key→field assignment all derive from; no second literal list anywhere in production code.
- Implement `func (t Theme) All() []Token` returning the 19 tokens in that order, and `func TokenNames() []string` returning the 19 names in that order.
- Write `internal/theme/theme_test.go` holding the guard: a literal expected ordered name slice (the one place the names are re-stated, deliberately, so a rename must be made twice), a `reflect`-based field-count cross-check, and the `Color()` cases.

**Acceptance Criteria**:
- [ ] `Theme` declares exactly 19 fields, all of type `Token`, and `reflect.TypeOf(Theme{}).NumField() == len(Theme{}.All())` — so a field added without a table entry fails the suite rather than being silently invisible to `All()`.
- [ ] `TokenNames()` equals, in order, `text.primary, text.secondary, text.tertiary, text.muted, text.subtle, text.faint, text.on-selection, accent.primary, accent.key, accent.mode, accent.attention, state.positive, state.destructive, canvas, bg.selection, bg.attention, bg.subtle, border, text.on-attention`.
- [ ] `All()` returns tokens in exactly that order — position 1 is `text.primary` and position 19 is `text.on-attention`; the assertion is on the whole ordered slice, not a set.
- [ ] No `border.footer` / `BorderFooter` field exists; no `Light`/`Dark` fields, no `Mode` type, no `ColorFor` method, and no package-level `MV` (or any other) built-in `Theme` var.
- [ ] `Token{}.Color()` returns a `color.Color` without panicking; `Token{Value: "#C0CAF5"}.Color()` resolves through `lipgloss.Color`.
- [ ] `internal/tui/theme` is byte-unchanged and `go build ./... && go test ./...` is green.

**Tests**:
- `"it declares exactly nineteen tokens"` — `TestTokenCount_IsNineteen`
- `"it returns All() in the spec table order 1 through 19"` — `TestAll_ReturnsSpecTableOrder`
- `"it exposes the exact nineteen token names"` — `TestTokenNames_MatchExactSet`
- `"it reaches every struct field through All()"` — `TestAll_CoversEveryStructField` (reflect field count vs `len(All())`, so a skipped, duplicated or reordered field is caught)
- `"it does not panic on a zero-value token's Color()"` — `TestTokenColor_ZeroValueDoesNotPanic`
- `"it resolves a hex value through lipgloss"` — `TestTokenColor_ResolvesHexThroughLipgloss`
- `"it carries no light/dark variant surface"` — `TestVocabulary_HasNoModeSurface` (compile-level: the package exports no `Mode`, no `ColorFor`; assert via the absence of the symbols in the guard's expected export list)

**Edge Cases**:
- `All()` must not skip, duplicate or reorder a field — the reflect cross-check catches a field added to the struct but not to the canonical table; the literal ordered name slice catches a reordering.
- A renamed Go field carrying a stale token name: the guard compares `All()[i].Name` against the literal list, so `AccentPrimary` populated with `"accent.violet"` fails.
- Zero-value `Token.Color()`: `Value` is `""`, which `lipgloss.Color` turns into its no-colour sentinel rather than erroring — pin that it is safe, since a hand-constructed `Theme` (Phase 4's synthetic guard themes, §3.2's "constructible in a test without going through the loader") can legitimately hold partly-empty tokens.
- The order is the §2.4 table rows 1–19 **and nothing else** — not alphabetical, not grouped-by-kind, not the struct's textual order if those ever diverge.

**Context**:
> §3.2 pins the data shape: `Token` becomes `{Name, Value}`; `Token.ColorFor` is removed and replaced by a no-argument `Token.Color()`; `theme.Mode` is removed; `Theme` remains a struct of 19 named `Token` fields with a stable-order `All()` accessor but is no longer a package-level `var` holding one built-in — it is the parse result of a theme file. **`All()`'s stable order is the §2.4 table order, 1 through 19** — the numbering *is* the definition. `Theme` carries **no identity field**: the slug is held alongside the palette by whatever loaded it, which is what lets `capturetool --theme <path>` work at all.
> §2.2: `border.separator` and `border.footer` consolidate to a single `border` token — the footer rule renders with the same token as the title rule.
> §2.3/§2.4: names are meaning-and-weight, never hue or place. `text.on-selection` / `text.on-attention` are the deliberate fifth "pairing" kind.
> Phase boundary: `internal/tui/theme` stays untouched and in use — the render-layer rename and the old package's deletion are **Phase 3**. §13.6 retires `TestMVTokenCount` (20 → 19) there; this task's guard is the new home for that assertion, not a replacement landing in the old package.
> Unlike `internal/prefs`, `internal/theme` is **not** a no-logging leaf — it binds the `theme` log component (task 1-8, §12.3), so it may import `internal/log`.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §2.1–§2.5, §3.2, §13.6

## theming-system-1-2

### Task 1.2: Lex `.theme` lines into key/value pairs or exactly one `bad syntax` rejection

**Problem**: A theme file is a hand-rolled flat `key = value` format, and `#` is **both** the comment marker and the hex prefix — so `text.primary = #ECEFF4 # tuned for the lighter canvas` is either a colour plus a note or one invalid value, and nothing decides which unless the lexical rules are pinned exactly. Get it wrong in the lax direction and a malformed line is silently dropped, surfacing much later as a misleading `missing tokens`; get it wrong in the strict direction and legal files are rejected. There is no existing test home for any of this — §7.6's embedded-set test only ever sees valid files by construction.

**Solution**: A line-oriented lexer in `internal/theme` turning file bytes into an ordered `[]Pair{Key, Value, Line}`, or a single `bad syntax` `Rejection` carrying the 1-based line number and §14A's detail phrase. The reason vocabulary type is declared here too, since `bad syntax` is the first of the seven to have a producer.

**Outcome**: Every row of §4.2's branch table that reaches a lexical verdict is a passing table-driven case, and `line N: duplicate key text.primary` / `line N: quoted value` / `line N: not a key = value pair` are produced verbatim for Phase 7's doctor detail.

**Do**:
- Create `internal/theme/reason.go`: `type Reason string` with the seven §6.2 constants whose **string values are the terse labels verbatim** — `missing tokens`, `bad colour`, `bad syntax`, `bad name`, `reserved name`, `unreadable`, `not found` (these are what the panel row renders and what §14A prefixes with `⚠ `). Declare `type Rejection struct { Reason Reason; Detail string; Line int; ... }` implementing `error`, with a doc comment stating the invariant that a `Rejection` always carries exactly one reason and its detail never spans two (§6.2).
- Create `internal/theme/lex.go` with `func lexPairs(data []byte) ([]Pair, *Rejection)` and `type Pair struct { Key, Value string; Line int }`.
- Pre-pass: strip a UTF-8 BOM (`\xEF\xBB\xBF`) from the **first bytes of the file only**; split on `\n` and drop one trailing `\r` per line (CRLF tolerance); **trim each line at both ends before any classification** so leading indentation before a key or before `#` is fine.
- Classify the trimmed line: empty → skip; first byte `#` → comment, skip; otherwise split on the **first** `=` only. Key is the left part trimmed, value is everything after the first `=` trimmed — the format never re-interprets anything right of the first separator, so a `#`, a second `=` or spaces inside the value are all part of the value.
- Reject with `bad syntax` and the pinned detail: no `=` present, empty key, or a key containing whitespace or `=` → `line N: not a key = value pair`; a value whose **first character** is `"` or `'` (matched or unmatched, either quote) → `line N: quoted value`; a key already seen in this file → `line N: duplicate key <key>`, naming the **second** occurrence's line. Keep the duplicate check purely lexical, before any key is classified as known/unknown and without comparing values.
- Line numbers are 1-based over the original file including comments and blanks, so the detail is exact.

**Acceptance Criteria**:
- [ ] `text.primary = #ECEFF4 # tuned for the lighter canvas` lexes to one pair whose value is `#ECEFF4 # tuned for the lighter canvas` verbatim — no trailing-comment handling exists.
- [ ] `  text.primary  =  #ECEFF4  ` lexes to key `text.primary`, value `#ECEFF4`; `   # note` is a comment.
- [ ] A CRLF file lexes identically to its LF twin; a BOM at file start is invisible; a BOM anywhere else is `bad syntax`.
- [ ] An empty file and a comments-only file both lex to zero pairs with **no** rejection (the presence check in task 1.3 is what fails them as `missing tokens`).
- [ ] `= #FFFFFF`, `text.primary`, and `text primary = #FFF` each yield `bad syntax` with `line N: not a key = value pair`.
- [ ] A duplicated key yields `bad syntax` naming the second line, for all four combinations of {known, unknown} × {same value, different value}.
- [ ] `"#FFFFFF"`, `'#FFFFFF'` and `"#FFFFFF` all yield `bad syntax` / `line N: quoted value` — never `bad colour`.
- [ ] `text.primary = #ECEFF4 = x` lexes successfully to value `#ECEFF4 = x` (it fails later as `bad colour`, not here).
- [ ] `text.primary =` lexes successfully to an empty value — a well-formed pair, deliberately not a syntax error.
- [ ] The rejection is a single `*Rejection`; the lexer never returns partial pairs alongside an error.

**Tests**:
- `"it parses key = value pairs with 1-based line numbers"` — `TestLex_ParsesKeyValuePairsWithLineNumbers`
- `"it trims the whole line and around the separator"` — `TestLex_TrimsLineAndAroundSeparator`
- `"it treats # as a comment only at line start"` — `TestLex_HashStartsCommentOnlyAtLineStart` (covers the `#ECEFF4 # note` forcing case)
- `"it tolerates CRLF line endings"` — `TestLex_ToleratesCRLF`
- `"it strips a BOM at file start only"` — `TestLex_StripsBOMAtFileStartOnly`
- `"it rejects an interior BOM as bad syntax"` — `TestLex_InteriorBOMIsBadSyntax`
- `"it yields no pairs for a blank or comment-only file"` — `TestLex_BlankAndCommentOnlyFileYieldsNoPairs`
- `"it rejects a line that is not a key = value pair"` — `TestLex_RejectsMalformedPairLines` (table: no key, no `=`, key with whitespace, key containing `=`)
- `"it rejects a duplicate key regardless of knownness or value"` — `TestLex_RejectsDuplicateKey` (table of four; asserts the detail names the second occurrence's line)
- `"it rejects any leading quote, matched or not"` — `TestLex_RejectsAnyLeadingQuote` (table of three)
- `"it splits on the first = only"` — `TestLex_SplitsOnFirstEqualsOnly`
- `"it treats trailing whitespace after a value as trimmable, not an error"` — `TestLex_TrailingWhitespaceIsNotAnError`
- `"it treats an empty value as a well-formed pair"` — `TestLex_EmptyValueIsAWellFormedPair`

**Edge Cases**:
- CRLF; BOM stripped at file start only (interior BOM → `bad syntax`); leading whitespace before a key or `#`.
- `#` starts a comment only at line start, so a `#` after `=` is part of the colour.
- Blank/comment-only file parses to zero pairs with no error.
- `= #FFFFFF` (no key); `text.primary` (no `=`); a key containing whitespace or `=` — the last matters because without the well-formed-key rule the line would be a well-formed pair with an unknown key, get *ignored*, and the file would fail as `missing tokens`, pointing at the wrong thing for a plain typo.
- Duplicate key — known or unknown, same or different value; the check is lexical and unconditional because making it conditional adds branches to buy nothing.
- Any leading quote, matched or unmatched, is "quoted": defining it by a matched outer pair would send the unmatched case down the ladder to `bad colour`, telling the user their colour is wrong when their quoting is.
- Split on the first `=` only; trailing whitespace after a value is trimmed and is not an error.
- Line numbers are retained for §14A's `line N: …` detail — the only carrier of *which* line is wrong.

**Context**:
> §4.2's table is the specification of this task, branch by branch, "because each one is a user-visible reason label and a test case in the loader test". §4.1 records the format rationale: Portal already parses this shape (`aliases`), zero new dependencies, and JSON cannot carry the comments a theme file genuinely wants (attribution headers, eyeball-pin derivation notes).
> §4.1's forward note: the deferred transparent-theme idea would widen §4.3's **value** domain with a distinguished keyword. The route explicitly **closed** is btop's precedent of an *empty* value — §4.2 pins an empty value as `bad colour` deliberately, so do not add an empty-value branch here.
> §14A pins the `bad syntax` detail formats: `line 12: duplicate key text.primary` / `line 4: quoted value` / `line 7: not a key = value pair`. A duplicate names the **second** occurrence's line, which is the one to delete.
> §13.6 names the loader/parser test as new and table-driven over §4.2's branch table — "the single most branch-heavy component in the feature has no other test home".

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §4.1, §4.2, §6.2, §13.6, §14A

## theming-system-1-3

### Task 1.3: Turn lexed pairs into a validated `Theme` — value domain and presence

**Problem**: `lipgloss.Color` **never returns an error** and its accepted domain is wider and stranger than a theme format wants — `"212"` is a valid ANSI-256 index, `"-5"` is silently abs'd, `"16777215"` is reinterpreted as packed RGB, and every failure is the silent `noColor` sentinel. Without Portal owning a validator, a typo'd hex becomes an invisible wrong colour rather than one honest message. Separately, a file that lexed cleanly still has to declare all 19 tokens, and the two checks must be ordered and enumerated so doctor can say *which* value or *which* token is at fault.

**Solution**: A validator in `internal/theme` mapping `[]Pair` to a `Theme`: known keys' values are checked as `#RRGGBB` and canonicalised to uppercase, unknown keys are ignored entirely (key **and** value), then the presence check runs across the 19. Each stage produces a single rejection enumerating every offender within its own reason.

**Outcome**: A well-formed 19-key file becomes a `Theme` whose values are uppercase-canonical; anything else yields exactly one of `bad colour` (listing every offending `key = value` pair) or `missing tokens` (listing every absent name).

**Do**:
- Create `internal/theme/validate.go` with `func themeFromPairs(pairs []Pair) (Theme, *Rejection)`.
- Implement the hex validator: exactly `#` followed by six hex digits. Reject `#RGB` shorthand, 8-digit `#RRGGBBAA`, non-hex digits, an empty value, and any interior whitespace. Accept either input case and **canonicalise to uppercase** before storing in `Token.Value`.
- Classify each pair against the canonical name table from task 1.1, matched **case-sensitively**. An unknown key is skipped entirely — its value is never validated and it never appears in any detail.
- Collect **every** offending known pair in file order; if any exist, return one `bad colour` rejection whose `Detail` is those pairs rendered `key = value`, comma-separated (§14A: `text.primary = #GGGGGG, canvas = blue`).
- Otherwise collect every absent token name in §2.4 order; if any exist, return one `missing tokens` rejection whose `Detail` is those names comma-separated (§14A: `missing text.primary, bg.subtle`).
- Otherwise populate the `Theme` through the canonical table so each field's `Name` comes from the table and `Value` from the canonicalised hex.

**Acceptance Criteria**:
- [ ] `#FFF`, `#FFFFFFFF`, `#GGGGGG`, `` (empty), `#FF FFFF`, `blue`, `212`, `-5`, `16777215` each yield `bad colour` when carried by a **known** key.
- [ ] `text.primary = #c0caf5` produces `Token.Value == "#C0CAF5"`; a file written entirely in lowercase and its uppercase twin produce identical `Theme` values.
- [ ] An unknown key with a malformed value (`legacy.thing = nonsense`) contributes no rejection and appears in no detail; the file validates if the 19 known keys are present and well-formed.
- [ ] `Text.Primary = #FFFFFF` is treated as unknown, so the file fails `missing tokens` with `text.primary` named in the detail.
- [ ] A file that lexed to zero pairs yields `missing tokens` listing all 19 names in §2.4 order.
- [ ] When a file has both a bad colour and a missing token, the reason is `bad colour` and the detail contains no token-presence information.
- [ ] Details are deterministic: bad-colour pairs in file order, missing names in §2.4 order.

**Tests**:
- `"it builds a Theme from nineteen well-formed tokens"` — `TestValidate_AcceptsNineteenWellFormedTokens`
- `"it rejects every malformed hex form"` — `TestValidate_RejectsMalformedHexForms` (table over the nine forms above)
- `"it canonicalises hex values to uppercase"` — `TestValidate_CanonicalisesHexToUppercase`
- `"it ignores an unknown key and never validates its value"` — `TestValidate_IgnoresUnknownKeyAndItsValue`
- `"it treats a wrong-case key as unknown so the file fails missing tokens"` — `TestValidate_WrongCaseKeyFailsAsMissingTokens`
- `"it reports all nineteen names missing for an empty file"` — `TestValidate_EmptyFileMissesAllNineteen`
- `"it enumerates every offending pair within bad colour"` — `TestValidate_BadColourDetailEnumeratesEveryOffendingPair`
- `"it enumerates every absent name within missing tokens"` — `TestValidate_MissingTokensDetailEnumeratesEveryAbsentName`
- `"it reports bad colour rather than missing tokens when both apply"` — `TestValidate_BadColourPrecedesMissingTokens`

**Edge Cases**:
- `#FFF`, `#FFFFFFFF`, `#GGGGGG`, empty value, interior whitespace `#FF FFFF` — all `bad colour`; six digits cost nothing and remove a parse branch.
- Lowercase hex canonicalised to uppercase: §4.3 makes this load-bearing for two later comparison sites (the retained startup canvas hex compared at exit, §11.4; background diffing, §11.3) — a file written `#c0caf5` must not fail to match one written `#C0CAF5`.
- An unknown key's malformed value is **never** validated and the key is ignored entirely — §4.6's forward-compatibility lever requires it: if a removed token's stale line could reject a file on its value, "old files keep working" would only hold for values that happen to still be well-formed hex.
- Wrong-case key `Text.Primary` is unknown, so the file fails `missing tokens` — technically accurate but capable of misdirecting, which is exactly why the detail names the missing tokens.
- Empty / comment-only file → `missing tokens` ("it parsed; it declares nothing").
- `bad colour` enumerates every offending `key = value` pair; `missing tokens` every absent name — doctor "enumerates within the reason, not across reasons".

**Context**:
> §4.3: values are hex only, `#RRGGBB`. No ANSI indices — the MV spec's §2.4 is an explicit decision that Portal imposes its own exact hues via truecolor rather than inheriting the terminal's 16 ANSI colours, and decisively, **an ANSI index has no fixed RGB**, so admitting them would permanently foreclose checking a theme numerically, including Portal's own built-ins.
> §4.4: a theme file contains exactly the 19 token keys and nothing else — no `name` field, no behaviour, no includes, no nesting. Unknown keys are ignored. The security consequence is that Ghostty's "a theme can set any config option" caveat does not transfer.
> §4.5: every theme declares all 19 tokens; there is no merge-over-a-base and no partial file. The Portal-specific hazard is that the canvas is *itself* a token, so a partial theme supplying a new canvas while inheriting `text.primary` produces a foreground measured against a background it was never tuned for.
> §6.1: validity is syntactic, never perceptual — colours are never checked for being good, readable or clearing a contrast floor at load. Floors are Phase 2's bundled-tier concern.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §4.3–§4.6, §6.1, §6.2, §14A

## theming-system-1-4

### Task 1.4: Derive a slug from a filename — charset and extension rules, reject never normalise

**Problem**: The filename minus its extension **is** the theme's identity — the value persisted in `prefs.json`, displayed in the selector and typed at `portal theme export`. That makes the charset and extension rules safety properties rather than cosmetics: lowercasing `Nord.theme` to `nord` would let a user file **shadow the built-in that is the fallback**, so a typo'd drop-in could break the very thing Portal falls back to; and a persisted value like `../something` would otherwise be used verbatim as a path component. There is also no rule today for how `Nord.THEME` should behave, which decides whether a user's file is merely rejected or completely invisible.

**Solution**: `internal/theme/name.go` exposing `ValidSlug(s string) bool` — the anchored `^[a-z0-9][a-z0-9-]*$` rule, exported because §8.6 (a persisted slug) and §12.1 (a CLI argument) reuse it on non-file inputs — and `SlugFromFilename(base string) (string, *Rejection)` applying the exactly-lowercase `.theme` rule, with the two `bad name` causes kept distinguishable.

**Outcome**: A filename either yields a slug or exactly one `bad name` rejection whose cause records *which* of the two rules it broke, so Phase 7 can render §14A's two distinct doctor frames from one reason class. Nothing is ever lowercased, trimmed or otherwise normalised.

**Do**:
- Add `func ValidSlug(s string) bool` implementing `^[a-z0-9][a-z0-9-]*$`: at least one character, first character `[a-z0-9]`, remainder `[a-z0-9-]`. Document the three edges the anchoring closes — the empty slug is illegal (so the empty string stays unambiguously §8.1's *unset* sentinel), a leading hyphen is illegal (it reads as a flag wherever a slug is typed), and a trailing hyphen is legal but pointless. **No length bound.**
- Add `func SlugFromFilename(base string) (string, *Rejection)`: the extension must be **exactly** `.theme` by byte comparison (never `strings.EqualFold`) — otherwise `bad name` with cause "extension"; then the remaining stem must satisfy `ValidSlug` — otherwise `bad name` with cause "slug".
- Extend `Rejection` with a `BadNameCause` field (`BadNameSlug` / `BadNameExtension`, plus the zero value meaning "not a bad-name rejection"), documenting that the reason class is deliberately one (`bad name`) because "the user-facing fact is the same in all three [causes] and the panel row has no width to discriminate" — the cause exists solely so doctor and export can name which.
- Add doc comments stating the reuse contract: `ValidSlug` is the same rule applied to a persisted `prefs.json` slug (§8.6) and to `portal theme export`'s argument (§12.1), where **no extension is involved** — those callers use `ValidSlug` directly, not `SlugFromFilename`.
- State in-source why normalisation is forbidden: lowercasing `Nord.theme` would let it shadow the built-in `nord`, and rejecting rather than normalising is what keeps the reserved-name check **exact string equality** and makes `Nord.theme` beside a built-in `nord` safe on a case-insensitive macOS filesystem.

**Acceptance Criteria**:
- [ ] `nord.theme` → `nord`; `tokyo-night-day.theme` → `tokyo-night-day`; `a.theme` → `a`; `nord-.theme` → `nord-` (trailing hyphen legal).
- [ ] `.theme` (empty stem) → `bad name` with cause slug; `-nord.theme` → `bad name` with cause slug; `nord_lee.theme`, `nord lee.theme`, `Nord.theme` → `bad name` with cause slug.
- [ ] `Nord.THEME`, `nord.Theme`, `nord.THEME` → `bad name` with cause **extension** — distinguishable from the slug cause, because §14A's doctor line for a bad extension deliberately says the slug portion is fine.
- [ ] `nord.theme.bak` and `nord` (no extension) → `bad name` with cause extension.
- [ ] `Nord.theme` never produces the slug `nord` — the returned slug is empty and the rejection is the only result.
- [ ] A 300-character all-legal stem is accepted; there is no length bound anywhere in this file.
- [ ] `ValidSlug` returns false for `""`, `"../evil"`, `"-nord"`, `"Nord"`, `"nord lee"`, `"nord_lee"`, `"nord/evil"` and true for `"nord"`, `"a"`, `"a-b-c"`, `"tokyo-night-day"`, `"n0rd"`, `"nord-"`.

**Tests**:
- `"it accepts a slug matching the anchored charset"` — `TestValidSlug_AcceptsCharsetAndAnchors`
- `"it rejects empty, leading-hyphen, uppercase and path characters"` — `TestValidSlug_RejectsIllegalForms` (table incl. `../evil`)
- `"it imposes no length bound"` — `TestValidSlug_NoLengthBound`
- `"it derives the slug from the stem"` — `TestSlugFromFilename_DerivesStem`
- `"it rejects a non-lowercase extension as bad name"` — `TestSlugFromFilename_RejectsNonLowercaseExtension`
- `"it distinguishes the extension cause from the slug cause"` — `TestSlugFromFilename_CausesAreDistinct`
- `"it never lowercases or normalises a filename"` — `TestSlugFromFilename_NeverNormalisesCase`
- `"it rejects a file named exactly .theme"` — `TestSlugFromFilename_EmptyStemRejected`

**Edge Cases**:
- Empty slug (a file named exactly `.theme`); leading hyphen illegal; trailing hyphen legal; **no length bound** — §9.8/§9.5's truncation is display-only and must not silently become a validity rule.
- `Nord.theme` rejected, never lowercased, so it cannot shadow a built-in — the no-shadowing safety property §5.4 exists to protect.
- `Nord.THEME` / `nord.Theme` → `bad name` on extension casing. Enumeration (task 1.7) matches the extension case-insensitively so the file is *visible*; only the exact lowercase `.theme` is *accepted*, which is what preserves structural uniqueness — a non-exact extension never contributes a slug, so a duplicate slug cannot be minted and no precedence rule or ordering tie-break is needed.
- The two `bad name` causes stay distinguishable for §14A's two doctor frames: `⚠ theme file <filename>: slug must be lowercase letters, digits and hyphens` versus `⚠ theme file <filename>: extension must be lowercase .theme`.
- The charset rule is exposed as a reusable rule for a **non-file** input — a persisted slug (§8.6) and a CLI argument (§12.1), neither of which has an extension.

**Context**:
> §5.1: the filename minus its extension is the slug, and the slug is the durable identity persisted in `prefs.json`. There is no in-file `name` field and no separate display label — two files with distinct slugs could both carry `name = "Nord"`, so labels could collide even though identity could not.
> §5.2: **reject, never normalise.** "Lowercasing `Nord.theme` to `nord` would let it shadow the built-in, breaking the rule §5.4 exists to protect. This removes the case question outright rather than defining case-insensitive matching, so the reserved-name check stays **exact string equality**."
> §5.4: an invalid theme falls back to a built-in, so if a user file could shadow the built-in that is the fallback, **the fallback itself could be broken**. That must be impossible.
> Phase boundary: the reserved-name *check* (task 1.5) ships here with an injected, empty built-in slug set; Phase 2 populates it. §8.6's application of `ValidSlug` to a persisted slug is Phase 5; §12.1's application to a CLI argument is Phase 2.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §5.1–§5.4, §6.2, §14A

## theming-system-1-5

### Task 1.5: The §6.2 reason ladder — one file yields exactly one reason

**Problem**: There are seven reject classes and the panel row has width for exactly one, so a file that is simultaneously duplicate-keyed *and* missing tokens has two defensible answers — and any implementation that reports whichever it happens to notice first makes the panel's single-reason row a choice rather than a fact, and makes doctor and the panel capable of disagreeing. Nothing else in the feature pins this ordering.

**Solution**: A `Loader` in `internal/theme` running §6.2's fixed ladder over one file path — `bad name` → `reserved name` → `unreadable` → `bad syntax` → `bad colour` → `missing tokens` — short-circuiting at the first failure, with the built-in slug set injected (empty in this phase) so the loader never hardcodes reserved names.

**Outcome**: `LoadFile` returns either a slug plus a validated `Theme`, or exactly one `Rejection` whose detail is scoped to that one reason. A duplicate-keyed file that is also missing tokens reports `bad syntax`, and a `bad name` file is never opened at all.

**Do**:
- Create `internal/theme/load.go` with a `Loader` carrying its injected dependencies — the reserved built-in slug set (a `map[string]struct{}` or equivalent, **empty until Phase 2**) and (task 1.8) the event-logger seam. Document that the loader hardcodes no slugs and resolves no paths.
- Implement `func (l Loader) LoadFile(path string) (Result, *Rejection)` where `Result` carries `Slug string` and `Theme Theme`.
- Rung 1 — `bad name`: derive the slug from `filepath.Base(path)` via `SlugFromFilename` **before the file is opened**. Both causes live here, and both mean the file yields no usable slug, which is what lets the next rung assume one exists.
- Rung 2 — `reserved name`: exact string equality of the slug against the injected set, still before any read. Unreachable for a `bad name` file, which has no slug to collide.
- Rung 3 — `unreadable`: `os.ReadFile`; any error becomes `unreadable` carrying the OS error verbatim on the rejection (a dedicated `Err error` field), since §14A's detail for this reason is "the OS error verbatim".
- Rungs 4–6: `lexPairs` (a lexical failure aborts the parse, so no value-level or presence check runs), then `themeFromPairs`, which already sequences `bad colour` before `missing tokens`.
- Declare `not found` in the vocabulary but never produce it here, with a comment recording that it applies only to a persisted slug with no file (§9.4, Phase 5/8) "where there is nothing to check".

**Acceptance Criteria**:
- [ ] A file that is duplicate-keyed **and** missing tokens reports `bad syntax` only, with no missing-token content in its detail.
- [ ] A file that has a bad colour **and** is missing tokens reports `bad colour` only.
- [ ] A `bad name` file that does not exist on disk reports `bad name`, not `unreadable` — proven by passing a path that would fail to read.
- [ ] A file whose slug collides with an injected reserved slug reports `reserved name` even when its contents are perfectly valid, and even when the file is unreadable or absent.
- [ ] With the **empty** reserved set this phase wires, no input can produce `reserved name`.
- [ ] An unreadable file (mode `0000`, and separately a dangling symlink) reports `unreadable` with the OS error preserved verbatim and retrievable from the rejection.
- [ ] A valid file returns its slug and a `Theme` whose 19 values are uppercase-canonical.
- [ ] `LoadFile` never returns both a populated `Result` and a rejection, and `Rejection.Detail` is always scoped to its single reason.
- [ ] `not found` is never returned by `LoadFile` for any input.

**Tests**:
- `"it returns the slug and theme for a valid file"` — `TestLoadFile_ValidThemeReturnsSlugAndTheme`
- `"it short-circuits at the first failing rung"` — `TestLoadFile_LadderShortCircuits` (table: duplicate+missing → `bad syntax`; bad colour+missing → `bad colour`; bad name+unreadable → `bad name`; reserved+bad colour → `reserved name`)
- `"it decides bad name before opening the file"` — `TestLoadFile_BadNameDecidedBeforeOpen`
- `"it decides reserved name from the slug alone"` — `TestLoadFile_ReservedNameDecidedFromSlugAlone`
- `"it never rejects as reserved when the built-in set is empty"` — `TestLoadFile_EmptyReservedSetNeverRejects`
- `"it keeps the OS error verbatim for an unreadable file"` — `TestLoadFile_UnreadableKeepsOSErrorVerbatim` (0000-mode file and dangling symlink)
- `"it never produces not found"` — `TestLoadFile_NotFoundIsOutsideTheLadder`
- `"it scopes the detail to one reason"` — `TestLoadFile_DetailNeverSpansTwoReasons`

**Edge Cases**:
- A duplicate-keyed file that is also missing tokens reports `bad syntax` — "only meaningful if pinned; nothing else asserts that" (§13.6).
- A `bad name` file never reports `unreadable` or any content reason, because the filename is checked before the file is opened.
- `reserved name` is decided from the slug alone before any read, and its built-in slug source is **injected and empty until Phase 2** — so the rung is implemented and tested with a synthetic set, not with real built-in slugs.
- `unreadable` covers **every** read failure, not only permissions — a dangling link, an I/O error, or anything else that stops the bytes arriving — keeping the OS error verbatim because it is the only thing distinguishing a permission denial from a dangling symlink.
- `not found` is declared in the vocabulary but deliberately outside the ladder.
- Detail never spans two reasons — doctor "enumerates within the reason, not across reasons", so it never reports a file as both `bad colour` and `missing tokens`.
- The `0000`-mode test must `chmod` back in `t.Cleanup` and should skip when the suite is running as root, where mode bits do not deny.

**Context**:
> §6.2's ladder, verbatim in order: 1 `bad name` (the **filename** is checked before the file is opened, so a `bad name` file can never also report `unreadable` or anything about its contents); 2 `reserved name` (likewise decided from the slug alone, before any read); 3 `unreadable`; 4 `bad syntax` (lexical failure aborts the parse, so no value-level or presence check runs); 5 `bad colour` (across every **known** key — unknown keys' values are not validated); 6 `missing tokens` (last, on a file that parsed and whose every known value is well-formed).
> §6.1: a theme is valid iff all 19 tokens are present and every value is syntactically well-formed. Validity is what makes a theme **selectable**; an invalid theme is listed but unselectable and anything nominating it falls back.
> §6.3 splits the job by surface: the panel carries the terse reason, doctor the detail, the log the passive forensic trail. This task owns producing the single reason plus its detail; rendering is Phases 7–8.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §6.1–§6.3, §5.4, §13.6, §14A

## theming-system-1-6

### Task 1.6: `themesDirPath` in `cmd/config.go`, with the directory injected into the loader

**Problem**: The themes directory is a new user-facing documented contract (`PORTAL_THEMES_DIR`, printed in `docs/theming.md`), and Portal already owns every other config path in one place. If the loader resolved the path itself, two decided invariants break: the embedded built-in set must stay reachable **with no path at all** (`internal/capture` uses only the built-in lookup, and its no-real-config import guard forbids reaching config), and `portal doctor` / `portal theme export` would inherit config discovery they must not perform. The directory is also mechanically unlike every existing member of the chain — it resolves a *directory*, has no old macOS predecessor to migrate from, and must never be created or seeded.

**Solution**: A `themesDirPath()` function in `cmd/config.go` alongside `prefsFilePath`, resolving `PORTAL_THEMES_DIR` → `XDG_CONFIG_HOME/portal/themes/` → `~/.config/portal/themes/` and returning a path only, plus a dependency guard proving `internal/theme` contains no path-resolution code.

**Outcome**: `cmd` owns the chain, the loader takes the directory as an injected value, and a guard test fails if `internal/theme` ever grows an XDG lookup, a home-dir read or the env-var literal.

**Do**:
- Add `func themesDirPath() (string, error)` to `cmd/config.go`, sited next to `prefsFilePath`, with a doc comment stating it is deliberately **not** a `configFilePath` member: it resolves a directory rather than a file, and there is no one-shot macOS Application Support migration because the directory is new and nothing exists there to move.
- Resolve `PORTAL_THEMES_DIR` first, treating an empty value as unset (`os.Getenv(...) != ""`, matching `configFilePath`'s existing test) so an empty env var falls through rather than resolving to `""`.
- Otherwise `xdg.ConfigBase()` joined with `portal/themes`, propagating `ConfigBase`'s error — which is also the home-dir-resolution failure path. Never call `os.MkdirAll`, never `os.Stat`, never create or seed.
- Leave `configFileComponents` untouched — no entry, no migrate breadcrumb.
- Add `PORTAL_THEMES_DIR` to the poison set in `cmd/testmain_isolation_test.go`'s `TestMain` (`/nonexistent/portal-test-must-isolate-themes`), for the same structural reason the other `PORTAL_*` paths are poisoned: a later phase's cmd body that forgets to isolate must fail loudly rather than read the developer's real `~/.config/portal/themes/`.
- Add `internal/theme/leaf_guard_test.go` modelled on `internal/prefs/leaf_guard_test.go` (a `go list -deps` walk) asserting `internal/theme` does not import `internal/xdg`, plus a cheap source scan asserting the package contains no `PORTAL_THEMES_DIR` literal and no `os.UserHomeDir` call.

**Acceptance Criteria**:
- [ ] `PORTAL_THEMES_DIR=/tmp/x` resolves to `/tmp/x` regardless of `XDG_CONFIG_HOME` or `HOME`.
- [ ] `PORTAL_THEMES_DIR=""` with `XDG_CONFIG_HOME=/tmp/cfg` resolves to `/tmp/cfg/portal/themes` — an empty env var falls through rather than resolving to `""`.
- [ ] `XDG_CONFIG_HOME` unset with `HOME=/tmp/h` resolves to `/tmp/h/.config/portal/themes`.
- [ ] A home-directory resolution failure returns an error, not an empty path or a panic.
- [ ] Calling `themesDirPath()` any number of times creates nothing on disk — the resolved path still does not exist afterwards.
- [ ] `configFileComponents` is unchanged, `themesDirPath` does not route through `configFilePath`, and no migrate breadcrumb is emitted.
- [ ] The guard test fails if `internal/theme` gains an `internal/xdg` import, a `PORTAL_THEMES_DIR` literal or a home-dir read.
- [ ] `cmd`'s `TestMain` poisons `PORTAL_THEMES_DIR`; the new tests set it explicitly with `t.Setenv` and no test in the package uses `t.Parallel()`.

**Tests**:
- `"it returns the env var when set"` — `TestThemesDirPath_EnvVarWins`
- `"it falls through when the env var is empty"` — `TestThemesDirPath_EmptyEnvFallsThrough`
- `"it resolves under XDG_CONFIG_HOME"` — `TestThemesDirPath_XDGConfigHome`
- `"it falls back to ~/.config when XDG_CONFIG_HOME is unset"` — `TestThemesDirPath_HomeFallback`
- `"it propagates a home-directory resolution failure"` — `TestThemesDirPath_HomeResolutionFailurePropagates`
- `"it never creates or seeds the directory"` — `TestThemesDirPath_NeverCreatesDirectory`
- `"it is not a configFilePath member and emits no migrate breadcrumb"` — `TestThemesDirPath_IsNotAConfigFilePathMember` (asserts an absent `configFileComponents` entry and, via a `logtest.Sink`, zero emission)
- `"it keeps path resolution out of internal/theme"` — `TestThemePackage_ResolvesNoPaths`

**Edge Cases**:
- Empty `PORTAL_THEMES_DIR` falls through rather than resolving to `""` — the failure mode of a naive `os.LookupEnv` would be a loader pointed at the process working directory.
- `XDG_CONFIG_HOME` unset → `~/.config`; `xdg.ConfigBase` already owns that fallback and its empty-means-unset tolerance, so do not reimplement it.
- Home-dir resolution failure surfaces as an error.
- It resolves a **directory**, not a file, so it is deliberately not a `configFilePath` member and gains no `configFileComponents` entry.
- No one-shot macOS Application Support migration — the directory is new, so there is nothing to move.
- Never creates or seeds the directory: §5.5 makes an absent directory the common, silent case, and §12.1's published workflow starts with `mkdir -p` precisely because Portal will not do it.
- The loader holds no path-resolution code and takes the directory as an injected value — this is what keeps the embedded set reachable with no path at all and `internal/capture`'s import guard satisfiable.

**Context**:
> §5.5: the chain is **`PORTAL_THEMES_DIR` → `XDG_CONFIG_HOME/portal/themes/` → `~/.config/portal/themes/`**. "The env var is fixed in this specification because it is a user-facing documented contract — `docs/theming.md` has to print it… The `_DIR` suffix (rather than the `_FILE` of `PORTAL_TERMINALS_FILE` and siblings) marks the mechanical difference: this resolves a *directory* where `configFilePath` resolves *files*."
> §3.2: "`cmd/config.go` owns themes-directory path resolution, via a `themesDirPath` alongside `prefsFilePath`… **The loader takes the directory as an injected value and never resolves it**, which is what keeps the embedded set reachable with no path at all."
> Directory-state behaviour (absent → silent; unreadable or a file where a directory belongs → advisory plus a log entry) is task 1.7's, not this task's — `themesDirPath` returns a path and makes no judgement about what is there.
> CLAUDE.md's "Config path resolution" section and the README config table are corrected in Phase 10, not here.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §3.2, §5.5, §12.4, §12.6

## theming-system-1-7

### Task 1.7: Enumerate the themes directory into classified entries

**Problem**: The drop-in route's whole promise is that a file placed in the themes directory is auto-discovered with no registration step — which means a *broken* file must be **present and named** ("there's my theme, it's registered, but it's invalid") rather than silently absent, the state §9.4 exists to prevent. At the same time an absent directory is the common case and must stay completely silent, and the entry rules — symlinked files, symlinked directories, real subdirectories, extension casing — decide whether a user's file is seen at all. None of this can be inferred from a bare `os.ReadDir`.

**Solution**: `internal/theme/enumerate.go` — a method on the `Loader` reading the top level of the **injected** directory, running every candidate through the task 1.5 ladder, and returning one classified entry per candidate plus a distinct verdict for an unusable directory versus an absent one.

**Outcome**: Given a directory, the loader returns entries for every `.theme`-ish file (valid ones carrying a `Theme`, invalid ones carrying their one reason), returns nothing at all for an absent directory, and returns an `unreadable` rejection for a directory that cannot be read or is not a directory.

**Do**:
- Declare `type Entry struct { Path, Filename, Slug string; Theme Theme; Rejection *Rejection }` — one entry per candidate, valid or not, with `Slug` empty exactly when the rejection is `bad name`.
- Implement `func (l Loader) Enumerate(dir string) ([]Entry, *Rejection)`:
  - `os.Stat(dir)` — deliberately `Stat`, not `Lstat`, so a symlinked directory **root** is followed. `os.IsNotExist` → `(nil, nil)`, silently. Not a directory, or any other stat error, or a `ReadDir` failure → `(nil, &Rejection{Reason: ReasonUnreadable, Err: …})`.
  - For each `os.ReadDir` entry, `os.Stat` the joined path to learn what the entry **resolves to**: a directory (real or via symlink) is skipped silently — "what the entry resolves to is what decides, not whether a link is involved". A stat failure is **not** a skip: a dangling symlink must survive to the ladder, where it fails `unreadable`.
  - Candidate filter: `strings.EqualFold(filepath.Ext(name), ".theme")`. Everything else is ignored entirely — no entry, no reason, no log.
  - Run each candidate through `LoadFile`, so a non-lowercase extension lands `bad name` at rung 1 and a dangling symlink `unreadable` at rung 3, with no duplicated rules here.
  - No recursion — top-level only.
- Return entries in `os.ReadDir`'s filename order so results are deterministic; the §9.5 panel sort key (slug-with-filename-fallback, case-insensitive with a byte-wise tie-break) belongs to Phase 8 and is deliberately not applied here.
- Wire the event-logger seam from task 1.8 at this call site: one `theme: rejected` per rejected entry, one `theme: directory unusable` for the unusable-directory verdict, and **nothing** for an absent directory.

**Acceptance Criteria**:
- [ ] An absent directory yields zero entries, a nil rejection, and no log emission.
- [ ] A regular file where a directory belongs, and a directory with mode `0000`, both yield `unreadable` carrying the OS error.
- [ ] A themes directory reached through a symlink to a real directory enumerates normally.
- [ ] `sub/inner.theme` is not enumerated — top level only, no recursion.
- [ ] `link.theme` symlinked to a valid file elsewhere yields a valid entry whose slug derives from the **link** name.
- [ ] `gone.theme`, a dangling symlink, yields an entry with reason `unreadable`.
- [ ] A real subdirectory named `x.theme` and a symlink whose target is a directory named `y.theme` are both absent from the result with no rejection minted.
- [ ] `Nord.THEME` yields an entry with reason `bad name`; `notes.txt`, `README` and `theme` yield no entry at all.
- [ ] A directory holding one valid and two invalid files yields three entries — the valid one carrying a populated `Theme`, each invalid one carrying exactly one reason.

**Tests**:
- `"it is silent for an absent directory"` — `TestEnumerate_AbsentDirectoryIsSilent`
- `"it reports a regular file where a directory belongs as unreadable"` — `TestEnumerate_RegularFileWhereDirectoryBelongs`
- `"it reports an unreadable directory"` — `TestEnumerate_UnreadableDirectory`
- `"it follows a symlinked directory root"` — `TestEnumerate_FollowsSymlinkedRoot`
- `"it enumerates the top level only"` — `TestEnumerate_TopLevelOnly`
- `"it follows symlinked files and derives the slug from the link name"` — `TestEnumerate_FollowsSymlinkedFiles`
- `"it enumerates a dangling symlink then fails it as unreadable"` — `TestEnumerate_DanglingSymlinkIsUnreadable`
- `"it skips directory-valued entries silently"` — `TestEnumerate_SkipsDirectoryValuedEntriesSilently` (table: real directory, symlink-to-directory)
- `"it matches the extension case-insensitively then rejects it as bad name"` — `TestEnumerate_CaseInsensitiveExtensionVisibleThenBadName`
- `"it ignores non-theme files entirely"` — `TestEnumerate_IgnoresNonThemeFiles`
- `"it returns entries for valid and invalid files alike"` — `TestEnumerate_ValidAndInvalidFilesBothProduceEntries`

**Edge Cases**:
- Absent directory is silent — no rows, no error, no log; "zero drop-ins is not an error" and Portal never creates or seeds it.
- Unreadable directory, or a regular file where a directory belongs, reports `unreadable`.
- Top-level only, no subdirectory recursion.
- Symlinked files are followed — "the standard dotfiles shape, and dotfiles users are exactly who hand-authors a theme"; the slug derives from the link name as enumerated.
- A dangling symlink enumerates and then fails to read: reason `unreadable`.
- The resolved directory root may itself be a symlink and **is** followed — not following it "would make every drop-in vanish with no row and no doctor line", the "completely in the dark" state §9.4 exists to prevent.
- A real subdirectory named `x.theme` is skipped silently ("a directory is not a candidate that failed, it is not a candidate at all"), and a symlink whose target is a directory is treated identically — one rule, not two.
- The extension is matched case-insensitively for **enumeration** (so the file is visible) then rejected `bad name` (so it never contributes a slug and no duplicate slug can be minted).
- Non-`.theme` files are ignored entirely.
- The `0000`-mode directory test needs a `chmod` cleanup and should skip when running as root.

**Context**:
> §5.6 is the rule set, verbatim in its bullets. §5.5's directory-state table pins the two states: **absent** is the common case and silent, while **unreadable or a regular file where a directory belongs** is "a genuine misconfiguration" carrying a doctor advisory (Phase 7) and a `theme: directory unusable` log entry (task 1.8).
> §5.5 also fixes the reason: "A theme made unreachable by an unusable directory carries the reason `unreadable`, not `not found`" — `not found` sends the user to check the filename, `unreadable` sends them to check permissions, and permissions is the actual problem.
> §9.4's justification for enumerating everything: "an invalid theme is *present and named*, so the user sees 'there's my theme, it's registered, but it's invalid' rather than being completely in the dark about why it did not appear."
> Phase boundary: this task produces **directory entries only**. The §9.4 union — files ∪ built-ins ∪ persisted slugs resolving to neither, deduped one-slug-one-row behind the `ThemeEnumerator` seam — is Phase 8, and `theme: enumerated`'s `count`/`rejected` attrs belong there. §5.8's re-read-on-every-panel-open cadence is likewise Phase 8; this task supplies the operation it calls.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §5.5–§5.8, §6.2, §9.4, §13.3

## theming-system-1-8

### Task 1.8: The `theme` log component behind an injected logger seam

**Problem**: A TUI launch that rejects a theme must leave a **passive** record — the panel's row is only visible if the panel is opened and doctor must be invoked, so the log is the only trail that exists without the user going looking. But the same loader code is driven by `portal doctor`, `portal theme export` and `capturetool`, which must emit **nothing** (doctor is the run most likely to hit a full reject set, and a diagnosis command writing WARNs about the state it just printed breaks its read-only claim). And enumeration re-reads on every panel open, so an undeduplicated WARN per file per open turns a forensic trail into a running commentary.

**Solution**: An event-logger seam in `internal/theme` wrapping an **injected** `*slog.Logger` and owning per-process dedup state on the instance, with this phase's two events — `theme: rejected` and `theme: directory unusable`, both WARN — emitted from the task 1.7 call sites through it. `log.Discard()` silences the component entirely.

**Outcome**: A TUI-shaped caller gets one WARN per distinct problem per process no matter how many times the directory is enumerated; a diagnose-shaped caller constructed with `log.Discard()` produces zero output; and a test controls dedup state simply by injecting a fresh seam.

**Do**:
- Create `internal/theme/events.go` with `type EventLogger struct` and `func NewEventLogger(l *slog.Logger) *EventLogger`, guarding a nil logger through `log.OrDiscard` at entry (the `spawn` precedent). Hold the dedup set on the instance behind a mutex — several instances are normal (§8.9's concurrent burst processes) and enumeration may be driven from more than one path in a TUI process.
- Implement `Rejected(slug, path string, r *Rejection)` — WARN, message `rejected`; attrs `slug` when one exists else `path`, plus `reason`, plus `token` **only** where the reason names one (`missing tokens`, `bad colour`). Dedup key is `slug`+`reason` where a slug exists and `path`+`reason` where it does not.
- Implement `DirectoryUnusable(path string, r *Rejection)` — WARN, message `directory unusable`; attrs `path` and `reason`; dedup key `path`+`reason`.
- Thread the `*EventLogger` onto the `Loader` (constructor parameter) and emit from `Enumerate`: one `Rejected` per rejected entry, one `DirectoryUnusable` for the unusable-directory verdict, and **nothing whatsoever** for an absent directory.
- Record the closed vocabulary in-source as a comment — attr keys `slug`, `slot`, `reason`, `path`, `token`, `count`, `rejected`; component name `theme` — and note that the component is bound by the caller (`cmd` passes `log.For("theme")` on paths where a theme is *used*, `log.Discard()` on doctor/export/capturetool), which is why the loader emits but never decides.
- Leave the remaining five events unimplemented and name their phases in the same comment: `theme: loaded` and `theme: fallback applied` (Phase 5), `theme: appearance migrated` and `theme: commit failed` (Phase 6), `theme: enumerated` (Phase 8).

**Acceptance Criteria**:
- [ ] `NewEventLogger(log.Discard())` produces zero records for any sequence of calls, including a full reject set.
- [ ] `NewEventLogger(nil)` does not panic and produces zero records.
- [ ] Five successive `Enumerate` calls over the same broken directory emit exactly one `rejected` per distinct slug+reason and exactly one `directory unusable` per path+reason.
- [ ] The same slug reported with a **different** reason emits a second record.
- [ ] A `bad name` file (no slug) dedups on `path`+`reason` and carries a `path` attr, never an empty `slug`.
- [ ] Two separately constructed `EventLogger`s each emit once for the same input — dedup state is per-instance, not package state.
- [ ] An absent directory emits nothing at all.
- [ ] Every record is `WARN` with component `theme`, and every attr key used is drawn from the closed seven.
- [ ] `token` is present for `missing tokens` and `bad colour` and absent for every other reason.
- [ ] Concurrent calls from two goroutines do not race (`go test -race` clean).

**Tests**:
- `"it emits nothing when constructed with the discard logger"` — `TestEventLogger_DiscardSilencesEverything`
- `"it tolerates a nil logger"` — `TestEventLogger_NilLoggerIsSafe`
- `"it dedups a rejection on slug and reason"` — `TestEventLogger_DedupsRejectedOnSlugAndReason`
- `"it dedups on path when the file has no slug"` — `TestEventLogger_DedupsOnPathWhenNoSlug`
- `"it emits twice for the same slug with a different reason"` — `TestEventLogger_SameSlugDifferentReasonEmitsTwice`
- `"it gives a fresh instance fresh dedup state"` — `TestEventLogger_FreshInstanceHasFreshDedupState`
- `"it dedups an unusable directory on path and reason"` — `TestEventLogger_DirectoryUnusableDedupsOnPathAndReason`
- `"it emits nothing for an absent directory"` — `TestEnumerate_AbsentDirectoryEmitsNothing`
- `"it emits rejections at WARN"` — `TestEventLogger_RejectionsAreWarn`
- `"it carries the token attr only where the reason names one"` — `TestEventLogger_TokenAttrOnlyWhereReasonNamesOne`
- `"it uses only the closed attr-key set"` — `TestEventLogger_AttrKeysAreInTheClosedSet`
- `"it is race-free under concurrent emission"` — `TestEventLogger_ConcurrentEmissionIsRaceFree`

**Edge Cases**:
- `log.Discard` silences the component entirely — the diagnose-shaped callers' contract.
- Dedup key is `slug`+`reason` where a slug exists and `path`+`reason` where it does not; without the second half "the class most likely to recur across panel opens is the one class with no dedup key".
- The same slug with a different reason emits twice — dedup is per (key, reason) pair, not per file.
- Repeated enumeration in one process emits once per key; a fresh seam instance carries fresh dedup state, which is how a test controls it.
- An absent directory emits nothing.
- The `token` attr is carried only where the reason names one (`missing tokens`, `bad colour`) — this is that attr's only consumer.
- Rejections are **WARN**, not INFO: "doctor treats them as advisory for *exit-code* purposes, but 'your config did not work' is a warning in a log."

**Context**:
> §12.3: emission is controlled by an injected logger, not by the loader deciding. "The loader takes a logger seam; `cmd` passes a **real** component logger on the paths where a theme is used — TUI construction, the panel, the theme persister — and **`log.Discard`** on `portal doctor`, `portal theme export` and `capturetool`." **The per-process dedup state lives on that injected logger**, so it is shared by every path in a TUI process — which is what §5.5 requires when the construction-time by-name read and the panel's enumeration hit the same condition. "It is not package state in the leaf… and a test controls it by injecting a fresh one."
> §12.3 also fixes that the component "records where a theme is *used*, never where one is *diagnosed*" — doctor and export emit no `theme` event at all, on three compounding grounds: the log exists to be the record that survives without the user looking, doctor would put the largest WARN volume on the surface needing it least, and it keeps doctor's read-only claim literal.
> §15.1/§12.6: the `theme` component is a spec-governed amendment to the closed log-component vocabulary (17 → 18), with `spawn` and `resolve` as direct precedent. CLAUDE.md's logging-section count is corrected in **Phase 10**, not here.
> **Ambiguity flagged**: the spec pins the `token` attr key but not its cardinality when several tokens are missing or several keys carry a bad colour. Render the same comma-separated list §14A uses for the doctor detail into the single `token` attr — this keeps the key single-valued as declared and the line greppable — and record the choice in a source comment so a later phase can revisit it deliberately.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §12.3, §5.5, §6.2, §8.9, §15.1
