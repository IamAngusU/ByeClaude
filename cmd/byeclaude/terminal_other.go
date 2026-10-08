//go:build !windows

package main

import "os"

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	null, err := os.Stat(os.DevNull)
	return err != nil || !os.SameFile(info, null)
}

func enableTerminalColor(file *os.File) (bool, func()) { return isTerminal(file), func() {} }
