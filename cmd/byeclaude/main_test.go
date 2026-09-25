package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	batchpkg "github.com/IamAngusU/ByeClaude/internal/batch"
	"github.com/IamAngusU/ByeClaude/internal/clean"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/model"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func inWorkingDirectory(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
}

func TestHookInstallRefusesExistingHook(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	hook := filepath.Join(dir, ".git", "hooks", "commit-msg")
	original := "#!/bin/sh\necho existing\n"
	if err := os.WriteFile(hook, []byte(original), 0755); err != nil {
		t.Fatal(err)
	}
	if err := runHook([]string{"install", "--repo", dir}); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("expected overwrite refusal, got %v", err)
	}
	got, err := os.ReadFile(hook)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Fatalf("existing hook changed: %q", got)
	}
}

func TestHookInstallRefusesCustomHooksPath(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "core.hooksPath", ".githooks")
	if err := runHook([]string{"install", "--repo", dir}); err == nil || !strings.Contains(err.Error(), "core.hooksPath") {
		t.Fatalf("expected core.hooksPath refusal, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "hooks", "commit-msg")); !os.IsNotExist(err) {
		t.Fatalf("default hook unexpectedly created: %v", err)
	}
}

func TestHookInstallEmbedsRulesFile(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	rules := filepath.Join(dir, "rules.json")
	if err := os.WriteFile(rules, []byte(`{"rules":[{"id":"bot","name_contains":["bot"],"email_domains":["example.dev"]}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := runHook([]string{"install", "--repo", dir, "--rules", rules}); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(dir, ".git", "hooks", "commit-msg")
	got, err := os.ReadFile(hook)
	if err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(rules)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "--rules") || !strings.Contains(string(got), filepath.ToSlash(abs)) {
		t.Fatalf("hook does not embed validated rules path: %s", got)
	}
}

func TestHookInstallRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	target := filepath.Join(dir, "outside-hook")
	if err := os.WriteFile(target, []byte("outside\n"), 0600); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(dir, ".git", "hooks", "commit-msg")
	if err := os.Symlink(target, hook); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := runHook([]string{"install", "--repo", dir}); err == nil || !strings.Contains(err.Error(), "non-regular") {
		t.Fatalf("expected symlink refusal, got %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "outside\n" {
		t.Fatalf("symlink target changed: %q", got)
	}
}

func TestFilterCommitMessageIsConfinedToGitDirectory(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	inWorkingDirectory(t, dir)
	matcher, err := resolveMatcher("")
	if err != nil {
		t.Fatal(err)
	}

	message := filepath.Join(dir, ".git", "COMMIT_EDITMSG")
	input := "Subject\n\nCo-authored-by: Claude <noreply@anthropic.com>\n"
	if err := os.WriteFile(message, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	if err := filterCommitMessage(message, matcher); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(message)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "anthropic.com") || !strings.Contains(string(got), "Subject") {
		t.Fatalf("unexpected filtered message: %q", got)
	}

	outside := filepath.Join(dir, "outside.txt")
	if err := os.WriteFile(outside, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	if err := filterCommitMessage(outside, matcher); err == nil || !strings.Contains(err.Error(), "inside") {
		t.Fatalf("expected confinement error, got %v", err)
	}
	unchanged, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if string(unchanged) != input {
		t.Fatalf("outside file changed: %q", unchanged)
	}
}

func createCLIRepository(t *testing.T, withClaudeTrailer bool) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.name", "CLI Test")
	runGit(t, dir, "config", "user.email", "cli@example.invalid")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	message := "initial"
	if withClaudeTrailer {
		message += "\n\nCo-authored-by: Claude <noreply@anthropic.com>"
	}
	runGit(t, dir, "commit", "-q", "-m", message)
	return dir
}

func TestAuditAndRewriteCommandFlow(t *testing.T) {
	dir := createCLIRepository(t, true)
	if err := runScan([]string{"--repo", dir}); err != nil {
		t.Fatal(err)
	}
	if err := runScan([]string{"--repo", dir, "--json", "--include-remotes"}); err != nil {
		t.Fatal(err)
	}
	if err := runCheck([]string{"--repo", dir}); err == nil || !strings.Contains(err.Error(), "guard failed") {
		t.Fatalf("expected attribution guard failure, got %v", err)
	}
	if err := runCheck([]string{"--repo", dir, "--json"}); err == nil {
		t.Fatal("expected JSON attribution guard failure")
	}
	if err := runPlan([]string{"--repo", dir}); err != nil {
		t.Fatal(err)
	}
	if err := runPlan([]string{"--repo", dir, "--json"}); err != nil {
		t.Fatal(err)
	}
	if err := runClean([]string{"--repo", dir}); err != nil {
		t.Fatal(err)
	}
	if err := runClean([]string{"--repo", dir, "--apply", "--json"}); err != nil {
		t.Fatal(err)
	}
	if err := runCheck([]string{"--repo", dir}); err != nil {
		t.Fatal(err)
	}
	if err := runClean([]string{"--repo", dir}); err != nil {
		t.Fatal(err)
	}
	if err := runBackups([]string{"--repo", dir}); err != nil {
		t.Fatal(err)
	}
	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	backups, err := clean.BackupRefs(repo)
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups=%v err=%v", backups, err)
	}
	if err := runRestore([]string{"--repo", dir, "--backup", backups[0]}); err != nil {
		t.Fatal(err)
	}
	if err := runRestore([]string{"--repo", dir, "--backup", backups[0], "--apply"}); err != nil {
		t.Fatal(err)
	}
	if err := runCheck([]string{"--repo", dir}); err == nil {
		t.Fatal("restored history should contain matching trailer")
	}
}

func TestCleanRepositoryAndCommandValidation(t *testing.T) {
	dir := createCLIRepository(t, false)
	if err := runScan([]string{"--repo", dir}); err != nil {
		t.Fatal(err)
	}
	if err := runCheck([]string{"--repo", dir, "--json"}); err != nil {
		t.Fatal(err)
	}
	if err := runClean([]string{"--repo", dir, "--apply"}); err != nil {
		t.Fatal(err)
	}
	if err := runBackups([]string{"--repo", dir}); err != nil {
		t.Fatal(err)
	}
	if err := runPush([]string{"--repo", dir}); err == nil || !strings.Contains(err.Error(), "--backup is required") {
		t.Fatalf("expected missing backup error, got %v", err)
	}
	if err := runRestore([]string{"--repo", dir}); err == nil || !strings.Contains(err.Error(), "--backup is required") {
		t.Fatalf("expected missing backup error, got %v", err)
	}
	if err := runHook(nil); err == nil {
		t.Fatal("expected missing hook action error")
	}
	if err := runHook([]string{"unknown", "--repo", dir}); err == nil || !strings.Contains(err.Error(), "unknown hook action") {
		t.Fatalf("expected unknown hook error, got %v", err)
	}
}

func TestHookInstallRemoveLifecycle(t *testing.T) {
	dir := createCLIRepository(t, false)
	args := []string{"install", "--repo", dir}
	if err := runHook(args); err != nil {
		t.Fatal(err)
	}
	if err := runHook(args); err != nil {
		t.Fatal(err)
	}
	if err := runHook([]string{"remove", "--repo", dir}); err != nil {
		t.Fatal(err)
	}
	if err := runHook([]string{"remove", "--repo", dir}); err != nil {
		t.Fatal(err)
	}
}

func TestIdentityAndBatchLocalCommands(t *testing.T) {
	dir := createCLIRepository(t, true)
	if err := runIdentity([]string{"--repo", dir}); err != nil {
		t.Fatal(err)
	}
	if err := runIdentity([]string{"--repo", dir, "--json", "--github-id", "12345"}); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"scan", "plan"} {
		if err := runBatch([]string{mode, "--repo", dir, "--jobs", "1", "--json"}); err != nil {
			t.Fatalf("batch %s: %v", mode, err)
		}
	}
	if err := runBatch([]string{"check", "--repo", dir, "--jobs", "1"}); err == nil || !strings.Contains(err.Error(), "guard failed") {
		t.Fatalf("expected batch check failure, got %v", err)
	}
	if err := runBatch(nil); err == nil {
		t.Fatal("expected missing batch mode error")
	}
	if err := runBatch([]string{"scan", "--repo", dir, "--owner", "someone"}); err == nil {
		t.Fatal("expected incompatible selection error")
	}
	if err := runBatch([]string{"scan", "--public", "--private"}); err == nil {
		t.Fatal("expected visibility conflict error")
	}
}

func TestFormattingHelpers(t *testing.T) {
	if got := metricDuration(0); got != "<1ms" {
		t.Fatalf("metricDuration(0) = %q", got)
	}
	if got := trimWidth("abcdef", 4); got != "abc…" {
		t.Fatalf("trimWidth = %q", got)
	}
	if got := trimWidth("abc", 4); got != "abc" {
		t.Fatalf("short trimWidth = %q", got)
	}
	printPlanReport(model.PlanReport{Repository: "repo", RewriteReady: false, RewriteBlocker: "blocked", AffectedRefs: []string{"refs/heads/main"}})
	printBatchReportForTest()
}

func printBatchReportForTest() {
	// Exercise both status rendering and deterministic rule sorting without
	// coupling tests to terminal capture.
	printBatchReport(batchReportFixture())
}

func batchReportFixture() batchpkg.Report {
	ready := true
	return batchpkg.Report{
		Selection:           "fixture",
		Operation:           "plan",
		Jobs:                1,
		Repositories:        3,
		Scanned:             2,
		CleanRepositories:   1,
		MatchedRepositories: 1,
		FailedRepositories:  1,
		RuleMatches:         map[string]int{"z-rule": 1, "a-rule": 2},
		Results: []batchpkg.RepoMetrics{
			{Repository: "clean", RewriteReady: &ready},
			{Repository: "matching-repository-with-a-name-longer-than-thirty-six-characters", Matches: 2, RewriteReady: &ready},
			{Repository: "failed", Error: "fixture failure"},
		},
	}
}
