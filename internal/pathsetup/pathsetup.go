// Package pathsetup adds the installed executable to the user's PATH without
// elevation. Optional setup failures must not make the installed tool unusable.
package pathsetup

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"github.com/IamAngusU/ByeClaude/internal/userstate"
)

type Preference struct {
	Mode string `json:"mode"`
}

func ReadPreference() (Preference, error) {
	dir, err := userstate.Directory()
	if err != nil {
		return Preference{}, err
	}
	root, err := os.OpenRoot(dir)
	if os.IsNotExist(err) {
		return Preference{}, nil
	}
	if err != nil {
		return Preference{}, err
	}
	defer root.Close()
	info, err := root.Lstat("path.json")
	if os.IsNotExist(err) {
		return Preference{}, nil
	}
	if err != nil {
		return Preference{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1024 {
		return Preference{}, fmt.Errorf("invalid PATH preference file")
	}
	data, err := root.ReadFile("path.json")
	if err != nil {
		return Preference{}, err
	}
	var p Preference
	if err := json.Unmarshal(data, &p); err != nil {
		return p, err
	}
	switch p.Mode {
	case "ready", "pending", "skipped":
		return p, nil
	default:
		return p, fmt.Errorf("invalid PATH preference")
	}
}

func SavePreference(mode string) error {
	if mode != "ready" && mode != "pending" && mode != "skipped" {
		return fmt.Errorf("invalid PATH preference")
	}
	dir, err := userstate.Directory()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	if info, err := root.Lstat("path.json"); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("PATH preference is not a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	data, _ := json.Marshal(Preference{Mode: mode})
	return userstate.AtomicWrite(root, "path.json", append(data, '\n'))
}

func executableDirectory(executable string) (string, error) {
	name := "byeclaude"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if !strings.EqualFold(filepath.Base(executable), name) {
		return "", fmt.Errorf("install or rename the executable to %s first", name)
	}
	dir := filepath.Dir(executable)
	if !filepath.IsAbs(dir) || strings.ContainsRune(dir, os.PathListSeparator) || strings.IndexFunc(dir, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }) >= 0 {
		return "", fmt.Errorf("the executable directory cannot be represented safely in PATH")
	}
	normalized := filepath.ToSlash(dir)
	if strings.Contains(normalized, "/go-build") {
		return "", fmt.Errorf("install a persistent binary before configuring PATH")
	}
	return dir, nil
}

func Setup(executable string) error { return setup(executable, configureUserPath) }

func setup(executable string, configure func(string) error) error {
	dir, err := executableDirectory(executable)
	if err != nil {
		return err
	}
	// Save the retry request first; an interrupted attempt remains recoverable.
	if err := SavePreference("pending"); err != nil {
		return fmt.Errorf("cannot save PATH retry preference: %w", err)
	}
	if err := configure(dir); err != nil {
		return err
	}
	if err := SavePreference("ready"); err != nil {
		return fmt.Errorf("PATH was configured but its completion could not be saved: %w", err)
	}
	return nil
}

func CurrentCommand(executable string) string {
	found, err := exec.LookPath("byeclaude")
	if err != nil {
		return "not found in this terminal"
	}
	a, err := os.Stat(found)
	if err != nil {
		return "unable to inspect the current command"
	}
	b, err := os.Stat(executable)
	if err == nil && os.SameFile(a, b) {
		return "this installation"
	}
	return "another installation: " + found
}
