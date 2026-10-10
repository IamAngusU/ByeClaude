package attribution

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRuleSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	data := []byte(`{"rules":[
		{"id":"a","name_contains":["Claude"],"email_domains":["anthropic.com"]},
		{"id":"b","exact_emails":["bot@example.dev"]}
	]}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	set, err := LoadRuleSet(path)
	if err != nil {
		t.Fatal(err)
	}
	ids := set.MatchIDs("Claude", "noreply@anthropic.com")
	if len(ids) != 1 || ids[0] != "a" {
		t.Fatalf("ids=%v", ids)
	}
	ids = set.MatchIDs("Other", "bot@example.dev")
	if len(ids) != 1 || ids[0] != "b" {
		t.Fatalf("ids=%v", ids)
	}
}

func TestRuleSetRejectsAmbiguousAndInvalidConstraints(t *testing.T) {
	for _, data := range []string{
		`{"rules":[{"id":"bad","name_contain":["bot"],"email_domains":["example.org"]}]}`,
		`{"rules":[{"id":"bad","name_contains":[" "]}]}`,
		`{"rules":[{"id":"bad","name_contains":["bot\u001b[2J"]}]}`,
		`{"rules":[{"id":"bad","email_domains":["*"]}]}`,
		`{"rules":[{"id":"bad","email_domains":["example.org/"]}]}`,
		`{"rules":[{"id":"bad","exact_emails":["bot"]}]}`,
		`{"rules":[{"id":"bad","exact_emails":["bot@@example.org"]}]}`,
		`{"rules":[{"id":"bad","trailer_keys":["bad key"]}]}`,
		`{"rules":[{"id":"bad","exact_emails":["bot@example.org"]}]} {}`,
		strings.Repeat(" ", MaxRulesBytes+1),
	} {
		if _, err := ParseRuleSet([]byte(data)); err == nil {
			t.Fatal("invalid rules accepted")
		}
	}
	if _, err := ParseRuleSet([]byte(`{"rules":[{"id":"marker","message_lines":["Made by Agent"],"trailer_keys":["Agent-Session"]}]}`)); err != nil {
		t.Fatalf("structured message rule rejected: %v", err)
	}
	set, err := ParseRuleSet([]byte(`{"rules":[{"id":"valid","name_contains":["Bot"],"email_domains":[" @EXAMPLE.ORG "],"exact_emails":[" BOT@EXAMPLE.COM "]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !set.Match("Bot", "bot@example.org") || set.Match("Human", "bot@example.org") || set.Match("Bot", "bot@notexample.org") {
		t.Fatal("constraints broadened")
	}
}

func TestRuleSetAcceptsPowerShellUTF8BOM(t *testing.T) {
	data := append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"rules":[{"id":"bot","exact_emails":["bot@example.org"]}]}`)...)
	set, err := ParseRuleSet(data)
	if err != nil || !set.Match("Bot", "bot@example.org") {
		t.Fatalf("UTF-8 BOM: %v", err)
	}
}

func TestLoadRuleSetRejectsDuplicateIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	data := []byte(`{"rules":[
		{"id":"dup","name_contains":["a"]},
		{"id":"dup","name_contains":["b"]}
	]}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRuleSet(path); err == nil {
		t.Fatal("expected duplicate ID error")
	}
}
