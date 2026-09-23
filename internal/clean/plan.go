package clean

import (
	"fmt"
	"strings"
	"time"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/model"
)

type tagPlanResult struct {
	affected bool
}

func Plan(repo *gitx.Repo, matcher attribution.Matcher) (model.PlanReport, error) {
	started := time.Now()
	if matcher == nil {
		return model.PlanReport{}, fmt.Errorf("attribution matcher is required")
	}

	refs, err := LocalRefs(repo)
	if err != nil {
		return model.PlanReport{}, err
	}
	commits, err := commitsForRefs(repo, refs)
	if err != nil {
		return model.PlanReport{}, err
	}

	report := model.PlanReport{
		Repository:  repo.Root,
		Commits:     len(commits),
		RuleMatches: map[string]int{},
	}
	impacted := make(map[string]bool, len(commits))
	matched := make(map[string]bool)

	for _, sha := range commits {
		raw, err := repo.Run("cat-file", "commit", sha)
		if err != nil {
			return report, err
		}
		obj, err := parseCommit(raw)
		if err != nil {
			return report, err
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
				Rules:            append([]string(nil), evidence.RuleIDs...),
				Line:             evidence.Line,
			})
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
	}

	report.MatchedCommits = len(matched)
	report.DescendantCommits = report.CommitsToRewrite - report.MatchedCommits
	if report.Commits > 0 {
		report.CommitMatchPct = (float64(report.MatchedCommits) / float64(report.Commits)) * 100
	}

	tagMemo := map[string]tagPlanResult{}
	tagCounted := map[string]bool{}
	tagSignedCounted := map[string]bool{}
	for _, ref := range refs {
		affected := false
		switch ref.Type {
		case "commit":
			affected = impacted[ref.SHA]
		case "tag":
			tagResult, err := planTag(repo, ref.SHA, impacted, tagMemo, tagCounted, tagSignedCounted, &report)
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
	if err := Preflight(repo); err != nil {
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

func planTag(
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

	raw, err := repo.Run("cat-file", "tag", sha)
	if err != nil {
		return tagPlanResult{}, err
	}
	text := string(raw)
	sep := strings.Index(text, "

")
	if sep < 0 {
		return tagPlanResult{}, fmt.Errorf("invalid tag object %s", sha)
	}
	head, msg := text[:sep], text[sep+2:]
	var target, typ string
	for _, line := range strings.Split(head, "
") {
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
		child, err := planTag(repo, target, impacted, memo, counted, signedCounted, report)
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
