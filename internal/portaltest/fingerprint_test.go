// The diff logic is driven against a controlled t.TempDir() root — never the
// developer's real state directory — so a bug in the backstop cannot itself
// corrupt the host install.

package portaltest

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type recorder struct {
	msgs []string
}

func (r *recorder) report(format string, args ...any) {
	r.msgs = append(r.msgs, fmt.Sprintf(format, args...))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func hasDelta(msgs []string, path, deltaType string) bool {
	want := fmt.Sprintf(deltaFmt, path, deltaType)
	return slices.Contains(msgs, want)
}

func containsAny(msgs []string, fragment string) bool {
	for _, m := range msgs {
		if strings.Contains(m, fragment) {
			return true
		}
	}
	return false
}

func TestSnapshotStateDir_NonexistentRoot_ReturnsEmptyMap(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent")

	snap, err := SnapshotStateDir(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snap) != 0 {
		t.Errorf("expected empty map, got %d entries", len(snap))
	}
}

func TestSnapshotStateDir_RecordsRegularFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sessions.json"), "alpha")

	snap, err := SnapshotStateDir(root)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	fp, ok := snap["sessions.json"]
	if !ok {
		t.Fatalf("expected sessions.json entry, got keys: %v", keys(snap))
	}
	if fp.Size != 5 {
		t.Errorf("size = %d, want 5", fp.Size)
	}
	if !fp.Hashed {
		t.Errorf("expected hashed=true for small regular file")
	}
}

func TestSnapshotStateDir_RecordsSymlinkViaLstat(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.txt")
	writeFile(t, target, "x")
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	snap, err := SnapshotStateDir(root)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	fp, ok := snap["link"]
	if !ok {
		t.Fatalf("expected link entry; keys: %v", keys(snap))
	}
	if !fp.IsSymlink {
		t.Errorf("link not recorded as symlink")
	}
	if fp.SymlinkTarget != target {
		t.Errorf("symlinkTarget = %q, want %q", fp.SymlinkTarget, target)
	}
	// Hashing a symlink would read through to the target, defeating lstat semantics.
	if fp.Hashed {
		t.Errorf("symlink should not be hashed")
	}
}

func TestSnapshotStateDir_LargeFile_SkipsHash(t *testing.T) {
	root := t.TempDir()
	big := make([]byte, hashSizeCap+1)
	for i := range big {
		big[i] = 'a'
	}
	path := filepath.Join(root, "big.bin")
	if err := os.WriteFile(path, big, 0o600); err != nil {
		t.Fatalf("write big: %v", err)
	}
	snap, err := SnapshotStateDir(root)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	fp := snap["big.bin"]
	if fp.Hashed {
		t.Errorf("expected hashed=false for file >hashSizeCap")
	}
	if fp.Size != int64(hashSizeCap+1) {
		t.Errorf("size = %d, want %d", fp.Size, hashSizeCap+1)
	}
}

func TestSnapshotStateDir_DetectsModifiedBinFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "scrollback", "pane__0.0.bin")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir scrollback: %v", err)
	}
	if err := os.WriteFile(path, []byte("alpha"), 0o600); err != nil {
		t.Fatalf("write .bin: %v", err)
	}

	pre, err := SnapshotStateDir(root)
	if err != nil {
		t.Fatalf("pre snapshot: %v", err)
	}
	prePath := filepath.Join("scrollback", "pane__0.0.bin")
	preFP, ok := pre[prePath]
	if !ok {
		t.Fatalf("expected pre to contain %s; keys=%v", prePath, keys(pre))
	}

	// Size preserved and mtime pinned back, so the hash is the only channel
	// left that can report the change.
	if err := os.WriteFile(path, []byte("betaX"), 0o600); err != nil {
		t.Fatalf("rewrite .bin: %v", err)
	}
	resetTimes(t, path, preFP.MtimeNanos)

	post, err := SnapshotStateDir(root)
	if err != nil {
		t.Fatalf("post snapshot: %v", err)
	}
	postFP, ok := post[prePath]
	if !ok {
		t.Fatalf("expected post to contain %s; keys=%v", prePath, keys(post))
	}

	if preFP.Sha256 == postFP.Sha256 {
		t.Fatalf("SnapshotStateDir returned identical Sha256 across content mutation\n"+
			"  pre.Size=%d post.Size=%d\n"+
			"  pre.Hashed=%v post.Hashed=%v\n"+
			"  pre.Sha256=%x\n"+
			"  post.Sha256=%x",
			preFP.Size, postFP.Size,
			preFP.Hashed, postFP.Hashed,
			preFP.Sha256, postFP.Sha256)
	}
	if !preFP.Hashed || !postFP.Hashed {
		t.Errorf("expected both fingerprints Hashed=true (file ≤ 1 MiB); pre=%v post=%v",
			preFP.Hashed, postFP.Hashed)
	}
}

func TestReportStateDirDelta_NoChange_PassesCleanup(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sessions.json"), "alpha")

	pre, err := SnapshotStateDir(root)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	rec := &recorder{}
	reportStateDirDelta(rec.report, root, pre)

	if len(rec.msgs) != 0 {
		t.Errorf("expected no deltas, got: %v", rec.msgs)
	}
}

func TestReportStateDirDelta_FileCreated_FailsCleanup(t *testing.T) {
	root := t.TempDir()
	pre, _ := SnapshotStateDir(root)

	writeFile(t, filepath.Join(root, "leaked.json"), "leak")

	rec := &recorder{}
	reportStateDirDelta(rec.report, root, pre)

	if !hasDelta(rec.msgs, "leaked.json", "created") {
		t.Errorf("expected 'created' delta for leaked.json; got: %v", rec.msgs)
	}
}

func TestReportStateDirDelta_FileDeleted_FailsCleanup(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sessions.json")
	writeFile(t, path, "alpha")
	pre, _ := SnapshotStateDir(root)

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}

	rec := &recorder{}
	reportStateDirDelta(rec.report, root, pre)
	if !hasDelta(rec.msgs, "sessions.json", "deleted") {
		t.Errorf("expected 'deleted' delta; got: %v", rec.msgs)
	}
}

func TestReportStateDirDelta_SizeChanged_FailsCleanup(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sessions.json")
	writeFile(t, path, "alpha")
	pre, _ := SnapshotStateDir(root)

	writeFile(t, path, "alpha-extended")

	rec := &recorder{}
	reportStateDirDelta(rec.report, root, pre)

	if !hasDelta(rec.msgs, "sessions.json", "size-changed") {
		t.Errorf("expected 'size-changed' delta; got: %v", rec.msgs)
	}
}

func TestReportStateDirDelta_ContentChanged_FailsCleanup(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sessions.json")
	writeFile(t, path, "alpha")
	pre, _ := SnapshotStateDir(root)

	// Same size and a pinned mtime, so content is the only delta left to
	// report; otherwise mtime-changed dominates.
	writeFile(t, path, "betaX")
	preMtime, preCtime := lookupMtimes(t, root, "sessions.json", pre)
	resetTimes(t, path, preMtime)
	pre2, _ := SnapshotStateDir(root)
	if err := os.WriteFile(path, []byte("gamma"), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	resetTimes(t, path, preMtime)
	// ctime cannot be force-set portably; pinning size and mtime is enough.
	_ = preCtime

	rec := &recorder{}
	reportStateDirDelta(rec.report, root, pre2)

	if !hasDelta(rec.msgs, "sessions.json", "content-changed") {
		t.Errorf("expected 'content-changed' delta; got: %v", rec.msgs)
	}
}

func TestReportStateDirDelta_MtimeBumped_FailsCleanup(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sessions.json")
	writeFile(t, path, "alpha")
	pre, _ := SnapshotStateDir(root)

	future := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	rec := &recorder{}
	reportStateDirDelta(rec.report, root, pre)
	if !hasDelta(rec.msgs, "sessions.json", "mtime-changed") {
		t.Errorf("expected 'mtime-changed' delta; got: %v", rec.msgs)
	}
}

func TestReportStateDirDelta_BecameSymlink_FailsCleanup(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sessions.json")
	writeFile(t, path, "alpha")
	pre, _ := SnapshotStateDir(root)

	target := filepath.Join(root, "other.txt")
	writeFile(t, target, "x")
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	rec := &recorder{}
	reportStateDirDelta(rec.report, root, pre)
	if !hasDelta(rec.msgs, "sessions.json", "became-symlink") {
		t.Errorf("expected 'became-symlink' delta; got: %v", rec.msgs)
	}
}

func TestReportStateDirDelta_SymlinkTargetChanged_FailsCleanup(t *testing.T) {
	root := t.TempDir()
	targetA := filepath.Join(root, "a.txt")
	targetB := filepath.Join(root, "b.txt")
	writeFile(t, targetA, "a")
	writeFile(t, targetB, "b")
	link := filepath.Join(root, "link")
	if err := os.Symlink(targetA, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	pre, _ := SnapshotStateDir(root)

	if err := os.Remove(link); err != nil {
		t.Fatalf("remove link: %v", err)
	}
	if err := os.Symlink(targetB, link); err != nil {
		t.Fatalf("re-symlink: %v", err)
	}

	rec := &recorder{}
	reportStateDirDelta(rec.report, root, pre)
	if !hasDelta(rec.msgs, "link", "symlink-target-changed") {
		t.Errorf("expected 'symlink-target-changed' delta; got: %v", rec.msgs)
	}
}

func TestReportStateDirDelta_LargeFile_DetectsSizeWithoutHash(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "big.bin")
	big := make([]byte, hashSizeCap+1)
	if err := os.WriteFile(path, big, 0o600); err != nil {
		t.Fatalf("write big: %v", err)
	}
	pre, _ := SnapshotStateDir(root)
	if pre["big.bin"].Hashed {
		t.Fatalf("large file should not be hashed in pre-snapshot")
	}

	bigger := make([]byte, hashSizeCap+2)
	if err := os.WriteFile(path, bigger, 0o600); err != nil {
		t.Fatalf("rewrite big: %v", err)
	}

	rec := &recorder{}
	reportStateDirDelta(rec.report, root, pre)
	if !hasDelta(rec.msgs, "big.bin", "size-changed") {
		t.Errorf("expected 'size-changed' delta on large file; got: %v", rec.msgs)
	}
}

func TestReportStateDirDelta_ReportsAllDeltas_NotJustFirst(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "keep.json"), "k")
	writeFile(t, filepath.Join(root, "doomed.json"), "d")
	pre, _ := SnapshotStateDir(root)

	if err := os.Remove(filepath.Join(root, "doomed.json")); err != nil {
		t.Fatalf("remove doomed: %v", err)
	}
	writeFile(t, filepath.Join(root, "fresh.json"), "f")
	writeFile(t, filepath.Join(root, "keep.json"), "k-extended")

	rec := &recorder{}
	reportStateDirDelta(rec.report, root, pre)

	if !hasDelta(rec.msgs, "doomed.json", "deleted") {
		t.Errorf("missing 'deleted' for doomed.json; got: %v", rec.msgs)
	}
	if !hasDelta(rec.msgs, "fresh.json", "created") {
		t.Errorf("missing 'created' for fresh.json; got: %v", rec.msgs)
	}
	if !containsAny(rec.msgs, "keep.json") {
		t.Errorf("missing some delta for keep.json; got: %v", rec.msgs)
	}
	if len(rec.msgs) < 3 {
		t.Errorf("expected >=3 deltas reported, got %d: %v", len(rec.msgs), rec.msgs)
	}
}

func TestReportStateDirDelta_WalksOnlyRoot_NotSiblings(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "state")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	writeFile(t, filepath.Join(parent, "projects.json"), "p")

	pre, _ := SnapshotStateDir(root)

	writeFile(t, filepath.Join(parent, "projects.json"), "p-changed-and-bigger")

	rec := &recorder{}
	reportStateDirDelta(rec.report, root, pre)

	if len(rec.msgs) != 0 {
		t.Errorf("expected no deltas (siblings out of scope); got: %v", rec.msgs)
	}
}

func TestReportStateDirDelta_NonexistentRoot_EmptyPreSnapshot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent")

	pre, err := SnapshotStateDir(root)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(pre) != 0 {
		t.Fatalf("expected empty pre-snapshot; got %d entries", len(pre))
	}

	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	writeFile(t, filepath.Join(root, "late.json"), "l")

	rec := &recorder{}
	reportStateDirDelta(rec.report, root, pre)

	if !hasDelta(rec.msgs, "late.json", "created") {
		t.Errorf("expected 'created' for late.json; got: %v", rec.msgs)
	}
}

func TestResolveDevStateDir_UsesXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-fake")
	t.Setenv("HOME", "/tmp/home-fake")

	got := resolveDevStateDir()
	want := filepath.Join("/tmp/xdg-fake", "portal", "state")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveDevStateDir_FallsBackToHomeConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/tmp/home-fake")

	got := resolveDevStateDir()
	want := filepath.Join("/tmp/home-fake", ".config", "portal", "state")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

type fakeBackstopT struct {
	cleanups []func()
	errorfs  []string
}

func (f *fakeBackstopT) Cleanup(fn func()) {
	f.cleanups = append(f.cleanups, fn)
}

func (f *fakeBackstopT) Errorf(format string, args ...any) {
	f.errorfs = append(f.errorfs, fmt.Sprintf(format, args...))
}

// runCleanups runs the registered cleanups LIFO, as testing.T does.
func (f *fakeBackstopT) runCleanups() {
	for _, v := range slices.Backward(f.cleanups) {
		v()
	}
}

func TestBackstopCleanupFiresOnExternalMutation(t *testing.T) {
	devStateDir := t.TempDir()
	pre, err := SnapshotStateDir(devStateDir)
	if err != nil {
		t.Fatalf("pre-snapshot: %v", err)
	}

	fake := &fakeBackstopT{}
	installBackstopCleanup(fake, devStateDir, pre)

	leakPath := filepath.Join(devStateDir, "leaked.json")
	if err := os.WriteFile(leakPath, []byte("leak"), 0o600); err != nil {
		t.Fatalf("write leak: %v", err)
	}

	fake.runCleanups()

	if !hasDelta(fake.errorfs, "leaked.json", "created") {
		t.Errorf("expected backstop to Errorf about leaked.json:created; got: %v", fake.errorfs)
	}
}

func TestBackstopCleanupSilentOnClean(t *testing.T) {
	devStateDir := t.TempDir()
	pre, _ := SnapshotStateDir(devStateDir)

	fake := &fakeBackstopT{}
	installBackstopCleanup(fake, devStateDir, pre)

	fake.runCleanups()

	if len(fake.errorfs) != 0 {
		t.Errorf("expected zero Errorf calls on clean exit; got: %v", fake.errorfs)
	}
}

func keys(m map[string]Fingerprint) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func lookupMtimes(t *testing.T, root, rel string, snap map[string]Fingerprint) (mtime int64, ctime int64) {
	t.Helper()
	fp, ok := snap[rel]
	if !ok {
		t.Fatalf("snap missing %s", rel)
	}
	return fp.MtimeNanos, fp.CtimeNanos
}

func resetTimes(t *testing.T, path string, nanos int64) {
	t.Helper()
	ts := time.Unix(0, nanos)
	if err := os.Chtimes(path, ts, ts); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}
