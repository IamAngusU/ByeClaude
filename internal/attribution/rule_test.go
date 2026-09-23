package attribution

import "testing"

func TestRuleMatchesNameAndDomainCaseInsensitively(t *testing.T) {
	rule := Rule{
		RuleID:       "example",
		NameContains: []string{"build bot"},
		EmailDomains: []string{"example.dev"},
	}
	if !rule.Match("Build Bot v2", "BOT@EXAMPLE.DEV") {
		t.Fatal("expected rule to match")
	}
}

func TestRuleRequiresRealEmailDomainBoundary(t *testing.T) {
	rule := Rule{EmailDomains: []string{"anthropic.com"}}
	if rule.Match("Claude", "bot@notanthropic.com") {
		t.Fatal("domain suffix without @ boundary must not match")
	}
}

func TestEmptyRuleMatchesNothing(t *testing.T) {
	if (Rule{}).Match("Anything", "anything@example.com") {
		t.Fatal("empty rule must match nothing")
	}
}

func TestExactEmailCanMatchWithoutNameConstraint(t *testing.T) {
	rule := Rule{ExactEmails: []string{"bot@example.com"}}
	if !rule.Match("Any Display Name", "BOT@example.com") {
		t.Fatal("exact email should match case-insensitively")
	}
}

func TestRuleSetReturnsMatchingRuleIDs(t *testing.T) {
	set := RuleSet{Rules: []Rule{
		{RuleID: "claude", NameContains: []string{"claude"}, EmailDomains: []string{"anthropic.com"}},
		{RuleID: "broad-anthropic", EmailDomains: []string{"anthropic.com"}},
		{RuleID: "other", ExactEmails: []string{"bot@example.dev"}},
	}}
	ids := set.MatchIDs("Claude Opus", "noreply@anthropic.com")
	if len(ids) != 2 || ids[0] != "claude" || ids[1] != "broad-anthropic" {
		t.Fatalf("ids=%v", ids)
	}
	if !set.Match("Claude Opus", "noreply@anthropic.com") {
		t.Fatal("set should match")
	}
}
