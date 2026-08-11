//go:build linux

package portaltest

import "syscall"

func statTimeNanos(st *syscall.Stat_t) (mtime, ctime int64) {
	return st.Mtim.Nano(), st.Ctim.Nano()
}
