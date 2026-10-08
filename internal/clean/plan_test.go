package clean

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/preset"
	"github.com/IamAngusU/ByeClaude/internal/progress"
)

func TestPlanCountsDescendantsAndParentLinks(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "Plan Test")
	git(t, dir, "config", "user.email", "plan@example.invalid")

	for i, message := range []string{
		"initial",
		"assistant\n\nCo-Authored-By: Claude <noreply@anthropic.com>",
		"descendant one",
		"descendant two",
	} {
		path := filepath.Join(dir, "file.txt")
		if err := os.WriteFile(path, []byte{byte('a' + i), '\n'}, 0644); err != nil {
			t.Fatal(err)
		}
		git(t, dir, "add", "file.txt")
		git(t, dir, "commit", "-q", "-m", message)
	}

	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	var events []progress.Event
	ctx := progress.WithReporter(context.Background(), func(p progress.Event) { events = append(events, p) })
	report, err := PlanWithIdentityContext(ctx, repo, preset.Claude(), IdentityRewriteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Commits != 4 || report.MatchedCommits != 1 || len(report.Matches) != 1 {
		t.Fatalf("unexpected scan counts: %#v", report)
	}
	if report.CommitsToRewrite != 3 || report.DescendantCommits != 2 {
		t.Fatalf("unexpected rewrite counts: %#v", report)
	}
	if report.ParentLinksToRewrite != 2 {
		t.Fatalf("parent links=%d want 2", report.ParentLinksToRewrite)
	}
	if report.BranchesToMove != 1 || report.RefsToMove != 1 {
		t.Fatalf("unexpected ref counts: %#v", report)
	}
	if report.ObjectWritesEstimate != 3 {
		t.Fatalf("object writes=%d want 3", report.ObjectWritesEstimate)
	}
	if !report.RewriteReady || report.RewriteBlocker != "" {
		t.Fatalf("expected rewrite-ready plan: %#v", report)
	}
	for _, stage := range []string{"Loading commit objects", "Reviewing commits"} {
		var completed int
		for _, p := range events {
			if p.Stage == stage {
				if p.Total != 4 || p.Done < completed || p.Done > p.Total {
					t.Fatal("false progress", p)
				}
				completed = p.Done
			}
		}
		if completed != 4 {
			t.Fatal("missing progress", stage, events)
		}
	}
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancelCtx = progress.WithReporter(cancelCtx, func(p progress.Event) {
		if p.Stage == "Reviewing commits" {
			cancel()
		}
	})
	if _, err := PlanWithIdentityContext(cancelCtx, repo, preset.Claude(), IdentityRewriteOptions{}); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestPlanCountsAnnotatedTagRewrite(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "Plan Test")
	git(t, dir, "config", "user.email", "plan@example.invalid")

	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "file.txt")
	git(t, dir, "commit", "-q", "-m", "assistant\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	git(t, dir, "tag", "-a", "v-test", "-m", "fixture tag")

	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Plan(repo, preset.Claude())
	if err != nil {
		t.Fatal(err)
	}
	if report.CommitsToRewrite != 1 {
		t.Fatalf("commits to rewrite=%d want 1", report.CommitsToRewrite)
	}
	if report.AnnotatedTagsToRewrite != 1 || report.TagRefsToMove != 1 {
		t.Fatalf("unexpected tag impact: %#v", report)
	}
	if report.BranchesToMove != 1 || report.RefsToMove != 2 {
		t.Fatalf("unexpected ref impact: %#v", report)
	}
	if report.ObjectWritesEstimate != 2 {
		t.Fatalf("object writes=%d want 2", report.ObjectWritesEstimate)
	}
}

func TestPlanReportsPreflightBlockerWithoutFailingAudit(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "Plan Test")
	git(t, dir, "config", "user.email", "plan@example.invalid")
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "file.txt")
	git(t, dir, "commit", "-q", "-m", "assistant\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("dirty\n"), 0644); err != nil {
		t.Fatal(err)
	}

	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Plan(repo, preset.Claude())
	if err != nil {
		t.Fatal(err)
	}
	if report.RewriteReady {
		t.Fatalf("dirty repository unexpectedly rewrite-ready: %#v", report)
	}
	if report.RewriteBlocker == "" {
		t.Fatalf("expected preflight blocker: %#v", report)
	}
	if report.CommitsToRewrite != 1 {
		t.Fatalf("planning should still report graph impact: %#v", report)
	}
}
