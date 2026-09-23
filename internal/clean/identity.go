package clean

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/model"
)

type IdentityScanOptions struct {
	IncludeRemotes  bool
	IncludePullRefs bool
	GitHubIDs       []string
}

func ScanIdentities(repo *gitx.Repo, opts IdentityScanOptions) (model.IdentityReport, error) {
	return ScanIdentitiesContext(context.Background(), repo, opts)
}

func ScanIdentitiesContext(ctx context.Context, repo *gitx.Repo, opts IdentityScanOptions) (model.IdentityReport, error) {
	started := time.Now()
	refs, err := LocalRefsContext(ctx, repo)
	if err != nil {
		return model.IdentityReport{}, err
	}
	namespaces := []string{"refs/heads", "refs/tags"}
	if opts.IncludeRemotes {
		remoteRefs, err := refsFromNamespacesContext(ctx, repo, "refs/remotes")
		if err != nil {
			return model.IdentityReport{}, err
		}
		refs = append(refs, remoteRefs...)
		namespaces = append(namespaces, "refs/remotes")
	}
	if opts.IncludePullRefs {
		pullRefs, err := refsFromNamespacesContext(ctx, repo, "refs/pull")
		if err != nil {
			return model.IdentityReport{}, err
		}
		refs = append(refs, pullRefs...)
		namespaces = append(namespaces, "refs/pull")
	}
	if headOut, err := repo.RunContext(ctx, "rev-parse", "--verify", "HEAD"); err == nil {
		refs = append(refs, Ref{Name: "HEAD", SHA: strings.TrimSpace(string(headOut)), Type: "commit"})
	}

	commits, err := commitsForRefsContext(ctx, repo, refs)
	if err != nil {
		return model.IdentityReport{}, err
	}
	rawCommits, err := repo.CatFileBatch(ctx, commits, "commit")
	if err != nil {
		return model.IdentityReport{}, err
	}

	filter := map[string]bool{}
	for _, id := range opts.GitHubIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		for _, r := range id {
			if r < '0' || r > '9' {
				return model.IdentityReport{}, fmt.Errorf("github id %q must be numeric", id)
			}
		}
		filter[id] = true
	}

	type authorKey struct{ name, email, id string }
	authors := map[authorKey]int{}
	var evidence []model.IdentityEvidence
	for i, sha := range commits {
		obj, err := parseCommit(rawCommits[i])
		if err != nil {
			return model.IdentityReport{}, err
		}
		name, email := obj.author()
		githubID, _ := attribution.GitHubNoreplyID(email)
		authors[authorKey{name: name, email: email, id: githubID}]++
		if len(filter) == 0 || !filter[githubID] {
			continue
		}
		reachable, err := refsContainingCommit(ctx, repo, sha, namespaces)
		if err != nil {
			return model.IdentityReport{}, err
		}
		managed := false
		pullOnly := len(reachable) > 0
		for _, ref := range reachable {
			if strings.HasPrefix(ref, "refs/heads/") || strings.HasPrefix(ref, "refs/tags/") {
				managed = true
			}
			if !strings.HasPrefix(ref, "refs/pull/") {
				pullOnly = false
			}
		}
		evidence = append(evidence, model.IdentityEvidence{
			Commit:      sha,
			Name:        name,
			Email:       email,
			GitHubID:    githubID,
			ReachableBy: reachable,
			Managed:     managed,
			PullRefOnly: pullOnly,
		})
	}

	authorList := make([]model.AuthorIdentity, 0, len(authors))
	for key, count := range authors {
		authorList = append(authorList, model.AuthorIdentity{
			Name: key.name, Email: key.email, GitHubID: key.id, Commits: count,
		})
	}
	sort.Slice(authorList, func(i, j int) bool {
		if authorList[i].Commits != authorList[j].Commits {
			return authorList[i].Commits > authorList[j].Commits
		}
		if authorList[i].Email != authorList[j].Email {
			return authorList[i].Email < authorList[j].Email
		}
		return authorList[i].Name < authorList[j].Name
	})

	report := model.IdentityReport{
		Repository: repo.Root,
		Commits:    len(commits),
		Authors:    authorList,
		Matches:    evidence,
	}
	for _, match := range evidence {
		if match.Managed {
			report.ManagedMatches++
		}
		if match.PullRefOnly {
			report.PullOnlyMatches++
		}
	}
	report.DurationMS = time.Since(started).Milliseconds()
	return report, nil
}

func refsContainingCommit(ctx context.Context, repo *gitx.Repo, sha string, namespaces []string) ([]string, error) {
	args := []string{"for-each-ref", "--format=%(refname)", "--contains=" + sha}
	args = append(args, namespaces...)
	out, err := repo.RunContext(ctx, args...)
	if err != nil {
		return nil, err
	}
	refs := strings.Fields(string(out))
	sort.Strings(refs)
	return refs, nil
}
