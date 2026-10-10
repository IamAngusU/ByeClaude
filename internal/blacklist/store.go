// Package blacklist stores an explicitly managed policy in local Git config.
// It never trusts a rules file supplied by a checkout or remote repository.
package blacklist

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/preset"
)

const configKey = "byeclaude.blacklist"

func Default() attribution.RuleSet {
	return attribution.RuleSet{Rules: []attribution.Rule{preset.Claude()}}
}

func Load(repo *gitx.Repo) (attribution.RuleSet, bool, error) {
	raw, found, err := repo.RunOptional("config", "--local", "--get-all", configKey)
	if err != nil {
		return attribution.RuleSet{}, false, err
	}
	if !found {
		return Default(), false, nil
	}
	set, err := attribution.ParseRuleSet(raw)
	if err != nil {
		return set, true, fmt.Errorf("invalid saved blacklist; repair with 'byeclaude blacklist reset': %w", err)
	}
	// Saved alpha-era Claude rules predate structured message markers. Hydrate
	// the known built-in ID in memory so upgrades cover the same Claude preset
	// without silently widening user-created rules.
	defaults := preset.Claude()
	for i := range set.Rules {
		if legacyClaudeRule(set.Rules[i], defaults) {
			if len(set.Rules[i].MessageLines) == 0 {
				set.Rules[i].MessageLines = append([]string(nil), defaults.MessageLines...)
			}
			if len(set.Rules[i].TrailerKeys) == 0 {
				set.Rules[i].TrailerKeys = append([]string(nil), defaults.TrailerKeys...)
			}
		}
	}
	return set, true, nil
}

func legacyClaudeRule(rule, defaults attribution.Rule) bool {
	return rule.RuleID == defaults.RuleID &&
		len(rule.NameContains) == 1 && strings.EqualFold(strings.TrimSpace(rule.NameContains[0]), defaults.NameContains[0]) &&
		len(rule.EmailDomains) == 1 && strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(rule.EmailDomains[0]), "@"), defaults.EmailDomains[0]) &&
		len(rule.ExactEmails) == 0
}

func Resolve(repo *gitx.Repo, rulesFile string) (attribution.Matcher, error) {
	if strings.TrimSpace(rulesFile) != "" {
		return preset.Resolve(rulesFile)
	}
	set, _, err := Load(repo)
	return set, err
}

// Save uses Git's config lock and atomic replacement, preserving other keys.
// Linked worktrees share this policy and must be acknowledged explicitly.
func Save(repo *gitx.Repo, set attribution.RuleSet, shared bool) error {
	data, err := json.Marshal(set)
	if err != nil {
		return err
	}
	validated, err := attribution.ParseRuleSet(data)
	if err != nil {
		return err
	}
	worktrees, err := repo.Run("worktree", "list", "--porcelain")
	if err != nil {
		return err
	}
	if strings.Count("\n"+string(worktrees), "\nworktree ") > 1 && !shared {
		return fmt.Errorf("linked worktrees share this blacklist; use --shared-worktrees to acknowledge the change")
	}
	data, err = json.Marshal(validated)
	if err != nil {
		return err
	}
	_, err = repo.Run("config", "--local", "--replace-all", configKey, string(data))
	return err
}
