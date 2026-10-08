package main

import "testing"

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
