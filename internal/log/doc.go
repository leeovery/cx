// Package log is the single owner of all logging machinery in portal, wrapping
// the standard library's log/slog.
//
// No *slog.Logger may be constructed outside this package. Consumers bind a
// component-scoped child logger once at package init:
//
//	var logger = log.For("<component>")
//
// For never returns nil, even before Init, and a logger cached that way picks up
// a later handler swap: the swap lives in a shared indirection, not on the
// logger.
package log
