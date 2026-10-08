package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/IamAngusU/ByeClaude/internal/metrics"
)

func metricsReset(before, now metrics.State) bool {
	a, b := before.Counters, now.Counters
	return before.Since != now.Since && before.Since != "" ||
		b.CleanupCredits < a.CleanupCredits || b.HookCredits < a.HookCredits ||
		b.CommitsRewritten < a.CommitsRewritten || b.PushBlocks < a.PushBlocks ||
		b.Scans < a.Scans || b.Checks < a.Checks || b.Cleanups < a.Cleanups ||
		b.PushChecks < a.PushChecks || b.IdentityFields < a.IdentityFields ||
		b.HookEdits < a.HookEdits || b.CommitsInspected < a.CommitsInspected || b.WorkMS < a.WorkMS
}

func (ui *terminalUI) watchMetrics(store metrics.Store, seconds float64) error {
	state, err := store.Read()
	if err != nil {
		return err
	}
	w, h := terminalSize(ui.out)
	if !ui.motion || w < 72 || h < 30 {
		fmt.Fprint(ui.out, renderMetrics(state, metricsView{color: ui.color, width: w, seconds: seconds}))
		ui.hint("Live view needs an interactive terminal of at least 72 x 30.")
		ui.hint("Showing a snapshot. Resize and run metrics --watch to refresh live.")
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return ui.metricsLoop(ctx, store, state, seconds, time.Second)
}

// One bounded line reader owns input for the entire view. It is joined before
// returning to the menu. Interrupt and output failures terminate the caller's
// CLI (EOF / errTerminalIO), so no reader can consume later menu answers.
func (ui *terminalUI) metricsLoop(ctx context.Context, store metrics.Store, state metrics.State, seconds float64, interval time.Duration) error {
	if _, err := fmt.Fprint(ui.out, "\x1b[?1049h\x1b[2J\x1b[H"); err != nil {
		return fmt.Errorf("%w: %w", errTerminalIO, err)
	}
	defer func() { _, _ = fmt.Fprint(ui.out, "\x1b[0m\x1b[?1049l") }()
	baseline := state
	notice := ""
	lastWidth, lastHeight := terminalSize(ui.out)
	draw := func(first bool) error {
		w, h := terminalSize(ui.out)
		view := renderMetrics(state, metricsView{color: ui.color, width: w, live: true, seconds: seconds, baseline: &baseline, notice: notice})
		if w < 72 || h < 30 {
			view = "  Resize to 72 x 30 for the live dashboard.\n  Counters continue to update. Enter returns.\n"
		}
		var frame strings.Builder
		frame.WriteString("\x1b7\x1b[H")
		lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
		for i := 0; i < max(0, h-3); i++ {
			frame.WriteString("\x1b[2K")
			if i < len(lines) {
				frame.WriteString(lines[i])
			}
			frame.WriteString("\r\n")
		}
		frame.WriteString("\x1b8")
		if first {
			fmt.Fprintf(&frame, "\x1b[%d;1H  Enter to return > ", max(1, h-1))
		}
		_, err := fmt.Fprint(ui.out, frame.String())
		return err
	}
	if err := draw(true); err != nil {
		return fmt.Errorf("%w: %w", errTerminalIO, err)
	}
	answer := make(chan error, 1)
	go func() { _, err := readTerminalLine(ui.in); answer <- err }()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case err := <-answer:
			// Any completed line closes this read-only view, including an invalid
			// answer. It never becomes a confirmation in the next screen.
			if err == io.EOF {
				return io.EOF
			}
			if err != nil && !errors.Is(err, errTerminalAnswer) {
				return fmt.Errorf("%w: %w", errTerminalIO, err)
			}
			return nil
		case <-ctx.Done():
			return io.EOF
		case <-ticker.C:
			previous, oldNotice := state, notice
			next, err := store.Read()
			if err != nil {
				notice = "Storage unavailable. Last known totals; retrying..."
			} else {
				notice = ""
				if metricsReset(state, next) {
					baseline = next
					notice = "Counters reset. Session changes restarted."
				}
				state = next
			}
			w, h := terminalSize(ui.out)
			resized := w != lastWidth || h != lastHeight
			if previous == state && oldNotice == notice && !resized {
				continue
			}
			lastWidth, lastHeight = w, h
			// Re-anchor the input hint after resizing; otherwise a taller frame
			// would erase its old position. Any completed line simply returns.
			if err := draw(resized); err != nil {
				return fmt.Errorf("%w: %w", errTerminalIO, err)
			}
		}
	}
}
