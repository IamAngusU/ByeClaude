package batch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/preset"
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

func makeRepo(t *testing.T, name, message string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "Batch Test")
	git(t, dir, "config", "user.email", "batch@example.invalid")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte(name+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-q", "-m", message)
	return dir
}

func TestRunLocalBatchAggregatesMetrics(t *testing.T) {
	cleanRepo := makeRepo(t, "clean-repo", "clean")
	dirtyRepo := makeRepo(t, "dirty-repo", "feat\n\nCo-Authored-By: Claude <noreply@anthropic.com>")

	report, err := Run(context.Background(), []Spec{
		{Name: "clean", Source: cleanRepo, Visibility: "local", Local: true},
		{Name: "dirty", Source: dirtyRepo, Visibility: "local", Local: true},
	}, Options{Jobs: 2, Matcher: preset.Claude(), IncludeRemotes: true, Selection: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Repositories != 2 || report.Scanned != 2 || report.FailedRepositories != 0 {
		t.Fatalf("unexpected counts: %#v", report)
	}
	if report.CleanRepositories != 1 || report.MatchedRepositories != 1 || report.Matches != 1 {
		t.Fatalf("unexpected match counts: %#v", report)
	}
	if report.Commits != 2 {
		t.Fatalf("commits=%d want 2", report.Commits)
	}
	if report.MatchedCommits != 1 || report.CommitMatchPct != 50 || report.RepositoryMatchPct != 50 {
		t.Fatalf("unexpected percentages: %#v", report)
	}
	if report.RuleMatches["claude-anthropic"] != 1 {
		t.Fatalf("rule matches=%#v", report.RuleMatches)
	}
	if len(report.Results) != 2 || report.Results[0].Repository != "clean" || report.Results[1].Repository != "dirty" {
		t.Fatalf("results not stable/sorted: %#v", report.Results)
	}
	for _, result := range report.Results {
		if result.TotalMS < 0 || result.ScanMS < 0 || result.PrepareMS < 0 {
			t.Fatalf("negative metric: %#v", result)
		}
		if result.RewriteReady != nil {
			t.Fatalf("scan result unexpectedly includes rewrite_ready: %#v", result)
		}
	}
}

func TestResolveExplicitSupportsLocalAndSlug(t *testing.T) {
	local := makeRepo(t, "local", "clean")
	specs, err := ResolveExplicit([]string{local, "IamAngusU/ByeClaude", "IamAngusU/ByeClaude"})
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 {
		t.Fatalf("spec count=%d want 2: %#v", len(specs), specs)
	}
	if !specs[0].Local || specs[0].Visibility != "local" {
		t.Fatalf("local spec=%#v", specs[0])
	}
	if specs[1].Source != "https://github.com/IamAngusU/ByeClaude.git" {
		t.Fatalf("slug source=%q", specs[1].Source)
	}
}

func TestRunRemoteMirrorTarget(t *testing.T) {
	source := makeRepo(t, "remote-source", "feat\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	bare := filepath.Join(t.TempDir(), "remote.git")
	cmd := exec.Command("git", "clone", "--bare", "-q", source, bare)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create bare remote: %v\n%s", err, out)
	}

	report, err := Run(context.Background(), []Spec{{
		Name:       "fixture/remote",
		Source:     "file://" + bare,
		Visibility: "unknown",
	}}, Options{Jobs: 1, Matcher: preset.Claude(), IncludeRemotes: true, Selection: "remote-fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Scanned != 1 || report.MatchedRepositories != 1 || report.Matches != 1 || report.FailedRepositories != 0 {
		t.Fatalf("unexpected report: %#v", report)
	}
	if len(report.Results) != 1 || report.Results[0].TotalMS < report.Results[0].ScanMS {
		t.Fatalf("unexpected metrics: %#v", report.Results)
	}
}

func TestRunBatchPlanAggregatesRewriteImpact(t *testing.T) {
	dir := makeRepo(t, "plan-repo", "assistant\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("descendant\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "b.txt")
	git(t, dir, "commit", "-q", "-m", "descendant")

	report, err := Run(context.Background(), []Spec{{
		Name:       "plan",
		Source:     dir,
		Visibility: "local",
		Local:      true,
	}}, Options{Jobs: 1, Matcher: preset.Claude(), Selection: "fixture-plan", Plan: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Operation != "plan" {
		t.Fatalf("operation=%q", report.Operation)
	}
	if report.MatchedCommits != 1 || report.CommitsToRewrite != 2 || report.DescendantCommits != 1 {
		t.Fatalf("unexpected plan counts: %#v", report)
	}
	if report.ParentLinksToRewrite != 1 || report.RefsToMove != 1 || report.ObjectWritesEstimate != 2 {
		t.Fatalf("unexpected plan impact: %#v", report)
	}
}

func TestIsolatedPublicGitEnvironment(t *testing.T) {
	env := []string{
		"HOME=/tmp/example",
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=url.file:///tmp/evil.insteadOf",
		"GIT_CONFIG_VALUE_0=https://github.com/",
		"GIT_CONFIG_GLOBAL=/tmp/global-config",
		"GIT_CONFIG_SYSTEM=/tmp/system-config",
		"GIT_DIR=/tmp/forced.git",
		"PATH=/usr/bin",
	}
	got := isolatedPublicGitEnvironment(env)
	joined := strings.Join(got, "\n")
	for _, forbidden := range []string{
		"GIT_CONFIG_COUNT=",
		"GIT_CONFIG_KEY_0=",
		"GIT_CONFIG_VALUE_0=",
		"GIT_DIR=/tmp/forced.git",
		"GIT_CONFIG_GLOBAL=/tmp/global-config",
		"GIT_CONFIG_SYSTEM=/tmp/system-config",
	} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("isolated environment retained %q: %s", forbidden, joined)
		}
	}
	if !strings.Contains(joined, "HOME=/tmp/example") || !strings.Contains(joined, "PATH=/usr/bin") {
		t.Fatalf("ordinary environment was removed: %s", joined)
	}
	if !strings.Contains(joined, "GIT_CONFIG_NOSYSTEM=1") ||
		!strings.Contains(joined, "GIT_CONFIG_GLOBAL="+os.DevNull) ||
		!strings.Contains(joined, "GIT_TERMINAL_PROMPT=0") {
		t.Fatalf("isolation controls missing: %s", joined)
	}
}
