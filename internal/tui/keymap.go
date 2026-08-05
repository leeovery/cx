package tui

// keymapEntry is one declarative binding in a per-page keymap descriptor: a key
// glyph, its action label, a Core flag marking footer membership (the §3.4
// condensed core keys) vs help-only keys (surfaced only in the ? help modal,
// §8.5), and a RightAligned flag for the trailing entry the footer pins to the
// right (the ? help hint).
//
// The descriptor is the single source of truth for the footer + help DISPLAY —
// it drives BOTH the condensed footer (task 2-4) and the per-page ? help modal
// (Phase 3, §8.5/§14.4): the help modal lists every entry, the footer renders
// only the Core entries. It is pure data — no rendering lives here; consumers
// map the glyphs to display forms and lay them out.
//
// SCOPE (important): the descriptor governs the two DISPLAY surfaces ONLY. It
// does NOT govern key DISPATCH — the live per-page Update switch
// (updateSessionList / updateProjectsPage / handlePreviewKey) is a separate
// hand-coded match over tea key codes with no compiler-enforced link back here.
// So a binding change must be made in BOTH places; the descriptor↔dispatch
// correspondence is held instead by the guard tests in
// keymap_dispatch_guard_test.go, which fail if the two silently diverge.
type keymapEntry struct {
	// Key is the key glyph as it appears to the user (e.g. "↑↓", "⏎", "^↑/↓"). It
	// is a display token, not a tea key code — the live dispatch in
	// updateSessionList owns the actual key matching. This is the TERSE footer
	// form: the condensed §3.4 footer ALWAYS reads Key (never HelpKey).
	Key string
	// HelpKey is the longer, glyph-rich key form the ? help modal (§8.5) renders
	// where it diverges from the footer Key form. The footer Key forms are glyphs
	// (nav "↑↓", attach "⏎", preview "␣"); the HelpKey override is nav→"↑/↓" (footer
	// "↑↓" vs help's slashed "↑/↓") and page keeps "^↑/↓". When empty the help modal
	// falls back to Key. The footer NEVER reads this field. It mirrors HelpAction's
	// footer-vs-help split, keeping both forms on the ONE descriptor so the
	// single-source-of-truth-for-DISPLAY contract holds: a binding change updates the
	// footer and the help together. (Dispatch is out of scope — see the type doc.)
	HelpKey string
	// Action is the action label shown beside the glyph (e.g. "attach",
	// "switch view"). It is the TERSE footer form — the condensed §3.4 footer is
	// space-constrained, so the labels are short.
	Action string
	// HelpAction is the longer, friendlier action label the ? help modal (§8.5)
	// shows — the help panel has room for full descriptions ("Open / attach
	// session" vs the footer's "attach"). When empty the help modal falls back to
	// Action, so an entry that needs no longer form sets it once. The footer NEVER
	// reads this field. Keeping both forms on the ONE descriptor preserves the
	// single-source-of-truth-for-DISPLAY contract (§8.5): a binding change updates
	// the footer and the help together, neither hand-authored as a second list.
	// (Dispatch is out of scope — see the type doc.)
	HelpAction string
	// Core marks an entry as a §3.4 footer-core key. Core entries appear in the
	// condensed footer; non-Core entries are help-only (the ? help modal lists
	// both).
	Core bool
	// RightAligned marks the single trailing entry the footer pins to the right
	// (the "? help" hint). At most one entry sets this.
	RightAligned bool
	// Destructive marks an entry whose key glyph renders in state.red in the ? help
	// body (§2.9 red is destructive-only — the kill / delete actions). Modelled
	// structurally so a future non-destructive `d`/`k` binding cannot render red.
	Destructive bool
}

// sessionsKeymap returns the ordered Sessions keymap descriptor (§12.1, post the
// §14 footer revision). It enumerates every Sessions binding exactly once and
// classifies each as a footer-core key or a help-only key.
//
// §14 IS THE AMENDMENT, NOT A REGRESSION (§15.1). The theming feature would
// otherwise be near-invisible — no `--theme`, no `portal theme list`, a silent
// themes directory, and no active-theme indicator when the panel is closed — so
// `t theme` goes in the footer, and three consequences follow:
//
//   - `↑↓ navigate` LOSES its Core flag (§14.1). Arrows in a list are a given, and
//     they are the entry that genuinely deserves non-core status. It is still
//     LISTED — the help body keeps the row, exactly as `^↑/↓` already is.
//   - `t` and `m` are BOTH promoted to Core (§14.1), so both appear in the footer as
//     well as `?` help.
//   - `m`'s FOOTER label shortens to `multi` (§14.3), buying back ~47px against an
//     arithmetic that fits with ~5px spare and no headroom at the reference
//     86-column width. Its HelpAction is untouched (`Multi-select mode`) — the
//     footer-vs-help split this descriptor already models.
//
// Descriptor order stays NAV-FIRST as the §8.5 help reference
// (testdata/vhs/reference/sessions-help-modal-mv.png) established: ↑/↓ → ^↑/↓
// (page) → ⏎ → / → ␣ → s → n → r → k → q → x → t → m, then a right-aligned
// ? help last. ONLY THE TAIL ORDER MOVES — `m` relocates from after `s` to after
// the new `t`, because the footer renders Core entries in DESCRIPTOR order and
// §14.2's row is pinned. The help body's order moves with it, which is the
// amendment §15.1 names rather than a drift to be corrected.
//
// The Core relative order is therefore ⏎ · / · ␣ · s · x · t · m · ?, giving
// §14.2's pinned left cluster (⏎ attach · / filter · ␣ preview · s switch view ·
// x projects · t theme · m multi) plus the right-aligned ? help.
//
// The footer reads the GLYPH Key forms (§3.4): nav "↑↓" (no slash), attach "⏎",
// preview "␣". The help body diverges where its reference frame
// (sessions-help-modal-mv.png) shows the slashed nav/page forms — so nav carries
// a HelpKey override "↑/↓" (footer "↑↓" vs help "↑/↓") and page keeps "^↑/↓".
// enter/space keep their HelpKey "⏎"/"␣", now coinciding with the glyph Key.
//
// Per §12.2 the descriptor carries no vim aliases (h/j/k-nav/l/g/G), no
// PgUp/PgDn/Home/End, and no uppercase bindings; nav is ↑/↓ and paging is
// ^↑/↓ only. ? help is modelled for the footer hint only — the descriptor
// does not bind it (Phase 3 binds the key).
//
// IT STAYS A PURE STATIC FUNCTION. §9.10's `t` block and §7's `m` block are
// applied at the CALL SITE (Model.sessionsHelpKeymap), never here: the keymap
// dispatch guard probes THIS descriptor with detection unwired and `colourless`
// false, so a filter re-homed into it is precisely what breaks descriptor↔dispatch
// parity.
func sessionsKeymap() []keymapEntry {
	return []keymapEntry{
		{Key: "↑↓", HelpKey: "↑/↓", Action: "navigate", HelpAction: "Move selection"},
		{Key: "^↑/↓", Action: "page", HelpAction: "Next / prev page"},
		{Key: "⏎", HelpKey: "⏎", Action: "attach", HelpAction: "Open / attach session", Core: true},
		{Key: "/", Action: "filter", HelpAction: "Filter sessions", Core: true},
		{Key: "␣", HelpKey: "␣", Action: "preview", HelpAction: "Preview scrollback", Core: true},
		{Key: "s", Action: "switch view", HelpAction: "Switch view — flat / project / tag", Core: true},
		{Key: "n", Action: "new in cwd", HelpAction: "New session in cwd"},
		{Key: "r", Action: "rename", HelpAction: "Rename session"},
		{Key: "k", Action: "kill", HelpAction: "Kill session", Destructive: true},
		{Key: "q", Action: "quit", HelpAction: "Quit"},
		{Key: "x", Action: "projects", HelpAction: "Switch to Projects", Core: true},
		{Key: "t", Action: "theme", HelpAction: "Theme picker", Core: true},
		{Key: "m", Action: "multi", HelpAction: "Multi-select mode", Core: true},
		{Key: "?", Action: "help", HelpAction: "Show this help", Core: true, RightAligned: true},
	}
}

// projectsKeymap returns the ordered Projects keymap descriptor (§12.1, post
// §12.2 revision). It enumerates every Projects binding exactly once and
// classifies each as a §6.3 core-footer key or a help-only key, so the SAME
// descriptor drives BOTH the condensed Projects footer (task 3-2) and the
// per-page ? help modal (Phase 3, §8.5) — neither hand-authored. It is the same
// shape as sessionsKeymap, so the shared condensed-footer renderer
// (renderCondensedFooter) drives both pages from one descriptor; this retired
// the legacy projectHelpKeys footer/help source.
//
// Descriptor order follows the SAME nav-first principle as sessionsKeymap (FIX 2
// internal consistency; there is no Projects help reference frame): the
// navigation/paging entries lead, then the page actions, then the right-aligned
// ? help last. The ⏎ Key is already a glyph. As on Sessions the nav entry carries
// a HelpKey override so the footer/help-only nav reads the glyph "↑↓" while the
// help body keeps the slashed "↑/↓" (page stays "^↑/↓").
//
// §14.2 ADDS `t theme` HERE TOO, slotted after `/` so the Core relative order the
// footer reads is ⏎ · x · e · / · t · ?. Theme is a GLOBAL setting (§9.6), so a
// Projects footer without it would make the setting feel page-scoped for no
// reason. Nothing else moves: the Projects nav entry was ALREADY non-core, so
// §14.1's arrows change is a no-op on this page.
//
// Per §12.2 the descriptor carries NO Projects-side s→Sessions alias (x is the
// sole both-directions page toggle), no vim aliases, no PgUp/PgDn/Home/End, and
// no uppercase bindings; nav is ↑/↓ and paging is ^↑/↓ only. ? help is
// modelled for the footer hint only — the descriptor does not bind the key
// (Phase 3 binds it).
//
// LIKE sessionsKeymap IT STAYS A PURE STATIC FUNCTION: §9.10's `t` block is
// applied at the call site (Model.projectsHelpKeymap), so the dispatch guard keeps
// probing the full descriptor.
func projectsKeymap() []keymapEntry {
	return []keymapEntry{
		{Key: "↑↓", HelpKey: "↑/↓", Action: "navigate", HelpAction: "Move selection"},
		{Key: "^↑/↓", Action: "page", HelpAction: "Next / prev page"},
		{Key: "⏎", Action: "new session", HelpAction: "New session from project", Core: true},
		{Key: "x", Action: "sessions", HelpAction: "Switch to Sessions", Core: true},
		{Key: "e", Action: "edit", HelpAction: "Edit project", Core: true},
		{Key: "/", Action: "filter", HelpAction: "Filter projects", Core: true},
		{Key: "t", Action: "theme", HelpAction: "Theme picker", Core: true},
		{Key: "d", Action: "delete", HelpAction: "Delete project", Destructive: true},
		{Key: "n", Action: "new in cwd", HelpAction: "New session in cwd"},
		{Key: "q", Action: "quit", HelpAction: "Quit"},
		{Key: "esc", Action: "back", HelpAction: "Back / quit"},
		{Key: "?", Action: "help", HelpAction: "Show this help", Core: true, RightAligned: true},
	}
}

// themePanelKeymap returns the §9.12 theme-panel keymap scope — the panel's own
// descriptor, beside the two page descriptors, so the panel's keys are NOT a
// second binding list hand-authored inside a bespoke footer. That is the exact
// drift class this file exists to close and the one this codebase has already paid
// for once (the retired projectHelpKeys second binding list).
//
// THE SCOPE IS COMPLETE — all six keys the panel dispatches (§9.12): `↑`/`↓`,
// `Ctrl+↑`/`Ctrl+↓`, `Enter`, `d`, `l`, `Esc`. Phase 9's dispatch guard consumes
// this scope (it lands with the Enter/d/l dispatch it would otherwise probe
// against a no-op), and that guard's contract is descriptor↔dispatch PARITY — so
// authoring the scope as just the four keys the footer shows is precisely what
// BREAKS it rather than what satisfies it. Arrows and paging are dispatched inside
// the panel, so they are listed here.
//
// THE FOOTER RENDERS ONLY THE Core ENTRIES — `⏎`, `d`, `l`, `esc` (§14A's pinned
// four-row footer). Arrows and paging are present as NON-core, which is exactly
// the distinction §14.1 applies to arrows on the main footer and for the same
// reason: arrows in a list are a given. That split is how the six-entry descriptor
// and the four-row footer are BOTH satisfied without either being a special case —
// the descriptor is not trimmed to the footer, and the footer is not filtered by a
// second rule of its own.
//
// The Action strings are §14A's pinned copy, so the rendered rows read exactly
// `⏎ set theme` / `d set as dark` / `l set as light` / `esc close`.
//
// No entry sets RightAligned: the panel's footer is VERTICAL (a horizontal keymap
// does not fit a ~34-column panel, §9.1), and a vertical footer has no right anchor
// to pin an entry to. There is NO `?` entry either — §9.12: `?` does nothing inside
// the panel. It is swallowed with everything else (§9.7); there is no panel help
// modal, so this scope exists to drive the vertical footer and the guard rather
// than a help body, and a help modal over a non-blanking overlay would stack the
// shape §9.6 rejects.
//
// The NESTED CONFIRM SCOPE beneath this one is themePanelConfirmKeymap, whose
// footer temporarily replaces this one while §9.2's slot-from-constant confirm is
// live. That is why renderThemePanelFooter takes its entries as a PARAMETER and
// never calls this function: the shape must admit substitution rather than assume
// one footer per panel.
func themePanelKeymap() []keymapEntry {
	return []keymapEntry{
		{Key: "↑↓", HelpKey: "↑/↓", Action: "navigate", HelpAction: "Move selection"},
		{Key: "^↑/↓", Action: "page", HelpAction: "Next / prev page"},
		{Key: "⏎", Action: "set theme", HelpAction: "Set as the theme", Core: true},
		{Key: "d", Action: "set as dark", HelpAction: "Assign to the dark slot", Core: true},
		{Key: "l", Action: "set as light", HelpAction: "Assign to the light slot", Core: true},
		{Key: "esc", Action: "close", HelpAction: "Close the theme picker", Core: true},
	}
}

// themePanelConfirmKeymap returns §9.2's NESTED CONFIRM SCOPE — the keys the
// slot-from-constant confirm resolves on, as a scope BENEATH the panel scope above.
//
// IT IS A SECOND SCOPE, NOT A LONGER FIRST ONE. §9.12's "all six" is the PANEL
// scope's own membership; the confirm is a nested scope beside it, not a
// sixth-plus-four list. That is what keeps the panel's own footer at §14A's pinned
// four rows while the confirm's substitutes two of its own.
//
// IT EXISTS BECAUSE THE CONFIRM'S FOOTER IS THE SECOND PLACE A KEY LABEL COULD GO
// STALE. While the confirm is live it is key-exclusive within the panel and
// resolves on `y`/`Y`, `n`/`N` or `Esc` alone — so the standing footer would list
// four keys of which NONE would act, and §14.3 is firm that advertising a key that
// will not act is the dead end a proactive block exists to prevent. Authoring the
// substituted footer as a bespoke binding list inside the renderer is exactly the
// drift keymap_dispatch_guard_test exists to close, one layer down; the guard
// consumes THIS scope (the uppercase `Y`/`N` dispatch included, which the descriptor
// does not restate because the glyphs are the terse footer form).
//
// No entry sets RightAligned and there is no `?` entry, for the SAME reasons the
// panel scope has neither: the footer is VERTICAL, so there is no right anchor to
// pin an entry to, and §9.12 gives `?` nothing to do inside the panel — it is
// swallowed with everything else, and a help modal over a non-blanking overlay would
// stack the shape §9.6 rejects.
//
// The Action strings are §14A's terse footer copy; the HelpAction forms say what
// each answer DOES to the setting, which is the distinction the user is being asked
// to make.
func themePanelConfirmKeymap() []keymapEntry {
	return []keymapEntry{
		{Key: "y", Action: "confirm", HelpAction: "Clear the constant and set the slot", Core: true},
		{Key: "n", Action: "cancel", HelpAction: "Keep the constant", Core: true},
	}
}

// commandPendingKeymap returns the §11.4 command-pending footer descriptor:
// `enter run here · n run in cwd · esc cancel`. It is the single binding source for
// the swapped Projects footer (renderCommandPendingFooter renders these in MV
// chrome). Authoring it as a keymapEntry slice folds the command-pending footer into
// the SAME descriptor/entry vocabulary as every other footer — the `enter` glyph is
// encoded declaratively via HelpKey ("⏎", resolved by helpKeyGlyph), retiring the
// former inline enter→⏎ string rewrite that was the one footer source outside the
// vocabulary. The footer Key stays the terse "enter" word (the descriptor convention,
// mirroring sessionsKeymap's enter binding). q quit is deferred to the ? help modal;
// s/x/e/d are suppressed in command-pending, so the descriptor lists only these three
// (it is a footer-copy source, not a help reference, so no Core/RightAligned flags).
func commandPendingKeymap() []keymapEntry {
	return []keymapEntry{
		{Key: "enter", HelpKey: "⏎", Action: "run here"},
		{Key: "n", Action: "run in cwd"},
		{Key: "esc", Action: "cancel"},
	}
}

// previewKeymap returns the ordered §9.3 Preview keymap descriptor — the single
// source of truth that drives the §9.1 full-screen overlay footer (the four nav
// hints) and the per-page ? help reference (§8.5). The descriptor lists the
// COMPLETE Preview keymap (§12.1): the help shows EVERY entry, the footer filters
// to the Core entries. The bindings follow the §9 spatial model (post the §9
// chrome restructure, task 4-6/4-7): window is `←`/`→`, pane is `Tab` (REPLACING
// the former `]`/`[` window + `Ctrl+←`/`Ctrl+→` pane — `Ctrl+←/→` is hijacked by
// macOS Mission Control Spaces switching, so pane reverts to the pre-rebuild `Tab`
// forward-cycle). The footer renders only the Core entries (the four nav hints,
// space-separated); the scroll/page/top/bottom keys are help-only (the footer has
// no room and scrolling is the obvious arrow default).
//
// The top/bottom jumps (`Home`/`End`) are preview-owned (Update's handlePreviewKey
// binds them) so they appear in the help reference even though the §12.2 nav
// revision dropped Home/End from the LIST pages — the preview is a scrollback
// viewport, not a list, and keeps the explicit top/bottom jumps.
//
// Glyphs follow the project key-glyph convention: `←→` (left+right arrows) for the
// window pair, `⇥` (U+21E5 rightwards-arrow-to-bar / Tab) for the pane forward
// cycle, `⏎` (U+23CE) for attach, `␣` (U+2423) for the space-back, and the literal
// `Home`/`End` words for the top/bottom jumps. The footer reads Key directly; no
// HelpKey override is needed since the Core Key forms are already glyphs.
func previewKeymap() []keymapEntry {
	return []keymapEntry{
		{Key: "↑/↓", Action: "scroll", HelpAction: "Scroll up / down"},
		{Key: "^↑/↓", Action: "page", HelpAction: "Page up / down"},
		{Key: "Home/End", Action: "top/bottom", HelpAction: "Jump to top / bottom"},
		{Key: "←→", Action: "window", HelpAction: "Prev / next window", Core: true},
		{Key: "⇥", Action: "pane", HelpAction: "Next pane", Core: true},
		{Key: "⏎", Action: "attach", HelpAction: "Attach this pane", Core: true},
		{Key: "␣", Action: "back", HelpAction: "Back to sessions", Core: true},
	}
}
