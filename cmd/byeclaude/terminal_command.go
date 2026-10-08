package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

const maxMenuOutput = 1024 * 1024

// Keep subprocess output bounded and remove terminal controls from repository
// metadata before displaying it. An incomplete preview must never be approved.
type menuOutput struct {
	mu        sync.Mutex
	data      bytes.Buffer
	truncated bool
}

func (out *menuOutput) Write(data []byte) (int, error) {
	out.mu.Lock()
	defer out.mu.Unlock()
	room := maxMenuOutput - out.data.Len()
	if len(data) > room {
		out.truncated = true
		_, _ = out.data.Write(data[:room])
	} else {
		_, _ = out.data.Write(data)
	}
	return len(data), nil
}

func safeMenuOutput(data string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unsafeTerminalRune(r) {
			return -1
		}
		return r
	}, data)
}

func executeMenuCommand(command string, args []string, out io.Writer) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, append([]string{command}, args...)...) // #nosec G204 -- fixed self executable and separate arguments, never a shell
	var output menuOutput
	cmd.Stdout = &output
	cmd.Stderr = &output
	err = cmd.Run()
	if _, writeErr := fmt.Fprint(out, safeMenuOutput(output.data.String())); writeErr != nil {
		return writeErr
	}
	if output.truncated {
		return fmt.Errorf("output exceeded 1 MiB; inspect this repository with the explicit CLI command before continuing")
	}
	if err != nil {
		return fmt.Errorf("%s stopped; see the explanation above", command)
	}
	return nil
}
