package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
	"github.com/IamAngusU/ByeClaude/internal/clean"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/model"
	"github.com/IamAngusU/ByeClaude/internal/preset"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "scan":
		err = runScan(os.Args[2:])
	case "check":
		err = runCheck(os.Args[2:])
	case "plan":
		err = runPlan(os.Args[2:])
	case "identity":
		err = runIdentity(os.Args[2:])
	case "clean":
		err = runClean(os.Args[2:])
	case "batch":
		err = runBatch(os.Args[2:])
	case "serve":
		err = runServe(os.Args[2:])
	case "push":
		err = runPush(os.Args[2:])
	case "hook":
		err = runHook(os.Args[2:])
	case "backups":
		err = runBackups(os.Args[2:])
	case "restore":
		err = runRestore(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("byeclaude", version)
		return
	case "help", "--help", "-h":
		usage()
		return
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`ByeClaude audits and removes matching Co-Authored-By attribution from Git history.

Usage:
  byeclaude scan [--repo PATH] [--include-remotes] [--rules FILE] [--json]
  byeclaude check [--repo PATH] [--include-remotes] [--rules FILE] [--json]
  byeclaude plan [--repo PATH] [--rules FILE] [--json]
  byeclaude identity [--repo PATH|OWNER/NAME] [--github-user LOGIN ...] [--github-id ID ...] [--json]
  byeclaude batch scan --repo OWNER/NAME [--repo ...] [--jobs N] [--json]
  byeclaude batch scan --owner OWNER [--public|--private|--all] [--jobs N] [--json]
  byeclaude batch check ...
  byeclaude batch plan ...
  byeclaude serve [--listen 127.0.0.1:8080] [--max-inflight 2] [--timeout 60s] [--rules FILE]
  byeclaude clean --apply [--repo PATH] [--rules FILE] [--push] [--remote origin] [--json]
  byeclaude push --backup ID [--repo PATH] [--rules FILE] [--remote origin]
  byeclaude hook install|remove [--repo PATH] [--rules FILE]
  byeclaude backups [--repo PATH]
  byeclaude restore --backup ID --apply [--repo PATH]
  byeclaude version

Nothing is rewritten unless --apply is present. Remote writes require either --push on clean or the explicit push command.
`)
}

func rulesFlag(fs *flag.FlagSet) *string {
	return fs.String("rules", "", "structured attribution rules JSON; defaults to the built-in Claude/Anthropic rule")
}

func resolveMatcher(path string) (attribution.Matcher, error) {
	return preset.Resolve(path)
}

func common(fs *flag.FlagSet) (*string, *bool) {
	repo := fs.String("repo", ".", "repository path")
	jsonOut := fs.Bool("json", false, "machine-readable JSON")
	return repo, jsonOut
}

func runScan(args []string) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	repoPath, jsonOut := common(fs)
	includeRemotes := fs.Bool("include-remotes", false, "also scan fetched remote-tracking refs")
	rulesFile := rulesFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	matcher, err := resolveMatcher(*rulesFile)
	if err != nil {
		return err
	}
	repo, err := gitx.Open(*repoPath)
	if err != nil {
		return err
	}
	report, err := clean.ScanIncludingRemotes(repo, *includeRemotes, matcher)
	if err != nil {
		return err
	}
	if *jsonOut {
		fmt.Println(clean.JSON(report))
		return nil
	}
	fmt.Printf("repository  %s\ncommits     %d\nmatched     %d (%.2f%%)\ntrailers    %d\nduration    %s\n", report.Repository, report.Commits, report.MatchedCommits, report.CommitMatchPct, len(report.Matches), metricDuration(report.DurationMS))
	for _, m := range report.Matches {
		fmt.Printf("  %.12s  [%s] %s <%s>\n", m.Commit, strings.Join(m.Rules, ","), m.AttributionName, m.AttributionEmail)
	}
	if len(report.Matches) == 0 {
		fmt.Println("clean       no matching attribution trailers found")
	}
	return nil
}

func runCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	repoPath, jsonOut := common(fs)
	includeRemotes := fs.Bool("include-remotes", false, "also scan fetched remote-tracking refs")
	rulesFile := rulesFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	matcher, err := resolveMatcher(*rulesFile)
	if err != nil {
		return err
	}
	repo, err := gitx.Open(*repoPath)
	if err != nil {
		return err
	}
	report, err := clean.ScanIncludingRemotes(repo, *includeRemotes, matcher)
	if err != nil {
		return err
	}
	if *jsonOut {
		fmt.Println(clean.JSON(report))
	} else {
		fmt.Printf("repository  %s\ncommits     %d\nmatched     %d (%.2f%%)\ntrailers    %d\nduration    %s\n", report.Repository, report.Commits, report.MatchedCommits, report.CommitMatchPct, len(report.Matches), metricDuration(report.DurationMS))
		for _, m := range report.Matches {
			fmt.Printf("  %.12s  [%s] %s <%s>\n", m.Commit, strings.Join(m.Rules, ","), m.AttributionName, m.AttributionEmail)
		}
	}
	if len(report.Matches) != 0 {
		return fmt.Errorf("attribution guard failed: %d matching attribution trailer(s) found", len(report.Matches))
	}
	if !*jsonOut {
		fmt.Println("clean       attribution guard passed")
	}
	return nil
}

func runPlan(args []string) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	repoPath, jsonOut := common(fs)
	rulesFile := rulesFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	matcher, err := resolveMatcher(*rulesFile)
	if err != nil {
		return err
	}
	repo, err := gitx.Open(*repoPath)
	if err != nil {
		return err
	}
	report, err := clean.Plan(repo, matcher)
	if err != nil {
		return err
	}
	if *jsonOut {
		fmt.Println(clean.JSON(report))
		return nil
	}
	printPlanReport(report)
	return nil
}

func printPlanReport(report model.PlanReport) {
	fmt.Printf("repository   %s\n", report.Repository)
	fmt.Printf("commits      %d\n", report.Commits)
	fmt.Printf("matched      %d (%.2f%%)\n", report.MatchedCommits, report.CommitMatchPct)
	fmt.Printf("trailers     %d\n", len(report.Matches))
	fmt.Printf("rewrite      %d commit(s)\n", report.CommitsToRewrite)
	fmt.Printf("descendants  %d\n", report.DescendantCommits)
	fmt.Printf("connections  %d parent link(s)\n", report.ParentLinksToRewrite)
	fmt.Printf("refs         %d (%d branch(es), %d tag ref(s))\n", report.RefsToMove, report.BranchesToMove, report.TagRefsToMove)
	fmt.Printf("tag objects  %d annotated tag(s)\n", report.AnnotatedTagsToRewrite)
	fmt.Printf("signatures   %d at risk\n", report.SignaturesAtRisk)
	fmt.Printf("objects      ~%d write(s)\n", report.ObjectWritesEstimate)
	if report.RewriteReady {
		fmt.Printf("ready        yes\n")
	} else {
		fmt.Printf("ready        no · %s\n", report.RewriteBlocker)
	}
	fmt.Printf("duration     %s\n", metricDuration(report.DurationMS))
	if len(report.AffectedRefs) > 0 {
		fmt.Println("affected")
		for _, ref := range report.AffectedRefs {
			fmt.Printf("  %s\n", ref)
		}
	}
}

func runClean(args []string) error {
	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	repoPath, jsonOut := common(fs)
	apply := fs.Bool("apply", false, "rewrite local history")
	push := fs.Bool("push", false, "push rewritten refs using explicit force-with-lease")
	remote := fs.String("remote", "origin", "remote to push")
	rulesFile := rulesFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	matcher, err := resolveMatcher(*rulesFile)
	if err != nil {
		return err
	}
	repo, err := gitx.Open(*repoPath)
	if err != nil {
		return err
	}
	plan, err := clean.Plan(repo, matcher)
	if err != nil {
		return err
	}
	if len(plan.Matches) == 0 {
		fmt.Println("No matching attribution trailers found. Nothing to do.")
		return nil
	}
	if !*apply {
		printPlanReport(plan)
		fmt.Println("dry run      no refs changed; re-run with --apply to rewrite locally")
		return nil
	}
	oldRefs, err := clean.LocalRefs(repo)
	if err != nil {
		return err
	}
	report, _, err := clean.Rewrite(repo, matcher)
	if err != nil {
		return err
	}
	if *push {
		if err := clean.Push(repo, *remote, oldRefs); err != nil {
			return fmt.Errorf("local rewrite succeeded, push failed: %w", err)
		}
	}
	after, err := clean.Scan(repo, matcher)
	if err != nil {
		return err
	}
	if len(after.Matches) != 0 {
		return fmt.Errorf("verification failed: %d matching trailers remain", len(after.Matches))
	}
	if *jsonOut {
		fmt.Println(clean.JSON(report))
		return nil
	}
	fmt.Printf("backup      %s\nrewritten   %d commit(s)\nrefs        %d updated\ntags        %d rewritten\nduration    %s\n", report.Backup, report.CommitsRewritten, report.RefsUpdated, report.TagsRewritten, metricDuration(report.DurationMS))
	if report.SignaturesDropped > 0 {
		fmt.Printf("signatures  %d signature/mergetag field(s) dropped because rewritten objects cannot retain valid signatures\n", report.SignaturesDropped)
	}
	if *push {
		fmt.Printf("push        %s updated with force-with-lease\n", *remote)
	} else {
		fmt.Println("push        not requested; GitHub is unchanged")
	}
	fmt.Println("verify      no matching attribution trailers remain in local heads/tags")
	return nil
}

func runPush(args []string) error {
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	repoPath, _ := common(fs)
	backup := fs.String("backup", "", "backup ID printed by clean --apply")
	remote := fs.String("remote", "origin", "remote to update")
	rulesFile := rulesFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *backup == "" {
		return fmt.Errorf("--backup is required; use the ID printed by clean --apply")
	}
	matcher, err := resolveMatcher(*rulesFile)
	if err != nil {
		return err
	}
	repo, err := gitx.Open(*repoPath)
	if err != nil {
		return err
	}
	report, err := clean.Scan(repo, matcher)
	if err != nil {
		return err
	}
	if len(report.Matches) != 0 {
		return fmt.Errorf("local history still contains %d matching trailer(s); refusing to publish", len(report.Matches))
	}
	if err := clean.PushBackup(repo, *remote, *backup); err != nil {
		return err
	}
	fmt.Printf("push        %s updated from rewrite backup %s using atomic force-with-lease\n", *remote, *backup)
	return nil
}

func runBackups(args []string) error {
	fs := flag.NewFlagSet("backups", flag.ContinueOnError)
	repoPath, _ := common(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	repo, err := gitx.Open(*repoPath)
	if err != nil {
		return err
	}
	ids, err := clean.BackupRefs(repo)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		fmt.Println("No ByeClaude backups found.")
		return nil
	}
	for _, id := range ids {
		fmt.Println(id)
	}
	return nil
}

func runRestore(args []string) error {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	repoPath, _ := common(fs)
	backup := fs.String("backup", "", "backup ID from byeclaude backups")
	apply := fs.Bool("apply", false, "restore refs from the selected backup")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *backup == "" {
		return fmt.Errorf("--backup is required")
	}
	repo, err := gitx.Open(*repoPath)
	if err != nil {
		return err
	}
	if !*apply {
		fmt.Printf("Dry run only. Re-run with: byeclaude restore --backup %s --apply\n", *backup)
		return nil
	}
	count, err := clean.Restore(repo, *backup)
	if err != nil {
		return err
	}
	fmt.Printf("Restored %d ref(s) from backup %s. Remote refs were not changed.\n", count, *backup)
	return nil
}

func runHook(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("hook requires install or remove")
	}
	action := args[0]
	fs := flag.NewFlagSet("hook "+action, flag.ContinueOnError)
	repoPath, _ := common(fs)
	rulesFile := rulesFlag(fs)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	repo, err := gitx.Open(*repoPath)
	if err != nil {
		return err
	}
	path := filepath.Join(repo.GitDir, "hooks", "commit-msg")
	switch action {
	case "install":
		matcherPath := strings.TrimSpace(*rulesFile)
		if matcherPath != "" {
			absRules, err := filepath.Abs(matcherPath)
			if err != nil {
				return err
			}
			if _, err := resolveMatcher(absRules); err != nil {
				return err
			}
			matcherPath = filepath.ToSlash(absRules)
		}
		if configured, ok, err := repo.RunOptional("config", "--path", "--get", "core.hooksPath"); err != nil {
			return err
		} else if ok && strings.TrimSpace(string(configured)) != "" {
			return fmt.Errorf("core.hooksPath is configured as %q; refusing to install into a hook directory that may be shared or externally managed", strings.TrimSpace(string(configured)))
		}
		// The directory is Git's resolved per-repository hooks directory. Hooks
		// must be executable by the repository owner and readable by Git.
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil { // #nosec G301,G703 -- resolved per-repository Git hooks directory and standard mode
			return err
		}
		if info, err := os.Lstat(path); err == nil { // #nosec G703 -- resolved Git directory plus hooks/commit-msg
			if !info.Mode().IsRegular() {
				return fmt.Errorf("refusing to replace non-regular commit-msg hook at %s", path)
			}
			b, err := os.ReadFile(path) // #nosec G304,G703 -- path is the resolved Git directory plus hooks/commit-msg
			if err != nil {
				return err
			}
			if strings.Contains(string(b), "Installed by ByeClaude") {
				fmt.Println("Already installed", path)
				return nil
			}
			return fmt.Errorf("refusing to overwrite existing commit-msg hook at %s", path)
		} else if !os.IsNotExist(err) {
			return err
		}
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		exe, _ = filepath.Abs(exe)
		// Git for Windows executes hooks through its POSIX shell. Forward slashes
		// keep the executable path valid there and are harmless on Unix hosts.
		exe = filepath.ToSlash(exe)
		hookArgs := ""
		if matcherPath != "" {
			hookArgs = " --rules " + shellQuote(matcherPath)
		}
		script := fmt.Sprintf("#!/bin/sh\n# Installed by ByeClaude.\nexec %s hook-filter%s \"$1\"\n", shellQuote(exe), hookArgs)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0755) // #nosec G302,G304,G703 -- executable Git hook created exclusively at the resolved hook path
		if err != nil {
			return err
		}
		if _, err := f.WriteString(script); err != nil {
			_ = f.Close()
			_ = os.Remove(path) // #nosec G703 -- same exclusive hook path created immediately above
			return err
		}
		if err := f.Close(); err != nil {
			_ = os.Remove(path) // #nosec G703 -- same exclusive hook path created immediately above
			return err
		}
		fmt.Println("Installed", path)
		return nil
	case "remove":
		info, err := os.Lstat(path) // #nosec G703 -- resolved Git directory plus hooks/commit-msg
		if os.IsNotExist(err) {
			fmt.Println("No commit-msg hook installed.")
			return nil
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to remove non-regular commit-msg hook at %s", path)
		}
		b, err := os.ReadFile(path) // #nosec G304,G703 -- path is the resolved Git directory plus hooks/commit-msg
		if err != nil {
			return err
		}
		if !strings.Contains(string(b), "Installed by ByeClaude") {
			return fmt.Errorf("refusing to remove a commit-msg hook not owned by ByeClaude")
		}
		if err := os.Remove(path); err != nil { // #nosec G703 -- ownership and regular-file checks completed above
			return err
		}
		fmt.Println("Removed", path)
		return nil
	default:
		if action == "hook-filter" {
			return nil
		}
		return fmt.Errorf("unknown hook action %q", action)
	}
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

func filterCommitMessage(path string, matcher attribution.Matcher) error {
	repo, err := gitx.Open(".")
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing non-regular commit message path %s", abs)
	}
	inside, err := pathIsInsideDirectory(abs, repo.GitDir)
	if err != nil {
		return err
	}
	if !inside {
		return fmt.Errorf("commit message path must be inside the repository Git directory")
	}
	b, err := os.ReadFile(abs) // #nosec G304,G703 -- absolute path is constrained to the resolved Git directory and is not a symlink
	if err != nil {
		return err
	}
	out, matches := clean.StripMatchingTrailers(string(b), matcher)
	if len(matches) == 0 {
		return nil
	}
	f, err := os.OpenFile(abs, os.O_WRONLY, 0) // #nosec G304,G703 -- same validated Git-owned regular file
	if err != nil {
		return err
	}
	defer f.Close()
	openedInfo, err := f.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(info, openedInfo) {
		return fmt.Errorf("commit message file changed during validation")
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	if _, err := f.WriteString(out); err != nil {
		return err
	}
	return f.Sync()
}

// pathIsInsideDirectory compares filesystem identities instead of path strings.
// This handles macOS /private aliases, Windows short paths, worktrees, and
// symlinked parent directories without weakening the Git-directory boundary.
func pathIsInsideDirectory(path, directory string) (bool, error) {
	directoryInfo, err := os.Stat(directory)
	if err != nil {
		return false, err
	}
	current, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return false, err
	}
	for {
		currentInfo, err := os.Stat(current)
		if err != nil {
			return false, err
		}
		if os.SameFile(directoryInfo, currentInfo) {
			return true, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false, nil
		}
		current = parent
	}
}

func init() {
	// Internal hook entrypoint is intentionally hidden from normal help.
	if len(os.Args) >= 3 && os.Args[1] == "hook-filter" {
		fs := flag.NewFlagSet("hook-filter", flag.ContinueOnError)
		rulesFile := rulesFlag(fs)
		if err := fs.Parse(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if fs.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "hook-filter requires a commit message path")
			os.Exit(1)
		}
		matcher, err := resolveMatcher(*rulesFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := filterCommitMessage(fs.Arg(0), matcher); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}
