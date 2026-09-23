package attribution

import (
	"os"
	"path/filepath"
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
