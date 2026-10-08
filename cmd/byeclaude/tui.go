package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/IamAngusU/ByeClaude/internal/blacklist"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

func terminalInput() bool {
	ci := strings.ToLower(os.Getenv("CI"))
	return (ci == "" || ci == "false" || ci == "0") && isTerminal(os.Stdin) && isTerminal(os.Stdout)
}

func runTUI(args []string) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	repoPath := fs.String("repo", ".", "repository to manage")
	plain := fs.Bool("no-color", false, "plain terminal output")
	guide := fs.Bool("guide", false, "start the guided first-use walkthrough")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments; use --repo PATH")
	}
	if !terminalInput() {
		return fmt.Errorf("the menu needs terminal input and output, outside CI; use scan, blacklist or setup for scripts")
	}
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("Git was not found on PATH; install Git from https://git-scm.com/downloads, reopen your terminal and try again")
	}
	_, noColor := os.LookupEnv("NO_COLOR")
	color := false
	if !*plain && !noColor && os.Getenv("TERM") != "dumb" {
		var restore func()
		color, restore = enableTerminalColor(os.Stdout)
		defer restore()
	}
	ui := terminalUI{in: bufio.NewReader(os.Stdin), out: os.Stdout, repo: *repoPath, color: color, guided: *guide}
	ui.invoke = func(command string, args []string) error { return executeMenuCommand(command, args, ui.out) }
	return ui.run()
}

// Ordinary line input works in cmd, PowerShell, SSH and screen readers. The
// same instructions remain visible without color. No raw terminal mode is used.
type terminalUI struct {
	in     *bufio.Reader
	out    io.Writer
	repo   string
	color  bool
	guided bool
	invoke func(string, []string) error
}

func (ui *terminalUI) heading(value string) {
	if ui.color {
		fmt.Fprintf(ui.out, "\n\x1b[1;36m  %s\x1b[0m\n", terminalText(value))
	} else {
		fmt.Fprintf(ui.out, "\n  %s\n", terminalText(value))
	}
}

func (ui *terminalUI) run() error {
	for {
		repo, err := gitx.Open(ui.repo)
		if err != nil {
			ui.heading("BYECLAUDE / Choose your repository")
			ui.hint("Select the local folder containing your Git project.")
			if err = ui.selectRepository(); err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, errMenuBack) {
					return nil
				}
				return err
			}
			continue
		}
		ui.repo = repo.Root
		if ui.guided {
			ui.guided = false
			err = ui.guide(repo)
		} else {
			ui.dashboard(repo)
			var choice string
			choice, err = ui.choice("Choose [Enter: guided start]", "g", "g", "1", "2", "3", "4", "5", "6", "7", "h", "q")
			if err == nil {
				args := []string{"--repo", ui.repo}
				switch choice {
				case "q":
					return nil
				case "g":
					err = ui.guide(repo)
				case "1":
					err = ui.audit()
				case "2":
					err = ui.manageBlacklist()
				case "3":
					err = ui.cleanup(repo)
				case "4":
					err = ui.protect(repo)
				case "5":
					ui.hint("Checking what prevents protection from working...")
					err = ui.invoke("doctor", args)
				case "6":
					ui.heading("Saved backups / Local recovery")
					err = ui.invoke("backups", args)
					ui.hint("Backups keep the original history in this clone.")
					ui.hint("To restore: open a terminal in the repository shown above, then run")
					ui.hint("byeclaude restore --backup ID --apply (replace ID with a listed backup).")
					ui.hint("Restore refuses to overwrite work added after cleanup. GitHub is unchanged.")
				case "7":
					err = ui.selectRepository()
				case "h":
					ui.help()
				}
			}
		}
		if errors.Is(err, errTerminalIO) {
			return err
		}
		if errors.Is(err, io.EOF) {
			ui.hint("Input closed. The pending choice was cancelled.")
			return nil
		}
		if errors.Is(err, errMenuBack) {
			ui.hint("Back to the menu.")
			continue
		}
		if err != nil {
			fmt.Fprintln(ui.out, "\n  Could not finish: "+terminalText(err.Error()))
			ui.hint("Your menu is still available. Fix the reported problem and try again.")
			ui.hint("Choose 5 for diagnostics, 7 for another repository, or q to exit.")
		}
	}
}

func (ui *terminalUI) dashboard(repo *gitx.Repo) {
	ui.heading("BYECLAUDE / Your history. Your attribution.")
	fmt.Fprintln(ui.out, "  Repository  "+terminalText(repo.Root))
	set, saved, err := blacklist.Load(repo)
	if err != nil {
		ui.hint("Blacklist needs repair. Choose 2 to review or reset it.")
	} else {
		ids := make([]string, 0, len(set.Rules))
		for _, r := range set.Rules {
			ids = append(ids, r.RuleID)
		}
		source := "default"
		if saved {
			source = "saved in this clone"
		}
		fmt.Fprintf(ui.out, "  Blacklist   %s (%s)\n", strings.Join(ids, ", "), source)
	}
	installed := 0
	for _, name := range []string{"commit-msg", "pre-push"} {
		state, err := inspectHook(repo, name)
		if err == nil && state.Status == "installed" {
			installed++
		}
	}
	if installed == 2 {
		ui.hint("Protection: both local Git hooks are installed. Check history with 1.")
	} else {
		ui.hint("Protection: not fully set up. Choose 4, or 5 to diagnose an existing hook.")
	}
	fmt.Fprintln(ui.out)
	ui.option("g", "Guided start", "Choose identities, check history, then decide what to change.")
	ui.option("1", "Check this repository", "Find matching credits and identities. No changes.")
	ui.option("2", "Choose blocked identities", "Keep Claude, add other exact emails, or remove a rule.")
	ui.option("3", "Preview a cleanup", "See the impact first. Nothing changes without typing CLEAN.")
	ui.option("4", "Protect future commits", "Preview and install local commit and push checks.")
	fmt.Fprintln(ui.out, "  5  Diagnose protection    6  Backups and recovery    7  Change folder")
	fmt.Fprintln(ui.out, "  h  Explain the basics    q  Exit")
	ui.hint("Type a choice and press Enter. GitHub is unchanged by this menu.")
}

func repositoryFolder(value string) (string, error) {
	if len(value) >= 2 && (value[0] == '\'' && value[len(value)-1] == '\'' || value[0] == '"' && value[len(value)-1] == '"') {
		value = value[1 : len(value)-1]
	}
	if strings.Contains(value, "://") || strings.HasPrefix(value, "git@") {
		return "", fmt.Errorf("use a local folder, not a URL; clone the repository with Git first")
	}
	if value == "~" || strings.HasPrefix(value, "~/") || strings.HasPrefix(value, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		value = filepath.Join(home, strings.TrimLeft(value[1:], "/\\"))
	}
	if value == "" {
		return "", fmt.Errorf("enter a folder path, such as C:\\Projects\\my-repo or ~/projects/my-repo")
	}
	info, err := os.Stat(value)
	if err != nil {
		return "", fmt.Errorf("cannot open that folder; check the path and access permissions")
	}
	if !info.IsDir() {
		return "", fmt.Errorf("that is a file; choose the folder containing the repository")
	}
	return value, nil
}

func (ui *terminalUI) selectRepository() error {
	ui.hint("Paste a folder path, with or without quotes. Enter or q goes back.")
	for attempt := 0; attempt < maxPromptAttempts; attempt++ {
		value, err := ui.ask("Repository folder")
		if err != nil {
			return err
		}
		if value == "" || strings.EqualFold(value, "q") {
			return errMenuBack
		}
		path, err := repositoryFolder(value)
		if err != nil {
			ui.hint(err.Error())
			continue
		}
		repo, err := gitx.Open(path)
		if err != nil {
			ui.hint("That folder is not an accessible Git repository. Choose your existing clone.")
			continue
		}
		ui.repo = repo.Root
		ui.hint("Repository selected. No repository files or history were changed.")
		return nil
	}
	ui.hint("Folder selection cancelled. The previous repository is kept.")
	return errMenuBack
}

func (ui *terminalUI) help() {
	ui.heading("What ByeClaude does")
	ui.hint("A co-author credit is a Co-authored-by line in a Git commit message.")
	ui.hint("The blacklist chooses which declared names or emails match.")
	ui.hint("It does not detect AI-written code or prove who wrote a file.")
	ui.hint("Check is read-only. Cleanup changes matching metadata and commit IDs.")
	ui.hint("Committed file contents stay the same; affected signatures cannot stay valid.")
	ui.hint("Protection installs hooks in this clone; it does not change GitHub settings.")
	ui.hint("GitHub web commits, bypassed hooks and other clones need their own protection.")
	ui.hint("Suggested order: 2 choose identities, 1 check, 3 preview, 4 protect.")
	ui.hint("q goes back in a form and exits at the main menu. Ctrl+C exits the program.")
	ui.hint("Completed changes stay saved; cancelling only stops the pending choice.")
}

func (ui *terminalUI) audit() error {
	ui.heading("Check history / No changes")
	ui.hint("Looking for matching co-author credits, authors and committers...")
	if err := ui.invoke("scan", []string{"--repo", ui.repo, "--include-identities"}); err != nil {
		return err
	}
	ui.hint("If matches were found, choose 3 to review a cleanup. Otherwise choose 4.")
	return nil
}

func (ui *terminalUI) guide(repo *gitx.Repo) error {
	ui.heading("Guided start / 1 of 4: Choose identities")
	ui.hint("Claude/Anthropic is selected by default. Other tools need their exact email.")
	ui.hint("Each change is confirmed separately. q returns to the main menu.")
	if set, _, err := blacklist.Load(repo); err == nil {
		printBlacklist(ui.out, set)
	} else {
		ui.hint("The saved blacklist needs repair. Choose m to reset it.")
	}
	choice, err := ui.choice("Keep this blacklist, or manage it? [Enter: keep, m: manage, q: back]", "keep", "keep", "m", "q")
	if err != nil {
		return err
	}
	if choice == "q" {
		return errMenuBack
	}
	if choice == "m" {
		if err := ui.manageBlacklist(); err != nil {
			return err
		}
	}
	ui.heading("Guided start / 2 of 4: Check history")
	if err := ui.audit(); err != nil {
		return err
	}
	ui.heading("Guided start / 3 of 4: Review existing history")
	choice, err = ui.choice("Preview cleanup? [Enter: preview, s: skip, q: back]", "preview", "preview", "s", "q")
	if err != nil {
		return err
	}
	if choice == "q" {
		return errMenuBack
	}
	if choice == "preview" {
		if err := ui.cleanup(repo); err != nil {
			return err
		}
	}
	ui.heading("Guided start / 4 of 4: Protect future commits")
	if err := ui.protect(repo); err != nil {
		return err
	}
	ui.hint("Walkthrough finished. You can review or change any choice from the menu.")
	return nil
}

func reviewState(repo *gitx.Repo) ([32]byte, error) {
	set, _, err := blacklist.Load(repo)
	if err != nil {
		return [32]byte{}, err
	}
	refs, err := repo.Run("for-each-ref", "--format=%(refname) %(objectname)", "refs/heads", "refs/tags")
	if err != nil {
		return [32]byte{}, err
	}
	head, _, err := repo.RunOptional("rev-parse", "--verify", "--quiet", "HEAD")
	if err != nil {
		return [32]byte{}, err
	}
	identity, _, err := repo.RunOptional("config", "--get-regexp", `^user\.(name|email)$`)
	if err != nil {
		return [32]byte{}, err
	}
	data, err := json.Marshal(struct {
		Rules                any
		Refs, Head, Identity string
	}{set, string(refs), string(head), string(identity)})
	return sha256.Sum256(data), err
}
