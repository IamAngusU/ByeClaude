package main

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/IamAngusU/ByeClaude/internal/metrics"
)

type metricsView struct {
	color, details, live bool
	width                int
	seconds              float64
	baseline             *metrics.State
	notice               string
}

func creditsTotal(c metrics.Counters) string {
	// Each stored counter fits uint64, but their combined total might not.
	a := new(big.Int).SetUint64(c.CleanupCredits)
	return a.Add(a, new(big.Int).SetUint64(c.HookCredits)).String()
}

func countDelta(now, before uint64) string {
	if now > before {
		return "+" + strconv.FormatUint(now-before, 10)
	}
	return ""
}

func renderMetrics(state metrics.State, v metricsView) string {
	var out strings.Builder
	accent := func(s string) string { return tint(v.color, "1;36", s) }
	dim := func(s string) string { return tint(v.color, "90", s) }
	line := func(s string) { fmt.Fprintln(&out, "  "+s) }
	w := max(28, min(68, v.width-4))
	rule := func() { line(dim(strings.Repeat("-", w))) }
	c := state.Counters
	line(accent("BYECLAUDE / MY IMPACT"))
	line(dim("Powered by angusu.de | Angus Uelsmann"))
	status := "ON"
	if !state.Enabled {
		status = "OFF"
	} else if metrics.SessionPaused() {
		status = "PAUSED (env/CI)"
	}
	mode := "SNAPSHOT"
	if v.live {
		mode = "LIVE / 1s"
	}
	line(dim(mode+"  |  LOCAL ONLY  |  COLLECTION ") + accent(status))
	rule()
	base := metrics.Counters{}
	if v.baseline != nil {
		base = v.baseline.Counters
	}
	creditDelta := ""
	if v.baseline != nil {
		n, _ := new(big.Int).SetString(creditsTotal(c), 10)
		b, _ := new(big.Int).SetString(creditsTotal(base), 10)
		if n.Cmp(b) > 0 {
			creditDelta = "+" + n.Sub(n, b).String()
		}
	}
	heroes := []struct{ label, number, delta string }{
		{"Credits removed", creditsTotal(c), creditDelta},
		{"Pushes blocked", strconv.FormatUint(c.PushBlocks, 10), countDelta(c.PushBlocks, base.PushBlocks)},
		{"Commits rewritten", strconv.FormatUint(c.CommitsRewritten, 10), countDelta(c.CommitsRewritten, base.CommitsRewritten)},
	}
	columns := w >= 66
	for _, h := range heroes {
		columns = columns && len(h.number) <= 20 && len(h.delta) <= 20
	}
	if columns {
		for _, h := range heroes {
			fmt.Fprint(&out, "  "+dim(fmt.Sprintf("%-20s", h.label)))
		}
		fmt.Fprintln(&out)
		for _, h := range heroes {
			fmt.Fprint(&out, "  "+accent(fmt.Sprintf("%-20s", h.number)))
		}
		fmt.Fprintln(&out)
		if v.live {
			for _, h := range heroes {
				fmt.Fprint(&out, "  "+accent(fmt.Sprintf("%-20s", h.delta)))
			}
			fmt.Fprintln(&out)
		}
	} else {
		for _, h := range heroes {
			line(dim(h.label) + "  " + accent(h.number))
			if v.live && h.delta != "" {
				line(accent(h.delta + " this session"))
			}
		}
	}
	rule()
	line(dim(fmt.Sprintf("%-29s %20s", "Activity", "Total")))
	rows := []struct {
		label string
		n, b  uint64
	}{
		{"History credits", c.CleanupCredits, base.CleanupCredits},
		{"Commit-hook credits", c.HookCredits, base.HookCredits},
		{"Pushes checked", c.PushChecks, base.PushChecks},
		{"Identity fields corrected", c.IdentityFields, base.IdentityFields},
		{"Scans", c.Scans, base.Scans},
		{"Checks", c.Checks, base.Checks},
		{"Cleanups applied", c.Cleanups, base.Cleanups},
	}
	for _, row := range rows {
		if w < 52 {
			line(row.label + "  " + accent(strconv.FormatUint(row.n, 10)))
			continue
		}
		value := fmt.Sprintf("%-29s ", row.label) + accent(fmt.Sprintf("%20d", row.n))
		if v.live && v.baseline != nil {
			d := countDelta(row.n, row.b)
			if len(d) <= w-53 {
				value += "  " + accent(d)
			}
		}
		line(value)
	}
	rule()
	if v.seconds > 0 {
		estimate := (float64(c.CleanupCredits) + float64(c.HookCredits)) * v.seconds / 60
		line(accent(fmt.Sprintf("~%.1f min", estimate)) + "  modeled manual editing effort")
		line(dim(fmt.Sprintf("Your assumption: %.1fs/credit; not measured savings.", v.seconds)))
	} else {
		line(dim("No time-savings claim. Estimate: --seconds-per-credit 30."))
	}
	if c == (metrics.Counters{}) {
		line(dim("Ready for your first scan. Completed work appears here."))
	} else if since, err := time.Parse(time.RFC3339, state.Since); err == nil {
		line(dim("Activity since " + since.Format("02 Jan 2006") + "; includes later-undone work."))
	}
	if v.notice != "" {
		line(tint(v.color, "33", terminalText(v.notice)))
	}
	if v.live {
		line(dim("+ values = changes since opening this view."))
	} else {
		line(dim("--watch: live   --details: counting notes"))
	}
	if v.details {
		line("")
		line(accent("Counting notes"))
		line(fmt.Sprintf("Hook edits: %d; commits inspected: %d", c.HookEdits, c.CommitsInspected))
		line(fmt.Sprintf("Processing: %.2fs (not time saved)", float64(c.WorkMS)/1000))
		line(dim("Repeated inspections count again; rewrites include descendants."))
		line(dim("Hook edits count even if Git later cancels the commit."))
		line(dim("Best-effort storage may omit work when unavailable or busy."))
		line(dim("No uploads, repository names, emails or commit content."))
	}
	return out.String()
}
