package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"

	"github.com/IamAngusU/ByeClaude/internal/metrics"
)

func metricCount(value int64) uint64 {
	if value < 0 {
		return 0
	}
	return uint64(value)
}

func (ui *terminalUI) showMetrics() error {
	store, err := metrics.DefaultStore()
	if err != nil {
		return err
	}
	state, err := store.Read()
	if err != nil {
		return fmt.Errorf("local metrics unavailable: %w; repair with 'byeclaude metrics reset --confirm'; Git operations still work", err)
	}
	w, _ := terminalSize(ui.out)
	fmt.Fprint(ui.out, "\n"+renderMetrics(state, metricsView{color: ui.color, width: w}))
	choice, err := ui.choice("Metrics [w: live, d: details, e: estimate, on/off, reset, Enter: back]", "back", "w", "d", "e", "on", "off", "reset", "back", "q")
	if err != nil {
		return err
	}
	switch choice {
	case "w":
		return ui.watchMetrics(store, 0)
	case "d":
		fmt.Fprint(ui.out, renderMetrics(state, metricsView{color: ui.color, width: w, details: true}))
		_, err := ui.ask("Enter to return")
		return err
	case "on", "off":
		return ui.invoke("metrics", []string{choice})
	case "reset":
		ui.hint("This deletes your local counters, not Git backups or history.")
		ok, err := ui.typedConfirmation("RESET", "clear the local counters")
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		return ui.invoke("metrics", []string{"reset", "--confirm"})
	case "e":
		ui.hint("Choose how long you would spend manually removing one credit.")
		ui.hint("This models manual editing effort; it is not measured time savings.")
		value, err := ui.validated("Seconds per credit (1-3600)", func(value string) error {
			n, err := strconv.ParseFloat(value, 64)
			if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 1 || n > 3600 {
				return fmt.Errorf("Enter a number between 1 and 3600, such as 30.")
			}
			return nil
		})
		if err != nil {
			return err
		}
		seconds, _ := strconv.ParseFloat(value, 64)
		fmt.Fprint(ui.out, renderMetrics(state, metricsView{color: ui.color, width: w, seconds: seconds}))
		_, err = ui.ask("Enter to return")
		return err
	}
	return nil
}

func runMetrics(args []string) error {
	action := "show"
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		action, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("metrics", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print local counters as JSON")
	watch := fs.Bool("watch", false, "refresh local counters every second; Enter returns")
	details := fs.Bool("details", false, "show counting notes and processing time")
	plain := fs.Bool("no-color", false, "disable color")
	confirm := fs.Bool("confirm", false, "confirm deleting local counters")
	seconds := fs.Float64("seconds-per-credit", 0, "optional manual-work estimate; seconds per removed credit (not measured savings)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected metrics arguments")
	}
	if *watch && (*jsonOut || action != "show") {
		return fmt.Errorf("--watch works with metrics show, without --json")
	}
	if *watch && *details {
		return fmt.Errorf("use --details without --watch to read the counting notes")
	}
	if math.IsNaN(*seconds) || math.IsInf(*seconds, 0) || *seconds < 0 || *seconds > 3600 {
		return fmt.Errorf("seconds-per-credit must be between 0 and 3600")
	}
	store, err := metrics.DefaultStore()
	if err != nil {
		return err
	}
	switch action {
	case "show":
	case "on", "off":
		err = store.Enable(action == "on")
	case "reset":
		if !*confirm {
			return fmt.Errorf("reset deletes local counters; run byeclaude metrics reset --confirm to proceed")
		}
		err = store.Reset()
	default:
		return fmt.Errorf("choose metrics show, on, off, or reset --confirm")
	}
	if err != nil {
		return fmt.Errorf("local metrics unavailable: %w; Git operations still work", err)
	}
	state, err := store.Read()
	if err != nil {
		return fmt.Errorf("local metrics unavailable: %w; repair counters with 'byeclaude metrics reset --confirm'; Git operations still work", err)
	}
	var estimate *float64
	if *seconds > 0 {
		value := (float64(state.Counters.CleanupCredits) + float64(state.Counters.HookCredits)) * *seconds
		estimate = &value
	}
	paused := metrics.SessionPaused()
	if *jsonOut {
		return json.NewEncoder(os.Stdout).Encode(struct {
			metrics.State
			SessionPaused          bool     `json:"session_paused"`
			SecondsPerCredit       float64  `json:"assumed_seconds_per_credit,omitempty"`
			EstimatedManualSeconds *float64 `json:"estimated_manual_seconds,omitempty"`
		}{state, paused, *seconds, estimate})
	}
	color, motion, restore := terminalPalette(*plain)
	defer restore()
	ui := terminalUI{in: bufio.NewReader(os.Stdin), out: os.Stdout, color: color, motion: motion}
	if *watch {
		err := ui.watchMetrics(store, *seconds)
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	w, _ := terminalSize(os.Stdout)
	_, err = fmt.Fprint(os.Stdout, renderMetrics(state, metricsView{color: color, width: w, details: *details, seconds: *seconds}))
	return err
}
