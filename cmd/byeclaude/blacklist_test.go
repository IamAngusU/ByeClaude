package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/blacklist"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

func TestSavedBlacklistDrivesAuditRewriteAndDefaultHooks(t *testing.T) {
	dir := createCLIRepository(t, false)
	runGit(t, dir, "commit", "--allow-empty", "-m", "Feature\n\nCo-authored-by: Helper <helper@example.org>\nCo-authored-by: Human <human@example.org>")
	beforeTree := runGit(t, dir, "rev-parse", "HEAD^{tree}")
	before := runGit(t, dir, "rev-parse", "HEAD")
	if err := runSetup([]string{"--repo", dir, "--apply"}); err != nil {
		t.Fatal(err)
	}
	if err := runBlacklist([]string{"add", "--repo", dir, "--id", "helper", "--email", "helper@example.org"}); err != nil {
		t.Fatal(err)
	}
	if err := runCheck([]string{"--repo", dir}); err == nil {
		t.Fatal("saved policy did not detect helper")
	}
	if err := runPlan([]string{"--repo", dir}); err != nil {
		t.Fatal(err)
	}
	if after := runGit(t, dir, "rev-parse", "HEAD"); after != before {
		t.Fatal("preview changed history")
	}
	if err := runSetup([]string{"--repo", dir, "--apply"}); err != nil {
		t.Fatalf("policy updates broke idempotent setup: %v", err)
	}
	if err := runClean([]string{"--repo", dir, "--apply"}); err != nil {
		t.Fatal(err)
	}
	if err := runCheck([]string{"--repo", dir}); err != nil {
		t.Fatal(err)
	}
	if tree := runGit(t, dir, "rev-parse", "HEAD^{tree}"); tree != beforeTree {
		t.Fatal("cleanup changed file tree")
	}
	if message := runGit(t, dir, "log", "-1", "--format=%B"); !strings.Contains(message, "human@example.org") || strings.Contains(message, "helper@example.org") {
		t.Fatal(message)
	}
	if err := runBlacklist([]string{"remove", "--repo", dir, "--id", "helper"}); err != nil {
		t.Fatal(err)
	}
	repo, _ := gitx.Open(dir)
	set, saved, err := blacklist.Load(repo)
	if err != nil || !saved || len(set.Rules) != 1 {
		t.Fatalf("policy: %+v %v %v", set, saved, err)
	}
}

func TestBlacklistFailsClosedAndExplicitRulesOverride(t *testing.T) {
	dir := createCLIRepository(t, false)
	repo, _ := gitx.Open(dir)
	runGit(t, dir, "config", "--local", "byeclaude.blacklist", `{"rules":[{"id":"bad","email_domain":["example.org"]}]}`)
	if err := runScan([]string{"--repo", dir}); err == nil {
		t.Fatal("invalid policy must not fall back")
	}
	if err := runSetup([]string{"--repo", dir, "--apply"}); err == nil {
		t.Fatal("invalid policy must block setup")
	}
	if err := runBlacklist([]string{"reset", "--repo", dir}); err != nil {
		t.Fatal(err)
	}
	if err := runBlacklist([]string{"add", "--repo", dir, "--id", "helper", "--email", "helper@example.org"}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := blacklistCommand([]string{"export", "--repo", dir}, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "helper@example.org") {
		t.Fatal(output.String())
	}
	rules := filepath.Join(t.TempDir(), "rules.json")
	if err := os.WriteFile(rules, []byte(`{"rules":[{"id":"only-other","exact_emails":["other@example.org"]}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	matcher, err := blacklist.Resolve(repo, rules)
	if err != nil || matcher.Match("Helper", "helper@example.org") || !matcher.Match("Other", "other@example.org") {
		t.Fatalf("override: %v", err)
	}
	if err := runSetup([]string{"--repo", dir, "--rules", rules, "--apply"}); err != nil {
		t.Fatal(err)
	}
	if err := runBlacklist([]string{"reset", "--repo", dir}); err == nil {
		t.Fatal("must explain explicit hook policy conflict")
	}
}

func TestBlacklistRejectsInvalidEditsWithoutChangingPolicy(t *testing.T) {
	dir := createCLIRepository(t, false)
	if err := runBlacklist([]string{"reset", "--repo", dir}); err != nil {
		t.Fatal(err)
	}
	before := runGit(t, dir, "config", "--local", "--get", "byeclaude.blacklist")
	for _, args := range [][]string{
		{"add", "--id", "oops"},
		{"add", "--id", "bad", "--email", "not-an-address"},
		{"add", "--id", "claude-anthropic", "--email", "other@example.org"},
		{"remove", "--id", "missing"},
		{"remove", "--id", "claude-anthropic"},
		{"reset", "--email", "other@example.org"},
		{"add", "--id", "oops", "stray", "--email", "other@example.org"},
	} {
		args = append(args[:1], append([]string{"--repo", dir}, args[1:]...)...)
		if err := runBlacklist(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
		if after := runGit(t, dir, "config", "--local", "--get", "byeclaude.blacklist"); after != before {
			t.Fatal("invalid edit changed policy")
		}
	}
}

func TestBlacklistRequiresSharedWorktreeAcknowledgement(t *testing.T) {
	dir := createCLIRepository(t, false)
	linked := filepath.Join(t.TempDir(), "linked")
	runGit(t, dir, "worktree", "add", "-b", "linked", linked)
	args := []string{"add", "--repo", linked, "--id", "helper", "--email", "helper@example.org"}
	if err := runBlacklist(args); err == nil {
		t.Fatal("shared policy update needs acknowledgement")
	}
	if err := runBlacklist(append(args, "--shared-worktrees")); err != nil {
		t.Fatal(err)
	}
	matcher, err := resolveLocalMatcher(dir, "")
	if err != nil || !matcher.Match("Helper", "helper@example.org") {
		t.Fatalf("main worktree policy not shared: %v", err)
	}
}
