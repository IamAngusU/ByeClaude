package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxPromptBytes = 4096
const maxPromptAttempts = 3

var errMenuBack = errors.New("return to menu")
var errTerminalAnswer = errors.New("invalid terminal answer")
var errTerminalIO = errors.New("terminal connection failed")

// Read a complete, bounded line. EOF never confirms a partial answer, and an
// oversized line is drained so its tail cannot become the next menu choice.
func readTerminalLine(in *bufio.Reader) (string, error) {
	var line []byte
	tooLong := false
	for {
		part, err := in.ReadSlice('\n')
		if len(line)+len(part) > maxPromptBytes {
			tooLong = true
		}
		if !tooLong {
			line = append(line, part...)
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if err != nil {
			return "", err
		}
		if tooLong {
			return "", fmt.Errorf("%w: answer is too long (maximum %d bytes); please try again", errTerminalAnswer, maxPromptBytes)
		}
		break
	}
	value := strings.TrimSuffix(strings.TrimSuffix(string(line), "\n"), "\r")
	if !utf8.ValidString(value) || strings.IndexFunc(value, unsafeTerminalRune) >= 0 {
		return "", fmt.Errorf("%w: hidden control characters or invalid UTF-8; please type it again", errTerminalAnswer)
	}
	return strings.TrimSpace(value), nil
}

func unsafeTerminalRune(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }

func terminalText(value string) string {
	return strings.Map(func(r rune) rune {
		if unsafeTerminalRune(r) {
			return ' '
		}
		return r
	}, value)
}

func (ui *terminalUI) ask(prompt string) (string, error) {
	for attempt := 0; attempt < maxPromptAttempts; attempt++ {
		if _, err := fmt.Fprint(ui.out, prompt+" > "); err != nil {
			return "", fmt.Errorf("%w: %w", errTerminalIO, err)
		}
		value, err := readTerminalLine(ui.in)
		if err == nil {
			return value, nil
		}
		if errors.Is(err, io.EOF) {
			return "", io.EOF
		}
		// A broken input stream is not a validation error and cannot be retried.
		if !errors.Is(err, errTerminalAnswer) {
			return "", fmt.Errorf("%w: %w", errTerminalIO, err)
		}
		ui.hint(err.Error())
	}
	ui.hint("Too many invalid answers. The pending action was cancelled.")
	return "", errMenuBack
}

func (ui *terminalUI) choice(prompt, fallback string, allowed ...string) (string, error) {
	for attempt := 0; attempt < maxPromptAttempts; attempt++ {
		value, err := ui.ask(prompt)
		if err != nil {
			return "", err
		}
		value = strings.ToLower(value)
		if value == "" {
			value = fallback
		}
		for _, option := range allowed {
			if value == option {
				return value, nil
			}
		}
		ui.hint("Choose " + strings.Join(allowed, ", ") + ". Nothing was changed by that answer.")
	}
	ui.hint("Too many invalid choices. Returning to the menu.")
	return "", errMenuBack
}

func (ui *terminalUI) confirm(prompt string) (bool, error) {
	value, err := ui.choice(prompt+" [y/N, q back]", "n", "y", "yes", "n", "no", "q")
	if err != nil {
		return false, err
	}
	if value == "q" {
		return false, errMenuBack
	}
	return value == "y" || value == "yes", nil
}

func (ui *terminalUI) hint(value string) {
	if ui.color {
		fmt.Fprintln(ui.out, "  \x1b[90m"+terminalText(value)+"\x1b[0m")
	} else {
		fmt.Fprintln(ui.out, "  "+terminalText(value))
	}
}

func (ui *terminalUI) option(key, title, help string) {
	fmt.Fprintf(ui.out, "  %s  %s\n", tint(ui.color, "1;36", key), tint(ui.color, "1", terminalText(title)))
	ui.hint("   " + help)
}
