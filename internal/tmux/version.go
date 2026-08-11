package tmux

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// tmux 3.0 is the floor for array-indexed global hooks (`set-hook -ga`) and
// the `show-hooks -g` output format Portal parses.
const minTmuxMajor = 3

// ParseTmuxVersion extracts the major and minor numbers from raw `tmux -V`
// output, plus the original version token as a label for user-facing
// messages. A missing minor component is treated as zero; an input with no
// digit-prefixed token returns a descriptive error.
func ParseTmuxVersion(raw string) (major, minor int, label string, err error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, 0, "", errors.New("tmux version string is empty")
	}

	token := findVersionToken(trimmed)
	if token == "" {
		return 0, 0, "", fmt.Errorf("could not parse tmux version from %q", raw)
	}

	major, minor, err = splitMajorMinor(token)
	if err != nil {
		return 0, 0, "", fmt.Errorf("could not parse tmux version from %q: %w", raw, err)
	}
	return major, minor, token, nil
}

func findVersionToken(s string) string {
	for field := range strings.FieldsSeq(s) {
		if field == "" || field[0] < '0' || field[0] > '9' {
			continue
		}
		return field
	}
	return ""
}

func splitMajorMinor(token string) (int, int, error) {
	majorStr, rest := takeDigits(token)
	if majorStr == "" {
		return 0, 0, fmt.Errorf("no major version digit in %q", token)
	}
	major, err := strconv.Atoi(majorStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid major %q: %w", majorStr, err)
	}

	if !strings.HasPrefix(rest, ".") {
		return major, 0, nil
	}

	minorStr, _ := takeDigits(rest[1:])
	if minorStr == "" {
		return major, 0, nil
	}
	minor, err := strconv.Atoi(minorStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid minor %q: %w", minorStr, err)
	}
	return major, minor, nil
}

func takeDigits(s string) (digits, rest string) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return s[:i], s[i:]
}

// CheckTmuxVersion returns nil when the installed tmux meets Portal's minimum,
// and otherwise an error describing the shortfall, the Commander failure or the
// unparseable output.
func CheckTmuxVersion(cmd Commander) error {
	output, err := cmd.Run("-V")
	if err != nil {
		return fmt.Errorf("failed to detect tmux version: %w", err)
	}
	if strings.TrimSpace(output) == "" {
		return errors.New("tmux -V returned no output")
	}

	major, _, label, err := ParseTmuxVersion(output)
	if err != nil {
		return err
	}
	if major < minTmuxMajor {
		return fmt.Errorf("Portal requires tmux \u2265 3.0 (found %s). Please upgrade.", label) //nolint:staticcheck // user-facing message requires capitalization per spec
	}
	return nil
}
