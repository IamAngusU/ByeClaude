package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/IamAngusU/ByeClaude/internal/clean"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/metrics"
)

func runPrePushFilter(args []string) error {
	fs := flag.NewFlagSet("pre-push-filter", flag.ContinueOnError)
	rules := rulesFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 2 {
		return fmt.Errorf("expected Git remote name and URL")
	}
	matcher, err := resolveLocalMatcher(".", *rules)
	if err != nil {
		return err
	}
	repo, err := gitx.Open(".")
	if err != nil {
		return err
	}
	report, err := clean.CheckPushInput(repo, os.Stdin, matcher)
	if err != nil {
		return err
	}
	if report.CommitsWithMatch > 0 {
		metrics.Record(metrics.Counters{PushChecks: 1, PushBlocks: 1})
		for _, finding := range report.Findings {
			fmt.Fprintf(os.Stderr, "  %.12s %s: %s\n", finding.Commit, finding.Field, finding.Identity)
		}
		return fmt.Errorf("push blocked: %d commit(s) with matching attribution or Git identities (%d trailer(s), %d author(s), %d committer(s)); use byeclaude plan and clean before retrying", report.CommitsWithMatch, report.Trailers, report.Authors, report.Committers)
	}
	metrics.Record(metrics.Counters{PushChecks: 1})
	return nil
}
