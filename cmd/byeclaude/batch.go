package main

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"
	"time"

	batchpkg "github.com/IamAngusU/ByeClaude/internal/batch"
)

type repeatedFlag []string

func (v *repeatedFlag) String() string { return strings.Join(*v, ",") }
func (v *repeatedFlag) Set(value string) error {
	*v = append(*v, value)
	return nil
}

func runBatch(args []string) error {
	if len(args) == 0 || (args[0] != "scan" && args[0] != "check") {
		return fmt.Errorf("batch requires scan or check")
	}
	mode := args[0]
	fs := flag.NewFlagSet("batch "+mode, flag.ContinueOnError)
	var repos repeatedFlag
	fs.Var(&repos, "repo", "repository target; repeat for multiple OWNER/NAME values, clone URLs, or local paths")
	owner := fs.String("owner", "", "GitHub owner for account-wide discovery; defaults to authenticated user")
	publicOnly := fs.Bool("public", false, "discover public repositories only")
	privateOnly := fs.Bool("private", false, "discover private repositories only; requires GH_TOKEN or GITHUB_TOKEN")
	allRepos := fs.Bool("all", false, "discover public and private repositories; requires GH_TOKEN or GITHUB_TOKEN")
	jobs := fs.Int("jobs", batchpkg.DefaultJobs(), "parallel repository scans (1-32)")
	includeRemotes := fs.Bool("include-remotes", true, "include fetched remote-tracking refs for local repository targets")
	jsonOut := fs.Bool("json", false, "machine-readable JSON including per-repository metrics")
	rulesFile := rulesFlag(fs)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	visibilityFlags := 0
	if *publicOnly {
		visibilityFlags++
	}
	if *privateOnly {
		visibilityFlags++
	}
	if *allRepos {
		visibilityFlags++
	}
	if visibilityFlags > 1 {
		return fmt.Errorf("choose only one of --public, --private, or --all")
	}
	if len(repos) > 0 && (*owner != "" || visibilityFlags > 0) {
		return fmt.Errorf("--repo cannot be combined with --owner/--public/--private/--all")
	}

	matcher, err := resolveMatcher(*rulesFile)
	if err != nil {
		return err
	}
	token := batchpkg.TokenFromEnv()
	var specs []batchpkg.Spec
	var selection string
	if len(repos) > 0 {
		specs, err = batchpkg.ResolveExplicit(repos)
		selection = fmt.Sprintf("explicit:%d", len(specs))
	} else {
		visibility := "public"
		if *privateOnly {
			visibility = "private"
		} else if *allRepos {
			visibility = "all"
		}
		client := batchpkg.GitHubClient{Token: token}
		resolvedOwner := strings.TrimSpace(*owner)
		if resolvedOwner == "" {
			resolvedOwner, err = client.CurrentLogin(context.Background())
			if err != nil {
				return err
			}
		}
		specs, err = client.ListOwned(context.Background(), resolvedOwner, visibility)
		selection = resolvedOwner + ":" + visibility
	}
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		return fmt.Errorf("selection %q resolved to zero repositories", selection)
	}

	report, err := batchpkg.Run(context.Background(), specs, batchpkg.Options{
		Jobs:           *jobs,
		Token:          token,
		Matcher:        matcher,
		IncludeRemotes: *includeRemotes,
		Selection:      selection,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		fmt.Println(batchpkg.JSON(report))
	} else {
		printBatchReport(report)
	}

	if report.FailedRepositories != 0 {
		return fmt.Errorf("batch audit incomplete: %d repository scan(s) failed", report.FailedRepositories)
	}
	if mode == "check" && report.Matches != 0 {
		return fmt.Errorf("batch attribution guard failed: %d matching trailer(s) across %d repository/repositories", report.Matches, report.MatchedRepositories)
	}
	return nil
}

func printBatchReport(report batchpkg.Report) {
	fmt.Printf("selection   %s\nrepos       %d\njobs        %d\n\n", report.Selection, report.Repositories, report.Jobs)
	for _, result := range report.Results {
		status := "clean"
		if result.Error != "" {
			status = "error"
		} else if result.Matches != 0 {
			status = "match"
		}
		fmt.Printf("%-5s  %-36s  %7d commits  %3d matches  prep %7s  scan %7s  total %7s\n",
			status,
			trimWidth(result.Repository, 36),
			result.Commits,
			result.Matches,
			metricDuration(result.PrepareMS),
			metricDuration(result.ScanMS),
			metricDuration(result.TotalMS),
		)
		if result.Error != "" {
			fmt.Printf("       %s\n", result.Error)
		}
	}
	fmt.Printf("\nsummary     %d scanned · %d clean · %d with matches (%.2f%%) · %d failed\n", report.Scanned, report.CleanRepositories, report.MatchedRepositories, report.RepositoryMatchPct, report.FailedRepositories)
	fmt.Printf("history     %d commits · %d matched commits (%.2f%%) · %d matching trailers\n", report.Commits, report.MatchedCommits, report.CommitMatchPct, report.Matches)
	if len(report.RuleMatches) > 0 {
		keys := make([]string, 0, len(report.RuleMatches))
		for key := range report.RuleMatches {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		fmt.Print("rules       ")
		for i, key := range keys {
			if i > 0 {
				fmt.Print(" · ")
			}
			fmt.Printf("%s=%d", key, report.RuleMatches[key])
		}
		fmt.Println()
	}
	fmt.Printf("timing      %s wall · %s prepare sum · %s scan sum\n", metricDuration(report.WallMS), metricDuration(report.PrepareMS), metricDuration(report.ScanMS))
}

func metricDuration(ms int64) string {
	if ms <= 0 {
		return "<1ms"
	}
	return time.Duration(ms * int64(time.Millisecond)).Round(time.Millisecond).String()
}

func trimWidth(s string, width int) string {
	if width < 2 || len(s) <= width {
		return s
	}
	return s[:width-1] + "…"
}
