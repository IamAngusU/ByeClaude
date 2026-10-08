package clean

import (
	"context"
	"fmt"
	"time"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/model"
)

// ScanPullRefsContext audits commit objects reachable via refs/pull/* alone.
// These refs are GitHub-managed and are never rewritten by ByeClaude.
func ScanPullRefsContext(ctx context.Context, repo *gitx.Repo, matcher attribution.Matcher) (model.ScanReport, error) {
	refs, err := refsFromNamespacesContext(ctx, repo, "refs/pull")
	if err != nil {
		return model.ScanReport{}, err
	}
	return ScanSelectedPullRefsContext(ctx, repo, matcher, refs)
}

// ScanSelectedPullRefsContext limits the commit walk to the explicitly
// selected pull refs, even when a mirror already contains other PR refs.
func ScanSelectedPullRefsContext(ctx context.Context, repo *gitx.Repo, matcher attribution.Matcher, refs []Ref) (model.ScanReport, error) {
	started := time.Now()
	report := model.ScanReport{Repository: repo.Root, RuleMatches: map[string]int{}}
	if matcher == nil {
		return report, fmt.Errorf("attribution matcher is required")
	}
	commits, err := commitsForRefsContext(ctx, repo, refs)
	if err != nil {
		return report, err
	}
	raw, err := repo.CatFileBatch(ctx, commits, "commit")
	if err != nil {
		return report, err
	}
	report.Commits = len(commits)
	for i, sha := range commits {
		obj, err := parseCommit(raw[i])
		if err != nil {
			return report, err
		}
		author, email := obj.author()
		items := MatchingEvidence(obj.Message, matcher)
		if len(items) > 0 {
			report.MatchedCommits++
		}
		for _, item := range items {
			for _, id := range item.RuleIDs {
				report.RuleMatches[id]++
			}
			report.Matches = append(report.Matches, model.Match{
				Commit: sha, Author: author, Email: email,
				AttributionName: item.Name, AttributionEmail: item.Email,
				Line: item.Line, Rules: append([]string(nil), item.RuleIDs...),
			})
		}
	}
	if report.Commits > 0 {
		report.CommitMatchPct = float64(report.MatchedCommits) / float64(report.Commits) * 100
	}
	report.DurationMS = time.Since(started).Milliseconds()
	return report, nil
}
