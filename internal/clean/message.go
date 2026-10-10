package clean

import (
	"regexp"
	"strings"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
)

var (
	coAuthorRE = regexp.MustCompile(`(?i)^\s*co-authored-by\s*:\s*(.*?)\s*<([^>]+)>\s*$`)
	trailerRE  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*\s*:\s*.+$`)
)

type TrailerEvidence struct {
	Line    string
	Field   string
	Name    string
	Email   string
	RuleIDs []string
}

type explainingMatcher interface {
	attribution.Matcher
	MatchIDs(name, email string) []string
}

func messageRuleIDs(matcher attribution.Matcher, line string, trailer bool) []string {
	if matcher == nil {
		return nil
	}
	if m, ok := matcher.(attribution.MessageMatcher); ok {
		return m.MatchMessageLine(line, trailer)
	}
	return nil
}

func evidenceForMessage(message string, matcher attribution.Matcher) ([]TrailerEvidence, []int, []string, int, int) {
	normalized := strings.ReplaceAll(message, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	start, end, hasTrailers := trailerBlock(lines)
	var evidence []TrailerEvidence
	var indexes []int

	if hasTrailers {
		for i := start; i < end; i++ {
			line := lines[i]
			name, email, parsed := parseCoAuthor(line)
			var ids []string
			field := ""
			if parsed && matcher.Match(name, email) {
				ids = []string{matcher.ID()}
				if explaining, ok := matcher.(explainingMatcher); ok {
					ids = explaining.MatchIDs(name, email)
				}
				field = "Co-Authored-By"
			} else {
				ids = messageRuleIDs(matcher, line, true)
				if len(ids) > 0 {
					field, _, _ = strings.Cut(strings.TrimSpace(line), ":")
				}
			}
			if len(ids) == 0 {
				continue
			}
			evidence = append(evidence, TrailerEvidence{Line: strings.TrimSpace(line), Field: field, Name: name, Email: email, RuleIDs: ids})
			indexes = append(indexes, i)
			// A matched trailer owns its folded continuation lines.
			for i+1 < end && (strings.HasPrefix(lines[i+1], " ") || strings.HasPrefix(lines[i+1], "\t")) {
				i++
				indexes = append(indexes, i)
			}
		}
	}

	// Historical Claude Code markers are plain text immediately before the
	// final trailer block (or the last non-empty line when no trailer exists).
	marker := len(lines) - 1
	if hasTrailers {
		marker = start - 1
	}
	for marker >= 0 && strings.TrimSpace(lines[marker]) == "" {
		marker--
	}
	if marker >= 0 {
		if ids := messageRuleIDs(matcher, lines[marker], false); len(ids) > 0 {
			evidence = append([]TrailerEvidence{{Line: strings.TrimSpace(lines[marker]), Field: "message", RuleIDs: ids}}, evidence...)
			indexes = append(indexes, marker)
		}
	}
	return evidence, indexes, lines, start, end
}

func parseCoAuthor(line string) (name, email string, ok bool) {
	m := coAuthorRE.FindStringSubmatch(line)
	if len(m) != 3 {
		return "", "", false
	}
	return strings.TrimSpace(m[1]), strings.TrimSpace(m[2]), true
}

// trailerBlock returns the inclusive start and exclusive end of the final Git
// trailer block. A trailer block must be separated from the body by a blank
// line, unless the whole message consists only of trailers.
func trailerBlock(lines []string) (int, int, bool) {
	end := len(lines)
	for end > 0 && lines[end-1] == "" {
		end--
	}
	if end == 0 {
		return 0, 0, false
	}

	start := end
	for start > 0 {
		line := lines[start-1]
		if trailerRE.MatchString(line) || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			start--
			continue
		}
		break
	}
	if start == end {
		return 0, 0, false
	}
	if start > 0 && lines[start-1] != "" {
		return 0, 0, false
	}
	return start, end, true
}

// MatchingEvidence returns parsed Co-Authored-By evidence from the final Git
// trailer block whose identity is accepted by matcher. Prose elsewhere in the
// commit message is never considered.
func MatchingEvidence(message string, matcher attribution.Matcher) []TrailerEvidence {
	if matcher == nil {
		return nil
	}
	evidence, _, _, _, _ := evidenceForMessage(message, matcher)
	return evidence
}

// MatchingTrailers preserves the simple line-only helper used by rewrite tests
// and callers that do not need provider/rule attribution details.
func MatchingTrailers(message string, matcher attribution.Matcher) []string {
	evidence := MatchingEvidence(message, matcher)
	lines := make([]string, 0, len(evidence))
	for _, item := range evidence {
		lines = append(lines, item.Line)
	}
	return lines
}

// StripMatchingTrailers removes matching identities, explicit trailer keys and
// exact end markers. Unrelated trailers and message prose are preserved.
func StripMatchingTrailers(message string, matcher attribution.Matcher) (string, []string) {
	if matcher == nil {
		return message, nil
	}
	newline := "\n"
	if strings.Contains(message, "\r\n") {
		newline = "\r\n"
	}
	evidence, indexes, lines, start, end := evidenceForMessage(message, matcher)
	if len(evidence) == 0 {
		return message, nil
	}
	remove := make(map[int]bool, len(indexes)+2)
	removed := make([]string, 0, len(evidence))
	for _, i := range indexes {
		remove[i] = true
	}
	for _, item := range evidence {
		removed = append(removed, item.Line)
	}

	// If every line in the trailer block is removed, remove its blank
	// separator too. Otherwise retain the separator for remaining trailers.
	if start < end {
		allRemoved := true
		for i := start; i < end; i++ {
			if !remove[i] {
				allRemoved = false
				break
			}
		}
		if allRemoved && start > 0 && lines[start-1] == "" {
			remove[start-1] = true
		}
	}
	// Removing a standalone marker must not leave a new double-blank gap.
	for _, i := range indexes {
		if i >= start && i < end {
			continue
		}
		next := i + 1
		for next < len(lines) && strings.TrimSpace(lines[next]) == "" {
			remove[next] = true
			next++
		}
		if next >= len(lines) && i > 0 && lines[i-1] == "" {
			remove[i-1] = true
		}
	}
	kept := make([]string, 0, len(lines))
	for i, line := range lines {
		if !remove[i] {
			kept = append(kept, line)
		}
	}
	for len(kept) > 1 && kept[len(kept)-1] == "" && kept[len(kept)-2] == "" {
		kept = kept[:len(kept)-1]
	}
	out := strings.Join(kept, "\n")
	if newline == "\r\n" {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	return out, removed
}
