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
	"runtime"
	"strings"
	"unicode"

	"github.com/IamAngusU/ByeClaude/internal/blacklist"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

func terminalInput() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func runTUI(args []string) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	repoPath := fs.String("repo", ".", "repository to manage")
	plain := fs.Bool("no-color", false, "plain terminal output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments; use --repo PATH")
	}
	if !terminalInput() {
		return fmt.Errorf("tui needs an interactive terminal; use scan, blacklist or setup for scripts")
	}
	_, noColor := os.LookupEnv("NO_COLOR")
	color := !*plain && !noColor && os.Getenv("TERM") != "dumb"
	if runtime.GOOS == "windows" && os.Getenv("WT_SESSION") == "" && os.Getenv("TERM") == "" && os.Getenv("ANSICON") == "" {
		color = false
	}
	ui := terminalUI{in: bufio.NewScanner(os.Stdin), out: os.Stdout, repo: *repoPath, color: color, invoke: invokeMenuCommand}
	ui.in.Buffer(make([]byte, 4096), 64*1024)
	return ui.run()
}

// The menu uses line input deliberately: cmd.exe, PowerShell, SSH, screen
// readers and ordinary terminals work without raw-mode or terminal dependencies.
type terminalUI struct {
	in     *bufio.Scanner
	out    io.Writer
	repo   string
	color  bool
	invoke func(string, []string) error
}

func invokeMenuCommand(command string, args []string) error {
	switch command {
	case "scan":
		return runScan(args)
	case "plan":
		return runPlan(args)
	case "clean":
		return runClean(args)
	case "setup":
		return runSetup(args)
	case "doctor":
		return runDoctor(args)
	case "backups":
		return runBackups(args)
	case "blacklist":
		return runBlacklist(args)
	}
	return fmt.Errorf("unknown menu command %q", command)
}

func (ui *terminalUI) ask(prompt string) (string, error) {
	fmt.Fprint(ui.out, prompt+" > ")
	if !ui.in.Scan() {
		if err := ui.in.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return strings.TrimSpace(ui.in.Text()), nil
}

// Repository names and diagnostics can contain terminal control characters.
func terminalText(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
}

func (ui *terminalUI) heading(value string) {
	if ui.color {
		fmt.Fprintf(ui.out, "\n\x1b[1;36m  %s\x1b[0m\n", value)
	} else {
		fmt.Fprintf(ui.out, "\n  %s\n", value)
	}
}

func (ui *terminalUI) run() error {
	for {
		repo, err := gitx.Open(ui.repo)
		if err != nil {
			fmt.Fprintln(ui.out, "Choose a local Git repository to get started.")
			path, err := ui.ask("Repository folder (q to quit)")
			if errors.Is(err, io.EOF) || path == "q" {
				return nil
			}
			if err != nil {
				return err
			}
			ui.repo = strings.Trim(path, "\"")
			continue
		}
		ui.repo = repo.Root
		ui.heading("BYECLAUDE  /  Your history. Your attribution.")
		fmt.Fprintln(ui.out, "  ------------------------------------------------------------")
		fmt.Fprintln(ui.out, "  Repository  "+terminalText(repo.Root))
		set, saved, policyErr := blacklist.Load(repo)
		if policyErr != nil {
			fmt.Fprintln(ui.out, "  Blacklist   INVALID - use blacklist reset to repair")
		} else {
			var ids []string
			for _, rule := range set.Rules {
				ids = append(ids, rule.RuleID)
			}
			source := "default"
			if saved {
				source = "saved"
			}
			fmt.Fprintf(ui.out, "  Blacklist   %s (%s)\n", strings.Join(ids, ", "), source)
		}
		for _, name := range []string{"commit-msg", "pre-push"} {
			state, err := inspectHook(repo, name)
			if err != nil {
				fmt.Fprintf(ui.out, "  %-12s unavailable\n", name)
			} else {
				fmt.Fprintf(ui.out, "  %-12s %s\n", name, state.Status)
			}
		}
		fmt.Fprintln(ui.out, "\n  1  Audit history          Trailers + authors + committers")
		fmt.Fprintln(ui.out, "  2  Manage blacklist       Add or remove identities")
		fmt.Fprintln(ui.out, "  3  Preview cleanup        Review first, apply locally")
		fmt.Fprintln(ui.out, "  4  Protect future work    Set up both Git hooks")
		fmt.Fprintln(ui.out, "  5  Diagnose setup         Explain hook problems")
		fmt.Fprintln(ui.out, "  6  Show backups           Recovery IDs")
		fmt.Fprintln(ui.out, "  7  Switch repository      Choose another local folder")
		fmt.Fprintln(ui.out, "  q  Exit\n\n  Nothing runs until selected. Publishing is a separate command.")
		choice, err := ui.ask("Choose")
		if errors.Is(err, io.EOF) || choice == "q" || choice == "Q" {
			return nil
		}
		if err != nil {
			return err
		}
		args := []string{"--repo", ui.repo}
		switch choice {
		case "1":
			err = ui.invoke("scan", append(args, "--include-identities"))
		case "2":
			err = ui.manageBlacklist()
		case "3":
			err = ui.cleanup(repo)
		case "4":
			err = ui.invoke("setup", args)
			if err == nil {
				var answer string
				answer, err = ui.ask("Install both hooks for this repository? [y/N]")
				if err == nil && strings.EqualFold(answer, "y") {
					err = ui.invoke("setup", append(args, "--apply"))
				}
			}
		case "5":
			err = ui.invoke("doctor", args)
		case "6":
			err = ui.invoke("backups", args)
		case "7":
			var path string
			path, err = ui.ask("Repository folder (blank to keep current)")
			if err == nil && path != "" {
				ui.repo = strings.Trim(path, "\"")
			}
		default:
			fmt.Fprintln(ui.out, "Choose 1-7 or q.")
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			fmt.Fprintln(ui.out, "\n  Stopped: "+terminalText(err.Error()))
		}
	}
}

func (ui *terminalUI) manageBlacklist() error {
	if err := ui.invoke("blacklist", []string{"list", "--repo", ui.repo}); err != nil {
		return err
	}
	fmt.Fprintln(ui.out, "\n  a  Add exact email    r  Remove rule    Enter  Back")
	choice, err := ui.ask("Blacklist")
	if err != nil {
		return err
	}
	if choice != "a" && choice != "r" {
		return nil
	}
	id, err := ui.ask("Rule ID (blank to cancel)")
	if err != nil {
		return err
	}
	if id == "" {
		return nil
	}
	if choice == "r" {
		return ui.invoke("blacklist", []string{"remove", "--repo", ui.repo, "--id", id})
	}
	email, err := ui.ask("Exact email to block (blank to cancel)")
	if err != nil {
		return err
	}
	if email == "" {
		return nil
	}
	return ui.invoke("blacklist", []string{"add", "--repo", ui.repo, "--id", id, "--email", email})
}

func (ui *terminalUI) cleanup(repo *gitx.Repo) error {
	fmt.Fprintln(ui.out, "\n  1  Matching co-author trailers only (default)")
	fmt.Fprintln(ui.out, "  2  Also correct matching authors using your Git identity")
	fmt.Fprintln(ui.out, "  3  Also correct matching committers using your Git identity")
	fmt.Fprintln(ui.out, "  4  Also correct both roles using your Git identity")
	mode, err := ui.ask("Preview mode [1] (q to cancel)")
	if err != nil {
		return err
	}
	args := []string{"--repo", ui.repo}
	switch mode {
	case "", "1":
	case "2":
		args = append(args, "--author-from-git")
	case "3":
		args = append(args, "--committer-from-git")
	case "4":
		args = append(args, "--identity-from-git")
	case "q":
		return nil
	default:
		return fmt.Errorf("choose preview mode 1-4")
	}
	before, err := reviewState(repo)
	if err != nil {
		return err
	}
	if err := ui.invoke("plan", args); err != nil {
		return err
	}
	fmt.Fprintln(ui.out, "\n  Cleanup changes commit IDs and affected signatures. Backup refs are created.")
	fmt.Fprintln(ui.out, "  Confirm any replacement identity belongs to the actual contributor.")
	answer, err := ui.ask("Type CLEAN to apply locally, or Enter to keep preview only")
	if err != nil {
		return err
	}
	if answer != "CLEAN" {
		fmt.Fprintln(ui.out, "Preview only. No history changed.")
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
	fmt.Fprintln(ui.out, "\n  Local cleanup complete. Keep the backup ID for recovery.")
	fmt.Fprintln(ui.out, "  Review the result; publish separately with byeclaude push in this repository.")
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
	head, err := repo.Run("rev-parse", "--verify", "HEAD")
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
