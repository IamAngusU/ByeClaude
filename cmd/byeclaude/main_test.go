package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestHookInstallRefusesExistingHook(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	hook := filepath.Join(dir, ".git", "hooks", "commit-msg")
	original := "#!/bin/sh\necho existing\n"
	if err := os.WriteFile(hook, []byte(original), 0755); err != nil {
		t.Fatal(err)
	}
	if err := runHook([]string{"install", "--repo", dir}); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("expected overwrite refusal, got %v", err)
	}
	got, err := os.ReadFile(hook)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Fatalf("existing hook changed: %q", got)
	}
}

func TestHookInstallRefusesCustomHooksPath(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "core.hooksPath", ".githooks")
	if err := runHook([]string{"install", "--repo", dir}); err == nil || !strings.Contains(err.Error(), "core.hooksPath") {
		t.Fatalf("expected core.hooksPath refusal, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "hooks", "commit-msg")); !os.IsNotExist(err) {
		t.Fatalf("default hook unexpectedly created: %v", err)
	}
}

func TestHookInstallEmbedsRulesFile(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	rules := filepath.Join(dir, "rules.json")
	if err := os.WriteFile(rules, []byte(`{"rules":[{"id":"bot","name_contains":["bot"],"email_domains":["example.dev"]}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := runHook([]string{"install", "--repo", dir, "--rules", rules}); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(dir, ".git", "hooks", "commit-msg")
	got, err := os.ReadFile(hook)
	if err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(rules)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "--rules") || !strings.Contains(string(got), filepath.ToSlash(abs)) {
		t.Fatalf("hook does not embed validated rules path: %s", got)
	}
}
