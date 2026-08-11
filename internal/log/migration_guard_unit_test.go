package log

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationGuard_RemovesLegacyRegularFilePortalLog(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(symlinkPath(dir), []byte("legacy log\n"), 0o600); err != nil {
		t.Fatalf("seed regular-file portal.log: %v", err)
	}

	if err := migrationGuard(dir); err != nil {
		t.Fatalf("migrationGuard: %v", err)
	}

	if _, err := os.Lstat(symlinkPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("portal.log still present after guard (lstat err = %v); want removed", err)
	}
}

func TestMigrationGuard_RemovesPortalLogOldAlongsideRegularFile(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(symlinkPath(dir), []byte("legacy log\n"), 0o600); err != nil {
		t.Fatalf("seed regular-file portal.log: %v", err)
	}
	oldPath := filepath.Join(dir, legacyOldName)
	if err := os.WriteFile(oldPath, []byte("legacy old\n"), 0o600); err != nil {
		t.Fatalf("seed portal.log.old: %v", err)
	}

	if err := migrationGuard(dir); err != nil {
		t.Fatalf("migrationGuard: %v", err)
	}

	if _, err := os.Lstat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("portal.log.old still present after guard (lstat err = %v); want removed", err)
	}
}

func TestMigrationGuard_NoOpsWhenPortalLogAlreadySymlink(t *testing.T) {
	dir := t.TempDir()

	const targetName = "portal.log.2026-05-30"
	targetPath := filepath.Join(dir, targetName)
	if err := os.WriteFile(targetPath, []byte("day file\n"), 0o600); err != nil {
		t.Fatalf("seed day file: %v", err)
	}
	if err := os.Symlink(targetName, symlinkPath(dir)); err != nil {
		t.Fatalf("seed symlink: %v", err)
	}

	if err := migrationGuard(dir); err != nil {
		t.Fatalf("migrationGuard: %v", err)
	}

	got, err := os.Readlink(symlinkPath(dir))
	if err != nil {
		t.Fatalf("readlink after guard: %v", err)
	}
	if got != targetName {
		t.Errorf("symlink target = %q after guard, want %q (untouched)", got, targetName)
	}
	b, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target after guard: %v", err)
	}
	if string(b) != "day file\n" {
		t.Errorf("target contents = %q after guard, want %q (untouched)", string(b), "day file\n")
	}
}

func TestMigrationGuard_ToleratesAbsentPortalLogOld(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(symlinkPath(dir), []byte("legacy log\n"), 0o600); err != nil {
		t.Fatalf("seed regular-file portal.log: %v", err)
	}

	if err := migrationGuard(dir); err != nil {
		t.Fatalf("migrationGuard with absent portal.log.old: %v", err)
	}

	if _, err := os.Lstat(symlinkPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("portal.log still present after guard (lstat err = %v); want removed", err)
	}
}

func TestMigrationGuard_NoOpsWhenPortalLogAbsentEntirely(t *testing.T) {
	dir := t.TempDir()

	if err := migrationGuard(dir); err != nil {
		t.Fatalf("migrationGuard on empty dir: %v", err)
	}

	if _, err := os.Lstat(symlinkPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("portal.log unexpectedly present after guard on empty dir (lstat err = %v)", err)
	}
}

func TestMigrationGuard_DoesNotFireOnSecondRunAfterSymlinkExists(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(symlinkPath(dir), []byte("legacy log\n"), 0o600); err != nil {
		t.Fatalf("seed regular-file portal.log: %v", err)
	}
	oldPath := filepath.Join(dir, legacyOldName)
	if err := os.WriteFile(oldPath, []byte("legacy old\n"), 0o600); err != nil {
		t.Fatalf("seed portal.log.old: %v", err)
	}

	if err := migrationGuard(dir); err != nil {
		t.Fatalf("first migrationGuard: %v", err)
	}

	const targetName = "portal.log.2026-05-30"
	targetPath := filepath.Join(dir, targetName)
	if err := os.WriteFile(targetPath, []byte("day file\n"), 0o600); err != nil {
		t.Fatalf("seed day file: %v", err)
	}
	if err := os.Symlink(targetName, symlinkPath(dir)); err != nil {
		t.Fatalf("seed symlink after first guard: %v", err)
	}
	if err := os.WriteFile(oldPath, []byte("new old\n"), 0o600); err != nil {
		t.Fatalf("re-seed portal.log.old: %v", err)
	}

	if err := migrationGuard(dir); err != nil {
		t.Fatalf("second migrationGuard: %v", err)
	}

	got, err := os.Readlink(symlinkPath(dir))
	if err != nil {
		t.Fatalf("readlink after second guard: %v", err)
	}
	if got != targetName {
		t.Errorf("symlink target = %q after second guard, want %q (untouched)", got, targetName)
	}
	if _, err := os.Lstat(oldPath); err != nil {
		t.Errorf("portal.log.old removed by second-run guard (lstat err = %v); want left intact", err)
	}
}
