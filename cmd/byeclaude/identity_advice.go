package main

import (
	"context"
	"fmt"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
	"github.com/IamAngusU/ByeClaude/internal/clean"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

// identityAdvice is deliberately advisory: legitimate third-party authors
// must not be silently replaced, even if a trailer rule also matches them.
func identityAdvice(repo *gitx.Repo, matcher attribution.Matcher) {
	report, err := clean.ScanMatchingHeadersContext(context.Background(), repo, matcher, []string{"refs/heads", "refs/tags"}, true)
	if err != nil {
		fmt.Printf("identity     could not inspect author/committer fields: %v\n", err)
		return
	}
	if report.Authors+report.Committers == 0 {
		return
	}
	fmt.Printf("identity     %d matching author(s), %d committer(s) remain\n", report.Authors, report.Committers)
	if report.Authors > 0 {
		fmt.Println("next         For a wrongly attributed author: byeclaude plan --author-from-git")
	}
	if report.Committers > 0 {
		fmt.Println("next         For a wrongly attributed committer: byeclaude plan --committer-from-git")
	}
	if report.Authors > 0 && report.Committers > 0 {
		fmt.Println("option       Both roles: byeclaude plan --identity-from-git")
	}
	fmt.Println("note         Do not reassign another contributor's genuine authorship.")
}
