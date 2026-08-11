// Package tmuxerr holds the typed error sentinels shared across the
// internal/tmux and internal/state boundary. internal/tmux already imports
// internal/state, so sentinels wrapped in one and classified in the other live
// here instead.
//
// It must not import any other internal package: that would reintroduce the
// cycle it exists to break.
package tmuxerr
