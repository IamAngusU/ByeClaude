package main

import (
	"os"
	"syscall"
)

func isTerminal(file *os.File) bool {
	var mode uint32
	return syscall.GetConsoleMode(syscall.Handle(file.Fd()), &mode) == nil
}

func enableTerminalColor(file *os.File) (bool, func()) {
	handle := syscall.Handle(file.Fd())
	var original uint32
	if syscall.GetConsoleMode(handle, &original) != nil {
		return false, func() {}
	}
	setMode := syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleMode")
	ok, _, _ := setMode.Call(uintptr(handle), uintptr(original|0x0004))
	if ok == 0 {
		return false, func() {}
	}
	return true, func() { _, _, _ = setMode.Call(uintptr(handle), uintptr(original)) }
}
