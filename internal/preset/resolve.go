package preset

import (
	"strings"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
)

// Resolve returns the built-in Claude preset when path is empty, otherwise it
// loads a validated structured multi-rule file.
func Resolve(path string) (attribution.Matcher, error) {
	if strings.TrimSpace(path) == "" {
		return Claude(), nil
	}
	set, err := attribution.LoadRuleSet(path)
	if err != nil {
		return nil, err
	}
	return set, nil
}
