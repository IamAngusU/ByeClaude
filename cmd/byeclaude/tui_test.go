package main

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestMenuPreviewNeverAppliesOnDefaultOrEOF(t *testing.T) {
	for _, input := range []string{"q\n", "3\n\n\nq\n", "3\n1\n", "4\n\nq\n", "4\n"} {
		t.Run(strings.ReplaceAll(input, "\n", "_"), func(t *testing.T) {
			dir := createCLIRepository(t, true)
			before := runGit(t, dir, "rev-parse", "HEAD")
			var output bytes.Buffer
			ui := terminalUI{repo: dir, in: bufio.NewReader(strings.NewReader(input)), out: &output, invoke: func(command string, args []string) error {
				if command == "clean" || strings.Contains(strings.Join(args, " "), "--apply") {
					t.Fatal("default/EOF attempted mutation")
				}
				return nil
			}}
			if err := ui.run(); err != nil {
				t.Fatal(err)
			}
			if runGit(t, dir, "rev-parse", "HEAD") != before {
				t.Fatal("menu changed history")
			}
			if !strings.Contains(output.String(), "BYECLAUDE") {
				t.Fatal(output.String())
			}
		})
	}
}

func TestMenuRequiresExactCleanupConfirmationAndKeepsRoleSelection(t *testing.T) {
	for _, answer := range []string{"yes", "clean", "CLEAN"} {
		dir := createCLIRepository(t, true)
		applied := false
		var output bytes.Buffer
		ui := terminalUI{repo: dir, in: bufio.NewReader(strings.NewReader("3\n2\n" + answer + "\nq\n")), out: &output, invoke: func(command string, args []string) error {
			joined := strings.Join(args, " ")
			if command == "plan" || command == "clean" {
				if !strings.Contains(joined, "--author-from-git") || strings.Contains(joined, "--identity-from-git") {
					t.Fatal(joined)
				}
			}
			if command == "clean" {
				applied = true
				if !strings.Contains(joined, "--apply") || strings.Contains(joined, "--push") {
					t.Fatal(joined)
				}
			}
			return nil
		}}
		if err := ui.run(); err != nil {
			t.Fatal(err)
		}
		if applied != (answer == "CLEAN") {
			t.Fatalf("confirmation %q applied=%v", answer, applied)
		}
	}
}

func TestMenuRefusesChangedHistoryAfterPreview(t *testing.T) {
	dir := createCLIRepository(t, true)
	var output bytes.Buffer
	observer := &promptObserver{buffer: &output, prompt: "Type CLEAN", onPrompt: func() { runGit(t, dir, "commit", "--allow-empty", "-m", "Concurrent change") }}
	ui := terminalUI{repo: dir, in: bufio.NewReader(strings.NewReader("3\n1\nCLEAN\nq\n")), out: observer, invoke: func(command string, args []string) error {
		if command == "clean" {
			t.Fatal("applied changed history")
		}
		return nil
	}}
	if err := ui.run(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "changed during review") {
		t.Fatal(output.String())
	}
}

func TestMenuAddsAndRemovesExplicitEmails(t *testing.T) {
	dir := createCLIRepository(t, false)
	var output bytes.Buffer
	ui := terminalUI{repo: dir, in: bufio.NewReader(strings.NewReader("2\na\nhelper\nhelper@example.org\ny\n2\nr\nhelper\ny\nq\n")), out: &output, invoke: invokeMenuCommand}
	if err := ui.run(); err != nil {
		t.Fatal(err)
	}
	matcher, err := resolveLocalMatcher(dir, "")
	if err != nil || matcher.Match("Helper", "helper@example.org") {
		t.Fatalf("remove: %v", err)
	}
}

func TestMenuReadOnlyActionsAndRepositorySwitch(t *testing.T) {
	first := createCLIRepository(t, true)
	second := createCLIRepository(t, false)
	before := runGit(t, first, "rev-parse", "HEAD")
	var output bytes.Buffer
	input := "1\n5\n6\n7\n" + second + "\n2\n\n4\n\n3\nq\nunknown\nq\n"
	ui := terminalUI{repo: first, in: bufio.NewReader(strings.NewReader(input)), out: &output, invoke: invokeMenuCommand}
	if err := ui.run(); err != nil {
		t.Fatal(err)
	}
	if runGit(t, first, "rev-parse", "HEAD") != before {
		t.Fatal("read-only menu changed first repository")
	}
	if !strings.Contains(output.String(), "Nothing was changed by that answer") || !strings.Contains(output.String(), "claude-anthropic") {
		t.Fatal(output.String())
	}
	if !sameFile(t, ui.repo, second) {
		t.Fatal("repository switch did not take effect")
	}
}

func TestMenuInstallsHooksOnlyAfterSuccessfulPreview(t *testing.T) {
	for _, allow := range []bool{true, false} {
		dir := createCLIRepository(t, false)
		applied := false
		var output bytes.Buffer
		ui := terminalUI{repo: dir, in: bufio.NewReader(strings.NewReader("4\ny\nq\n")), out: &output, invoke: func(command string, args []string) error {
			if strings.Contains(strings.Join(args, " "), "--apply") {
				applied = true
				return nil
			}
			if !allow {
				return fmt.Errorf("foreign hook conflict")
			}
			return nil
		}}
		if err := ui.run(); err != nil {
			t.Fatal(err)
		}
		if applied != allow {
			t.Fatal("setup did not respect failed preview")
		}
	}
}

func invokeMenuCommand(command string, args []string) error {
	switch command {
	case "scan":
		return runScan(args)
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
