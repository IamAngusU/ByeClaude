package attribution

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type ruleFile struct {
	Rules []Rule `json:"rules"`
}

func LoadRuleSet(path string) (RuleSet, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- the rules file is an explicit operator-selected input
	if err != nil {
		return RuleSet{}, fmt.Errorf("read rules file: %w", err)
	}
	var file ruleFile
	if err := json.Unmarshal(data, &file); err != nil {
		return RuleSet{}, fmt.Errorf("parse rules file: %w", err)
	}
	if len(file.Rules) == 0 {
		return RuleSet{}, fmt.Errorf("rules file contains no rules")
	}

	seen := map[string]bool{}
	for i := range file.Rules {
		rule := &file.Rules[i]
		rule.RuleID = strings.TrimSpace(rule.RuleID)
		if rule.RuleID == "" {
			return RuleSet{}, fmt.Errorf("rule %d has an empty id", i+1)
		}
		if seen[rule.RuleID] {
			return RuleSet{}, fmt.Errorf("duplicate rule id %q", rule.RuleID)
		}
		seen[rule.RuleID] = true
		if len(rule.NameContains) == 0 && len(rule.EmailDomains) == 0 && len(rule.ExactEmails) == 0 {
			return RuleSet{}, fmt.Errorf("rule %q has no match constraints", rule.RuleID)
		}
	}
	return RuleSet{Rules: file.Rules}, nil
}
