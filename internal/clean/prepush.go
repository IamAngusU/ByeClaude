package clean

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

var objectIDRE = regexp.MustCompile("^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$")

type PushFinding struct {
	Commit   string `json:"commit"`
	Field    string `json:"field"`
	Identity string `json:"identity,omitempty"`
}

type PushReport struct {
	Commits          int           `json:"commits_scanned"`
	Trailers         int           `json:"matching_trailers"`
	Authors          int           `json:"matching_authors"`
	Committers       int           `json:"matching_committers"`
	CommitsWithMatch int           `json:"commits_with_matches"`
	Findings         []PushFinding `json:"findings"`
}

// CheckPushInput consumes the ref-update protocol sent to git's pre-push hook.
// All commits reachable from pushed commit/tag tips are checked, including
// ancestors, so the guard does not miss old trailers brought by a new branch.
// Tag-only deletion and branch deletion have no new commits to inspect.
func CheckPushInput(repo *gitx.Repo, input io.Reader, matcher attribution.Matcher) (PushReport, error) {
	report := PushReport{}
	if matcher == nil {
		return report, fmt.Errorf("attribution matcher is required")
	}
	reader := bufio.NewScanner(input)
	reader.Buffer(make([]byte, 4096), 1024*1024)
	tips := map[string]bool{}
	for reader.Scan() {
		line := strings.TrimSpace(reader.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 4 {
			return report, fmt.Errorf("invalid Git pre-push ref-update line")
		}
		localRef, localSHA := fields[0], fields[1]
		if !objectIDRE.MatchString(localSHA) || !objectIDRE.MatchString(fields[3]) {
			return report, fmt.Errorf("invalid Git object ID in pre-push input")
		}
		if isZeroSHA(localSHA) {
			continue // a deletion has no new commit graph
		}
		if !strings.HasPrefix(localRef, "refs/") && localRef != "HEAD" {
			return report, fmt.Errorf("invalid local ref in pre-push input")
		}
		peeled, err := repo.RunContext(context.Background(), "rev-parse", "--verify", localSHA+"^{commit}")
		if err != nil {
			typ, typeErr := repo.RunContext(context.Background(), "cat-file", "-t", localSHA)
			if typeErr == nil && (strings.TrimSpace(string(typ)) == "blob" || strings.TrimSpace(string(typ)) == "tree") {
				continue // a tag pointing at a non-commit object
			}
			return report, fmt.Errorf("cannot resolve pushed commit %s: %w", localSHA, err)
		}
		tips[strings.TrimSpace(string(peeled))] = true
	}
	if err := reader.Err(); err != nil {
		return report, err
	}
	if len(tips) == 0 {
		return report, nil
	}
	var inputBuf bytes.Buffer
	for sha := range tips {
		fmt.Fprintln(&inputBuf, sha)
	}
	commitsOut, err := repo.RunInput(inputBuf.Bytes(), "rev-list", "--stdin")
	if err != nil {
		return report, fmt.Errorf("list pushed commit ancestry: %w", err)
	}
	commits := strings.Fields(string(commitsOut))
	raw, err := repo.CatFileBatch(context.Background(), commits, "commit")
	if err != nil {
		return report, err
	}
	report.Commits = len(commits)
	for i, sha := range commits {
		obj, err := parseCommit(raw[i])
		if err != nil {
			return report, err
		}
		matched := false
		for _, trailer := range MatchingEvidence(obj.Message, matcher) {
			report.Trailers++
			matched = true
			addPushFinding(&report, PushFinding{Commit: sha, Field: "Co-Authored-By", Identity: trailer.Name + " <" + trailer.Email + ">"})
		}
		for _, h := range obj.Headers {
			if h.Key != "author" && h.Key != "committer" {
				continue
			}
			if len(h.Lines) != 1 {
				return report, fmt.Errorf("invalid %s header in commit %s", h.Key, sha)
			}
			parts := gitIdentityHeader.FindStringSubmatch(h.Lines[0])
			if len(parts) != 6 {
				return report, fmt.Errorf("invalid %s header in commit %s", h.Key, sha)
			}
			if !matcher.Match(strings.TrimSpace(parts[2]), parts[3]) {
				continue
			}
			matched = true
			if h.Key == "author" {
				report.Authors++
			} else {
				report.Committers++
			}
			addPushFinding(&report, PushFinding{Commit: sha, Field: h.Key, Identity: strings.TrimSpace(parts[2]) + " <" + parts[3] + ">"})
		}
		if matched {
			report.CommitsWithMatch++
		}
	}
	return report, nil
}

func addPushFinding(report *PushReport, value PushFinding) {
	if len(report.Findings) < 15 {
		report.Findings = append(report.Findings, value)
	}
}

func isZeroSHA(sha string) bool {
	return strings.Trim(sha, "0") == ""
}
