package cmd

import (
	"strings"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/spf13/cobra"
)

// completionSessionNames builds its own client rather than reaching for the
// context-injected one: completion runs on the bootstrap-exempt __complete
// path, where cmd.Context() carries no client and tmuxClient would panic.
var completionSessionNames = func() []string {
	names, err := tmux.DefaultClient().ListSessionNames()
	if err != nil {
		return nil
	}
	return names
}

// completeSessionNames filters by prefix itself, because cobra does not
// prefix-filter a dynamic completion func's returns, and suppresses file
// completion so the shell never merges paths into the session-name list.
func completeSessionNames(toComplete string) ([]string, cobra.ShellCompDirective) {
	var matches []string
	for _, name := range completionSessionNames() {
		if strings.HasPrefix(name, toComplete) {
			matches = append(matches, name)
		}
	}
	return matches, cobra.ShellCompDirectiveNoFileComp
}

// completionAliasKeys reads only the aliases config file, so it needs no tmux
// client on the bootstrap-exempt __complete path. Any error yields nil, which
// degrades to zero suggestions rather than a failure.
var completionAliasKeys = func() []string {
	store, err := loadAliasStore()
	if err != nil {
		return nil
	}
	return store.Keys()
}

// completeAliasKeys filters by prefix itself and suppresses file completion,
// as completeSessionNames does.
func completeAliasKeys(toComplete string) ([]string, cobra.ShellCompDirective) {
	var matches []string
	for _, key := range completionAliasKeys() {
		if strings.HasPrefix(key, toComplete) {
			matches = append(matches, key)
		}
	}
	return matches, cobra.ShellCompDirectiveNoFileComp
}
