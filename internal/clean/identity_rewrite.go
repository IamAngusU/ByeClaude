package clean

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
)

// IdentityReplacement is an explicit, validated Git name and email. Git commits
// cannot omit their author/committer headers; changing one rewrites the commit
// and all reachable descendants that refer to it.
type IdentityReplacement struct {
	Name  string
	Email string
}

type IdentityRewriteOptions struct {
	Author    *IdentityReplacement
	Committer *IdentityReplacement
}

var gitIdentityHeader = regexp.MustCompile("^(author|committer) ([^<>\r\n]*) <([^<>\r\n]+)> ([0-9]+) ([+-][0-9]{4})$")

func ParseIdentityReplacement(raw string) (IdentityReplacement, error) {
	raw = strings.TrimSpace(raw)
	lt, gt := strings.LastIndex(raw, "<"), strings.LastIndex(raw, ">")
	if lt < 1 || gt != len(raw)-1 || gt <= lt+1 {
		return IdentityReplacement{}, fmt.Errorf("identity must be 'Name <email@example.com>'")
	}
	value := IdentityReplacement{
		Name: strings.TrimSpace(raw[:lt]),
		Email: strings.TrimSpace(raw[lt+1 : gt]),
	}
	if err := value.validate(); err != nil {
		return IdentityReplacement{}, err
	}
	return value, nil
}

func (r IdentityReplacement) validate() error {
	if r.Name == "" || r.Email == "" || strings.ContainsAny(r.Name, "<>\r\n\x00") || strings.ContainsAny(r.Email, "<>\r\n\x00 \t") {
		return fmt.Errorf("replacement must contain a valid nonempty name and email")
	}
	if strings.Count(r.Email, "@") != 1 || strings.HasPrefix(r.Email, "@") || strings.HasSuffix(r.Email, "@") {
		return fmt.Errorf("replacement email must contain a complete address")
	}
	return nil
}

func (opts IdentityRewriteOptions) Validate(matcher attribution.Matcher) error {
	if matcher == nil {
		return fmt.Errorf("attribution matcher is required")
	}
	for role, replacement := range map[string]*IdentityReplacement{"author": opts.Author, "committer": opts.Committer} {
		if replacement == nil {
			continue
		}
		if err := replacement.validate(); err != nil {
			return fmt.Errorf("%s: %w", role, err)
		}
		if matcher.Match(replacement.Name, replacement.Email) {
			return fmt.Errorf("%s replacement still matches the selected attribution rule", role)
		}
	}
	return nil
}

// ReplaceMatchingCommitIdentities changes only explicitly selected author and/or
// committer headers when their original name+email match the active rule.
// The timestamp and timezone suffixes are preserved byte-for-byte. Other
// metadata is left for rebuildCommit, which handles parent links/signatures.
func ReplaceMatchingCommitIdentities(original commitObject, matcher attribution.Matcher, opts IdentityRewriteOptions) (commitObject, int, int, error) {
	if opts.Author == nil && opts.Committer == nil {
		return original, 0, 0, nil
	}
	result := commitObject{Headers: append([]header(nil), original.Headers...), Message: original.Message}
	var authors, committers int
	for i, h := range original.Headers {
		var replacement *IdentityReplacement
		switch h.Key {
		case "author":
			replacement = opts.Author
		case "committer":
			replacement = opts.Committer
		default:
			continue
		}
		if replacement == nil {
			continue
		}
		if len(h.Lines) != 1 {
			return commitObject{}, 0, 0, fmt.Errorf("malformed %s identity header", h.Key)
		}
		parts := gitIdentityHeader.FindStringSubmatch(h.Lines[0])
		if len(parts) != 6 {
			return commitObject{}, 0, 0, fmt.Errorf("invalid %s header; refusing identity rewrite", h.Key)
		}
		if !matcher.Match(strings.TrimSpace(parts[2]), parts[3]) {
			continue
		}
		result.Headers[i].Lines = []string{fmt.Sprintf("%s %s <%s> %s %s", h.Key, replacement.Name, replacement.Email, parts[4], parts[5])}
		if h.Key == "author" {
			authors++
		} else {
			committers++
		}
	}
	return result, authors, committers, nil
}
