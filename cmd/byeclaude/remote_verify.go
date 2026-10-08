package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
	"github.com/IamAngusU/ByeClaude/internal/batch"
	"github.com/IamAngusU/ByeClaude/internal/githubpolicy"
	"github.com/IamAngusU/ByeClaude/internal/githubverify"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

func canonicalGitHubRemote(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	var path string
	if strings.HasPrefix(raw, "git@github.com:") {
		path = strings.TrimPrefix(raw, "git@github.com:")
	} else {
		u, err := url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf("invalid remote URL")
		}
		if (u.Scheme != "https" && u.Scheme != "ssh") || !strings.EqualFold(u.Hostname(), "github.com") || u.Port() != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil && u.Scheme == "https" {
			return "", fmt.Errorf("remote is not a supported GitHub URL")
		}
		path = strings.TrimPrefix(u.Path, "/")
	}
	path = strings.TrimSuffix(path, ".git")
	if err := githubpolicy.ValidateRepo(path); err != nil {
		return "", err
	}
	return path, nil
}
func verifyGitHubRemoteAfterPush(repo *gitx.Repo, remote string, matcher attribution.Matcher, githubUser string) error {
	if err := githubverify.ValidateGitHubUser(githubUser); err != nil {
		return err
	}
	value, err := repo.Run("remote", "get-url", remote)
	if err != nil {
		return fmt.Errorf("push succeeded; cannot determine GitHub remote for verification: %w", err)
	}
	slug, err := canonicalGitHubRemote(string(value))
	if err != nil {
		return fmt.Errorf("push succeeded; GitHub verification unavailable: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	r, err := githubverify.Audit(ctx, githubverify.Options{Repo: slug, Token: batch.TokenFromEnv(), Matcher: matcher, GitHubUser: githubUser, MaxPullRefs: 200})
	if err != nil {
		return fmt.Errorf("push succeeded, but GitHub verification failed: %w", err)
	}
	fmt.Printf("github      %s: %s (remote=%s, pull=%s, contributors=%s)\n", slug, r.Overall, r.Remote.Status, r.PullRefs.Status, r.Contributors.Status)
	fmt.Println("github      cached views, old PR refs and external copies can persist; run verify again later")
	return nil
}
