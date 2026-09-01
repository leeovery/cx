package xdg

import (
	"path/filepath"
	"strings"
)

// Lookup resolves an environment variable name to its value, reporting the
// empty string for one that is not set. It is the seam that lets a single
// precedence rule serve both the process environment and an exec.Cmd-style env
// slice a test is about to hand a subprocess.
type Lookup func(name string) string

// EnvSlice returns a Lookup over an exec.Cmd-style "NAME=value" slice. A later
// entry wins, matching exec.Cmd's own dedupe, so the value it reports is the
// one a subprocess launched with the slice would see.
func EnvSlice(env []string) Lookup {
	return func(name string) string {
		prefix := name + "="
		value := ""
		for _, entry := range env {
			if after, ok := strings.CutPrefix(entry, prefix); ok {
				value = after
			}
		}
		return value
	}
}

// ConfigFile is a resolved config-file location.
type ConfigFile struct {
	// Path is where the file lives under the resolved precedence.
	Path string
	// Overridden reports that Path came from the per-file environment variable
	// rather than from the config base. Callers gate on it what only makes sense
	// for the default location — the one-shot Application Support migration
	// among them: a file the user pointed at explicitly is never moved.
	Overridden bool
}

// ConfigFilePath declares Portal's config-file precedence, once and for every
// reader of it: the per-file environment variable, else
// <config base>/portal/<filename>, where the config base is $XDG_CONFIG_HOME
// and then $HOME/.config. The lookup supplies every environment layer, so a
// caller resolving against a subprocess's env slice resolves by the same rule
// as the subprocess itself — a layer added here reaches both by construction.
//
// It resolves a path and nothing more: it creates nothing, stats nothing and
// migrates nothing. Only the home fallback reads outside the lookup, through
// os.UserHomeDir.
func ConfigFilePath(lookup Lookup, envVar, filename string) (ConfigFile, error) {
	if path := lookup(envVar); path != "" {
		return ConfigFile{Path: path, Overridden: true}, nil
	}

	base, err := ConfigBaseFrom(lookup)
	if err != nil {
		return ConfigFile{}, err
	}

	return ConfigFile{Path: filepath.Join(base, "portal", filename)}, nil
}
