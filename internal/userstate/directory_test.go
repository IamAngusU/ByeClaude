package userstate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectoryOverrideMustBeAbsolute(t *testing.T) {
	t.Setenv("BYECLAUDE_STATE_DIR", "relative")
	if _, err := Directory(); err == nil {
		t.Fatal("relative state could land in a repository")
	}
	dir := t.TempDir()
	t.Setenv("BYECLAUDE_STATE_DIR", dir)
	if got, err := Directory(); err != nil || got != dir {
		t.Fatal(got, err)
	}
	t.Setenv("BYECLAUDE_STATE_DIR", "")
	if got, err := Directory(); err != nil || !filepath.IsAbs(got) || filepath.Base(got) != "ByeClaude" {
		t.Fatal(got, err)
	}
}

func TestAtomicReplacementLeavesNoTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, value := range []string{"first", "replacement"} {
		if err := AtomicWrite(root, "settings.json", []byte(value)); err != nil {
			t.Fatal(err)
		}
		got, err := root.ReadFile("settings.json")
		if err != nil || string(got) != value {
			t.Fatal(string(got), err)
		}
	}
	if err := root.Mkdir("directory.json", 0700); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(root, "directory.json", []byte("must fail")); err == nil {
		t.Fatal("overwrote a directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".state-") {
			t.Fatal("temporary file leaked")
		}
	}
}
