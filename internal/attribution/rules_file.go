package attribution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode"
)

type ruleFile struct {
	Rules []Rule `json:"rules"`
}

func LoadRuleSet(path string) (RuleSet, error) {
	f, err := os.Open(path) // #nosec G304 -- explicit operator-selected rules file
	if err != nil {
		return RuleSet{}, fmt.Errorf("read rules file: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxRulesBytes+1))
	if err != nil {
		return RuleSet{}, err
	}
	return ParseRuleSet(data)
}

const MaxRulesBytes = 1024 * 1024

var ruleIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

// ParseRuleSet rejects misspelled constraints rather than silently broadening
// the identities that a history rewrite would remove.
func ParseRuleSet(data []byte) (RuleSet, error) {
	if len(data) > MaxRulesBytes {
		return RuleSet{}, fmt.Errorf("rules exceed 1 MiB")
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	var file ruleFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return RuleSet{}, fmt.Errorf("parse rules file: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return RuleSet{}, fmt.Errorf("rules must contain exactly one JSON object")
	}
	if len(file.Rules) == 0 {
		return RuleSet{}, fmt.Errorf("rules file contains no rules")
	}

	seen := map[string]bool{}
	for i := range file.Rules {
		rule := &file.Rules[i]
		rule.RuleID = strings.TrimSpace(rule.RuleID)
		if !ruleIDPattern.MatchString(rule.RuleID) {
			return RuleSet{}, fmt.Errorf("rule %d needs an id of 1-64 letters, digits, dots, underscores or hyphens, starting with a letter or digit", i+1)
		}
		if seen[rule.RuleID] {
			return RuleSet{}, fmt.Errorf("duplicate rule id %q", rule.RuleID)
		}
		seen[rule.RuleID] = true
		if len(rule.NameContains) == 0 && len(rule.EmailDomains) == 0 && len(rule.ExactEmails) == 0 {
			return RuleSet{}, fmt.Errorf("rule %q has no match constraints", rule.RuleID)
		}
		for _, group := range [][]string{rule.NameContains, rule.EmailDomains, rule.ExactEmails} {
			for _, value := range group {
				if strings.TrimSpace(value) == "" || strings.IndexFunc(value, unicode.IsControl) >= 0 {
					return RuleSet{}, fmt.Errorf("rule %q has a blank or control-character constraint", rule.RuleID)
				}
			}
		}
		for j, domain := range rule.EmailDomains {
			domain = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(domain), "@"))
			if !validDomain(domain) {
				return RuleSet{}, fmt.Errorf("rule %q has invalid email domain %q", rule.RuleID, domain)
			}
			rule.EmailDomains[j] = domain
		}
		for j, email := range rule.ExactEmails {
			email = strings.ToLower(strings.TrimSpace(email))
			parts := strings.Split(email, "@")
			if len(parts) != 2 || parts[0] == "" || !validDomain(parts[1]) || strings.ContainsAny(parts[0], "<> ,;:\"\\") || strings.IndexFunc(email, unicode.IsSpace) >= 0 {
				return RuleSet{}, fmt.Errorf("rule %q needs a complete exact email address", rule.RuleID)
			}
			rule.ExactEmails[j] = email
		}
	}
	return RuleSet{Rules: file.Rules}, nil
}

func validDomain(domain string) bool {
	if len(domain) == 0 || len(domain) > 253 {
		return false
	}
	for _, label := range strings.Split(domain, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}
