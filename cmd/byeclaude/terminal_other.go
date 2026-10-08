//go:build !windows

package main

import (
	"golang.org/x/term"
	"os"
)

func isTerminal(file *os.File) bool {
	return term.IsTerminal(int(file.Fd()))
}

func enableTerminalColor(file *os.File) (bool, func()) { return isTerminal(file), func() {} }
