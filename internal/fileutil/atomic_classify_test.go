package fileutil_test

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/fileutil"
)

// forceTempCreateFailure returns a path whose parent is a regular file, so the
// directory and temp-file creation cannot succeed.
func forceTempCreateFailure(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	return filepath.Join(blocker, "child", "test.json")
}

func TestAtomicWriteSentinels(t *testing.T) {
	t.Run("wraps a temp-create failure with ErrWriteTempCreate", func(t *testing.T) {
		err := fileutil.AtomicWrite(forceTempCreateFailure(t), []byte("data"))
		if err == nil {
			t.Fatalf("expected error when temp file cannot be created; got nil")
		}
		if !errors.Is(err, fileutil.ErrWriteTempCreate) {
			t.Errorf("errors.Is(err, ErrWriteTempCreate) = false; err = %v", err)
		}
		if got := fileutil.ClassifyWriteError(err); got != "write-failed-temp-create" {
			t.Errorf("ClassifyWriteError = %q, want %q", got, "write-failed-temp-create")
		}
	})

	t.Run("preserves the *os.PathError chain on a temp-create failure", func(t *testing.T) {
		err := fileutil.AtomicWrite(forceTempCreateFailure(t), []byte("data"))
		if err == nil {
			t.Fatalf("expected error; got nil")
		}
		if _, ok := errors.AsType[*os.PathError](err); !ok {
			t.Errorf("errors.AsType[*os.PathError](err) = false; underlying *os.PathError not preserved; err = %v", err)
		}
	})

	t.Run("wraps a rename failure with ErrWriteRename and preserves *os.PathError", func(t *testing.T) {
		// Renaming a file over a non-empty directory fails.
		dir := t.TempDir()
		dest := filepath.Join(dir, "dest")
		if err := os.MkdirAll(filepath.Join(dest, "occupant"), 0o755); err != nil {
			t.Fatalf("seed dest dir: %v", err)
		}

		err := fileutil.AtomicWrite(dest, []byte("data"))
		if err == nil {
			t.Fatalf("expected error renaming temp file onto a non-empty directory; got nil")
		}
		if !errors.Is(err, fileutil.ErrWriteRename) {
			t.Errorf("errors.Is(err, ErrWriteRename) = false; err = %v", err)
		}
		// The wrap must preserve the concrete *os.LinkError, so a caller keeps
		// both paths and the errno.
		if _, ok := errors.AsType[*os.LinkError](err); !ok {
			t.Errorf("errors.AsType[*os.LinkError](err) = false; underlying *os.LinkError not preserved; err = %v", err)
		}
		if got := fileutil.ClassifyWriteError(err); got != "write-failed-rename" {
			t.Errorf("ClassifyWriteError = %q, want %q", got, "write-failed-rename")
		}
	})
}

func TestClassifyWriteError(t *testing.T) {
	someErr := errors.New("underlying")
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "temp-create sentinel",
			err:  fmt.Errorf("%w: %w", fileutil.ErrWriteTempCreate, someErr),
			want: "write-failed-temp-create",
		},
		{
			name: "write sentinel",
			err:  fmt.Errorf("%w: %w", fileutil.ErrWriteWrite, someErr),
			want: "write-failed-write",
		},
		{
			name: "fsync sentinel (Close mapping)",
			err:  fmt.Errorf("%w: %w", fileutil.ErrWriteFsync, someErr),
			want: "write-failed-fsync",
		},
		{
			name: "rename sentinel",
			err:  fmt.Errorf("%w: %w", fileutil.ErrWriteRename, someErr),
			want: "write-failed-rename",
		},
		{
			name: "unrecognised error falls back to the documented safe default",
			err:  errors.New("random"),
			want: "write-failed-write",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fileutil.ClassifyWriteError(tt.err); got != tt.want {
				t.Errorf("ClassifyWriteError = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAtomicWrite0600PreservesSentinel(t *testing.T) {
	err := fileutil.AtomicWrite0600(forceTempCreateFailure(t), []byte("data"))
	if err == nil {
		t.Fatalf("expected error; got nil")
	}
	if !errors.Is(err, fileutil.ErrWriteTempCreate) {
		t.Errorf("errors.Is(err, ErrWriteTempCreate) = false through AtomicWrite0600; err = %v", err)
	}
}

func TestAtomicWriteHasNoLoggingDependency(t *testing.T) {
	// fileutil must stay audit-unaware: no internal/log import anywhere in the
	// package source.
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(path, "internal/log") {
				t.Errorf("%s imports %q; fileutil must stay audit-unaware", name, path)
			}
		}
	}
}
