package clean

import (
	"regexp"
	"strings"
)

var (
	coAuthorRE = regexp.MustCompile(`(?i)^\s*co-authored-by\s*:\s*(.*?)\s*<([^>]+)>\s*$`)
	trailerRE  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*\s*:\s*.+$`)
)

func IsClaudeTrailer(line string) (name, email string, ok bool) {
	m := coAuthorRE.FindStringSubmatch(line)
	if len(m) != 3 {
		return "", "", false
	}
	name = strings.TrimSpace(m[1])
	email = strings.TrimSpace(m[2])
	nameLower := strings.ToLower(name)
	emailLower := strings.ToLower(email)
	if strings.Contains(nameLower, "claude") && (emailLower == "noreply@anthropic.com" || strings.HasSuffix(emailLower, "@anthropic.com")) {
		return name, email, true
	}
	return "", "", false
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

func ClaudeTrailers(message string) []string {
	normalized := strings.ReplaceAll(message, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	start, end, ok := trailerBlock(lines)
	if !ok {
		return nil
	}
	var matches []string
	for _, line := range lines[start:end] {
		if _, _, match := IsClaudeTrailer(line); match {
			matches = append(matches, line)
		}
	}
	return matches
}

func StripClaudeTrailers(message string) (string, []string) {
	newline := "\n"
	if strings.Contains(message, "\r\n") {
		newline = "\r\n"
	}
	normalized := strings.ReplaceAll(message, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	start, end, ok := trailerBlock(lines)
	if !ok {
		return message, nil
	}

	removed := make([]string, 0)
	keptBlock := make([]string, 0, end-start)
	for _, line := range lines[start:end] {
		if _, _, match := IsClaudeTrailer(line); match {
			removed = append(removed, line)
			continue
		}
		keptBlock = append(keptBlock, line)
	}
	if len(removed) == 0 {
		return message, nil
	}

	kept := append([]string{}, lines[:start]...)
	if len(keptBlock) > 0 {
		kept = append(kept, keptBlock...)
	} else if len(kept) > 0 && kept[len(kept)-1] == "" {
		kept = kept[:len(kept)-1]
	}
	kept = append(kept, lines[end:]...)
	for len(kept) > 1 && kept[len(kept)-1] == "" && kept[len(kept)-2] == "" {
		kept = kept[:len(kept)-1]
	}
	out := strings.Join(kept, "\n")
	if newline == "\r\n" {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	return out, removed
}
