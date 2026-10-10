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

type lineRange struct {
	start int
	end   int
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

func evidenceForMessage(message string, matcher attribution.Matcher) ([]TrailerEvidence, []int, []string, []lineRange, int) {
	normalized := strings.ReplaceAll(message, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	start, end, hasTrailers := trailerBlock(lines)
	var blocks []lineRange
	if hasTrailers {
		blocks = append(blocks, lineRange{start: start, end: end})
	}
	finalEvidence, finalIndexes := evidenceForTrailerBlock(lines, start, end, hasTrailers, matcher)
	evidence := finalEvidence
	indexes := finalIndexes
	separator := -1

	// GitHub merge/squash messages can preserve an earlier generated footer,
	// then append an exact separator and a second co-author block. Only inspect
	// that earlier suffix when the final block already contains selected
	// attribution, which keeps quoted body examples out of scope.
	if hasTrailers && len(finalEvidence) > 0 {
		candidate := start - 1
		for candidate >= 0 && strings.TrimSpace(lines[candidate]) == "" {
			candidate--
		}
		if candidate >= 0 && lines[candidate] == "---------" {
			bodyEvidence, bodyIndexes, bodyBlocks := evidenceForGitHubSquashBody(lines, candidate, matcher)
			evidence = append(bodyEvidence, finalEvidence...)
			indexes = append(bodyIndexes, finalIndexes...)
			blocks = append(bodyBlocks, blocks...)
			separator = candidate
		}
	}
	if separator < 0 {
		markerEvidence, markerIndexes := evidenceForEndMarker(lines, start, hasTrailers, len(lines), matcher)
		evidence = append(markerEvidence, evidence...)
		indexes = append(markerIndexes, indexes...)
	}
	return evidence, indexes, lines, blocks, separator
}

func evidenceForGitHubSquashBody(lines []string, limit int, matcher attribution.Matcher) ([]TrailerEvidence, []int, []lineRange) {
	var evidence []TrailerEvidence
	var indexes []int
	var blocks []lineRange
	for marker := 0; marker < limit; marker++ {
		markerIDs := messageRuleIDs(matcher, lines[marker], false)
		if len(markerIDs) == 0 {
			continue
		}
		start := marker + 1
		for start < limit && strings.TrimSpace(lines[start]) == "" {
			start++
		}
		end := start
		for end < limit {
			line := lines[end]
			if trailerRE.MatchString(line) || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
				end++
				continue
			}
			break
		}
		if start == end {
			continue
		}
		after := end
		for after < limit && strings.TrimSpace(lines[after]) == "" {
			after++
		}
		if after < limit && !strings.HasPrefix(lines[after], "* ") {
			continue
		}
		trailerEvidence, trailerIndexes := evidenceForTrailerBlock(lines, start, end, true, matcher)
		if len(trailerEvidence) == 0 {
			continue
		}
		evidence = append(evidence, TrailerEvidence{Line: strings.TrimSpace(lines[marker]), Field: "message", RuleIDs: markerIDs})
		evidence = append(evidence, trailerEvidence...)
		indexes = append(indexes, marker)
		indexes = append(indexes, trailerIndexes...)
		blocks = append(blocks, lineRange{start: start, end: end})
		marker = end - 1
	}
	return evidence, indexes, blocks
}

func evidenceForTrailerBlock(lines []string, start, end int, present bool, matcher attribution.Matcher) ([]TrailerEvidence, []int) {
	if !present {
		return nil, nil
	}
	var evidence []TrailerEvidence
	var indexes []int
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
		for i+1 < end && (strings.HasPrefix(lines[i+1], " ") || strings.HasPrefix(lines[i+1], "\t")) {
			i++
			indexes = append(indexes, i)
		}
	}
	return evidence, indexes
}

func evidenceForEndMarker(lines []string, trailerStart int, hasTrailers bool, limit int, matcher attribution.Matcher) ([]TrailerEvidence, []int) {
	marker := limit - 1
	if hasTrailers {
		marker = trailerStart - 1
	}
	for marker >= 0 && strings.TrimSpace(lines[marker]) == "" {
		marker--
	}
	if marker < 0 {
		return nil, nil
	}
	ids := messageRuleIDs(matcher, lines[marker], false)
	if len(ids) == 0 {
		return nil, nil
	}
	return []TrailerEvidence{{Line: strings.TrimSpace(lines[marker]), Field: "message", RuleIDs: ids}}, []int{marker}
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

// MatchingEvidence returns selected evidence from the final attribution
// suffix, including an exact GitHub-separated predecessor when the final
// trailer block already matches. Prose elsewhere is never considered.
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
	evidence, indexes, lines, blocks, separator := evidenceForMessage(message, matcher)
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
	for _, block := range blocks {
		allRemoved := true
		for i := block.start; i < block.end; i++ {
			if !remove[i] {
				allRemoved = false
				break
			}
		}
		if allRemoved && block.start > 0 && lines[block.start-1] == "" {
			remove[block.start-1] = true
		}
		if allRemoved && separator >= 0 && block.end <= separator {
			for i := block.end; i < separator && strings.TrimSpace(lines[i]) == ""; i++ {
				remove[i] = true
			}
		}
	}
	if separator >= 0 {
		allAttribution := true
		for i := separator; allAttribution && i < len(lines); i++ {
			if i == separator || strings.TrimSpace(lines[i]) == "" || remove[i] {
				continue
			}
			allAttribution = false
		}
		if allAttribution {
			remove[separator] = true
			for i := separator - 1; i >= 0 && strings.TrimSpace(lines[i]) == ""; i-- {
				remove[i] = true
			}
			for i := separator + 1; i < len(lines) && strings.TrimSpace(lines[i]) == ""; i++ {
				remove[i] = true
			}
		}
	}
	// Removing a standalone marker must not leave a new double-blank gap.
	for _, i := range indexes {
		inTrailer := false
		for _, block := range blocks {
			if i >= block.start && i < block.end {
				inTrailer = true
				break
			}
		}
		if inTrailer {
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
