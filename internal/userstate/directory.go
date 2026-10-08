// Package userstate locates local, per-user settings outside Git repositories.
package userstate

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func Directory() (string, error) {
	if dir := os.Getenv("BYECLAUDE_STATE_DIR"); dir != "" {
		if !filepath.IsAbs(dir) {
			return "", fmt.Errorf("BYECLAUDE_STATE_DIR must be an absolute directory")
		}
		return filepath.Clean(dir), nil
	}
	var base string
	var err error
	if runtime.GOOS == "windows" {
		base, err = os.UserCacheDir() // LocalAppData, not the roaming profile.
	} else {
		base, err = os.UserConfigDir()
	}
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "ByeClaude"), nil
}
