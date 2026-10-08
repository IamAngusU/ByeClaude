//go:build !windows

package pathsetup

import (
	"fmt"
	"os"
	"path/filepath"
)

func configureUserPath(dir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	shell := filepath.Base(os.Getenv("SHELL"))
	var paths []string
	switch shell {
	case "zsh":
		base := os.Getenv("ZDOTDIR")
		if base == "" {
			base = home
		}
		if !filepath.IsAbs(base) {
			return fmt.Errorf("ZDOTDIR must be absolute")
		}
		paths = []string{filepath.Join(base, ".zshrc")}
	case "bash":
		login := ".profile"
		for _, name := range []string{".bash_profile", ".bash_login"} {
			if _, err := os.Lstat(filepath.Join(home, name)); err == nil {
				login = name
				break
			}
		}
		paths = []string{filepath.Join(home, ".bashrc"), filepath.Join(home, login)}
	case "sh", "dash", ".":
		paths = []string{filepath.Join(home, ".profile")}
	default:
		return fmt.Errorf("automatic PATH setup does not support %s; add the installation directory in your shell settings", shell)
	}
	for _, path := range paths {
		if err := updateProfile(path, dir); err != nil {
			return err
		}
	}
	return nil
}
