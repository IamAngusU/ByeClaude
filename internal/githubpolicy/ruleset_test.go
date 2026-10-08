package githubpolicy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
)

func TestGeneratedRulesetMatchesClaudeTrailerOnly(t *testing.T) {
	r, err := GenerateRuleset(attribution.Rule{RuleID: "claude", NameContains: []string{"claude"}, EmailDomains: []string{"anthropic.com"}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Rules) != 3 || r.Enforcement != "active" || r.Target != "branch" || len(r.Conditions.RefName.Include) != 1 || r.Conditions.RefName.Include[0] != "~DEFAULT_BRANCH" {
		t.Fatalf("invalid policy: %+v", r)
	}
	pattern := regexp.MustCompile(r.Rules[0].Parameters.Pattern)
	for _, text := range []string{
		"fix\n\nCo-Authored-By: Claude <noreply@anthropic.com>\n",
		"Co-Authored-By: Claude Code <bot@anthropic.com>",
	} {
		if !pattern.MatchString(text) {
			t.Errorf("missed %q", text)
		}
	}
	for _, text := range []string{"Co-Authored-By: Claude Shannon <shannon@example.com>",
		"Co-Authored-By: Claudette <bad@notanthropic.com>",
	} {
		if pattern.MatchString(text) {
			t.Errorf("false hit %q", text)
		}
	}
	if regexp.MustCompile(r.Rules[1].Parameters.Pattern).MatchString("agent@notanthropic.com") {
		t.Fatal("domain not anchored")
	}
}
func TestInstallFailsClosedOnExistingPolicy(t *testing.T) {
	var posts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			posts++
			t.Error("must not overwrite existing policy")
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("missing token")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]Ruleset{{Name: "ByeClaude attribution guard", ID: 5}})
	}))
	defer server.Close()
	p, _ := GenerateRuleset(attribution.Rule{ExactEmails: []string{"bot@example.org"}}, false)
	client := Client{BaseURL: server.URL, HTTPClient: server.Client(), Token: "secret"}
	_, err := client.InstallRuleset(context.Background(), "owner/repo", p)
	if err == nil || !strings.Contains(err.Error(), "already exists") || posts != 0 {
		t.Fatalf("duplicate guard failed: %v", err)
	}
}
func TestInvalidRepositoryRejected(t *testing.T) {
	for _, s := range []string{"", "https://github.com/o/r", "a/../b", "a/b c", "a/b/x"} {
		if ValidateRepo(s) == nil {
			t.Errorf("accepted %q", s)
		}
	}
}
