package pathsetup

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDeniedSetupDefersAndRetryRecovers(t *testing.T) {
	t.Setenv("BYECLAUDE_STATE_DIR", t.TempDir())
	exe := filepath.Join(t.TempDir(), "byeclaude")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	denied := errors.New("access denied or elevation declined")
	if err := setup(exe, func(string) error { return denied }); !errors.Is(err, denied) {
		t.Fatal(err)
	}
	p, err := ReadPreference()
	if err != nil || p.Mode != "pending" {
		t.Fatal(p, err)
	}
	if err := setup(exe, func(dir string) error {
		if dir != filepath.Dir(exe) {
			t.Fatal(dir)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	p, err = ReadPreference()
	if err != nil || p.Mode != "ready" {
		t.Fatal(p, err)
	}
	if err := SavePreference("skipped"); err != nil {
		t.Fatal(err)
	}
	p, _ = ReadPreference()
	if p.Mode != "skipped" {
		t.Fatal(p)
	}
}

func TestUnwritableSettingsNeverTriggersConfiguration(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(dir, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BYECLAUDE_STATE_DIR", dir)
	exe := filepath.Join(t.TempDir(), "byeclaude")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if err := setup(exe, func(string) error { t.Fatal("configuration ran without a saved retry preference"); return nil }); err == nil {
		t.Fatal("accepted unavailable settings")
	}
}

func TestUnsafeExecutableDirectoriesRejected(t *testing.T) {
	for _, exe := range []string{"byeclaude", filepath.Join(t.TempDir(), "unexpected-name"), filepath.Join(t.TempDir(), "bad\npath", "byeclaude"), filepath.Join(t.TempDir(), "go-build123", "exe", "byeclaude")} {
		if runtime.GOOS == "windows" {
			exe += ".exe"
		}
		if _, err := executableDirectory(exe); err == nil {
			t.Fatalf("accepted %q", exe)
		}
	}
}

func TestProfilePreservesContentAndQuotesPath(t *testing.T) {
	original := "# existing settings\nexport EDITOR=vi\n"
	dir := `/home/demo/work with space's/$literal;keep`
	first, err := profileText(original, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first, original) || !strings.Contains(first, `space'"'"'s`) {
		t.Fatal(first)
	}
	second, err := profileText(first, dir)
	if err != nil || first != second {
		t.Fatal("not idempotent", err)
	}
	updated, err := profileText(first, "/new/location")
	if err != nil || strings.Contains(updated, "$literal") || strings.Count(updated, beginMarker) != 1 || !strings.HasPrefix(updated, original) {
		t.Fatal(updated, err)
	}
	for _, bad := range []string{beginMarker, endMarker, first + first, "prefix" + beginMarker + "\n" + endMarker, beginMarker + "\n" + endMarker + " suffix"} {
		if _, err := profileText(bad, dir); err == nil {
			t.Fatal("accepted ambiguous markers")
		}
	}
	path := filepath.Join(t.TempDir(), ".profile")
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := updateProfile(path, dir); err != nil {
		t.Fatal(err)
	}
	actual, _ := os.ReadFile(path)
	if string(actual) != first {
		t.Fatal(string(actual))
	}
	if err := updateProfile(path, dir); err != nil {
		t.Fatal(err)
	}
}

func TestLinkedProfileIsPreserved(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "managed")
	profile := filepath.Join(dir, ".profile")
	if err := os.WriteFile(target, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, profile); err != nil {
		t.Skip("symlinks unavailable")
	}
	if err := updateProfile(profile, "/somewhere"); err == nil {
		t.Fatal("linked profile overwritten")
	}
	actual, _ := os.ReadFile(target)
	if string(actual) != "keep" {
		t.Fatal(string(actual))
	}
}
