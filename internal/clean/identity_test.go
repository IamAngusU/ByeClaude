package clean

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

func TestIdentityScanFindsGitHubIDOnlyInPullRefs(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "Correct User")
	git(t, dir, "config", "user.email", "110540546+IamAngusU@users.noreply.github.com")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "base.txt")
	git(t, dir, "commit", "-q", "-m", "correct history")
	main := git(t, dir, "rev-parse", "HEAD")

	git(t, dir, "config", "user.name", "IamAngusU")
	git(t, dir, "config", "user.email", "113889733+IamAngusU@users.noreply.github.com")
	if err := os.WriteFile(filepath.Join(dir, "old.txt"), []byte("old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "old.txt")
	git(t, dir, "commit", "-q", "-m", "old pull-ref history")
	old := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "update-ref", "refs/pull/1/head", old)
	git(t, dir, "reset", "-q", "--hard", main)

	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	withoutPull, err := ScanIdentities(repo, IdentityScanOptions{GitHubIDs: []string{"113889733"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(withoutPull.Matches) != 0 {
		t.Fatalf("normal refs unexpectedly contain old identity: %#v", withoutPull.Matches)
	}

	withPull, err := ScanIdentities(repo, IdentityScanOptions{
		GitHubIDs:       []string{"113889733"},
		IncludePullRefs: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(withPull.Matches) != 1 {
		t.Fatalf("matches=%#v", withPull.Matches)
	}
	match := withPull.Matches[0]
	if match.Commit != old || match.GitHubID != "113889733" {
		t.Fatalf("match=%#v", match)
	}
	if match.Managed || !match.PullRefOnly {
		t.Fatalf("expected pull-ref-only unmanaged evidence: %#v", match)
	}
	if len(match.ReachableBy) != 1 || match.ReachableBy[0] != "refs/pull/1/head" {
		t.Fatalf("reachable_by=%v", match.ReachableBy)
	}
	if withPull.PullOnlyMatches != 1 || withPull.ManagedMatches != 0 {
		t.Fatalf("report=%#v", withPull)
	}
}

func TestIdentityScanRejectsNonNumericGitHubID(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "User")
	git(t, dir, "config", "user.email", "user@example.invalid")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-q", "-m", "one")
	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ScanIdentities(repo, IdentityScanOptions{GitHubIDs: []string{"not-a-number"}}); err == nil {
		t.Fatal("expected numeric GitHub ID validation error")
	}
}
