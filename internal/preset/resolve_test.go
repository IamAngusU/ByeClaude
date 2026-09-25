package preset

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeMatchesBuiltInIdentity(t *testing.T) {
	rule := Claude()
	if rule.ID() != "claude-anthropic" {
		t.Fatalf("rule ID = %q", rule.ID())
	}
	if !rule.Match("Claude", "noreply@anthropic.com") {
		t.Fatal("built-in rule did not match Claude identity")
	}
	if rule.Match("Human", "human@example.com") {
		t.Fatal("built-in rule matched unrelated identity")
	}
}

func TestResolveBuiltInAndRulesFile(t *testing.T) {
	matcher, err := Resolve("  ")
	if err != nil {
		t.Fatal(err)
	}
	if !matcher.Match("Claude", "noreply@anthropic.com") {
		t.Fatal("default matcher did not use Claude preset")
	}

	path := filepath.Join(t.TempDir(), "rules.json")
	if err := os.WriteFile(path, []byte(`{"rules":[{"id":"bot","email_domains":["example.dev"]}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	matcher, err = Resolve(path)
	if err != nil {
		t.Fatal(err)
	}
	if !matcher.Match("Bot", "bot@example.dev") {
		t.Fatal("rules-file matcher did not match configured identity")
	}
	if _, err := Resolve(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected missing rules file error")
	}
}
