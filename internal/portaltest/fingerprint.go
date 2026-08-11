package portaltest

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"syscall"
)

// Larger files are still fingerprinted but not hashed: a content change that
// moves neither size nor mtime nor ctime slips past, an accepted trade against
// hashing arbitrarily large files in every test cleanup.
const hashSizeCap = 1 << 20

// Fingerprint detects mutation of a single filesystem entry. Stats come from
// os.Lstat, so a symlink reports its own metadata rather than its target's;
// Hashed is false for non-regular entries and for files over hashSizeCap.
type Fingerprint struct {
	Exists        bool
	Size          int64
	MtimeNanos    int64
	CtimeNanos    int64
	Sha256        [32]byte
	Hashed        bool
	IsSymlink     bool
	SymlinkTarget string
}

// SnapshotStateDir walks root and returns a fingerprint per entry, keyed by path
// relative to root. A non-existent root yields an empty map and a nil error, so
// anything appearing later counts as created. Symlinks are not descended into.
func SnapshotStateDir(root string) (map[string]Fingerprint, error) {
	out := make(map[string]Fingerprint)

	if _, err := os.Lstat(root); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return out, nil
		}
		return nil, err
	}

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}

		fp, fpErr := fingerprintEntry(path)
		if fpErr != nil {
			return fpErr
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		out[rel] = fp
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return out, nil
}

func fingerprintEntry(path string) (Fingerprint, error) {
	fp := Fingerprint{Exists: true}

	info, err := os.Lstat(path)
	if err != nil {
		return Fingerprint{}, err
	}
	fp.Size = info.Size()
	fp.MtimeNanos, fp.CtimeNanos = statNanos(info)

	mode := info.Mode()
	switch {
	case mode&os.ModeSymlink != 0:
		fp.IsSymlink = true
		target, readErr := os.Readlink(path)
		if readErr != nil {
			return Fingerprint{}, readErr
		}
		fp.SymlinkTarget = target
	case mode.IsRegular() && info.Size() <= hashSizeCap:
		sum, hashErr := hashFile(path)
		if hashErr != nil {
			return Fingerprint{}, hashErr
		}
		fp.Sha256 = sum
		fp.Hashed = true
	}
	return fp, nil
}

func hashFile(path string) ([32]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(data), nil
}

// Narrowed to t.Errorf so a recorder can stand in for a real *testing.T.
type errorReporter func(format string, args ...any)

// FingerprintDelta is a single per-(path, field) change between two snapshots.
// Pre is the zero value for a created path, Post for a deleted one.
type FingerprintDelta struct {
	Path  string
	Field string
	Pre   Fingerprint
	Post  Fingerprint
}

const (
	fieldCreated       = "created"
	fieldDeleted       = "deleted"
	fieldSize          = "size"
	fieldMtime         = "mtime"
	fieldCtime         = "ctime"
	fieldContent       = "content"
	fieldHashed        = "hashed"
	fieldSymlinkTarget = "symlink-target"
	fieldBecameSymlink = "became-symlink"
)

// DiffFingerprints returns the deltas between pre and post, sorted by (Path,
// Field) so diagnostics are reproducible. A path present on one side only yields
// a single created/deleted delta, a type swap a single became-symlink delta.
func DiffFingerprints(pre, post map[string]Fingerprint) []FingerprintDelta {
	paths := unionPaths(pre, post)
	out := make([]FingerprintDelta, 0, len(paths))
	for _, path := range paths {
		before, hadBefore := pre[path]
		after, hasAfter := post[path]
		switch {
		case !hadBefore && hasAfter:
			out = append(out, FingerprintDelta{Path: path, Field: fieldCreated, Post: after})
		case hadBefore && !hasAfter:
			out = append(out, FingerprintDelta{Path: path, Field: fieldDeleted, Pre: before})
		case hadBefore && hasAfter:
			out = append(out, fieldDeltas(path, before, after)...)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Field < out[j].Field
	})
	return out
}

func fieldDeltas(path string, before, after Fingerprint) []FingerprintDelta {
	mk := func(field string) FingerprintDelta {
		return FingerprintDelta{Path: path, Field: field, Pre: before, Post: after}
	}
	if before.IsSymlink != after.IsSymlink {
		return []FingerprintDelta{mk(fieldBecameSymlink)}
	}
	if before.IsSymlink && after.IsSymlink && before.SymlinkTarget != after.SymlinkTarget {
		return []FingerprintDelta{mk(fieldSymlinkTarget)}
	}

	var out []FingerprintDelta
	if before.Size != after.Size {
		out = append(out, mk(fieldSize))
	}
	if before.MtimeNanos != after.MtimeNanos {
		out = append(out, mk(fieldMtime))
	}
	if before.CtimeNanos != after.CtimeNanos {
		out = append(out, mk(fieldCtime))
	}
	if before.Hashed != after.Hashed {
		out = append(out, mk(fieldHashed))
	}
	if before.Hashed && after.Hashed && before.Sha256 != after.Sha256 {
		out = append(out, mk(fieldContent))
	}
	return out
}

func FormatFingerprint(fp Fingerprint) string {
	if fp.IsSymlink {
		return fmt.Sprintf("symlink(target=%q, mtime=%d ns, ctime=%d ns)",
			fp.SymlinkTarget, fp.MtimeNanos, fp.CtimeNanos)
	}
	if fp.Hashed {
		return fmt.Sprintf("file(size=%d, mtime=%d ns, ctime=%d ns, sha256=%x)",
			fp.Size, fp.MtimeNanos, fp.CtimeNanos, fp.Sha256)
	}
	return fmt.Sprintf("entry(size=%d, mtime=%d ns, ctime=%d ns, hashed=false)",
		fp.Size, fp.MtimeNanos, fp.CtimeNanos)
}

func FormatDelta(d FingerprintDelta) string {
	switch d.Field {
	case fieldCreated:
		return fmt.Sprintf("%s: created (post=%s)", d.Path, FormatFingerprint(d.Post))
	case fieldDeleted:
		return fmt.Sprintf("%s: deleted (pre=%s)", d.Path, FormatFingerprint(d.Pre))
	case fieldBecameSymlink:
		return fmt.Sprintf("%s.IsSymlink: pre=%v post=%v", d.Path, d.Pre.IsSymlink, d.Post.IsSymlink)
	case fieldSymlinkTarget:
		return fmt.Sprintf("%s.SymlinkTarget: pre=%q post=%q",
			d.Path, d.Pre.SymlinkTarget, d.Post.SymlinkTarget)
	case fieldSize:
		return fmt.Sprintf("%s.Size: pre=%d post=%d", d.Path, d.Pre.Size, d.Post.Size)
	case fieldMtime:
		return fmt.Sprintf("%s.MtimeNanos: pre=%d post=%d (Δ=%d ns)",
			d.Path, d.Pre.MtimeNanos, d.Post.MtimeNanos, d.Post.MtimeNanos-d.Pre.MtimeNanos)
	case fieldCtime:
		return fmt.Sprintf("%s.CtimeNanos: pre=%d post=%d (Δ=%d ns)",
			d.Path, d.Pre.CtimeNanos, d.Post.CtimeNanos, d.Post.CtimeNanos-d.Pre.CtimeNanos)
	case fieldHashed:
		return fmt.Sprintf("%s.Hashed: pre=%v post=%v", d.Path, d.Pre.Hashed, d.Post.Hashed)
	case fieldContent:
		return fmt.Sprintf("%s.Sha256: pre=%x post=%x", d.Path, d.Pre.Sha256, d.Post.Sha256)
	default:
		return fmt.Sprintf("%s.%s: pre=%+v post=%+v", d.Path, d.Field, d.Pre, d.Post)
	}
}

func reportStateDirDelta(report errorReporter, root string, pre map[string]Fingerprint) {
	post, err := SnapshotStateDir(root)
	if err != nil {
		report("portaltest backstop: post-test snapshot of %s failed: %v", root, err)
		return
	}
	for _, d := range DiffFingerprints(pre, post) {
		report(deltaFmt, d.Path, backstopFieldLabel(d.Field))
	}
}

const deltaFmt = "portaltest backstop: developer state dir mutated at %s: %s"

// Callers match on these exact strings, so the "-changed" suffix must be
// preserved. created / deleted / became-symlink pass through verbatim.
var backstopFieldLabels = map[string]string{
	fieldSize:          "size-changed",
	fieldMtime:         "mtime-changed",
	fieldCtime:         "ctime-changed",
	fieldContent:       "content-changed",
	fieldHashed:        "hashed-changed",
	fieldSymlinkTarget: "symlink-target-changed",
}

func backstopFieldLabel(field string) string {
	if label, ok := backstopFieldLabels[field]; ok {
		return label
	}
	return field
}

func unionPaths(a, b map[string]Fingerprint) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// resolveDevStateDir reads the ambient env, so it must run before any override
// is installed or it resolves the per-test temp dir instead of the developer's
// install. The precedence mirrors internal/xdg, inlined to avoid the dependency.
func resolveDevStateDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "portal", "state")
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "portal", "state")
	}
	return ""
}

// statNanos degrades to (0, 0) on a platform whose FileInfo is not backed by a
// syscall.Stat_t, leaving size and content hash to carry the comparison.
func statNanos(info os.FileInfo) (mtime, ctime int64) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0
	}
	return statTimeNanos(st)
}
