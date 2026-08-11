// Package log is the single owner of all logging machinery in portal, wrapping
// the standard library's log/slog.
//
// No *slog.Logger may be constructed outside this package. Consumers bind a
// component-scoped child logger once at package init:
//
//	var logger = log.For("<component>")
//
// The root logger is built in this package's own init, so For never returns nil
// even when called before Init. A logger cached that way picks up a later handler
// swap automatically: the swap lives in a shared indirection, not on the logger.
package log
