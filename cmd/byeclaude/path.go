package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/IamAngusU/ByeClaude/internal/pathsetup"
)

func runPath(args []string) error {
	action := "status"
	if len(args) == 1 {
		action = args[0]
	} else if len(args) > 1 {
		return fmt.Errorf("choose path status, setup, or skip")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	switch action {
	case "setup":
		reportPathSetup(os.Stdout, exe, pathsetup.Setup)
	case "skip":
		if err := pathsetup.SavePreference("skipped"); err != nil {
			fmt.Println("Could not save the preference. ByeClaude still works; use path setup to retry.")
			return nil
		}
		fmt.Println("Automatic PATH reminders disabled. Use byeclaude path setup whenever ready.")
	case "status":
		fmt.Println("Command in this terminal: " + terminalText(pathsetup.CurrentCommand(exe)))
		pref, err := pathsetup.ReadPreference()
		if err != nil {
			fmt.Println("PATH preference unavailable. Run byeclaude path setup to retry.")
		} else {
			fmt.Println("Saved PATH setup: " + map[string]string{"": "not configured", "ready": "ready for new terminals", "pending": "deferred; retry offered on next interactive start", "skipped": "reminders disabled"}[pref.Mode])
		}
		printDirectStart(os.Stdout, exe)
	default:
		return fmt.Errorf("choose path status, setup, or skip")
	}
	return nil
}

func reportPathSetup(out io.Writer, exe string, setup func(string) error) {
	if err := setup(exe); err != nil {
		fmt.Fprintln(out, "PATH setup deferred: "+terminalText(err.Error()))
		fmt.Fprintln(out, "ByeClaude remains usable at its current location. No administrator access was requested.")
		printDirectStart(out, exe)
		if pref, readErr := pathsetup.ReadPreference(); readErr == nil && pref.Mode == "pending" {
			fmt.Fprintln(out, "The next interactive start will offer another attempt. Choose Later to continue.")
		} else {
			fmt.Fprintln(out, "A retry reminder could not be saved. Run the executable with path setup to retry.")
		}
		return
	}
	fmt.Fprintln(out, "User PATH configured. Open a new terminal and run byeclaude.")
	fmt.Fprintln(out, "No administrator access is needed. Existing terminals keep their previous PATH.")
}

func printDirectStart(out io.Writer, exe string) {
	if runtime.GOOS == "windows" {
		fmt.Fprintf(out, "PowerShell: & '%s'\ncmd: \"%s\"\n", strings.ReplaceAll(terminalText(exe), "'", "''"), terminalText(exe))
	} else {
		fmt.Fprintln(out, "Direct start: '"+strings.ReplaceAll(terminalText(exe), "'", `'"'"'`)+"'")
	}
}

func (ui *terminalUI) offerPathRetry() error {
	pref, err := pathsetup.ReadPreference()
	if err != nil || pref.Mode != "pending" {
		return nil
	}
	ui.heading("PATH setup can wait")
	ui.hint("ByeClaude already runs. Adding its folder makes the command easier to start.")
	ui.hint("Only your user settings are changed; administrator access is not needed.")
	choice, err := ui.choice("Retry PATH setup? [y: retry, Enter: later, n: stop reminders]", "later", "y", "yes", "later", "n", "no")
	if err != nil {
		return err
	}
	switch choice {
	case "y", "yes":
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		reportPathSetup(ui.out, exe, pathsetup.Setup)
	case "n", "no":
		if err := pathsetup.SavePreference("skipped"); err != nil {
			ui.hint("Could not save your preference; the reminder may return next time.")
		}
	default:
		ui.hint("Continuing without PATH setup. You can retry on the next interactive start.")
	}
	return nil
}
