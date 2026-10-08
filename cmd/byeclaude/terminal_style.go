package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Only application-owned strings receive styling. Repository text is sanitized
// before reaching any terminal renderer.
func tint(color bool, code, text string) string {
	if color {
		return "\x1b[" + code + "m" + text + "\x1b[0m"
	}
	return text
}

func terminalSize(out io.Writer) (int, int) {
	if f, ok := out.(*os.File); ok {
		if w, h, err := term.GetSize(int(f.Fd())); err == nil && w > 0 && h > 0 {
			return w, h
		}
	}
	return 80, 30
}

func terminalPalette(plain bool) (bool, bool, func()) {
	if !terminalInput() || os.Getenv("TERM") == "dumb" {
		return false, false, func() {}
	}
	vt, restore := enableTerminalColor(os.Stdout)
	_, noColor := os.LookupEnv("NO_COLOR")
	styled := vt && !plain && !noColor
	return styled, styled, restore
}

func (ui *terminalUI) rule() {
	w, _ := terminalSize(ui.out)
	fmt.Fprintln(ui.out, "  "+tint(ui.color, "90", strings.Repeat("-", max(1, min(68, w-4)))))
}

func (ui *terminalUI) stat(label string, n int) {
	fmt.Fprintf(ui.out, "  %-27s %s\n", label, tint(ui.color, "1;36", fmt.Sprintf("%10d", n)))
}

func (ui *terminalUI) step(current int) {
	labels := []string{"Identities", "Check", "Preview", "Protect"}
	ui.rule()
	for i, label := range labels {
		mark, style := "[ ]", "90"
		if i+1 < current {
			mark, style = "[x]", "90"
		} else if i+1 == current {
			mark, style = "[>]", "1;36"
		}
		fmt.Fprintln(ui.out, "  "+tint(ui.color, style, mark+" "+label))
	}
	ui.hint("[x] reviewed or skipped  [>] current  [ ] upcoming")
}
