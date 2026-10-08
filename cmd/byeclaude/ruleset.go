package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/IamAngusU/ByeClaude/internal/batch"
	"github.com/IamAngusU/ByeClaude/internal/githubpolicy"
)

func runRuleset(args []string) error {
	if len(args)==0 {
		return fmt.Errorf("usage: byeclaude ruleset export|install|status --repo OWNER/REPO")
	}
	action:=args[0]
	fs:=flag.NewFlagSet("ruleset "+action,flag.ContinueOnError)
	repo:=fs.String("repo","","GitHub repository OWNER/NAME")
	rules:=rulesFlag(fs)
	identities:=fs.Bool("include-identities",false,"also block author and committer emails matching selected rules")
	allBranches:=fs.Bool("all-branches",false,"enforce on all branches instead of only the default branch")
	confirm:=fs.Bool("confirm",false,"explicitly approve a GitHub repository ruleset creation")
	if err:=fs.Parse(args[1:]);err!=nil{return err}
	if err:=githubpolicy.ValidateRepo(*repo);err!=nil{return err}
	client:=githubpolicy.Client{Token:batch.TokenFromEnv()}
	ctx:=context.Background()
	switch action {
	case "status":
		existing,err:=client.ListRulesets(ctx,*repo)
		if err!=nil{return err}
		data,_:=json.MarshalIndent(existing,"","  ")
		fmt.Println(string(data))
		return nil
	case "export","install":
		matcher,err:=resolveMatcher(*rules)
		if err!=nil{return err}
		policy,err:=githubpolicy.GenerateRuleset(matcher,*identities)
		if err!=nil{return err}
		if *allBranches {policy.Conditions.RefName.Include=[]string{"~ALL"}}
		if action=="export" || !*confirm {
			data,_:=json.MarshalIndent(policy,"","  ")
			fmt.Println(string(data))
			if action=="install" {
				fmt.Fprintln(os.Stderr,"dry run: GitHub unchanged. Add --confirm to create this ruleset.")
			}
			return nil
		}
		installed,err:=client.InstallRuleset(ctx,*repo,policy)
		if err!=nil{return err}
		fmt.Printf("installed GitHub ruleset %d (%s) on %s\n",installed.ID,installed.Name,*repo)
		return nil
	default:
		return fmt.Errorf("unknown ruleset action %q",action)
	}
}
