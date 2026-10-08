package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/metrics"
	"github.com/IamAngusU/ByeClaude/internal/pathsetup"
	"github.com/IamAngusU/ByeClaude/internal/preset"
)

func metricStoreForTest(t *testing.T) metrics.Store {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("BYECLAUDE_STATE_DIR", dir)
	t.Setenv("BYECLAUDE_METRICS", "on")
	return metrics.Store{Dir: dir}
}

func TestMetricCountRejectsNegativeDurations(t *testing.T) {
	if metricCount(-1) != 0 || metricCount(0) != 0 || metricCount(42) != 42 {
		t.Fatal("invalid metric conversion")
	}
}

func captureCommand(t *testing.T, command func() error) (string, error) {
	t.Helper()
	previous := os.Stdout
	f, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	os.Stdout = f
	defer func() { os.Stdout = previous }()
	commandErr := command()
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), commandErr
}

func TestMetricsRecordCompletedWorkWithoutCountingPreviews(t *testing.T) {
	s := metricStoreForTest(t)
	dir := createCLIRepository(t, true)
	for i := 0; i < 2; i++ {
		if err := runScan([]string{"--repo", dir}); err != nil {
			t.Fatal(err)
		}
	}
	if err := runCheck([]string{"--repo", dir}); err == nil {
		t.Fatal("matching check should fail")
	}
	if err := runClean([]string{"--repo", dir}); err != nil {
		t.Fatal(err)
	}
	before, _ := s.Read()
	if before.Counters.Scans != 2 || before.Counters.Checks != 1 || before.Counters.Cleanups != 0 {
		t.Fatal(before)
	}
	if err := runClean([]string{"--repo", dir, "--apply"}); err != nil {
		t.Fatal(err)
	}
	after, _ := s.Read()
	if after.Counters.Cleanups != 1 || after.Counters.CleanupCredits != 1 || after.Counters.CommitsRewritten != 1 {
		t.Fatal(after)
	}
	if err := runClean([]string{"--repo", dir, "--apply"}); err != nil {
		t.Fatal(err)
	}
	again, _ := s.Read()
	if again.Counters != after.Counters {
		t.Fatal("no-op counted as work", again)
	}
}

func TestHookMetricsCountEditsAndPushAttempts(t *testing.T) {
	s := metricStoreForTest(t)
	dir := createCLIRepository(t, true)
	inWorkingDirectory(t, dir)
	message := filepath.Join(dir, ".git", "COMMIT_EDITMSG")
	if err := os.WriteFile(message, []byte("Subject\n\nCo-authored-by: Claude <noreply@anthropic.com>\nCo-authored-by: Human <human@example.org>\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := filterCommitMessage(message, preset.Claude()); err != nil {
			t.Fatal(err)
		}
	}
	stdin, err := os.CreateTemp(t.TempDir(), "push-input")
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	head := runGit(t, dir, "rev-parse", "HEAD")
	if _, err := stdin.WriteString("refs/heads/main " + head + " refs/heads/main " + strings.Repeat("0", 40) + "\n"); err != nil {
		t.Fatal(err)
	}
	_, _ = stdin.Seek(0, 0)
	previous := os.Stdin
	os.Stdin = stdin
	defer func() { os.Stdin = previous }()
	if err := runPrePushFilter(nil); err == nil || !strings.Contains(err.Error(), "push blocked") {
		t.Fatal(err)
	}
	// A malformed protocol is an error, not a prevented matching push.
	if err := stdin.Truncate(0); err != nil {
		t.Fatal(err)
	}
	_, _ = stdin.Seek(0, 0)
	_, _ = stdin.WriteString("broken\n")
	_, _ = stdin.Seek(0, 0)
	if err := runPrePushFilter(nil); err == nil {
		t.Fatal("malformed push accepted")
	}
	state, _ := s.Read()
	if state.Counters.HookCredits != 1 || state.Counters.HookEdits != 1 || state.Counters.PushBlocks != 1 || state.Counters.PushChecks != 1 {
		t.Fatal(state)
	}
}

func TestMetricControlsJSONAndExplicitEstimate(t *testing.T) {
	s := metricStoreForTest(t)
	if err := s.Add(metrics.Counters{CleanupCredits: 2, HookCredits: 4}); err != nil {
		t.Fatal(err)
	}
	output, err := captureCommand(t, func() error { return runMetrics([]string{"--json", "--seconds-per-credit", "30"}) })
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatal(output, err)
	}
	if result["estimated_manual_seconds"] != float64(180) || result["assumed_seconds_per_credit"] != float64(30) {
		t.Fatal(result)
	}
	output, err = captureCommand(t, func() error { return runMetrics(nil) })
	if err != nil || !strings.Contains(output, "not time saved") || !strings.Contains(output, "No time-savings claim") {
		t.Fatal(output, err)
	}
	for _, args := range [][]string{{"reset"}, {"unknown"}, {"--seconds-per-credit", "NaN"}, {"--seconds-per-credit", "Inf"}, {"--seconds-per-credit", "-1"}} {
		if err := runMetrics(args); err == nil {
			t.Fatal(args)
		}
	}
	if err := runMetrics([]string{"off"}); err != nil {
		t.Fatal(err)
	}
	metrics.Record(metrics.Counters{Scans: 1})
	state, _ := s.Read()
	if state.Enabled || state.Counters.Scans != 0 {
		t.Fatal(state)
	}
	if err := runMetrics([]string{"reset", "--confirm"}); err != nil {
		t.Fatal(err)
	}
	state, _ = s.Read()
	if state.Enabled || state.Counters != (metrics.Counters{}) {
		t.Fatal(state)
	}
	if err := runMetrics([]string{"on"}); err != nil {
		t.Fatal(err)
	}
}

func TestMetricsStorageFailureDoesNotBlockCleanup(t *testing.T) {
	s := metricStoreForTest(t)
	if err := os.WriteFile(filepath.Join(s.Dir, "metrics.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	dir := createCLIRepository(t, true)
	if err := runClean([]string{"--repo", dir, "--apply"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(runGit(t, dir, "log", "-1", "--format=%B"), "anthropic.com") {
		t.Fatal("cleanup did not finish")
	}
	if _, err := captureCommand(t, func() error { return runMetrics(nil) }); err == nil {
		t.Fatal("corrupt counters hidden")
	}
}

func TestDeferredPathPromptCanWaitOrStopWithoutFailingMenu(t *testing.T) {
	t.Setenv("BYECLAUDE_STATE_DIR", t.TempDir())
	for _, input := range []string{"\n", "n\n"} {
		if err := pathsetup.SavePreference("pending"); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		ui := terminalUI{in: bufio.NewReader(strings.NewReader(input)), out: &out}
		if err := ui.offerPathRetry(); err != nil {
			t.Fatal(err)
		}
		p, _ := pathsetup.ReadPreference()
		expected := "pending"
		if input == "n\n" {
			expected = "skipped"
		}
		if p.Mode != expected {
			t.Fatal(p)
		}
	}
	var out bytes.Buffer
	reportPathSetup(&out, "C:/tools/byeclaude.exe", func(string) error { return errors.New("access denied") })
	if !strings.Contains(out.String(), "remains usable") || !strings.Contains(out.String(), "path setup") {
		t.Fatal(out.String())
	}
}

func TestMenuMetricsEstimateAndResetStayExplicit(t *testing.T) {
	s := metricStoreForTest(t)
	if err := s.Add(metrics.Counters{CleanupCredits: 2}); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"m\nreset\n\nq\n", "m\ne\nwrong\n30\nq\n"} {
		if _, err := captureCommand(t, func() error { runMenuFixture(t, createCLIRepository(t, false), input); return nil }); err != nil {
			t.Fatal(err)
		}
		state, _ := s.Read()
		if state.Counters.CleanupCredits != 2 {
			t.Fatal("unconfirmed reset changed metrics")
		}
	}
	_, _ = captureCommand(t, func() error { runMenuFixture(t, createCLIRepository(t, false), "m\nreset\nRESET\nq\n"); return nil })
	state, _ := s.Read()
	if state.Counters != (metrics.Counters{}) {
		t.Fatal(state)
	}
}
