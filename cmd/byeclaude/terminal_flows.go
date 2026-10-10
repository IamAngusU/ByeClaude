package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
	"github.com/IamAngusU/ByeClaude/internal/blacklist"
	"github.com/IamAngusU/ByeClaude/internal/clean"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/model"
)

func (ui *terminalUI) validated(prompt string, validate func(string) error) (string, error) {
	for attempt := 0; attempt < maxPromptAttempts; attempt++ {
		value, err := ui.ask(prompt + " [Enter or q: back]")
		if err != nil {
			return "", err
		}
		if value == "" || strings.EqualFold(value, "q") {
			return "", errMenuBack
		}
		if err := validate(value); err != nil {
			ui.hint(err.Error())
			continue
		}
		return value, nil
	}
	ui.hint("No valid value entered. The pending change was cancelled.")
	return "", errMenuBack
}

func validateMenuRule(rule attribution.Rule) error {
	data, err := json.Marshal(attribution.RuleSet{Rules: []attribution.Rule{rule}})
	if err != nil {
		return err
	}
	_, err = attribution.ParseRuleSet(data)
	return err
}

func sharedWorktrees(repo *gitx.Repo) (int, error) {
	output, err := repo.Run("worktree", "list", "--porcelain")
	return strings.Count("\n"+string(output), "\nworktree "), err
}

func policySnapshot(repo *gitx.Repo) (string, error) {
	raw, found, err := repo.RunOptional("config", "--local", "--get-all", "byeclaude.blacklist")
	return fmt.Sprintf("%t:%s", found, raw), err
}

func (ui *terminalUI) manageBlacklist() error {
	repo, err := gitx.Open(ui.repo)
	if err != nil {
		return err
	}
	before, err := policySnapshot(repo)
	if err != nil {
		return err
	}
	set, _, policyErr := blacklist.Load(repo)
	ui.heading("Choose blocked identities")
	ui.hint("Rules match declared Git metadata. They do not detect who wrote code.")
	ui.hint("Adding a rule keeps the others. Existing history changes only after cleanup.")
	if policyErr == nil {
		printBlacklist(ui.out, set)
	} else {
		ui.hint("The saved blacklist is invalid. Reset it here, or q to leave it untouched.")
	}
	fmt.Fprintln(ui.out, "  a  Add exact email    r  Remove a rule    reset  Restore Claude default")
	choice, err := ui.choice("Blacklist [Enter: done, q: back]", "done", "a", "r", "reset", "done", "q")
	if err != nil {
		return err
	}
	if choice == "done" {
		return nil
	}
	if choice == "q" {
		return errMenuBack
	}
	if policyErr != nil && choice != "reset" {
		return fmt.Errorf("repair the blacklist with reset before adding or removing rules")
	}
	args := []string{choice, "--repo", ui.repo}
	switch choice {
	case "a":
		ui.hint("Give the rule a short label, such as helper-bot (letters, digits, . _ -).")
		id, err := ui.validated("Rule label", func(value string) error {
			for _, rule := range set.Rules {
				if rule.RuleID == value {
					return fmt.Errorf("That label already exists. Choose another label or remove the old rule first.")
				}
			}
			return validateMenuRule(attribution.Rule{RuleID: value, ExactEmails: []string{"example@example.org"}})
		})
		if err != nil {
			return err
		}
		ui.hint("Copy the exact email from a commit's co-author credit, e.g. helper@example.org.")
		email, err := ui.validated("Email to block", func(value string) error {
			return validateMenuRule(attribution.Rule{RuleID: id, ExactEmails: []string{value}})
		})
		if err != nil {
			return err
		}
		email = strings.ToLower(email)
		ui.hint("Save rule " + id + " for exact email " + email + ".")
		args = []string{"add", "--repo", ui.repo, "--id", id, "--email", email}
	case "r":
		if len(set.Rules) == 1 {
			return fmt.Errorf("this is the last rule; add a replacement before removing it")
		}
		for i, rule := range set.Rules {
			fmt.Fprintf(ui.out, "  %d  %s\n", i+1, rule.RuleID)
		}
		selected := ""
		_, err := ui.validated("Rule label or number to remove", func(value string) error {
			for _, rule := range set.Rules {
				if rule.RuleID == value {
					selected = value
					return nil
				}
			}
			if n, err := strconv.Atoi(value); err == nil && n > 0 && n <= len(set.Rules) {
				selected = set.Rules[n-1].RuleID
				return nil
			}
			return fmt.Errorf("Choose one of the listed labels or numbers.")
		})
		if err != nil {
			return err
		}
		ui.hint("Remove " + selected + ". Default hooks will stop using this rule.")
		args = []string{"remove", "--repo", ui.repo, "--id", selected}
	case "reset":
		ui.hint("Replace all saved rules with the built-in Claude/Anthropic rule.")
	}
	count, err := sharedWorktrees(repo)
	if err != nil {
		return err
	}
	if count > 1 {
		ui.hint(fmt.Sprintf("This changes the policy shared by all %d linked worktrees.", count))
		args = append(args, "--shared-worktrees")
	}
	if choice == "reset" {
		ok, err := ui.typedConfirmation("RESET", "replace the saved blacklist")
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	} else {
		ok, err := ui.confirm("Save this blacklist change?")
		if err != nil {
			return err
		}
		if !ok {
			ui.hint("Blacklist unchanged.")
			return nil
		}
	}
	after, err := policySnapshot(repo)
	if err != nil {
		return err
	}
	if before != after {
		return fmt.Errorf("the blacklist changed while you were editing; review it again before saving")
	}
	return ui.invoke("blacklist", args)
}

func (ui *terminalUI) typedConfirmation(word, purpose string) (bool, error) {
	for attempt := 0; attempt < maxPromptAttempts; attempt++ {
		value, err := ui.ask("Type " + word + " to " + purpose + " [Enter: cancel, q: back]")
		if err != nil {
			return false, err
		}
		if value == "" {
			ui.hint("Cancelled. The pending change was not applied.")
			return false, nil
		}
		if strings.EqualFold(value, "q") {
			return false, errMenuBack
		}
		if value == word {
			return true, nil
		}
		ui.hint("That was not a confirmation. Type exactly " + word + ", or press Enter to cancel.")
	}
	return false, errMenuBack
}

func (ui *terminalUI) protect(repo *gitx.Repo) error {
	ui.heading("Protect future commits / Preview")
	ui.hint("The commit hook removes matching co-author credit lines from new commits.")
	ui.hint("The push hook blocks matching credits, authors or committers in history.")
	ui.hint("Existing history is not cleaned by installing hooks.")
	before, err := reviewState(repo)
	if err != nil {
		return err
	}
	args := []string{"--repo", ui.repo}
	if err := ui.invoke("setup", args); err != nil {
		return err
	}
	count, err := sharedWorktrees(repo)
	if err != nil {
		return err
	}
	if count > 1 {
		ui.hint(fmt.Sprintf("These hooks affect all %d linked worktrees.", count))
		args = append(args, "--shared-worktrees")
	}
	ok, err := ui.confirm("Install both hooks?")
	if err != nil {
		return err
	}
	if !ok {
		ui.hint("No hooks changed. Choose 4 whenever you are ready.")
		return nil
	}
	after, err := reviewState(repo)
	if err != nil {
		return err
	}
	if before != after {
		return fmt.Errorf("repository or blacklist changed during review; preview protection again")
	}
	return ui.invoke("setup", append(args, "--apply"))
}

func (ui *terminalUI) cleanup(repo *gitx.Repo) error {
	ui.heading("Preview cleanup / Choose what changes")
	useUnpushed := false
	if _, upstreamErr := repo.Run("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); upstreamErr == nil {
		ui.option("u", "Only unpublished commits on this branch (recommended)", "Verifies the live upstream first; other branches and tags stay unchanged.")
		ui.option("a", "All local branches and tags", "Use when matching metadata was already published or lives elsewhere.")
		scope, err := ui.choice("History scope [Enter: u, q: back]", "u", "u", "a", "q")
		if err != nil {
			return err
		}
		if scope == "q" {
			return errMenuBack
		}
		useUnpushed = scope == "u"
	} else {
		ui.hint("No configured upstream was found, so this preview covers all local branches and tags.")
	}
	ui.option("1", "Remove matching co-author credit lines (recommended)", "Keeps actual author and committer fields unchanged.")
	ui.option("2", "Also correct matching authors", "Advanced: use your Git identity only if it is the actual author.")
	ui.option("3", "Also correct matching committers", "Advanced: use your Git identity only if it is the actual committer.")
	ui.option("4", "Also correct both identity fields", "Advanced: replaces only identities matching your blacklist.")
	mode, err := ui.choice("Preview mode [Enter: 1, q: back]", "1", "1", "2", "3", "4", "q")
	if err != nil {
		return err
	}
	if mode == "q" {
		return errMenuBack
	}
	args := []string{"--repo", ui.repo}
	if useUnpushed {
		args = append(args, "--unpushed")
	}
	opts := clean.IdentityRewriteOptions{}
	if mode != "1" {
		identity, err := configuredGitIdentity(repo)
		if err != nil {
			return err
		}
		if mode == "2" || mode == "4" {
			opts.Author = &identity
		}
		if mode == "3" || mode == "4" {
			opts.Committer = &identity
		}
		flag := map[string]string{"2": "--author-from-git", "3": "--committer-from-git", "4": "--identity-from-git"}[mode]
		args = append(args, flag)
		ui.hint("Replacement: " + identity.Name + " <" + identity.Email + ">. Check actual authorship.")
	}
	before, err := reviewState(repo)
	if err != nil {
		return err
	}
	matcher, err := resolveLocalMatcher(repo.Root, "")
	if err != nil {
		return err
	}
	ui.hint("Calculating the impact on local history. No changes yet...")
	var plan model.PlanReport
	err = ui.working("Review cleanup impact", func(ctx context.Context) error {
		var planErr error
		if useUnpushed {
			plan, planErr = clean.PlanUnpushedWithIdentityContext(ctx, repo, matcher, opts)
		} else {
			plan, planErr = clean.PlanWithIdentityContext(ctx, repo, matcher, opts)
		}
		return planErr
	})
	if err != nil {
		return err
	}
	ui.rule()
	fmt.Fprintf(ui.out, "  %-27s %s\n", "Scope", terminalText(plan.Scope))
	if plan.Upstream != "" {
		fmt.Fprintf(ui.out, "  %-27s %s\n", "Upstream", terminalText(plan.Upstream)+" (live verified)")
	}
	ui.stat("Commits checked", plan.Commits)
	ui.stat("Matching commits", plan.MatchedCommits)
	ui.stat("Credit lines to remove", len(plan.Matches))
	ui.stat("Commits to rewrite", plan.CommitsToRewrite)
	ui.stat("Branches to move", plan.BranchesToMove)
	ui.stat("Tags to move", plan.TagRefsToMove)
	ui.stat("Signatures lost", plan.SignaturesAtRisk)
	ui.rule()
	if useUnpushed {
		ui.hint("Only commits above the verified upstream can change in this preview.")
	} else {
		ui.hint("Rewritten commits include descendants of matching commits.")
	}
	if plan.AuthorsToReplace+plan.CommittersToReplace > 0 {
		fmt.Fprintf(ui.out, "  Identity changes  %d author(s), %d committer(s)\n", plan.AuthorsToReplace, plan.CommittersToReplace)
	}
	for _, ref := range plan.AffectedRefs {
		fmt.Fprintln(ui.out, "    "+terminalText(ref))
	}
	if plan.MatchedCommits == 0 {
		ui.hint("Nothing matches the selected cleanup. No rewrite is needed.")
		ui.hint("Check (1) also reports authors and committers. Protect future work with 4.")
		return nil
	}
	if !plan.RewriteReady {
		ui.hint("Cleanup is blocked: " + plan.RewriteBlocker)
		ui.hint("Resolve this first, then preview again. No history changed.")
		return nil
	}
	ui.hint("Committed file contents stay the same. Commit IDs and affected signatures change.")
	ui.hint("A local backup is created. Nothing is pushed to GitHub.")
	ok, err := ui.typedConfirmation("CLEAN", "apply this cleanup locally")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	after, err := reviewState(repo)
	if err != nil {
		return err
	}
	if before != after {
		return fmt.Errorf("repository refs, identity or blacklist changed during review; preview again")
	}
	if err := ui.invoke("clean", append(args, "--apply")); err != nil {
		return err
	}
	ui.hint("Local cleanup finished. Keep the backup ID; choose 6 for recovery instructions.")
	if useUnpushed {
		ui.hint("Published history stayed unchanged. A normal git push is usually enough.")
	} else {
		ui.hint("To publish later, open a terminal in this repository and run byeclaude push.")
		ui.hint("Coordinate with collaborators before publishing rewritten history.")
	}
	return nil
}
