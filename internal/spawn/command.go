package spawn

// ExecutableResolver resolves the running binary's own path; production callers
// pass os.Executable.
type ExecutableResolver func() (string, error)

// The TMUX/TMUX_PANE strip is load-bearing: a window inheriting them would take
// `open`'s switch-client path instead of a clean out-of-tmux exec-attach.
// exePath is the running binary rather than a PATH lookup, so the version-gated
// warm-command latch stays satisfied and each spawned open takes the abridged
// fast-path.
func composeOpenArgv(exePath, path string, surface Surface, batch, token string, command []string) []string {
	targetFlag := "--session"
	if surface.Kind == SurfaceMint {
		targetFlag = "--path"
	}
	argv := []string{
		"/usr/bin/env", "-u", "TMUX", "-u", "TMUX_PANE",
		"PATH=" + path,
		exePath, "open", targetFlag, surface.Value,
		"--ack", FormatSpawnAckFlag(batch, token),
	}
	if surface.Kind == SurfaceMint && len(command) > 0 {
		argv = append(argv, "--")
		argv = append(argv, command...)
	}
	return argv
}

func AttachSurfaces(names []string) []Surface {
	surfaces := make([]Surface, len(names))
	for i, name := range names {
		surfaces[i] = Surface{Kind: SurfaceAttach, Value: name}
	}
	return surfaces
}
