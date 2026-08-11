package tmux

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/leeovery/portal/internal/tmuxout"
)

// HookEntry is a single entry parsed from `tmux show-hooks -g` output: the
// array index tmux assigned it, and the command body with any matched outer
// quoting stripped.
type HookEntry struct {
	Index   int
	Command string
}

// Matches one line of `show-hooks -g` output, in either of its two forms:
//
//	<event>[<index>] => <command>
//	<event>[<index>] <command>
var hookLineRegexp = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9-]*)\[(\d+)\](?:\s*=>\s*|\s+)(.*)$`)

// ParseShowHooks parses raw `tmux show-hooks -g` output into a per-event map
// of HookEntry slices, each sorted by ascending Index. Empty input yields a
// non-nil empty map, and unrecognised lines are skipped rather than reported.
func ParseShowHooks(raw string) map[string][]HookEntry {
	out := make(map[string][]HookEntry)

	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		match := hookLineRegexp.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		event := match[1]
		index, err := strconv.Atoi(match[2])
		if err != nil {
			continue
		}
		command := tmuxout.StripMatchedOuterQuotes(match[3])

		out[event] = append(out[event], HookEntry{Index: index, Command: command})
	}

	for event := range out {
		sort.Slice(out[event], func(i, j int) bool {
			return out[event][i].Index < out[event][j].Index
		})
	}

	return out
}
