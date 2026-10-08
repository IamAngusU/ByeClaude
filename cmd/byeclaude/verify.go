package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/IamAngusU/ByeClaude/internal/batch"
	"github.com/IamAngusU/ByeClaude/internal/clean"
	"github.com/IamAngusU/ByeClaude/internal/githubverify"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

func runVerify(args []string) error {
	fs:=flag.NewFlagSet("verify",flag.ContinueOnError)
	repoPath:=fs.String("repo","","GitHub OWNER/NAME; defaults to current repository origin")
	strict:=fs.Bool("strict",false,"exit nonzero if residual attribution or incomplete verification")
	jsonOut:=fs.Bool("json",false,"JSON verification report")
	rules:=rulesFlag(fs)
	githubUser:=fs.String("github-user","","optional GitHub login to check in contributor API")
	maxPull:=fs.Int("max-pull-refs",200,"maximum advertised PR refs to fetch and inspect (1-1000)")
	timeout:=fs.Duration("timeout",3*time.Minute,"total network verification timeout")
	if err:=fs.Parse(args);err!=nil{return err}
	if err:=githubverify.ValidateGitHubUser(*githubUser);err!=nil{return err}
	matcher,err:=resolveMatcher(*rules)
	if err!=nil{return err}
	if *timeout<=0{return fmt.Errorf("timeout must be positive")}
	target:=*repoPath
	if target=="" {
		local,err:=gitx.Open(".")
		if err!=nil {return fmt.Errorf("not inside a Git repository; pass --repo OWNER/REPO: %w",err)}
		remote,err:=local.Run("remote","get-url","origin")
		if err!=nil{return fmt.Errorf("cannot detect origin; pass --repo OWNER/REPO: %w",err)}
		target,err=canonicalGitHubRemote(string(remote))
		if err!=nil{return fmt.Errorf("origin is not a supported GitHub repository; use --repo OWNER/REPO: %w",err)}
	}
	ctx,cancel:=context.WithTimeout(context.Background(),*timeout)
	defer cancel()
	report,err:=githubverify.Audit(ctx,githubverify.Options{
		Repo:target,Token:batch.TokenFromEnv(),Matcher:matcher,
		GitHubUser:*githubUser,MaxPullRefs:*maxPull,
	})
	if err!=nil{return err}
	if *jsonOut {
		fmt.Println(clean.JSON(report))
		if *strict && report.Overall!="clean_in_checked_scopes" {
			return fmt.Errorf("GitHub verification: %s",report.Overall)
		}
		return nil
	}
	fmt.Printf("repository    %s\nverified      %s\noverall       %s\n",report.Repository,report.VerifiedAt,report.Overall)
	fmt.Printf("remote       %s (%d commits, %d trailers, %d authors, %d committers)\n",report.Remote.Status,report.Remote.Commits,report.Remote.MatchingTrailers,report.Remote.MatchingAuthors,report.Remote.MatchingCommitters)
	fmt.Printf("pull refs    %s (%d/%d inspected, %d skipped, %d trailers, %d authors, %d committers)\n",
		report.PullRefs.Status,report.PullRefs.RefsSelected,report.PullRefs.RefsAdvertised,
		report.PullRefs.RefsSkipped,report.PullRefs.MatchingTrailers,report.PullRefs.MatchingAuthors,report.PullRefs.MatchingCommitters)
	if report.PullRefs.Error!=""{fmt.Printf("pull issue   %s\n",report.PullRefs.Error)}
	if report.Contributors.GitHubUser!="" {
		fmt.Printf("contributor  %s (%s, id=%s)\n",report.Contributors.Status,report.Contributors.GitHubUser,report.Contributors.GitHubID)
		if report.Contributors.Errors!=""{fmt.Printf("api issue    %s\n",report.Contributors.Errors)}
	} else {
		fmt.Println("contributor  not requested; use --github-user LOGIN")
	}
	fmt.Println("limit        GitHub caches, external forks and old PR objects cannot be declared erased")
	fmt.Println("refresh      contributor displays may take about 24h after a rewrite; rerun verify later")
	if *strict && report.Overall!="clean_in_checked_scopes" {
		return fmt.Errorf("GitHub verification: %s",report.Overall)
	}
	return nil
}
