package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

func TestManagedHookParserRejectsInjectedScript(t *testing.T) {
	for _, src := range []string{
		"#!/bin/sh\n# Installed by ByeClaude.\nexec '/bin/true' hook-filter \"$1\"; echo danger\n",
		"#!/bin/sh\n# Installed by ByeClaude.\nexec '/bin/true' pre-push-filter \"$@\"\necho another-command\n",
		"#!/bin/sh\n# Installed by ByeClaude.\nexec '/bin/true' hook-filter --rules '/tmp/rules' \"$1\" # injected\n",
		"#!/bin/sh\n# Installed by ByeClaude.\nexec relative hook-filter \"$1\"\n",
	} {
		for _, hook := range []string{"commit-msg", "pre-push"} {
			if _, ok := parseManagedHook(hook, []byte(src)); ok {
				t.Errorf("accepted modified %s script: %q", hook, src)
			}
		}
	}
}

func TestManagedHookParserPreservesShellQuotedPaths(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "Byte's tool")
	rules := filepath.Join(t.TempDir(), "config's rules.json")
	script := "#!/bin/sh\n# Installed by ByeClaude.\nexec " +
		shellQuote(filepath.ToSlash(exe)) + " hook-filter --rules " +
		shellQuote(filepath.ToSlash(rules)) + " \"$1\"\n"
	meta, ok := parseManagedHook("commit-msg", []byte(script))
	if !ok || meta.Executable != exe || meta.RulesFile != rules {
		t.Fatalf("escaped executable/rules changed: %+v (%v)", meta, ok)
	}
}

func TestSetupDifferentRulesCannotBeSilentlyReused(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.name", "Human")
	runGit(t, dir, "config", "user.email", "human@example.org")
	runGit(t, dir, "commit", "--allow-empty", "-m", "Clean")
	for _, name := range []string{"first.json", "second.json"} {
		data := `{"rules":[{"id":"example","name_contains":["sample"],"email_domains":["example.org"]}]}`
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil { t.Fatal(err) }
	}
	if err := runSetup([]string{"--repo", dir, "--rules", "first.json", "--apply"}); err != nil { t.Fatal(err) }
	repo, err := gitx.Open(dir)
	if err != nil { t.Fatal(err) }
	h, err := inspectHook(repo, "commit-msg")
	if err != nil { t.Fatal(err) }
	if h.Status != "installed" || h.RulesFile != filepath.Join(dir, "first.json") {
		t.Fatalf("unexpected configured rules: %+v", h)
	}
	err = runSetup([]string{"--repo", dir, "--rules", "second.json", "--apply"})
	if err == nil || !strings.Contains(err.Error(), "already uses") {
		t.Fatalf("expected policy mismatch error: %v", err)
	}
	h, err = inspectHook(repo, "commit-msg")
	if err != nil || h.RulesFile != filepath.Join(dir, "first.json") {
		t.Fatalf("mismatch mutated managed hook: %+v %v", h, err)
	}
	if err := os.Remove(filepath.Join(dir, "first.json")); err != nil { t.Fatal(err) }
	h, err = inspectHook(repo, "commit-msg")
	if err != nil || h.Status != "stale_rules" {
		t.Fatalf("missing rules should be detected: %+v %v", h, err)
	}
}
