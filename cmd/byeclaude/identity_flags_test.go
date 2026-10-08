package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/model"
)

func TestGitIdentityOptInUsesConfiguredIdentityOnly(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.name", "Actual Contributor")
	runGit(t, dir, "config", "user.email", "actual@example.org")
	repo, err := gitx.Open(dir)
	if err != nil { t.Fatal(err) }

	opts, err := resolveIdentityRewriteOptions(repo, "", "", true)
	if err != nil { t.Fatal(err) }
	if opts.Author == nil || opts.Committer == nil ||
		opts.Author.Name != "Actual Contributor" || opts.Committer.Email != "actual@example.org" {
		t.Fatalf("wrong configured identity: %+v", opts)
	}
	if _, err := resolveIdentityRewriteOptions(repo, "Different <other@example.org>", "", true); err == nil {
		t.Fatal("mixed explicit and Git-config identities should fail")
	}
	original, err := resolveIdentityRewriteOptions(repo, "", "", false)
	if err != nil || original.Author != nil || original.Committer != nil {
		t.Fatal("identity rewriting must never be the default")
	}

	runGit(t, dir, "config", "--unset", "user.email")
	// A different global Git profile may exist, so isolate fallback explicitly.
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	if _, err := resolveIdentityRewriteOptions(repo, "", "", true); err == nil {
		t.Fatal("missing configured email should fail")
	}
}

func TestPlanActuallyIncludesIdentityRewriteImpact(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.name", "Claude Bot")
	runGit(t, dir, "config", "user.email", "noreply@anthropic.com")
	runGit(t, dir, "commit", "--allow-empty", "-m", "A commit with matching author and committer")
	before := runGit(t, dir, "rev-parse", "HEAD")
	runGit(t, dir, "config", "user.name", "Correct Contributor")
	runGit(t, dir, "config", "user.email", "correct@example.org")
	options := [][]string{
		{"--repo", dir, "--json"},
		{"--repo", dir, "--json", "--identity-from-git"},
		{"--repo", dir, "--json", "--replace-author", "Correct Contributor <correct@example.org>"},
	}
	for index, args := range options {
		reportJSON := capturePlanOutput(t, args)
		var report model.PlanReport
		if err := json.Unmarshal([]byte(reportJSON), &report); err != nil {
			t.Fatalf("invalid JSON on case %d: %v\n%s", index, err, reportJSON)
		}
		switch index {
		case 0:
			if report.AuthorsToReplace != 0 || report.CommittersToReplace != 0 {
				t.Fatalf("default plan proposed identity rewriting: %+v", report)
			}
		case 1:
			if report.AuthorsToReplace != 1 || report.CommittersToReplace != 1 || report.CommitsToRewrite != 1 {
				t.Fatalf("Git identity was not included in preview: %+v", report)
			}
		case 2:
			if report.AuthorsToReplace != 1 || report.CommittersToReplace != 0 {
				t.Fatalf("manual author rewrite not reflected: %+v", report)
			}
		}
		if actual := runGit(t, dir, "rev-parse", "HEAD"); actual != before {
			t.Fatal("read-only plan moved Git refs")
		}
	}
}

func capturePlanOutput(t *testing.T, args []string) string {
	t.Helper()
	original := os.Stdout
	read, write, err := os.Pipe()
	if err != nil { t.Fatal(err) }
	os.Stdout = write
	defer func() {
		os.Stdout = original
		read.Close()
		write.Close()
	}()
	err = runPlan(args)
	if err != nil { t.Fatal(err) }
	write.Close()
	output, err := io.ReadAll(read)
	if err != nil { t.Fatal(err) }
	if strings.TrimSpace(string(output)) == "" { t.Fatal("empty preview") }
	return string(output)
}

func TestIdentityRejectsUnsafeLocalConfig(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.name", "Not a real <name>")
	runGit(t, dir, "config", "user.email", "name@example.org")
	repo, err := gitx.Open(dir)
	if err != nil { t.Fatal(err) }
	if _, err := resolveIdentityRewriteOptions(repo, "", "", true); err == nil {
		t.Fatal("unsafe name should not be accepted")
	}
	_ = filepath.Base(dir)
}
