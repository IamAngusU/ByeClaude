package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IamAngusU/ByeClaude/internal/metrics"
	"github.com/IamAngusU/ByeClaude/internal/progress"
)

func TestMetricsLayoutCountsDeltasAndOverflow(t *testing.T) {
	t.Setenv("BYECLAUDE_METRICS", "on")
	base := metrics.State{Enabled: true, Counters: metrics.Counters{CleanupCredits: 5, PushBlocks: 1}}
	now := base
	now.Counters.CleanupCredits = 8
	now.Counters.HookCredits = 2
	now.Counters.PushBlocks = 2
	for _, width := range []int{80, 60, 40} {
		out := renderMetrics(now, metricsView{width: width, live: true, baseline: &base})
		if !strings.Contains(out, "10") || !strings.Contains(out, "+5") || strings.Contains(out, "\x1b") {
			t.Fatal(out)
		}
		if strings.Contains(out, "Processing:") {
			t.Fatal("details leaked into default", out)
		}
	}
	now.Counters.CleanupCredits = math.MaxUint64
	now.Counters.HookCredits = math.MaxUint64
	out := renderMetrics(now, metricsView{width: 80, seconds: 30, details: true})
	if !strings.Contains(out, "36893488147419103230") || !strings.Contains(out, "not measured savings") || !strings.Contains(out, "not time saved") {
		t.Fatal(out)
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) > 72 {
			t.Fatal("overflow broke layout", line)
		}
	}
	if countDelta(0, math.MaxUint64) != "" || !metricsReset(base, metrics.State{}) {
		t.Fatal("reset underflow")
	}
}

type observedWriter struct {
	bytes.Buffer
	observe func(string)
}

func (w *observedWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if w.observe != nil {
		w.observe(string(p))
	}
	return n, err
}

func TestLiveMetricsRefreshResetFailureAndReturn(t *testing.T) {
	s := metricStoreForTest(t)
	if err := s.Add(metrics.Counters{CleanupCredits: 2}); err != nil {
		t.Fatal(err)
	}
	initial, _ := s.Read()
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()
	var out observedWriter
	frames := 0
	out.observe = func(frame string) {
		if !strings.Contains(frame, "BYECLAUDE / MY IMPACT") {
			return
		}
		frames++
		var err error
		switch frames {
		case 1:
			err = s.Add(metrics.Counters{CleanupCredits: 3, Scans: 1})
		case 2:
			if !strings.Contains(frame, "+3") {
				t.Error("no live delta", frame)
			}
			err = s.Reset()
		case 3:
			if !strings.Contains(frame, "Counters reset") {
				t.Error("reset not acknowledged", frame)
			}
			err = os.WriteFile(filepath.Join(s.Dir, "metrics.json"), []byte("broken"), 0600)
		case 4:
			if !strings.Contains(frame, "Storage unavailable") {
				t.Error("stale totals unlabelled", frame)
			}
			err = s.Reset()
		case 5:
			go func() { _, _ = io.WriteString(w, "\x1b[2J\nq\n") }()
		}
		if err != nil {
			t.Error(err)
		}
	}
	ui := terminalUI{in: bufio.NewReader(r), out: &out, color: true, motion: true}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := ui.metricsLoop(ctx, s, initial, 0, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if frames < 5 || !strings.HasSuffix(out.String(), "\x1b[0m\x1b[?1049l") {
		t.Fatal("view not restored", out.String())
	}
	line, err := readTerminalLine(ui.in)
	if err != nil || line != "q" {
		t.Fatal("reader stole following menu input", line, err)
	}
	state, _ := s.Read()
	if state.Counters != (metrics.Counters{}) || state.Enabled {
		t.Fatal("reading recorded work or changed repaired preference", state)
	}
}

func TestLiveMetricsFallbackAndInvalidFlagsAreReadOnly(t *testing.T) {
	s := metricStoreForTest(t)
	for _, args := range [][]string{{"off", "--watch"}, {"--watch", "--json"}} {
		if err := runMetrics(args); err == nil {
			t.Fatal(args)
		}
	}
	state, _ := s.Read()
	if !state.Enabled {
		t.Fatal("invalid flags changed preferences")
	}
	out, err := captureCommand(t, func() error { return runMetrics([]string{"--watch"}) })
	if err != nil || strings.Contains(out, "\x1b") || !strings.Contains(out, "snapshot") {
		t.Fatal(out, err)
	}
}

type brokenTerminalWriter struct{}

func (brokenTerminalWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestProgressFailureDoesNotReportSuccessOrAbandonWork(t *testing.T) {
	var out bytes.Buffer
	ui := terminalUI{out: &out, motion: true}
	err := ui.working("Check", func(context.Context) error { return errors.New("failure") })
	if err == nil || strings.Contains(out.String(), "[x]") || !strings.Contains(out.String(), "[!]") {
		t.Fatal(out.String(), err)
	}
	ui.out = brokenTerminalWriter{}
	finished := false
	err = ui.working("Cleanup", func(context.Context) error { finished = true; return nil })
	if !finished || !errors.Is(err, errTerminalIO) {
		t.Fatal("operation abandoned", err)
	}
	for _, event := range []progress.Event{{Done: -1, Total: 10}, {Done: 500, Total: 10}, {Total: 0, Stage: "Waiting"}} {
		text := progressText(event, 0, false)
		if strings.Contains(text, "NaN") || strings.Contains(text, "5000%") || strings.Contains(text, "-10%") {
			t.Fatal(text)
		}
	}
}
