package log

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Rotation keys on the date string, not elapsed duration, so DST 23/25-hour days
// and timezone changes need no special handling.
const dateLayout = "2006-01-02"

var nowFunc = time.Now

var openSegmentFunc = os.OpenFile

// rotatingSink surfaces open/write errors to its caller; the best-effort swallow
// policy lives in textHandler.Handle.
type rotatingSink struct {
	stateDir string

	mu sync.Mutex

	file *os.File
	date string
	dev  uint64
	ino  uint64

	rotateSize int64

	dayRoll func(today string)

	pendingDayRoll string
}

func newRotatingSink(stateDir string, rotateSize int64) *rotatingSink {
	s := &rotatingSink{stateDir: stateDir, rotateSize: rotateSize}
	s.dayRoll = func(today string) {
		sealPastDayFiles(s.stateDir, today)
		runRetentionSweep(s.stateDir, today, true)
	}
	return s
}

func (s *rotatingSink) Write(p []byte) (int, error) {
	n, err := s.lockedWrite(p)
	s.fireDayRoll()
	return n, err
}

func (s *rotatingSink) lockedWrite(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureCurrent(); err != nil {
		return 0, err
	}

	if err := s.rotateIfOverCap(p); err != nil {
		return 0, err
	}

	// Unbuffered by design: the record reaches the kernel before the originating
	// Info(...) returns, so os.Exit / syscall.Exec cannot discard it. Do not wrap
	// this in a bufio.Writer.
	return s.file.Write(p)
}

// fireDayRoll drains the pending roll under mu but runs the callback outside it:
// the sweeps log through this same sink, so firing them under the mutex
// self-deadlocks. The re-entrant record finds nothing pending, ending the loop.
func (s *rotatingSink) fireDayRoll() {
	s.mu.Lock()
	today := s.pendingDayRoll
	s.pendingDayRoll = ""
	s.mu.Unlock()

	if today == "" || s.dayRoll == nil {
		return
	}
	s.dayRoll(today)
}

// rotateIfOverCap must be called with s.mu held, after ensureCurrent. A size that
// cannot be stat'd means "do not rotate", so a transient error cannot corrupt
// the write path.
func (s *rotatingSink) rotateIfOverCap(p []byte) error {
	info, err := s.file.Stat()
	if err != nil {
		return nil
	}
	if info.Size()+int64(len(p)) < s.rotateSize {
		return nil
	}
	return s.rotateSameDay(nowFunc().Format(dateLayout))
}

// rotateSameDay must be called with s.mu held. The previous segment is
// deliberately not chmod'd: a peer process may still hold an open O_APPEND fd on
// it, so same-day segments are sealed only on the day roll.
func (s *rotatingSink) rotateSameDay(today string) error {
	f, n, err := s.claimNextSegment(today)
	if err != nil {
		return err
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	dev, ino, _ := devIno(info)

	_ = swingSymlink(s.stateDir, filepath.Base(daySegmentFile(s.stateDir, today, n)))

	if s.file != nil {
		_ = s.file.Close()
	}
	s.file = f
	s.date = today
	s.dev = dev
	s.ino = ino
	return nil
}

func (s *rotatingSink) claimNextSegment(today string) (*os.File, int, error) {
	for n := s.nextSegmentN(today); ; n++ {
		f, err := openSegmentFunc(daySegmentFile(s.stateDir, today, n), os.O_CREATE|os.O_EXCL|os.O_APPEND|os.O_WRONLY, logFileMode)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return nil, 0, err
		}
		return f, n, nil
	}
}

// nextSegmentN returns max+1 rather than filling a gap, so a claimed-then-removed
// segment number is never reused.
func (s *rotatingSink) nextSegmentN(today string) int {
	matches, err := filepath.Glob(filepath.Join(s.stateDir, portalLogName+"."+today+".*"))
	if err != nil {
		return 1
	}

	max := 0
	prefix := portalLogName + "." + today + "."
	for _, path := range matches {
		rest, found := strings.CutPrefix(filepath.Base(path), prefix)
		if !found {
			continue
		}
		n, err := strconv.Atoi(rest)
		if err != nil || n <= 0 {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max + 1
}

// ensureCurrent must be called with s.mu held.
func (s *rotatingSink) ensureCurrent() error {
	today := nowFunc().Format(dateLayout)

	if s.file != nil {
		dateChanged := s.date != today
		if !dateChanged && s.inodeMatchesSymlink() {
			return nil
		}
		if !dateChanged {
			return s.reopen(today, false)
		}
		return s.reopen(today, true)
	}

	// A fresh process's first write counts as a first-of-day roll: the retention
	// sweep is single-winner and the past-day seal is idempotent.
	return s.reopen(today, true)
}

// inodeMatchesSymlink treats a missing target or any stat error as a mismatch, so
// the caller reopens onto the live file.
func (s *rotatingSink) inodeMatchesSymlink() bool {
	fdInfo, err := s.file.Stat()
	if err != nil {
		return false
	}
	fdDev, fdIno, ok := devIno(fdInfo)
	if !ok {
		return false
	}

	linkInfo, err := os.Stat(symlinkPath(s.stateDir))
	if err != nil {
		return false
	}
	linkDev, linkIno, ok := devIno(linkInfo)
	if !ok {
		return false
	}
	return fdDev == linkDev && fdIno == linkIno
}

func (s *rotatingSink) reopen(today string, dateChanged bool) error {
	// Must precede the swing: it clears a pre-migration regular-file portal.log so
	// the symlink can claim that name. Best-effort, like the swing itself.
	_ = migrationGuard(s.stateDir)

	f, dev, ino, err := openDayFile(s.stateDir, today)
	if err != nil {
		return err
	}

	// Best-effort: a failed swing leaves the prior symlink in place and writes
	// continue on the fresh fd; the next Write's inode check forces a retry.
	target := portalLogName + "." + today
	_ = swingSymlink(s.stateDir, target)

	if s.file != nil {
		_ = s.file.Close()
	}
	s.file = f
	s.date = today
	s.dev = dev
	s.ino = ino

	if dateChanged {
		// Recorded, not fired: reopen holds s.mu and the sweeps log back through
		// this sink. Write drains it via fireDayRoll once the mutex is released.
		s.pendingDayRoll = today
	}
	return nil
}

func openDayFile(stateDir, today string) (*os.File, uint64, uint64, error) {
	// The state dir may not exist yet on first run — Init precedes bootstrap's
	// EnsureDir — so process: start would otherwise land on stderr.
	_ = os.MkdirAll(stateDir, 0o700)

	path := dayFile(stateDir, today)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_APPEND|os.O_WRONLY, logFileMode)
	if errors.Is(err, os.ErrExist) {
		f, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, logFileMode)
	}
	if err != nil {
		return nil, 0, 0, err
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, 0, err
	}
	dev, ino, _ := devIno(info)
	return f, dev, ino, nil
}

// probe eagerly opens today's file so an unwritable stateDir fails at Init rather
// than on the first record. Any first-of-day roll it queues stays pending: probe
// runs before the handler is installed, so the breadcrumbs would hit stderr.
func (s *rotatingSink) probe() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureCurrent()
}

func (s *rotatingSink) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	return err
}

// devIno normalises a file's (Dev, Ino) identity to uint64 for portable
// comparison. A false ok means the identity is unknown and the caller must
// reopen.
func devIno(info os.FileInfo) (dev, ino uint64, ok bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return uint64(st.Dev), uint64(st.Ino), true
}
