package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/IamAngusU/ByeClaude/internal/clean"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

func runSetup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	repoPath := fs.String("repo", ".", "existing local Git repository")
	rulesFile := rulesFlag(fs)
	apply := fs.Bool("apply", false, "install both local hooks (read-only without this flag)")
	shared := fs.Bool("shared-worktrees", false, "acknowledge that hooks affect all linked Git worktrees")
	if err := fs.Parse(args); err != nil {
		return err
	}
	repo, err := gitx.Open(*repoPath)
	if err != nil {
		return err
	}
	matcher, err := resolveMatcher(*rulesFile)
	if err != nil {
		return err
	}
	override, err := effectiveHooksPath(repo)
	if err != nil {
		return err
	}
	if override != "" {
		return fmt.Errorf("custom core.hooksPath=%q: cannot safely install default hooks; integrate ByeClaude into that hook manager", override)
	}
	// Worktrees share hooks stored in the common Git directory. Require an
	// explicit acknowledgement before modifying another worktree's behavior.
	out, err := repo.Run("worktree", "list", "--porcelain")
	if err != nil {
		return err
	}
	linked := strings.Count("\n"+string(out), "\nworktree ")
	if *apply && linked > 1 && !*shared {
		return fmt.Errorf("%d linked worktrees share these hooks; rerun with --shared-worktrees if this is intentional", linked)
	}
	kinds := []string{"commit-msg", "pre-push"}
	states := make([]inspectedHook, 0, len(kinds))
	for _, name := range kinds {
		status, err := inspectHook(repo, name)
		if err != nil {
			return err
		}
		if status.Status == "conflict" || status.Status == "not_executable" || status.Status == "stale_binary" {
			return fmt.Errorf("%s hook requires manual attention (%s): %s; setup will not overwrite it", name, status.Status, status.Path)
		}
		states = append(states, status)
	}
	scan, err := clean.Scan(repo, matcher)
	if err != nil {
		return err
	}
	headers, err := clean.ScanMatchingHeadersContext(context.Background(), repo, matcher, []string{"refs/heads", "refs/tags"}, true)
	if err != nil {
		return err
	}
	fmt.Printf("repository   %s\n", repo.Root)
	fmt.Printf("existing     %d matching trailer(s), %d author(s), %d committer(s) in local history\n",
		len(scan.Matches), headers.Authors, headers.Committers)
	for _, state := range states {
		fmt.Printf("%-12s %s\n", state.Name, state.Status)
	}
	if !*apply {
		fmt.Println("dry run      no hooks changed")
		fmt.Println("next         byeclaude setup --apply")
		if linked > 1 {
			fmt.Println("note         linked worktrees share hooks; --shared-worktrees acknowledges this")
		}
		return nil
	}
	var installed []string
	for _, state := range states {
		if state.Status == "installed" {
			continue
		}
		action := "install"
		if state.Name == "pre-push" {
			action = "pre-push-install"
		}
		installArgs := []string{action, "--repo", repo.Root}
		if strings.TrimSpace(*rulesFile) != "" {
			installArgs = append(installArgs, "--rules", *rulesFile)
		}
		if err := runHook(installArgs); err != nil {
			for i := len(installed) - 1; i >= 0; i-- {
				rollback := "remove"
				if installed[i] == "pre-push" {
					rollback = "pre-push-remove"
				}
				if rollbackErr := runHook([]string{rollback, "--repo", repo.Root}); rollbackErr != nil {
					fmt.Fprintln(os.Stderr, "rollback error:", rollbackErr)
				}
			}
			return fmt.Errorf("setup stopped at %s: %w", state.Name, err)
		}
		installed = append(installed, state.Name)
	}
	fmt.Println("ready        commit-msg and pre-push hooks installed")
	if len(scan.Matches) > 0 || headers.Authors+headers.Committers > 0 {
		fmt.Println("note         pre-push examines reachable history; existing matches can block your first push")
		fmt.Println("next         byeclaude plan")
	}
	fmt.Println("check        byeclaude doctor")
	return nil
}

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	repoPath := fs.String("repo", ".", "existing local Git repository")
	if err := fs.Parse(args); err != nil {
		return err
	}
	repo, err := gitx.Open(*repoPath)
	if err != nil {
		return err
	}
	fmt.Printf("repository   %s\n", repo.Root)
	override, err := effectiveHooksPath(repo)
	if err != nil {
		return err
	}
	if override != "" {
		fmt.Printf("hooksPath    externally managed: %s\n", override)
		fmt.Println("next         integrate with your existing hook manager")
		return nil
	}
	for _, name := range []string{"commit-msg", "pre-push"} {
		status, err := inspectHook(repo, name)
		if err != nil {
			return err
		}
		fmt.Printf("%-12s %-18s %s\n", name, status.Status, status.Path)
	}
	fmt.Println("note         hooks apply to this Git repository, not GitHub web/API commits")
	fmt.Println("next         byeclaude setup --apply (if a hook is missing)")
	return nil
}
