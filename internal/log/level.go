package log

import (
	"log/slog"
	"strings"
)

const (
	sourceDefault  = "default"
	sourceEnv      = "env"
	sourceFallback = "fallback"
)

// resolveLevel deliberately does not accept the legacy "warning" alias. raw is
// echoed back verbatim so the invalid-value warning can render the user's input.
func resolveLevel(raw string) (lvl slog.Level, source string, observed string) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return slog.LevelInfo, sourceDefault, raw
	}

	switch normalized {
	case "debug":
		return slog.LevelDebug, sourceEnv, raw
	case "info":
		return slog.LevelInfo, sourceEnv, raw
	case "warn":
		return slog.LevelWarn, sourceEnv, raw
	case "error":
		return slog.LevelError, sourceEnv, raw
	default:
		return slog.LevelInfo, sourceFallback, raw
	}
}

func levelString(lvl slog.Level) string {
	switch lvl {
	case slog.LevelDebug:
		return "debug"
	case slog.LevelInfo:
		return "info"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	default:
		return strings.ToLower(lvl.String())
	}
}
