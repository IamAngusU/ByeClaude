package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
	"github.com/IamAngusU/ByeClaude/internal/blacklist"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

func resolveLocalMatcher(repoPath, rulesFile string) (attribution.Matcher, error) {
	repo, err := gitx.Open(repoPath)
	if err != nil {
		return nil, err
	}
	return blacklist.Resolve(repo, rulesFile)
}

func runBlacklist(args []string) error { return blacklistCommand(args, os.Stdout) }

func blacklistCommand(args []string, out io.Writer) error {
	if len(args) == 0 {
		args = []string{"list"}
	}
	action := args[0]
	fs := flag.NewFlagSet("blacklist "+action, flag.ContinueOnError)
	repoPath := fs.String("repo", ".", "local repository")
	id := fs.String("id", "", "unique rule id (for add/remove)")
	jsonOut := fs.Bool("json", false, "export the active blacklist as JSON")
	shared := fs.Bool("shared-worktrees", false, "acknowledge shared worktree policy changes")
	var names, emails, domains repeatedFlag
	fs.Var(&names, "name", "name substring; repeat for alternatives")
	fs.Var(&emails, "email", "exact email; repeat for alternatives (recommended)")
	fs.Var(&domains, "domain", "email domain; repeat for alternatives")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s; use --id for a rule id", strings.Join(fs.Args(), " "))
	}
	if action != "list" && action != "export" && action != "add" && action != "remove" && action != "reset" && action != "test" {
		return fmt.Errorf("blacklist requires list, add, remove, reset, export or test")
	}
	if action != "add" && action != "test" && len(names)+len(emails)+len(domains) > 0 {
		return fmt.Errorf("identity flags require blacklist add or test")
	}
	if action != "add" && action != "remove" && *id != "" {
		return fmt.Errorf("--id requires add or remove")
	}
	if action == "test" && (len(names) > 1 || len(emails) != 1 || len(domains) > 0) {
		return fmt.Errorf("blacklist test requires one --email and at most one --name")
	}
	repo, err := gitx.Open(*repoPath)
	if err != nil {
		return err
	}
	set, saved, err := blacklist.Load(repo)
	if err != nil && action != "reset" {
		return err
	}
	switch action {
	case "list", "export":
		if *jsonOut || action == "export" {
			return json.NewEncoder(out).Encode(set)
		}
		if saved {
			fmt.Fprintln(out, "Blacklist: saved in local Git config")
		} else {
			fmt.Fprintln(out, "Blacklist: built-in Claude/Anthropic default")
		}
		printBlacklist(out, set)
		return nil
	case "test":
		name := ""
		if len(names) == 1 {
			name = names[0]
		}
		ids := set.MatchIDs(name, emails[0])
		if *jsonOut {
			return json.NewEncoder(out).Encode(struct {
				Rules []string `json:"matching_rules"`
			}{ids})
		}
		if len(ids) == 0 {
			fmt.Fprintln(out, "No blacklist rule matches this identity.")
		} else {
			fmt.Fprintln(out, "Matching rules:", strings.Join(ids, ", "))
		}
		return nil
	case "add":
		if *id == "" {
			return fmt.Errorf("add requires --id and at least one --email, --name or --domain")
		}
		for _, rule := range set.Rules {
			if rule.RuleID == *id {
				return fmt.Errorf("rule %q already exists; remove it explicitly before replacing it", *id)
			}
		}
		set.Rules = append(set.Rules, attribution.Rule{RuleID: *id, NameContains: names, ExactEmails: emails, EmailDomains: domains})
	case "remove":
		found := false
		for i, rule := range set.Rules {
			if rule.RuleID == *id {
				set.Rules = append(set.Rules[:i], set.Rules[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("unknown blacklist id %q", *id)
		}
		if len(set.Rules) == 0 {
			return fmt.Errorf("cannot remove the last rule; add a replacement first, or reset to Claude")
		}
	case "reset":
		set = blacklist.Default()
	}
	// Explicit --rules hooks intentionally use a different policy; never report
	// a saved blacklist as protection while those hooks bypass it.
	for _, name := range []string{"commit-msg", "pre-push"} {
		state, err := inspectHook(repo, name)
		if err != nil {
			return err
		}
		if state.RulesFile != "" {
			return fmt.Errorf("%s uses an explicit rules file; remove that managed hook and rerun setup before managing the saved blacklist", name)
		}
	}
	if err := blacklist.Save(repo, set, *shared); err != nil {
		return err
	}
	if *jsonOut {
		return json.NewEncoder(out).Encode(set)
	}
	fmt.Fprintln(out, "Saved. Local scan, check, plan, clean and default hooks use this blacklist.")
	printBlacklist(out, set)
	return nil
}

func printBlacklist(out io.Writer, set attribution.RuleSet) {
	for _, rule := range set.Rules {
		fmt.Fprintf(out, "  %s\n", rule.RuleID)
		if len(rule.NameContains) > 0 {
			fmt.Fprintf(out, "    name contains: %q\n", rule.NameContains)
		}
		if len(rule.ExactEmails) > 0 {
			fmt.Fprintf(out, "    exact emails:  %q\n", rule.ExactEmails)
		}
		if len(rule.EmailDomains) > 0 {
			fmt.Fprintf(out, "    email domains: %q\n", rule.EmailDomains)
		}
	}
}
