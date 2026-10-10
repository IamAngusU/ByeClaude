package clean

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/preset"
)

func initUnpushedFixture(t *testing.T) (string, string) {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "remote.git")
	git(t, t.TempDir(), "init", "--bare", "-q", remote)
	work := t.TempDir()
	git(t, work, "init", "-q", "-b", "main")
	git(t, work, "config", "user.name", "Human")
	git(t, work, "config", "user.email", "human@example.org")
	if err := os.WriteFile(filepath.Join(work, "file.txt"), []byte("published\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, work, "add", "file.txt")
	git(t, work, "commit", "-q", "-m", "published")
	git(t, work, "remote", "add", "origin", remote)
	git(t, work, "push", "-q", "-u", "origin", "main")
	return work, remote
}

func TestUnpushedRewriteOnlyTouchesCurrentBranchRangeAndPushesWithLiveLease(t *testing.T) {
	work, remote := initUnpushedFixture(t)
	published := git(t, work, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(work, "file.txt"), []byte("published\nlocal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, work, "add", "file.txt")
	git(t, work, "commit", "-q", "-m", "local work\n\nGenerated with Claude Code\n\nCo-authored-by: Claude <noreply@anthropic.com>")
	oldHead := git(t, work, "rev-parse", "HEAD")
	oldTree := git(t, work, "rev-parse", "HEAD^{tree}")

	repo, err := gitx.Open(work)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanUnpushedWithIdentityContext(context.Background(), repo, preset.Claude(), IdentityRewriteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Scope != "unpushed" || !plan.RemoteVerified || plan.Remote != "origin" || plan.Upstream != "refs/remotes/origin/main" {
		t.Fatalf("unexpected scope report: %#v", plan)
	}
	if plan.Commits != 1 || plan.MatchedCommits != 1 || plan.BranchesToMove != 1 || plan.TagRefsToMove != 0 {
		t.Fatalf("unexpected unpushed plan: %#v", plan)
	}

	report, _, err := RewriteUnpushedWithIdentityContext(context.Background(), repo, preset.Claude(), IdentityRewriteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.CommitsVisited != 1 || report.CommitsRewritten != 1 || report.RefsUpdated != 1 || report.TreesVerified != 1 {
		t.Fatalf("unexpected rewrite report: %#v", report)
	}
	boundaries, err := UpstreamLocalRefs(repo, report.Backup)
	if err != nil || len(boundaries) != 1 || boundaries[0].Name != "refs/heads/main" || boundaries[0].SHA != published {
		t.Fatalf("recorded upstream boundary = %#v, err=%v", boundaries, err)
	}
	newHead := git(t, work, "rev-parse", "HEAD")
	if newHead == oldHead {
		t.Fatal("unpushed HEAD did not change")
	}
	if got := git(t, work, "rev-parse", "HEAD^"); got != published {
		t.Fatalf("published boundary changed: got parent %s, want %s", got, published)
	}
	if got := git(t, work, "rev-parse", "HEAD^{tree}"); got != oldTree {
		t.Fatalf("tree changed: %s -> %s", oldTree, got)
	}
	if got := git(t, work, "ls-remote", "--refs", remote, "refs/heads/main"); !strings.HasPrefix(got, published+"\t") {
		t.Fatalf("remote changed before explicit push: %q", got)
	}

	after, err := PlanUnpushedWithIdentityContext(context.Background(), repo, preset.Claude(), IdentityRewriteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if after.MatchedCommits != 0 {
		t.Fatalf("matching metadata remains in unpushed range: %#v", after.Matches)
	}
	if err := PushBackup(repo, "origin", report.Backup); err != nil {
		t.Fatal(err)
	}
	if got := git(t, work, "ls-remote", "--refs", remote, "refs/heads/main"); !strings.HasPrefix(got, newHead+"\t") {
		t.Fatalf("remote was not updated to rewrite result: %q", got)
	}
	if count, err := PruneBackup(repo, report.Backup); err != nil || count != 3 {
		t.Fatalf("prune unpushed recovery set: count=%d err=%v", count, err)
	}
	if refs, _ := UpstreamLocalRefs(repo, report.Backup); len(refs) != 0 {
		t.Fatalf("upstream boundary refs remain after prune: %#v", refs)
	}
}

func TestUnpushedRefusesStaleTrackingBoundary(t *testing.T) {
	work, remote := initUnpushedFixture(t)
	other := t.TempDir()
	git(t, other, "clone", "-q", "-b", "main", remote, ".")
	git(t, other, "config", "user.name", "Other")
	git(t, other, "config", "user.email", "other@example.org")
	git(t, other, "commit", "--allow-empty", "-q", "-m", "remote moved")
	git(t, other, "push", "-q", "origin", "HEAD:main")
	git(t, work, "commit", "--allow-empty", "-q", "-m", "local\n\nCo-authored-by: Claude <noreply@anthropic.com>")

	repo, err := gitx.Open(work)
	if err != nil {
		t.Fatal(err)
	}
	_, err = PlanUnpushedWithIdentityContext(context.Background(), repo, preset.Claude(), IdentityRewriteOptions{})
	if err == nil || !strings.Contains(err.Error(), "git fetch origin") {
		t.Fatalf("expected stale tracking refusal, got %v", err)
	}
	if ids, backupErr := BackupRefs(repo); backupErr != nil || len(ids) != 0 {
		t.Fatalf("read-only refusal created backups: ids=%v err=%v", ids, backupErr)
	}
}

func TestUnpushedRequiresRemoteUpstream(t *testing.T) {
	work := t.TempDir()
	git(t, work, "init", "-q", "-b", "main")
	git(t, work, "config", "user.name", "Human")
	git(t, work, "config", "user.email", "human@example.org")
	git(t, work, "commit", "--allow-empty", "-q", "-m", "local")
	repo, err := gitx.Open(work)
	if err != nil {
		t.Fatal(err)
	}
	_, err = PlanUnpushedWithIdentityContext(context.Background(), repo, preset.Claude(), IdentityRewriteOptions{})
	if err == nil || !strings.Contains(err.Error(), "no remote upstream") {
		t.Fatalf("expected missing upstream error, got %v", err)
	}
}
