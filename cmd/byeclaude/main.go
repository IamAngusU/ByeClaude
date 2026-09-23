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
	case "clean":
		err = runClean(os.Args[2:])
	case "batch":
		err = runBatch(os.Args[2:])
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
	fmt.Print(`ByeClaude removes Claude Code co-author trailers from Git history.

Usage:
  byeclaude scan [--repo PATH] [--include-remotes] [--rules FILE] [--json]
  byeclaude check [--repo PATH] [--include-remotes] [--rules FILE] [--json]
  byeclaude batch scan --repo OWNER/NAME [--repo ...] [--jobs N] [--json]
  byeclaude batch scan --owner OWNER [--public|--private|--all] [--jobs N] [--json]
  byeclaude batch check ...
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
	before, err := clean.Scan(repo, matcher)
	if err != nil {
		return err
	}
	if len(before.Matches) == 0 {
		fmt.Println("No matching attribution trailers found. Nothing to do.")
		return nil
	}
	if !*apply {
		fmt.Printf("Found %d matching trailer(s) across %d scanned commits.\n", len(before.Matches), before.Commits)
		fmt.Println("Dry run only. Re-run with: byeclaude clean --apply")
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
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if b, err := os.ReadFile(path); err == nil {
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
		if err := os.WriteFile(path, []byte(script), 0755); err != nil {
			return err
		}
		fmt.Println("Installed", path)
		return nil
	case "remove":
		b, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			fmt.Println("No commit-msg hook installed.")
			return nil
		}
		if err != nil {
			return err
		}
		if !strings.Contains(string(b), "Installed by ByeClaude") {
			return fmt.Errorf("refusing to remove a commit-msg hook not owned by ByeClaude")
		}
		if err := os.Remove(path); err != nil {
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
		path := fs.Arg(0)
		b, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		out, _ := clean.StripMatchingTrailers(string(b), matcher)
		if err := os.WriteFile(path, []byte(out), 0644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}
