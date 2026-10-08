package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/IamAngusU/ByeClaude/internal/progress"
)

func progressText(p progress.Event, tick int, color bool) string {
	if p.Total > 0 {
		done := max(0, min(p.Done, p.Total))
		fraction := float64(done) / float64(p.Total)
		fill := int(fraction * 18)
		bar := tint(color, "1;36", strings.Repeat("=", fill)) + tint(color, "90", strings.Repeat("-", 18-fill))
		stage := []rune(terminalText(p.Stage))
		if p.Stage == "Loading commit objects" {
			stage = []rune("Reading commits")
		}
		if len(stage) > 18 {
			stage = stage[:18]
		}
		return fmt.Sprintf("%-18s [%s] %3d%%  %d/%d", string(stage), bar, int(fraction*100), done, p.Total)
	}
	return tint(color, "1;36", string("|/-\\"[tick%4])) + " " + terminalText(p.Stage)
}

// Rendering never controls success or cancels a mutation halfway through.
// Work is joined even if the terminal disappears; then the IO error exits the UI.
func (ui *terminalUI) working(label string, work func(context.Context) error) error {
	if !ui.motion {
		ui.hint(label + "...")
		err := work(context.Background())
		if err == nil {
			ui.hint("[x] " + label + " finished")
		}
		return err
	}
	var mu sync.Mutex
	p := progress.Event{Stage: label}
	ctx := progress.WithReporter(context.Background(), func(next progress.Event) { mu.Lock(); p = next; mu.Unlock() })
	done := make(chan error, 1)
	go func() { done <- work(ctx) }()
	started := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var outputErr error
	draw := func(line string, final bool) {
		if outputErr != nil {
			return
		}
		ending := ""
		if final {
			ending = "\n"
		}
		_, outputErr = fmt.Fprintf(ui.out, "\r\x1b[2K  %s%s", line, ending)
	}
	tick := 0
	for {
		select {
		case err := <-done:
			mark, style := "[x]", "1;36"
			if err != nil {
				mark, style = "[!]", "33"
			}
			draw(tint(ui.color, style, mark)+" "+label+fmt.Sprintf("  %.1fs", time.Since(started).Seconds()), true)
			if outputErr != nil {
				return fmt.Errorf("%w: %w", errTerminalIO, outputErr)
			}
			return err
		case <-ticker.C:
			mu.Lock()
			current := p
			mu.Unlock()
			w, _ := terminalSize(ui.out)
			if w < 65 {
				current = progress.Event{Stage: "Working..."}
			}
			draw(progressText(current, tick, ui.color), false)
			tick++
		}
	}
}
