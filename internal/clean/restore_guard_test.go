package clean

import (
	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/preset"
	"strings"
	"testing"
)

func TestRestoreGuardsLaterWorkAtomically(t *testing.T) {
	for _, move := range []string{"branch", "tag", "legacy"} {
		t.Run(move, func(t *testing.T) {
			dir := t.TempDir()
			git(t, dir, "init", "-q", "-b", "main")
			git(t, dir, "config", "user.name", "Human")
			git(t, dir, "config", "user.email", "human@example.org")
			git(t, dir, "commit", "--allow-empty", "-m", "Work\n\nCo-authored-by: Claude <noreply@anthropic.com>")
			git(t, dir, "tag", "release")
			repo, err := gitx.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			report, _, err := Rewrite(repo, preset.Claude())
			if err != nil {
				t.Fatal(err)
			}
			if move == "legacy" {
				for _, ref := range []string{"heads/main", "tags/release"} {
					git(t, dir, "update-ref", "-d", "refs/byeclaude/results/"+report.Backup+"/"+ref)
				}
			} else {
				git(t, dir, "commit", "--allow-empty", "-m", "Later work")
				if move == "tag" {
					git(t, dir, "tag", "-f", "release")
					git(t, dir, "reset", "--soft", "HEAD^")
				}
			}
			before := git(t, dir, "show-ref")
			if _, err := Restore(repo, report.Backup); err == nil || !strings.Contains(err.Error(), "refusing to discard later work") {
				t.Fatalf("unsafe restore: %v", err)
			}
			if got := git(t, dir, "show-ref"); got != before {
				t.Fatal("failed restore changed refs")
			}
		})
	}
}

func TestRestoreRecreatesDeletedRefsAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.name", "Human")
	git(t, dir, "config", "user.email", "human@example.org")
	git(t, dir, "commit", "--allow-empty", "-m", "Work\n\nCo-authored-by: Claude <noreply@anthropic.com>")
	git(t, dir, "tag", "release")
	original := git(t, dir, "rev-parse", "HEAD")
	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	report, _, err := Rewrite(repo, preset.Claude())
	if err != nil {
		t.Fatal(err)
	}
	git(t, dir, "tag", "-d", "release")
	for i := 0; i < 2; i++ {
		if n, err := Restore(repo, report.Backup); err != nil || n != 2 {
			t.Fatalf("restore: %d %v", n, err)
		}
		if git(t, dir, "rev-parse", "HEAD") != original || git(t, dir, "rev-parse", "release") != original {
			t.Fatal("original refs not restored")
		}
	}
}
