package cmd

import (
	"sync"

	"github.com/leeovery/portal/internal/tmux"
)

var versionChecker func(tmux.Commander) error = tmux.CheckTmuxVersion

// versionCheckOnce guards versionCheckErr so concurrent PersistentPreRunE
// invocations cannot race the check.
var versionCheckOnce sync.Once

var versionCheckErr error

func runVersionCheck() error {
	versionCheckOnce.Do(func() {
		versionCheckErr = versionChecker(&tmux.RealCommander{})
	})
	return versionCheckErr
}

func resetVersionCheckForTest() {
	versionCheckOnce = sync.Once{}
	versionCheckErr = nil
}
