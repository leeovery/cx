package state

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
)

const tailChunkSize = 64 * 1024

var openFileForTail = os.Open

// SetOpenFileForTest swaps TailScrollback's file-open seam and returns a func
// restoring the previous one. Test-only.
func SetOpenFileForTest(open func(name string) (*os.File, error)) (restore func()) {
	prev := openFileForTail
	openFileForTail = open
	return func() { openFileForTail = prev }
}

// TailScrollback returns the bytes of at most the last n newline-terminated
// records of the .bin scrollback file at path. The result always ends on '\n';
// a partial record after the final newline is excluded.
//
// Every "no content" outcome converges on (nil, nil): a missing file, an empty
// file, and a file containing no newline at all. Any other open, seek or read
// failure returns (nil, err).
func TailScrollback(path string, n int) ([]byte, error) {
	f, err := openFileForTail(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("tail scrollback %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("tail scrollback %s: %w", path, err)
	}
	if size == 0 {
		return nil, nil
	}

	// cursor is the offset of the next byte not yet read; tail holds the bytes
	// already read, in file order.
	cursor := size
	var tail []byte
	// n+1 newlines pinpoint the start of the n-th-from-last record: n record
	// terminators plus the one immediately before the slice.
	target := n + 1
	chunk := make([]byte, tailChunkSize)
	for cursor > 0 {
		stride := min(int64(tailChunkSize), cursor)
		readAt := cursor - stride
		if _, err := f.Seek(readAt, io.SeekStart); err != nil {
			return nil, fmt.Errorf("tail scrollback %s: %w", path, err)
		}
		buf := chunk[:stride]
		if _, err := io.ReadFull(f, buf); err != nil {
			return nil, fmt.Errorf("tail scrollback %s: %w", path, err)
		}
		merged := make([]byte, len(buf)+len(tail))
		copy(merged, buf)
		copy(merged[len(buf):], tail)
		tail = merged
		cursor = readAt

		if bytes.Count(tail, []byte{'\n'}) >= target {
			// Start after the cut newline to keep n whole records; end on the
			// last newline so a trailing partial record is dropped.
			cut := indexOfNthNewlineFromEnd(tail, target)
			last := bytes.LastIndexByte(tail, '\n')
			return tail[cut+1 : last+1], nil
		}
	}

	last := bytes.LastIndexByte(tail, '\n')
	if last < 0 {
		return nil, nil
	}
	return tail[:last+1], nil
}

// n counts backwards from the end, 1 being the last newline. The caller must
// guarantee buf holds at least n newlines.
func indexOfNthNewlineFromEnd(buf []byte, n int) int {
	seen := 0
	for i, v := range slices.Backward(buf) {
		if v != '\n' {
			continue
		}
		seen++
		if seen == n {
			return i
		}
	}
	return -1 // unreachable given precondition
}
