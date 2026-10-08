//go:build linux || darwin

package pathsetup

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnixProfileExecutesLiteralPathAndPreservesExistingPATH(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "work with space's $(touch should-not-exist)")
	text, err := profileText("", dir)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", text+"\nprintf '%s' \"$PATH\"")
	cmd.Dir = t.TempDir()
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "/usr/bin:/bin:"+dir {
		t.Fatal(string(output))
	}
	if _, err := os.Stat(filepath.Join(cmd.Dir, "should-not-exist")); !os.IsNotExist(err) {
		t.Fatal("path executed as shell code")
	}
	cmd = exec.Command("sh", "-c", text+"\nprintf '%s' \"$PATH\"")
	cmd.Env = []string{"PATH=" + string(output)}
	again, err := cmd.Output()
	if err != nil || string(again) != string(output) {
		t.Fatal("duplicate PATH entry", string(again), err)
	}
}

func TestUnixShellSelection(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "sh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("SHELL", "/bin/"+shell)
			t.Setenv("ZDOTDIR", "")
			err := configureUserPath("/test/bin")
			if shell == "fish" {
				if err == nil {
					t.Fatal("unsupported shell silently accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			profile := ".profile"
			if shell == "zsh" {
				profile = ".zshrc"
			}
			data, err := os.ReadFile(filepath.Join(home, profile))
			if err != nil || !strings.Contains(string(data), beginMarker) {
				t.Fatal(err)
			}
		})
	}
}
