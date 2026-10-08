package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/IamAngusU/ByeClaude/internal/blacklist"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

type promptObserver struct {
	buffer   *bytes.Buffer
	prompt   string
	onPrompt func()
}

type disconnectedTerminal struct{}

func (disconnectedTerminal) Read([]byte) (int, error)  { return 0, io.ErrClosedPipe }
func (disconnectedTerminal) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestDisconnectedTerminalExitsInsteadOfRetryingForever(t *testing.T) {
	dir := createCLIRepository(t, false)
	for _, outputBroken := range []bool{false, true} {
		var input io.Reader = disconnectedTerminal{}
		var output io.Writer = io.Discard
		if outputBroken {
			input = strings.NewReader("q\n")
			output = disconnectedTerminal{}
		}
		ui := terminalUI{repo: dir, in: bufio.NewReader(input), out: output}
		if err := ui.run(); !errors.Is(err, errTerminalIO) || !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("expected terminal failure, got %v", err)
		}
	}
}

func (w *promptObserver) Write(data []byte) (int, error) {
	if w.onPrompt != nil && strings.Contains(string(data), w.prompt) {
		fn := w.onPrompt
		w.onPrompt = nil
		fn()
	}
	return w.buffer.Write(data)
}

func runMenuFixture(t *testing.T, dir, input string) string {
	t.Helper()
	var output bytes.Buffer
	ui := terminalUI{repo: dir, in: bufio.NewReader(strings.NewReader(input)), out: &output, invoke: invokeMenuCommand}
	if err := ui.run(); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func TestTerminalInputDrainsOversizedLinesAndRejectsPartialConfirmation(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(strings.Repeat("x", 100000) + "\ny\nCLEAN"))
	if _, err := readTerminalLine(reader); !errors.Is(err, errTerminalAnswer) {
		t.Fatalf("oversize: %v", err)
	}
	if got, err := readTerminalLine(reader); err != nil || got != "y" {
		t.Fatalf("next answer damaged: %q %v", got, err)
	}
	if got, err := readTerminalLine(reader); !errors.Is(err, io.EOF) || got != "" {
		t.Fatalf("partial confirmation accepted: %q %v", got, err)
	}
	for _, input := range []string{"y\x1b[2J\n", "y\x00\n", "\xff\n", "yes\u202e\n", "yes\rno\n"} {
		if _, err := readTerminalLine(bufio.NewReader(strings.NewReader(input))); !errors.Is(err, errTerminalAnswer) {
			t.Errorf("accepted %q: %v", input, err)
		}
	}
	if got, err := readTerminalLine(bufio.NewReader(strings.NewReader("  YES  \r\n"))); err != nil || got != "YES" {
		t.Fatalf("CRLF: %q %v", got, err)
	}
}

func FuzzTerminalInput(f *testing.F) {
	for _, input := range []string{"CLEAN", "yes\n", "\x1b[2J\n", "\xff\n", strings.Repeat("x", 5000) + "\nq\n"} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		got, err := readTerminalLine(bufio.NewReader(strings.NewReader(input)))
		if err == nil && (len(got) > maxPromptBytes || !utf8.ValidString(got) || strings.IndexFunc(got, unsafeTerminalRune) >= 0 || !strings.Contains(input, "\n")) {
			t.Fatalf("unsafe result %q", got)
		}
	})
}

func TestBadMenuInputRecoversWithoutChangingState(t *testing.T) {
	dir := createCLIRepository(t, true)
	before := runGit(t, dir, "show-ref")
	input := "invalid\n-1\n999\n" + strings.Repeat("z", 70000) + "\n\x1b[2J\n1\nq\n"
	output := runMenuFixture(t, dir, input)
	for _, want := range []string{"Too many invalid choices", "too long", "hidden control", "Check history"} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(output, "\x1b") || runGit(t, dir, "show-ref") != before {
		t.Fatal("input changed terminal controls or refs")
	}
}

func TestMenuPartialEOFNeverApplies(t *testing.T) {
	for _, input := range []string{"3\n1\nCLEAN", "4\ny", "2\na\nhelper\nhelper@example.org\ny"} {
		dir := createCLIRepository(t, true)
		before := runGit(t, dir, "show-ref")
		var output bytes.Buffer
		ui := terminalUI{repo: dir, in: bufio.NewReader(strings.NewReader(input)), out: &output, invoke: func(command string, args []string) error {
			if command == "clean" || command == "blacklist" || strings.Contains(strings.Join(args, " "), "--apply") {
				t.Fatal("EOF applied a change")
			}
			return nil
		}}
		if err := ui.run(); err != nil {
			t.Fatal(err)
		}
		if runGit(t, dir, "show-ref") != before {
			t.Fatal("EOF changed refs")
		}
	}
}

func TestInvalidFolderKeepsPreviousSelectionAndQuotedPathsWork(t *testing.T) {
	first := createCLIRepository(t, false)
	second := createCLIRepository(t, false)
	var output bytes.Buffer
	ui := terminalUI{repo: first, in: bufio.NewReader(strings.NewReader("https://github.com/owner/repo\nmissing-folder\nq\n")), out: &output}
	if err := ui.selectRepository(); !errors.Is(err, errMenuBack) {
		t.Fatal(err)
	}
	if ui.repo != first {
		t.Fatal("invalid selection displaced current repository")
	}
	ui.in = bufio.NewReader(strings.NewReader("'" + second + "'\n"))
	if err := ui.selectRepository(); err != nil {
		t.Fatal(err)
	}
	if !sameFile(t, ui.repo, second) {
		t.Fatal("quoted folder was not selected")
	}
	if _, err := repositoryFolder(filepath.Join(first, ".git", "HEAD")); err == nil {
		t.Fatal("accepted a file")
	}
	if _, err := repositoryFolder("''"); err == nil {
		t.Fatal("accepted empty quoted path")
	}
	if _, err := repositoryFolder("~"); err != nil {
		t.Fatal(err)
	}
}

func TestBlacklistTyposCanBeCorrectedAndNeedConfirmation(t *testing.T) {
	dir := createCLIRepository(t, false)
	runMenuFixture(t, dir, "2\na\nbad label\nhelper\nnot-an-email\nHelper@Example.org\nmaybe\nYES\nq\n")
	matcher, err := resolveLocalMatcher(dir, "")
	if err != nil || !matcher.Match("Helper", "helper@example.org") {
		t.Fatalf("correction not saved: %v", err)
	}
	runMenuFixture(t, dir, "2\nr\nmissing\nhelper\n\nq\n")
	matcher, err = resolveLocalMatcher(dir, "")
	if err != nil || !matcher.Match("Helper", "helper@example.org") {
		t.Fatal("Enter removed a rule")
	}
	runMenuFixture(t, dir, "2\nr\nhelper\ny\nq\n")
	matcher, err = resolveLocalMatcher(dir, "")
	if err != nil || matcher.Match("Helper", "helper@example.org") {
		t.Fatal("confirmed removal did not apply")
	}
}

func TestInvalidBlacklistCanBeRepairedWithoutLeavingMenu(t *testing.T) {
	dir := createCLIRepository(t, false)
	runGit(t, dir, "config", "byeclaude.blacklist", "invalid-json")
	runMenuFixture(t, dir, "2\nreset\n\nq\n")
	if got := runGit(t, dir, "config", "byeclaude.blacklist"); got != "invalid-json" {
		t.Fatal("cancel changed policy")
	}
	runMenuFixture(t, dir, "2\nreset\nRESET\nq\n")
	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	set, _, err := blacklist.Load(repo)
	if err != nil || len(set.Rules) != 1 || !set.Match("Claude", "noreply@anthropic.com") {
		t.Fatalf("reset: %+v %v", set, err)
	}
}

func TestMenuRefusesPolicyChangesDuringConfirmation(t *testing.T) {
	dir := createCLIRepository(t, false)
	var output bytes.Buffer
	observer := &promptObserver{buffer: &output, prompt: "Save this blacklist", onPrompt: func() {
		runGit(t, dir, "config", "byeclaude.blacklist", `{"rules":[{"id":"other","exact_emails":["other@example.org"]}]}`)
	}}
	ui := terminalUI{repo: dir, in: bufio.NewReader(strings.NewReader("2\na\nhelper\nhelper@example.org\ny\nq\n")), out: observer, invoke: func(string, []string) error { t.Fatal("stale policy was saved"); return nil }}
	if err := ui.run(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "blacklist changed while") {
		t.Fatal(output.String())
	}
}

func TestCleanupDoesNotOfferImpossibleOrUnneededChanges(t *testing.T) {
	for _, state := range []string{"clean", "dirty", "empty"} {
		t.Run(state, func(t *testing.T) {
			dir := t.TempDir()
			if state == "empty" {
				runGit(t, dir, "init", "-q")
			} else {
				dir = createCLIRepository(t, state == "dirty")
			}
			if state == "dirty" {
				if err := os.WriteFile(filepath.Join(dir, "unsaved.txt"), []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			output := runMenuFixture(t, dir, "3\n1\nq\n")
			if strings.Contains(output, "Type CLEAN") {
				t.Fatal("offered mutation without a valid nonempty plan")
			}
			want := "Nothing matches"
			if state == "dirty" {
				want = "Cleanup is blocked"
			}
			if !strings.Contains(output, want) {
				t.Fatal(output)
			}
		})
	}
}

func TestGuidedDefaultsAreReadOnly(t *testing.T) {
	dir := createCLIRepository(t, true)
	before := runGit(t, dir, "show-ref")
	output := runMenuFixture(t, dir, "g\n\n\n\n\n\nq\n")
	for _, step := range []string{"1 of 4", "2 of 4", "3 of 4", "4 of 4", "Walkthrough finished"} {
		if !strings.Contains(output, step) {
			t.Errorf("missing %s", step)
		}
	}
	if runGit(t, dir, "show-ref") != before {
		t.Fatal("guided defaults changed history")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "hooks", "pre-push")); !os.IsNotExist(err) {
		t.Fatal("guided defaults installed a hook")
	}
}

func TestMenuExplanationSurvivesNoColorAndOperationFailure(t *testing.T) {
	dir := createCLIRepository(t, false)
	for _, color := range []bool{true, false} {
		var output bytes.Buffer
		ui := terminalUI{repo: dir, color: color, in: bufio.NewReader(strings.NewReader("5\nh\nq\n")), out: &output, invoke: func(string, []string) error { return fmt.Errorf("fixture error\x1b[2J") }}
		if err := ui.run(); err != nil {
			t.Fatal(err)
		}
		text := output.String()
		if strings.Contains(text, "\x1b[2J") || strings.Contains(text, "\x1b[90m") != color {
			t.Fatal("terminal formatting unsafe")
		}
		for _, want := range []string{"Could not finish", "Your menu is still available", "What ByeClaude does", "co-author credit"} {
			if !strings.Contains(text, want) {
				t.Errorf("missing %q", want)
			}
		}
	}
}

func TestMenuOutputBoundAndControlFiltering(t *testing.T) {
	var output menuOutput
	data := bytes.Repeat([]byte("x"), maxMenuOutput+100)
	if n, err := output.Write(data); n != len(data) || err != nil {
		t.Fatal(n, err)
	}
	if !output.truncated || output.data.Len() != maxMenuOutput {
		t.Fatal("unbounded output")
	}
	if _, err := output.Write(data); err != nil || output.data.Len() != maxMenuOutput {
		t.Fatal("repeated write exceeded bound")
	}
	if got := safeMenuOutput("hi\x1b[2J\rthere\u202e\n\tgood"); got != "hi[2Jthere\n\tgood" {
		t.Fatal(got)
	}
}

func TestMenuChildOutput(t *testing.T) {
	mode := os.Getenv("BYECLAUDE_TEST_CHILD_OUTPUT")
	if mode == "" {
		return
	}
	fmt.Print("child\x1b[2J\u202e\n")
	if mode == "fail" {
		t.Error("child explanation")
	}
}

func TestMenuExecutesSelfWithoutLeakingTerminalControls(t *testing.T) {
	for _, mode := range []string{"ok", "fail"} {
		t.Setenv("BYECLAUDE_TEST_CHILD_OUTPUT", mode)
		var output bytes.Buffer
		err := executeMenuCommand("-test.run=^TestMenuChildOutput$", nil, &output)
		if (err != nil) != (mode == "fail") {
			t.Fatalf("child result %v", err)
		}
		if !strings.Contains(output.String(), "child") || strings.ContainsAny(output.String(), "\x1b\u202e") {
			t.Fatal(output.String())
		}
	}
}
