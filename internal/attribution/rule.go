package attribution

import "strings"

// Matcher decides whether one parsed Co-Authored-By identity is in scope for
// removal. The Git rewrite engine depends only on this interface, not on a
// particular AI vendor or bot name.
type Matcher interface {
	ID() string
	Match(name, email string) bool
}

// Rule is a conservative identity matcher for Co-Authored-By trailers.
//
// NameContains terms are matched case-insensitively against the display name.
// EmailDomains require a real @domain suffix, so "notanthropic.com" cannot
// accidentally match "anthropic.com". ExactEmails are also case-insensitive.
//
// Empty name constraints mean "any name"; empty email constraints mean "any
// email". A completely empty rule matches nothing.
type Rule struct {
	RuleID        string
	NameContains  []string
	EmailDomains  []string
	ExactEmails   []string
}

func (r Rule) ID() string {
	if strings.TrimSpace(r.RuleID) == "" {
		return "custom"
	}
	return strings.TrimSpace(r.RuleID)
}

func (r Rule) Match(name, email string) bool {
	if len(r.NameContains) == 0 && len(r.EmailDomains) == 0 && len(r.ExactEmails) == 0 {
		return false
	}

	nameLower := strings.ToLower(strings.TrimSpace(name))
	emailLower := strings.ToLower(strings.TrimSpace(email))

	nameOK := len(r.NameContains) == 0
	for _, needle := range r.NameContains {
		needle = strings.ToLower(strings.TrimSpace(needle))
		if needle != "" && strings.Contains(nameLower, needle) {
			nameOK = true
			break
		}
	}

	emailOK := len(r.EmailDomains) == 0 && len(r.ExactEmails) == 0
	for _, exact := range r.ExactEmails {
		exact = strings.ToLower(strings.TrimSpace(exact))
		if exact != "" && emailLower == exact {
			emailOK = true
			break
		}
	}
	if !emailOK {
		for _, domain := range r.EmailDomains {
			domain = strings.ToLower(strings.TrimSpace(domain))
			domain = strings.TrimPrefix(domain, "@")
			if domain != "" && strings.HasSuffix(emailLower, "@"+domain) {
				emailOK = true
				break
			}
		}
	}

	return nameOK && emailOK
}
