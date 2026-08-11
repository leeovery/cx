//go:build darwin

package portaltest

import "syscall"

func statTimeNanos(st *syscall.Stat_t) (mtime, ctime int64) {
	return st.Mtimespec.Nano(), st.Ctimespec.Nano()
}
