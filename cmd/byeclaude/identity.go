package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	batchpkg "github.com/IamAngusU/ByeClaude/internal/batch"
	"github.com/IamAngusU/ByeClaude/internal/clean"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

func runIdentity(args []string) error {
	fs := flag.NewFlagSet("identity", flag.ContinueOnError)
	repoArg := fs.String("repo", ".", "local repository path, GitHub OWNER/NAME, or clone URL")
	includeRemotes := fs.Bool("include-remotes", true, "include fetched remote-tracking refs")
	includePullRefs := fs.Bool("include-pull-refs", true, "include refs/pull/* when present")
	jsonOut := fs.Bool("json", false, "machine-readable JSON")
	var githubIDs repeatedFlag
	fs.Var(&githubIDs, "github-id", "numeric GitHub account ID to locate; repeat for multiple IDs")
	if err := fs.Parse(args); err != nil {
		return err
	}

	specs, err := batchpkg.ResolveExplicit([]string{*repoArg})
	if err != nil {
		return err
	}
	if len(specs) != 1 {
		return fmt.Errorf("identity audit requires exactly one repository")
	}
	path, cleanup, err := batchpkg.PrepareRepository(context.Background(), specs[0], batchpkg.TokenFromEnv(), false)
	if err != nil {
		return err
	}
	defer cleanup()

	repo, err := gitx.Open(path)
	if err != nil {
		return err
	}
	report, err := clean.ScanIdentities(repo, clean.IdentityScanOptions{
		IncludeRemotes:  *includeRemotes,
		IncludePullRefs: *includePullRefs,
		GitHubIDs:       githubIDs,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		fmt.Println(clean.JSON(report))
		return nil
	}

	fmt.Printf("repository   %s\n", specs[0].Name)
	fmt.Printf("commits      %d\n", report.Commits)
	fmt.Printf("authors      %d git identity/identities\n", len(report.Authors))
	fmt.Printf("duration     %s\n", metricDuration(report.DurationMS))
	if len(githubIDs) == 0 {
		fmt.Println("\nauthors")
		for _, author := range report.Authors {
			id := ""
			if author.GitHubID != "" {
				id = " github-id=" + author.GitHubID
			}
			fmt.Printf("  %5d  %s <%s>%s\n", author.Commits, author.Name, author.Email, id)
		}
		return nil
	}

	fmt.Printf("matches      %d\n", len(report.Matches))
	fmt.Printf("managed      %d\n", report.ManagedMatches)
	fmt.Printf("pull-only    %d\n", report.PullOnlyMatches)
	for _, match := range report.Matches {
		kind := "other"
		if match.PullRefOnly {
			kind = "pull-only"
		} else if match.Managed {
			kind = "managed"
		}
		fmt.Printf("  %.12s  github-id=%s  %s <%s>  [%s]\n", match.Commit, match.GitHubID, match.Name, match.Email, kind)
		for _, ref := range match.ReachableBy {
			fmt.Printf("              %s\n", ref)
		}
	}
	if len(report.Matches) == 0 {
		fmt.Printf("clean        github id(s) %s not found in selected refs\n", strings.Join(githubIDs, ","))
	}
	return nil
}
