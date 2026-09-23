package preset

import "github.com/IamAngusU/ByeClaude/internal/attribution"

// Claude returns ByeClaude's built-in product rule. Keeping the vendor-specific
// identity here means the Git graph rewrite remains reusable without widening
// ByeClaude's public CLI into an arbitrary regex tool.
func Claude() attribution.Rule {
	return attribution.Rule{
		RuleID:       "claude-anthropic",
		NameContains: []string{"claude"},
		EmailDomains: []string{"anthropic.com"},
	}
}
