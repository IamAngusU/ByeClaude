package main

import (
	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"strings"
	"testing"
)

func TestCanonicalGitHubRemote(t *testing.T) {
	for _, s := range []string{"https://github.com/owner/project.git", "git@github.com:owner/project.git", "ssh://git@github.com/owner/project.git"} {
		got, err := canonicalGitHubRemote(s)
		if err != nil || got != "owner/project" {
			t.Errorf("%s -> %q, %v", s, got, err)
		}
	}
	for _, s := range []string{"https://other.example/owner/project.git", "file:///tmp/repo", "https://secret@github.com/owner/project", "git@github.com:owner/../bad"} {
		if _, err := canonicalGitHubRemote(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}

func TestInvalidVerificationCannotRewriteOrPublish(t *testing.T) {
	for _, tc := range []struct{ remote, user, message string }{
		{"https://github.com/owner/project.git", "bad/user", "GitHub"},
		{"https://example.org/owner/project.git", "", "GitHub"},
	} {
		dir := createCLIRepository(t, true)
		runGit(t, dir, "remote", "add", "origin", tc.remote)
		before := runGit(t, dir, "show-ref")
		for _, command := range []string{"clean", "push"} {
			args := []string{"--repo", dir, "--verify-github", "--github-user", tc.user}
			var err error
			if command == "clean" {
				err = runClean(append(args, "--apply", "--push"))
			} else {
				err = runPush(append(args, "--backup", "example"))
			}
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("%s: %v", command, err)
			}
			if runGit(t, dir, "show-ref") != before {
				t.Fatal("invalid verification mutated repository")
			}
		}
	}
}

func TestVerificationUsesSinglePushDestination(t *testing.T) {
	dir := createCLIRepository(t, false)
	runGit(t, dir, "remote", "add", "origin", "https://github.com/owner/source.git")
	runGit(t, dir, "config", "remote.origin.pushurl", "git@github.com:owner/destination.git")
	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	slug, err := verificationTarget(repo, "origin", "")
	if err != nil || slug != "owner/destination" {
		t.Fatalf("target %s: %v", slug, err)
	}
	runGit(t, dir, "config", "--add", "remote.origin.pushurl", "https://github.com/owner/second.git")
	if _, err := verificationTarget(repo, "origin", ""); err == nil {
		t.Fatal("accepted multiple verification destinations")
	}
}
