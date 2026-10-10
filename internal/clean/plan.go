package clean

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/model"
	"github.com/IamAngusU/ByeClaude/internal/progress"
)

type tagPlanResult struct {
	affected bool
}

func Plan(repo *gitx.Repo, matcher attribution.Matcher) (model.PlanReport, error) {
	return PlanContext(context.Background(), repo, matcher)
}

func PlanContext(ctx context.Context, repo *gitx.Repo, matcher attribution.Matcher) (model.PlanReport, error) {
	return PlanWithIdentityContext(ctx, repo, matcher, IdentityRewriteOptions{})
}

func PlanWithIdentity(repo *gitx.Repo, matcher attribution.Matcher, opts IdentityRewriteOptions) (model.PlanReport, error) {
	return PlanWithIdentityContext(context.Background(), repo, matcher, opts)
}

func PlanWithIdentityContext(ctx context.Context, repo *gitx.Repo, matcher attribution.Matcher, opts IdentityRewriteOptions) (model.PlanReport, error) {
	selection, err := allRewriteSelectionContext(ctx, repo)
	if err != nil {
		return model.PlanReport{}, err
	}
	return planWithSelectionContext(ctx, repo, matcher, opts, selection)
}

func PlanUnpushedWithIdentityContext(ctx context.Context, repo *gitx.Repo, matcher attribution.Matcher, opts IdentityRewriteOptions) (model.PlanReport, error) {
	selection, err := unpushedRewriteSelectionContext(ctx, repo)
	if err != nil {
		return model.PlanReport{}, err
	}
	return planWithSelectionContext(ctx, repo, matcher, opts, selection)
}

func planWithSelectionContext(ctx context.Context, repo *gitx.Repo, matcher attribution.Matcher, opts IdentityRewriteOptions, selection rewriteSelection) (model.PlanReport, error) {
	started := time.Now()
	if err := opts.Validate(matcher); err != nil {
		return model.PlanReport{}, err
	}
	if matcher == nil {
		return model.PlanReport{}, fmt.Errorf("attribution matcher is required")
	}

	progress.Report(ctx, "Reading local history", 0, 0)
	refs := selection.refs
	commits := selection.commits

	report := model.PlanReport{
		Repository:     repo.Root,
		Scope:          selection.scope,
		Upstream:       selection.upstream,
		UpstreamCommit: selection.upstreamSHA,
		Remote:         selection.remote,
		RemoteVerified: selection.remoteVerified,
		Commits:        len(commits),
		RuleMatches:    map[string]int{},
	}
	impacted := make(map[string]bool, len(commits))
	matched := make(map[string]bool)
	progress.Report(ctx, "Reviewing commits", 0, len(commits))
	err := repo.CatFileBatchEach(ctx, commits, "commit", func(i int, sha string, raw []byte) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		obj, err := parseCommit(raw)
		if err != nil {
			return err
		}

		commitMatched := false
		author, email := obj.author()
		for _, evidence := range MatchingEvidence(obj.Message, matcher) {
			commitMatched = true
			matched[sha] = true
			for _, ruleID := range evidence.RuleIDs {
				report.RuleMatches[ruleID]++
			}
			report.Matches = append(report.Matches, model.Match{
				Commit:           sha,
				Author:           author,
				Email:            email,
				AttributionName:  evidence.Name,
				AttributionEmail: evidence.Email,
				AttributionField: evidence.Field,
				Rules:            append([]string(nil), evidence.RuleIDs...),
				Line:             evidence.Line,
			})
		}

		_, authorMatches, committerMatches, err := ReplaceMatchingCommitIdentities(obj, matcher, opts)
		if err != nil {
			return err
		}
		report.AuthorsToReplace += authorMatches
		report.CommittersToReplace += committerMatches
		if authorMatches != 0 || committerMatches != 0 {
			commitMatched = true
			matched[sha] = true
		}

		parentChanged := false
		for _, parent := range obj.parents() {
			if impacted[parent] {
				parentChanged = true
				report.ParentLinksToRewrite++
			}
		}
		if commitMatched || parentChanged {
			impacted[sha] = true
			report.CommitsToRewrite++
			report.SignaturesAtRisk += commitSignatureFields(obj)
		}
		progress.Report(ctx, "Reviewing commits", i+1, len(commits))
		return nil
	})
	if err != nil {
		return report, err
	}

	report.MatchedCommits = len(matched)
	report.DescendantCommits = report.CommitsToRewrite - report.MatchedCommits
	if report.Commits > 0 {
		report.CommitMatchPct = (float64(report.MatchedCommits) / float64(report.Commits)) * 100
	}

	progress.Report(ctx, "Checking refs and rewrite safety", 0, 0)
	tagMemo := map[string]tagPlanResult{}
	tagCounted := map[string]bool{}
	tagSignedCounted := map[string]bool{}
	for _, ref := range refs {
		affected := false
		switch ref.Type {
		case "commit":
			affected = impacted[ref.SHA]
		case "tag":
			tagResult, err := planTagContext(ctx, repo, ref.SHA, impacted, tagMemo, tagCounted, tagSignedCounted, &report)
			if err != nil {
				return report, err
			}
			affected = tagResult.affected
		}
		if !affected {
			continue
		}
		report.RefsToMove++
		report.AffectedRefs = append(report.AffectedRefs, ref.Name)
		if strings.HasPrefix(ref.Name, "refs/heads/") {
			report.BranchesToMove++
		} else if strings.HasPrefix(ref.Name, "refs/tags/") {
			report.TagRefsToMove++
		}
	}

	report.ObjectWritesEstimate = report.CommitsToRewrite + report.AnnotatedTagsToRewrite
	if err := PreflightContext(ctx, repo); err != nil {
		report.RewriteBlocker = err.Error()
	} else {
		report.RewriteReady = true
	}
	report.DurationMS = time.Since(started).Milliseconds()
	return report, nil
}

func commitSignatureFields(obj commitObject) int {
	count := 0
	for _, h := range obj.Headers {
		switch h.Key {
		case "gpgsig", "gpgsig-sha256", "mergetag":
			count++
		}
	}
	return count
}

func planTagContext(
	ctx context.Context,
	repo *gitx.Repo,
	sha string,
	impacted map[string]bool,
	memo map[string]tagPlanResult,
	counted map[string]bool,
	signedCounted map[string]bool,
	report *model.PlanReport,
) (tagPlanResult, error) {
	if result, ok := memo[sha]; ok {
		return result, nil
	}

	raw, err := repo.RunContext(ctx, "cat-file", "tag", sha)
	if err != nil {
		return tagPlanResult{}, err
	}
	text := string(raw)
	sep := strings.Index(text, "\n\n")
	if sep < 0 {
		return tagPlanResult{}, fmt.Errorf("invalid tag object %s", sha)
	}
	head, msg := text[:sep], text[sep+2:]
	var target, typ string
	for _, line := range strings.Split(head, "\n") {
		if strings.HasPrefix(line, "object ") {
			target = strings.TrimSpace(strings.TrimPrefix(line, "object "))
		}
		if strings.HasPrefix(line, "type ") {
			typ = strings.TrimSpace(strings.TrimPrefix(line, "type "))
		}
	}

	affected := false
	switch typ {
	case "commit":
		affected = impacted[target]
	case "tag":
		child, err := planTagContext(ctx, repo, target, impacted, memo, counted, signedCounted, report)
		if err != nil {
			return tagPlanResult{}, err
		}
		affected = child.affected
	}

	result := tagPlanResult{affected: affected}
	memo[sha] = result
	if !affected {
		return result, nil
	}

	if !counted[sha] {
		counted[sha] = true
		report.AnnotatedTagsToRewrite++
	}
	if _, signed := stripTagSignature(msg); signed && !signedCounted[sha] {
		signedCounted[sha] = true
		report.SignaturesAtRisk++
	}
	return result, nil
}
