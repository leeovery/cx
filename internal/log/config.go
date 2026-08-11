package log

import (
	"strconv"
	"strings"
)

const defaultRotateSize int64 = 500 * 1024 * 1024

const (
	suffixK int64 = 1024
	suffixM int64 = 1024 * 1024
	suffixG int64 = 1024 * 1024 * 1024
)

// resolveRotateSize parses a base-10 integer with an optional case-insensitive
// K/M/G binary suffix. Zero is rejected along with the malformed cases: a 0-byte
// cap would rotate on every write.
func resolveRotateSize(raw string) (int64, string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultRotateSize, sourceDefault
	}

	digits, multiplier := splitRotateSize(trimmed)
	if digits == "" {
		return defaultRotateSize, sourceFallback
	}

	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || n <= 0 {
		return defaultRotateSize, sourceFallback
	}

	if multiplier > 1 && n > (1<<63-1)/multiplier {
		return defaultRotateSize, sourceFallback
	}

	return n * multiplier, sourceEnv
}

func splitRotateSize(s string) (digits string, multiplier int64) {
	last := s[len(s)-1]
	switch last {
	case 'K', 'k':
		return s[:len(s)-1], suffixK
	case 'M', 'm':
		return s[:len(s)-1], suffixM
	case 'G', 'g':
		return s[:len(s)-1], suffixG
	default:
		return s, 1
	}
}

const defaultRetentionDays = 30

const maxRetentionDays = 365

// resolveRetentionDays accepts an integer in [0, 365], with 0 valid — it means
// "keep only today". The raw value is returned verbatim for the invalid-value
// warning.
func resolveRetentionDays(raw string) (int, string, string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultRetentionDays, sourceDefault, raw
	}

	n, err := strconv.Atoi(trimmed)
	if err != nil || n < 0 || n > maxRetentionDays {
		return defaultRetentionDays, sourceFallback, raw
	}

	return n, sourceEnv, raw
}
