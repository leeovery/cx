package state

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// ErrCorruptIndex marks sessions.json as present but unusable — malformed,
// unreadable, or an unsupported schema version. An absent file is not wrapped.
var ErrCorruptIndex = errors.New("sessions.json corrupt")

// ReadIndex's bool reports whether the caller should skip restoration: an absent
// file gives (Index{}, true, nil), a present-but-unusable one (Index{}, true,
// err) wrapped with ErrCorruptIndex. It logs nothing; the caller surfaces it.
func ReadIndex(dir string) (Index, bool, error) {
	data, err := os.ReadFile(SessionsJSON(dir))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Index{}, true, nil
		}
		return Index{}, true, fmt.Errorf("read sessions.json: %w: %w", ErrCorruptIndex, err)
	}

	idx, err := DecodeIndex(data)
	if err != nil {
		return Index{}, true, fmt.Errorf("parse sessions.json: %w: %w", ErrCorruptIndex, err)
	}

	return idx, false, nil
}
