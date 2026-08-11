// Package spawn is Portal's host-terminal spawn service: it detects the host
// terminal a Portal process runs under, resolves that terminal to a
// window-spawning adapter, and drives the adapter to open new terminal windows.
// Detection yields an Identity — a macOS bundle id plus display name, or a NULL
// identity when there is no host-local terminal (remote/mosh client, unsupported
// terminal, or a transient failure).
package spawn
