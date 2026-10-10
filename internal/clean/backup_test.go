package clean

import (
	"regexp"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/preset"
)

func TestBackupIDIsUniqueAndRefSafe(t *testing.T) {
	pattern := regexp.MustCompile(`^[0-9]{8}T[0-9]{6}Z-[0-9a-f]{12}$`)
	seen := make(map[string]struct{}, 128)
	for i := 0; i < 128; i++ {
		id, err := backupID()
		if err != nil {
			t.Fatal(err)
		}
		if !pattern.MatchString(id) {
			t.Fatalf("backup ID %q does not match expected ref-safe format", id)
		}
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate backup ID %q", id)
		}
		seen[id] = struct{}{}
	}
}

func TestPruneBackupDeletesOnlySelectedRecoveryRefs(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.name", "Human")
	git(t, dir, "config", "user.email", "human@example.org")
	git(t, dir, "commit", "--allow-empty", "-m", "Work\n\nCo-authored-by: Claude <noreply@anthropic.com>")
	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	first, _, err := Rewrite(repo, preset.Claude())
	if err != nil {
		t.Fatal(err)
	}
	git(t, dir, "commit", "--allow-empty", "-m", "More\n\nCo-authored-by: Claude <noreply@anthropic.com>")
	second, _, err := Rewrite(repo, preset.Claude())
	if err != nil {
		t.Fatal(err)
	}
	count, err := PruneBackup(repo, first.Backup)
	if err != nil || count != 2 {
		t.Fatalf("prune count=%d err=%v", count, err)
	}
	ids, err := BackupRefs(repo)
	if err != nil || len(ids) != 1 || ids[0] != second.Backup {
		t.Fatalf("remaining=%v err=%v", ids, err)
	}
	if refs, _ := ResultLocalRefs(repo, first.Backup); len(refs) != 0 {
		t.Fatalf("result refs remain: %#v", refs)
	}
	if _, err := PruneBackup(repo, first.Backup); err == nil {
		t.Fatal("missing backup accepted")
	}
	if _, err := PruneBackup(repo, "../main"); err == nil {
		t.Fatal("invalid backup ID accepted")
	}
}
