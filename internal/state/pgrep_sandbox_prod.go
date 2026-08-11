//go:build !integration

package state

// Inert stubs for every non-integration build, so no test-only pgrep filtering
// can run in the shipped binary. The controls stay exported because
// internal/portaltest is compiled in both build modes.

func sandboxFilterPgrep(pids []int) []int { return pids }

func EnableDaemonSandbox()                           {}
func RegisterSandboxStateDir(string)                 {}
func RegisterSandboxDaemon(int)                      {}
func RegisterSandboxDaemonSource(func() (int, bool)) {}
func ResetDaemonSandbox()                            {}
