package clean

import (
	"context"
	"fmt"
	"strings"

	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

type rewriteSelection struct {
	refs           []Ref
	commits        []string
	scope          string
	upstream       string
	upstreamSHA    string
	remote         string
	remoteRef      string
	remoteVerified bool
}

func allRewriteSelectionContext(ctx context.Context, repo *gitx.Repo) (rewriteSelection, error) {
	refs, err := LocalRefsContext(ctx, repo)
	if err != nil {
		return rewriteSelection{}, err
	}
	commits, err := commitsForRefsContext(ctx, repo, refs)
	if err != nil {
		return rewriteSelection{}, err
	}
	return rewriteSelection{refs: refs, commits: commits, scope: "all-local-history"}, nil
}

// unpushedRewriteSelectionContext returns only commits on the checked-out
// branch that are ahead of its configured upstream. It verifies the tracking
// ref against the live remote before trusting that boundary.
func unpushedRewriteSelectionContext(ctx context.Context, repo *gitx.Repo) (rewriteSelection, error) {
	if repo.Bare {
		return rewriteSelection{}, fmt.Errorf("--unpushed requires a non-bare checkout")
	}
	branchOut, err := repo.RunContext(ctx, "symbolic-ref", "--quiet", "HEAD")
	if err != nil {
		return rewriteSelection{}, fmt.Errorf("--unpushed requires a checked-out local branch: %w", err)
	}
	branchRef := strings.TrimSpace(string(branchOut))
	if !strings.HasPrefix(branchRef, "refs/heads/") {
		return rewriteSelection{}, fmt.Errorf("--unpushed requires a local branch, got %s", branchRef)
	}
	branch := strings.TrimPrefix(branchRef, "refs/heads/")

	remoteOut, ok, err := repo.RunOptionalContext(ctx, "config", "--get", "branch."+branch+".remote")
	if err != nil {
		return rewriteSelection{}, err
	}
	remote := strings.TrimSpace(string(remoteOut))
	if !ok || remote == "" || remote == "." {
		return rewriteSelection{}, fmt.Errorf("branch %s has no remote upstream; publish it with git push -u <remote> %s, then retry", branch, branch)
	}
	mergeOut, ok, err := repo.RunOptionalContext(ctx, "config", "--get", "branch."+branch+".merge")
	if err != nil {
		return rewriteSelection{}, err
	}
	remoteRef := strings.TrimSpace(string(mergeOut))
	if !ok || !strings.HasPrefix(remoteRef, "refs/heads/") {
		return rewriteSelection{}, fmt.Errorf("branch %s has no branch upstream; run git push -u %s %s, then retry", branch, remote, branch)
	}
	if remoteRef != branchRef {
		return rewriteSelection{}, fmt.Errorf("--unpushed currently requires matching local and upstream branch names; %s tracks %s", branchRef, remoteRef)
	}

	upstreamOut, err := repo.RunContext(ctx, "rev-parse", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		return rewriteSelection{}, fmt.Errorf("resolve upstream for %s: %w", branch, err)
	}
	upstream := strings.TrimSpace(string(upstreamOut))
	upstreamSHAOut, err := repo.RunContext(ctx, "rev-parse", "--verify", upstream+"^{commit}")
	if err != nil {
		return rewriteSelection{}, fmt.Errorf("resolve local tracking ref %s: %w", upstream, err)
	}
	upstreamSHA := strings.TrimSpace(string(upstreamSHAOut))

	liveOut, err := repo.RunContext(ctx, "ls-remote", "--refs", remote, remoteRef)
	if err != nil {
		return rewriteSelection{}, fmt.Errorf("verify live upstream %s/%s: %w", remote, strings.TrimPrefix(remoteRef, "refs/heads/"), err)
	}
	liveSHA := ""
	for _, line := range strings.Split(strings.TrimSpace(string(liveOut)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == remoteRef {
			liveSHA = fields[0]
			break
		}
	}
	if liveSHA == "" {
		return rewriteSelection{}, fmt.Errorf("live upstream %s/%s does not exist", remote, strings.TrimPrefix(remoteRef, "refs/heads/"))
	}
	if liveSHA != upstreamSHA {
		return rewriteSelection{}, fmt.Errorf("local tracking ref %s is stale (%s; remote is %s); run git fetch %s and retry", upstream, upstreamSHA, liveSHA, remote)
	}

	headOut, err := repo.RunContext(ctx, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return rewriteSelection{}, err
	}
	headSHA := strings.TrimSpace(string(headOut))
	ancestor, err := isAncestorContext(ctx, repo, upstreamSHA, headSHA)
	if err != nil {
		return rewriteSelection{}, fmt.Errorf("verify upstream ancestry: %w", err)
	}
	if !ancestor {
		return rewriteSelection{}, fmt.Errorf("%s is not an ancestor of %s; reconcile the branch with its upstream before using --unpushed", upstream, branchRef)
	}

	commitsOut, err := repo.RunContext(ctx, "rev-list", "--topo-order", "--reverse", upstreamSHA+".."+headSHA)
	if err != nil {
		return rewriteSelection{}, err
	}
	commits := strings.Fields(string(commitsOut))
	return rewriteSelection{
		refs:           []Ref{{Name: branchRef, SHA: headSHA, Type: "commit"}},
		commits:        commits,
		scope:          "unpushed",
		upstream:       upstream,
		upstreamSHA:    upstreamSHA,
		remote:         remote,
		remoteRef:      remoteRef,
		remoteVerified: true,
	}, nil
}

func isAncestorContext(ctx context.Context, repo *gitx.Repo, ancestor, descendant string) (bool, error) {
	_, ok, err := repo.RunOptionalContext(ctx, "merge-base", "--is-ancestor", ancestor, descendant)
	return ok, err
}
