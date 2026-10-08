package main

import (
	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupInstallsBothHooksAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.name", "Human")
	runGit(t, dir, "config", "user.email", "human@example.org")
	runGit(t, dir, "commit", "--allow-empty", "-m", "Initial")
	if err := runSetup([]string{"--repo", dir}); err != nil {
		t.Fatal(err)
	}
	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, hook := range []string{"commit-msg", "pre-push"} {
		status, err := inspectHook(repo, hook)
		if err != nil || status.Status != "missing" {
			t.Fatalf("dry run changed %s hook: %+v (%v)", hook, status, err)
		}
	}
	if err := runSetup([]string{"--repo", dir, "--apply"}); err != nil {
		t.Fatal(err)
	}
	if err := runSetup([]string{"--repo", dir, "--apply"}); err != nil {
		t.Fatalf("setup should be idempotent: %v", err)
	}
	for _, hook := range []string{"commit-msg", "pre-push"} {
		status, err := inspectHook(repo, hook)
		if err != nil || status.Status != "installed" {
			t.Fatalf("hook %s: %+v (%v)", hook, status, err)
		}
	}
	if err := runDoctor([]string{"--repo", dir}); err != nil {
		t.Fatal(err)
	}
}

func TestSetupRefusesForeignHookWithoutPartialInstallation(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	existing := filepath.Join(dir, ".git", "hooks", "pre-push")
	if err := os.WriteFile(existing, []byte("#!/bin/sh\necho foreign\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := runSetup([]string{"--repo", dir, "--apply"}); err == nil {
		t.Fatal("setup must refuse foreign pre-push hook")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "hooks", "commit-msg")); !os.IsNotExist(err) {
		t.Fatalf("setup partially installed commit-msg: %v", err)
	}
}

func TestHookMarkerCannotAuthorizeOtherScriptRemoval(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	foreign := filepath.Join(dir, ".git", "hooks", "pre-push")
	if err := os.WriteFile(foreign, []byte("#!/bin/sh\n# Installed by ByeClaude.\nrm -rf something\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := runHook([]string{"pre-push-remove", "--repo", dir}); err == nil {
		t.Fatal("foreign script with ByeClaude comment must not be removed")
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatal(err)
	}
}

func TestHookPathsInLinkedWorktrees(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	linked := filepath.Join(dir, "linked")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.name", "Human")
	runGit(t, root, "config", "user.email", "human@example.org")
	runGit(t, root, "commit", "--allow-empty", "-m", "Initial")
	runGit(t, root, "worktree", "add", "-q", "-b", "linked-test", linked)
	first, err := gitx.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := gitx.Open(linked)
	if err != nil {
		t.Fatal(err)
	}
	p1, err := resolvedGitHookPath(first, "commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := resolvedGitHookPath(second, "commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	if p1 != p2 {
		t.Fatalf("worktrees must share a hooks path: %s vs %s", p1, p2)
	}
	if err := runSetup([]string{"--repo", linked, "--apply"}); err == nil || !strings.Contains(err.Error(), "worktrees") {
		t.Fatalf("expected shared worktree warning: %v", err)
	}
}

func TestHookBinarySourceParserAndTemporaryExecutables(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "byeclaude")
	for _, tc := range []struct{ file, name string }{
		{"#!/bin/sh\n# Installed by ByeClaude.\nexec '/tmp/byeclaude' hook-filter \"$1\"\n", "commit-msg"},
		{"#!/bin/sh\n# Installed by ByeClaude.\nexec '/tmp/byeclaude' pre-push-filter \"$@\"\n", "pre-push"},
	} {
		tc.file = strings.ReplaceAll(tc.file, "/tmp/byeclaude", filepath.ToSlash(executable))
		if !ownedByeClaudeHook(tc.name, []byte(tc.file)) {
			t.Fatalf("valid hook rejected: %s", tc.name)
		}
		if path, ok := hookBinaryPath(tc.name, tc.file); !ok || path != executable {
			t.Fatalf("invalid extracted binary: %q, %v", path, ok)
		}
	}
	if !ephemeralGoExecutable("/tmp/go-build123/b001/exe/byeclaude") {
		t.Fatal("go run binary must not be installed permanently")
	}
	if ephemeralGoExecutable("/tmp/go-build123/b001/exe/byeclaude.test") {
		t.Fatal("test binary must be allowed for integration testing")
	}
}
