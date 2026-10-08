package main

import (
	"encoding/json"
	"flag"
	"fmt"
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
	if err := ui.invoke("metrics", nil); err != nil {
		return err
	}
	choice, err := ui.choice("Metrics [e: estimate, on/off: collection, reset, Enter: back]", "back", "e", "on", "off", "reset", "back", "q")
	if err != nil {
		return err
	}
	switch choice {
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
		return ui.invoke("metrics", []string{"--seconds-per-credit", value})
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
	confirm := fs.Bool("confirm", false, "confirm deleting local counters")
	seconds := fs.Float64("seconds-per-credit", 0, "optional manual-work estimate; seconds per removed credit (not measured savings)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected metrics arguments")
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
	fmt.Println("ByeClaude / Local metrics")
	status := "on"
	if !state.Enabled {
		status = "off"
	}
	if paused {
		status += " (collection paused by environment / CI)"
	}
	fmt.Printf("Collection          %s\n", status)
	if state.Since != "" {
		fmt.Printf("Since               %s\n", state.Since)
	}
	c := state.Counters
	fmt.Printf("\nWork handled for you\n  History credits removed     %d\n  Commit-message credits      %d (across %d hook edits)\n  Commits rewritten           %d (includes descendants)\n  Identity fields corrected   %d\n  Push attempts blocked       %d / %d checked\n", c.CleanupCredits, c.HookCredits, c.HookEdits, c.CommitsRewritten, c.IdentityFields, c.PushBlocks, c.PushChecks)
	fmt.Printf("\nActivity\n  Scans / checks / cleanups   %d / %d / %d\n  Commits inspected          %d (repeat inspections count again)\n  Recorded processing time   %.2fs (not time saved)\n", c.Scans, c.Checks, c.Cleanups, c.CommitsInspected, float64(c.WorkMS)/1000)
	if estimate != nil {
		fmt.Printf("\nEstimated manual editing    %.1f minutes\nAssumption: %.1fs per removed credit; not measured time savings.\n", *estimate/60, *seconds)
	} else {
		fmt.Println("\nNo time-savings claim. Model an estimate with --seconds-per-credit 30.")
	}
	fmt.Println("Totals cover completed operations since reset, including later-undone work.")
	fmt.Println("Hook edits count message edits, even if Git later cancels that commit.")
	fmt.Println("Local counters only: no uploads, repository names, emails or commit content.")
	fmt.Println("Storage is best-effort; unavailable or busy storage can omit operations.")
	fmt.Println("Control: metrics off | metrics on | metrics reset --confirm")
	return nil
}
