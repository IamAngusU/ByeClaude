package clean

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestRewritePreservesTreesAndHumanTrailer(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "Angus Test")
	git(t, dir, "config", "user.email", "angus@example.com")

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("one\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "file.txt")
	git(t, dir, "commit", "-q", "-m", "initial")

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("one\ntwo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "file.txt")
	git(t, dir, "commit", "-q", "-m", "feat: second\n\nCo-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>")
	git(t, dir, "tag", "-a", "v0.1.0", "-m", "fixture tag")

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("one\ntwo\nthree\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "file.txt")
	git(t, dir, "commit", "-q", "-m", "feat: third\n\nCo-authored-by: Human <human@example.com>")

	oldHead := git(t, dir, "rev-parse", "HEAD")
	oldTree := git(t, dir, "rev-parse", "HEAD^{tree}")
	oldTag := git(t, dir, "rev-parse", "refs/tags/v0.1.0")

	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	before, err := Scan(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Matches) != 1 {
		t.Fatalf("matches before rewrite = %d", len(before.Matches))
	}

	report, _, err := Rewrite(repo)
	if err != nil {
		t.Fatal(err)
	}
	if report.CommitsRewritten != 2 {
		t.Fatalf("rewritten commits = %d, want 2", report.CommitsRewritten)
	}
	if report.TagsRewritten != 1 {
		t.Fatalf("rewritten tags = %d, want 1", report.TagsRewritten)
	}

	newHead := git(t, dir, "rev-parse", "HEAD")
	newTree := git(t, dir, "rev-parse", "HEAD^{tree}")
	newTag := git(t, dir, "rev-parse", "refs/tags/v0.1.0")
	if newHead == oldHead {
		t.Fatal("HEAD did not change")
	}
	if newTree != oldTree {
		t.Fatalf("tree changed: %s -> %s", oldTree, newTree)
	}
	if newTag == oldTag {
		t.Fatal("annotated tag object did not change")
	}

	after, err := Scan(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Matches) != 0 {
		t.Fatalf("matches after rewrite = %d", len(after.Matches))
	}
	log := git(t, dir, "log", "-1", "--format=%B")
	if !strings.Contains(log, "Human <human@example.com>") {
		t.Fatalf("human co-author trailer missing: %q", log)
	}

	backups, err := BackupRefs(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 || backups[0] != report.Backup {
		t.Fatalf("backups=%v report=%s", backups, report.Backup)
	}
	if _, err := Restore(repo, report.Backup); err != nil {
		t.Fatal(err)
	}
	if got := git(t, dir, "rev-parse", "HEAD"); got != oldHead {
		t.Fatalf("restore HEAD = %s, want %s", got, oldHead)
	}
	if got := git(t, dir, "rev-parse", "refs/tags/v0.1.0"); got != oldTag {
		t.Fatalf("restore tag = %s, want %s", got, oldTag)
	}
}

func TestRewriteRefusesDirtyWorktree(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "Angus Test")
	git(t, dir, "config", "user.email", "angus@example.com")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-q", "-m", "feat\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("dirty\n"), 0644); err != nil {
		t.Fatal(err)
	}
	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Rewrite(repo); err == nil || !strings.Contains(err.Error(), "working tree is not clean") {
		t.Fatalf("expected dirty-worktree error, got %v", err)
	}
}

func TestScanWalksAllLocalBranchesAndTags(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "Angus Test")
	git(t, dir, "config", "user.email", "angus@example.com")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "base.txt")
	git(t, dir, "commit", "-q", "-m", "base")
	baseBranch := git(t, dir, "branch", "--show-current")
	git(t, dir, "checkout", "-q", "-b", "old-work")
	if err := os.WriteFile(filepath.Join(dir, "old.txt"), []byte("old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "old.txt")
	git(t, dir, "commit", "-q", "-m", "old work\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	oldCommit := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "tag", "only-tagged", oldCommit)
	git(t, dir, "checkout", "-q", baseBranch)

	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Scan(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Matches) != 1 || report.Matches[0].Commit != oldCommit {
		t.Fatalf("matches=%#v want old branch commit %s", report.Matches, oldCommit)
	}
}

func TestScanIgnoresClaudeLookingBodyText(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "Angus Test")
	git(t, dir, "config", "user.email", "angus@example.com")
	if err := os.WriteFile(filepath.Join(dir, "docs.txt"), []byte("docs\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "docs.txt")
	git(t, dir, "commit", "-q", "-m", "docs", "-m", "Example:\nCo-Authored-By: Claude <noreply@anthropic.com>\nThis is explanatory body text.")
	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Scan(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Matches) != 0 {
		t.Fatalf("false positive matches: %#v", report.Matches)
	}
}

func TestRewriteRefusesShallowRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	source := t.TempDir()
	git(t, source, "init", "-q")
	git(t, source, "config", "user.name", "Angus Test")
	git(t, source, "config", "user.email", "angus@example.com")
	for i := 0; i < 2; i++ {
		if err := os.WriteFile(filepath.Join(source, "a.txt"), []byte(strings.Repeat("a", i+1)), 0644); err != nil {
			t.Fatal(err)
		}
		git(t, source, "add", "a.txt")
		git(t, source, "commit", "-q", "-m", "commit")
	}
	clone := filepath.Join(t.TempDir(), "clone")
	cmd := exec.Command("git", "clone", "-q", "--depth=1", "file://"+source, clone)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("shallow clone: %v\n%s", err, out)
	}
	repo, err := gitx.Open(clone)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Rewrite(repo); err == nil || !strings.Contains(err.Error(), "shallow repository") {
		t.Fatalf("expected shallow-repository error, got %v", err)
	}
}

func TestRewriteRefusesDetachedHead(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "Angus Test")
	git(t, dir, "config", "user.email", "angus@example.com")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-q", "-m", "feat\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	git(t, dir, "checkout", "-q", "--detach")
	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Rewrite(repo); err == nil || !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("expected detached-HEAD error, got %v", err)
	}
}

func TestRewriteRefusesReplaceRefs(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "Angus Test")
	git(t, dir, "config", "user.email", "angus@example.com")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-q", "-m", "one")
	one := git(t, dir, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("b\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-q", "-m", "two\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	two := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "replace", two, one)
	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Rewrite(repo); err == nil || !strings.Contains(err.Error(), "replace refs") {
		t.Fatalf("expected replace-ref error, got %v", err)
	}
}

func TestRewriteRefusesMultipleWorktrees(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "Angus Test")
	git(t, dir, "config", "user.email", "angus@example.com")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-q", "-m", "feat\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	other := filepath.Join(t.TempDir(), "other")
	git(t, dir, "worktree", "add", "-q", "-b", "other", other)
	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Rewrite(repo); err == nil || !strings.Contains(err.Error(), "multiple linked worktrees") {
		t.Fatalf("expected linked-worktree error, got %v", err)
	}
}

func TestPushUsesLeaseAndUpdatesRemote(t *testing.T) {
	remote := filepath.Join(t.TempDir(), "remote.git")
	git(t, t.TempDir(), "init", "--bare", "-q", remote)
	local := t.TempDir()
	git(t, local, "init", "-q")
	git(t, local, "config", "user.name", "Angus Test")
	git(t, local, "config", "user.email", "angus@example.com")
	git(t, local, "remote", "add", "origin", remote)
	if err := os.WriteFile(filepath.Join(local, "a.txt"), []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, local, "add", "a.txt")
	git(t, local, "commit", "-q", "-m", "feat\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	branch := git(t, local, "branch", "--show-current")
	git(t, local, "push", "-q", "-u", "origin", branch)

	repo, err := gitx.Open(local)
	if err != nil {
		t.Fatal(err)
	}
	oldRefs, err := LocalRefs(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Rewrite(repo); err != nil {
		t.Fatal(err)
	}
	if err := Push(repo, "origin", oldRefs); err != nil {
		t.Fatal(err)
	}

	remoteMsg := git(t, remote, "show", "refs/heads/"+branch, "--format=%B", "--no-patch")
	if strings.Contains(strings.ToLower(remoteMsg), "co-authored-by: claude") {
		t.Fatalf("remote still contains Claude trailer: %q", remoteMsg)
	}
}

func TestPushLeaseRejectsRemoteMovement(t *testing.T) {
	remote := filepath.Join(t.TempDir(), "remote.git")
	git(t, t.TempDir(), "init", "--bare", "-q", remote)
	local := t.TempDir()
	git(t, local, "init", "-q")
	git(t, local, "config", "user.name", "Angus Test")
	git(t, local, "config", "user.email", "angus@example.com")
	git(t, local, "remote", "add", "origin", remote)
	if err := os.WriteFile(filepath.Join(local, "a.txt"), []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, local, "add", "a.txt")
	git(t, local, "commit", "-q", "-m", "feat\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	branch := git(t, local, "branch", "--show-current")
	git(t, local, "push", "-q", "-u", "origin", branch)

	other := filepath.Join(t.TempDir(), "other")
	git(t, t.TempDir(), "clone", "-q", remote, other)
	git(t, other, "config", "user.name", "Other")
	git(t, other, "config", "user.email", "other@example.com")
	if err := os.WriteFile(filepath.Join(other, "remote.txt"), []byte("remote\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, other, "add", "remote.txt")
	git(t, other, "commit", "-q", "-m", "remote moved")
	git(t, other, "push", "-q", "origin", branch)

	repo, err := gitx.Open(local)
	if err != nil {
		t.Fatal(err)
	}
	oldRefs, err := LocalRefs(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Rewrite(repo); err != nil {
		t.Fatal(err)
	}
	if err := Push(repo, "origin", oldRefs); err == nil {
		t.Fatal("expected force-with-lease rejection after remote moved")
	}
}

func TestScanIncludesDetachedHead(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "Angus Test")
	git(t, dir, "config", "user.email", "angus@example.com")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("base\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-q", "-m", "base")
	git(t, dir, "checkout", "-q", "--detach")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("detached\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-q", "-m", "detached\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	detached := git(t, dir, "rev-parse", "HEAD")

	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Scan(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Matches) != 1 || report.Matches[0].Commit != detached {
		t.Fatalf("matches=%#v want detached HEAD %s", report.Matches, detached)
	}
}

func TestScanCanIncludeRemoteTrackingRefs(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "Angus Test")
	git(t, dir, "config", "user.email", "angus@example.com")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("base\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-q", "-m", "base")
	baseBranch := git(t, dir, "branch", "--show-current")
	git(t, dir, "checkout", "-q", "-b", "remote-only")
	if err := os.WriteFile(filepath.Join(dir, "r.txt"), []byte("remote\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "r.txt")
	git(t, dir, "commit", "-q", "-m", "remote history\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	remoteCommit := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "update-ref", "refs/remotes/origin/remote-only", remoteCommit)
	git(t, dir, "checkout", "-q", baseBranch)
	git(t, dir, "branch", "-D", "remote-only")

	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	localOnly, err := Scan(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(localOnly.Matches) != 0 {
		t.Fatalf("local scan unexpectedly saw remote-only history: %#v", localOnly.Matches)
	}
	withRemotes, err := ScanIncludingRemotes(repo, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(withRemotes.Matches) != 1 || withRemotes.Matches[0].Commit != remoteCommit {
		t.Fatalf("matches=%#v want remote commit %s", withRemotes.Matches, remoteCommit)
	}
}

func TestPushIsAtomicAcrossRefs(t *testing.T) {
	remote := filepath.Join(t.TempDir(), "remote.git")
	git(t, t.TempDir(), "init", "--bare", "-q", remote)
	local := t.TempDir()
	git(t, local, "init", "-q")
	git(t, local, "config", "user.name", "Angus Test")
	git(t, local, "config", "user.email", "angus@example.com")
	git(t, local, "remote", "add", "origin", remote)
	if err := os.WriteFile(filepath.Join(local, "a.txt"), []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, local, "add", "a.txt")
	git(t, local, "commit", "-q", "-m", "feat\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	branch := git(t, local, "branch", "--show-current")
	git(t, local, "tag", "v0-test")
	git(t, local, "push", "-q", "origin", branch, "refs/tags/v0-test")
	oldRemoteBranch := git(t, remote, "rev-parse", "refs/heads/"+branch)

	updateHook := filepath.Join(remote, "hooks", "update")
	hookBody := "#!/bin/sh\nif [ \"$1\" = \"refs/tags/v0-test\" ]; then exit 1; fi\nexit 0\n"
	if err := os.WriteFile(updateHook, []byte(hookBody), 0755); err != nil {
		t.Fatal(err)
	}

	repo, err := gitx.Open(local)
	if err != nil {
		t.Fatal(err)
	}
	oldRefs, err := LocalRefs(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Rewrite(repo); err != nil {
		t.Fatal(err)
	}
	if err := Push(repo, "origin", oldRefs); err == nil {
		t.Fatal("expected remote tag rejection")
	}
	if got := git(t, remote, "rev-parse", "refs/heads/"+branch); got != oldRemoteBranch {
		t.Fatalf("branch partially updated despite atomic push: %s -> %s", oldRemoteBranch, got)
	}
}

func TestPushDoesNotPublishLocalOnlyRefs(t *testing.T) {
	remote := filepath.Join(t.TempDir(), "remote.git")
	git(t, t.TempDir(), "init", "--bare", "-q", remote)
	local := t.TempDir()
	git(t, local, "init", "-q")
	git(t, local, "config", "user.name", "Angus Test")
	git(t, local, "config", "user.email", "angus@example.com")
	git(t, local, "remote", "add", "origin", remote)
	if err := os.WriteFile(filepath.Join(local, "main.txt"), []byte("main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, local, "add", "main.txt")
	git(t, local, "commit", "-q", "-m", "main")
	mainBranch := git(t, local, "branch", "--show-current")
	git(t, local, "push", "-q", "-u", "origin", mainBranch)
	git(t, local, "checkout", "-q", "-b", "local-only")
	if err := os.WriteFile(filepath.Join(local, "local.txt"), []byte("local\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, local, "add", "local.txt")
	git(t, local, "commit", "-q", "-m", "local\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	git(t, local, "checkout", "-q", mainBranch)

	repo, err := gitx.Open(local)
	if err != nil {
		t.Fatal(err)
	}
	oldRefs, err := LocalRefs(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Rewrite(repo); err != nil {
		t.Fatal(err)
	}
	if err := Push(repo, "origin", oldRefs); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/local-only")
	cmd.Dir = remote
	if err := cmd.Run(); err == nil {
		t.Fatal("local-only branch was unexpectedly published to remote")
	}
}

func TestRewriteRefusesGitNotes(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "Angus Test")
	git(t, dir, "config", "user.email", "angus@example.com")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-q", "-m", "feat\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	git(t, dir, "notes", "add", "-m", "important note")
	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Rewrite(repo); err == nil || !strings.Contains(err.Error(), "git notes") {
		t.Fatalf("expected git-notes error, got %v", err)
	}
}

func TestRewritePreservesMergeTopologyAndTree(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "Angus Test")
	git(t, dir, "config", "user.email", "angus@example.com")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "base.txt")
	git(t, dir, "commit", "-q", "-m", "base")
	mainBranch := git(t, dir, "branch", "--show-current")
	git(t, dir, "checkout", "-q", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "feature.txt")
	git(t, dir, "commit", "-q", "-m", "feature\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	oldFeature := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "checkout", "-q", mainBranch)
	if err := os.WriteFile(filepath.Join(dir, "main.txt"), []byte("main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "main.txt")
	git(t, dir, "commit", "-q", "-m", "main work")
	oldMainParent := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "merge", "-q", "--no-ff", "feature", "-m", "merge feature")
	oldMerge := git(t, dir, "rev-parse", "HEAD")
	oldTree := git(t, dir, "rev-parse", "HEAD^{tree}")

	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, mapping, err := Rewrite(repo)
	if err != nil {
		t.Fatal(err)
	}
	newMerge := git(t, dir, "rev-parse", "HEAD")
	if newMerge != mapping[oldMerge] {
		t.Fatalf("HEAD=%s mapping=%s", newMerge, mapping[oldMerge])
	}
	if got := git(t, dir, "rev-parse", "HEAD^{tree}"); got != oldTree {
		t.Fatalf("merge tree changed: %s -> %s", oldTree, got)
	}
	parents := strings.Fields(git(t, dir, "show", "-s", "--format=%P", "HEAD"))
	if len(parents) != 2 {
		t.Fatalf("merge parent count=%d want 2", len(parents))
	}
	if parents[0] != mapping[oldMainParent] || parents[1] != mapping[oldFeature] {
		t.Fatalf("merge parents=%v want [%s %s]", parents, mapping[oldMainParent], mapping[oldFeature])
	}
}

func TestRewriteSHA256Repository(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-q", "--object-format=sha256", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("SHA-256 repositories unsupported by this Git: %v\n%s", err, out)
	}
	git(t, dir, "config", "user.name", "Angus Test")
	git(t, dir, "config", "user.email", "angus@example.com")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-q", "-m", "sha256\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Rewrite(repo); err != nil {
		t.Fatal(err)
	}
	report, err := Scan(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Matches) != 0 {
		t.Fatalf("matches after SHA-256 rewrite: %#v", report.Matches)
	}
	if got := len(git(t, dir, "rev-parse", "HEAD")); got != 64 {
		t.Fatalf("SHA-256 object id length=%d want 64", got)
	}
}

func TestRewriteRefusesOperationInProgress(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "Angus Test")
	git(t, dir, "config", "user.email", "angus@example.com")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-q", "-m", "feat\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	gitDir := git(t, dir, "rev-parse", "--absolute-git-dir")
	if err := os.Mkdir(filepath.Join(gitDir, "rebase-merge"), 0755); err != nil {
		t.Fatal(err)
	}
	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Rewrite(repo); err == nil || !strings.Contains(err.Error(), "git operation in progress") {
		t.Fatalf("expected in-progress operation error, got %v", err)
	}
}
