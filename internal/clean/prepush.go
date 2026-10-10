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
// For updates to existing remote refs, only incoming commits not already in
// the remote tip's ancestry are examined. This avoids blocking every future
// push because of metadata already present in the remote's old history.
// A brand-new remote ref has no previous tip, so its complete reachable
// ancestry is checked. If the old remote tip is absent locally, fail closed
// and ask the user to fetch instead of pretending the push is clean.
// Deleting branches/tags has no new commits to inspect.
func CheckPushInput(repo *gitx.Repo, input io.Reader, matcher attribution.Matcher) (PushReport, error) {
	report := PushReport{}
	if matcher == nil {
		return report, fmt.Errorf("attribution matcher is required")
	}
	reader := bufio.NewScanner(input)
	reader.Buffer(make([]byte, 4096), 1024*1024)
	type pushRange struct{ tip, previous string }
	var ranges []pushRange
	for reader.Scan() {
		line := strings.TrimSpace(reader.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 4 {
			return report, fmt.Errorf("invalid Git pre-push ref-update line")
		}
		localRef, localSHA, remoteRef, oldSHA := fields[0], fields[1], fields[2], fields[3]
		if !objectIDRE.MatchString(localSHA) || !objectIDRE.MatchString(fields[3]) {
			return report, fmt.Errorf("invalid Git object ID in pre-push input")
		}
		if isZeroSHA(localSHA) {
			continue // a deletion has no new commit graph
		}
		if strings.HasPrefix(localRef, "refs/byeclaude/") || strings.HasPrefix(remoteRef, "refs/byeclaude/") {
			return report, fmt.Errorf("push blocked: ByeClaude recovery refs must stay local; use a normal branch/tag push, never git push --mirror")
		}
		// Git also supplies revision expressions or literal object IDs here
		// (for example `git push origin HEAD~1:main`). Only the validated object
		// IDs are used below; the display ref is never executed or resolved.
		peeled, err := repo.RunContext(context.Background(), "rev-parse", "--verify", localSHA+"^{commit}")
		if err != nil {
			typ, typeErr := repo.RunContext(context.Background(), "cat-file", "-t", localSHA)
			if typeErr == nil && (strings.TrimSpace(string(typ)) == "blob" || strings.TrimSpace(string(typ)) == "tree") {
				continue // a tag pointing at a non-commit object
			}
			return report, fmt.Errorf("cannot resolve pushed commit %s: %w", localSHA, err)
		}
		next := pushRange{tip: strings.TrimSpace(string(peeled))}
		if !isZeroSHA(oldSHA) {
			remote, err := repo.RunContext(context.Background(), "rev-parse", "--verify", oldSHA+"^{commit}")
			if err != nil {
				// Remote-tracking refs can be stale or missing. We must not
				// optimistically ignore an unknown remote history.
				return report, fmt.Errorf("remote ref %s points at %s which is not locally available; fetch the remote before pushing (or use --no-verify if you explicitly accept bypassing the local guard)", remoteRef, oldSHA)
			}
			next.previous = strings.TrimSpace(string(remote))
		}
		ranges = append(ranges, next)
	}
	if err := reader.Err(); err != nil {
		return report, err
	}
	if len(ranges) == 0 {
		return report, nil
	}
	// Evaluate each ref update independently. Combining exclusions from
	// multiple remote refs could hide attribution newly entering one ref
	// merely because it already exists on another ref being updated.
	seen := map[string]bool{}
	var commits []string
	for _, r := range ranges {
		var inputBuf bytes.Buffer
		fmt.Fprintln(&inputBuf, r.tip)
		if r.previous != "" {
			fmt.Fprintln(&inputBuf, "^"+r.previous)
		}
		commitsOut, err := repo.RunInput(inputBuf.Bytes(), "rev-list", "--stdin")
		if err != nil {
			return report, fmt.Errorf("list outgoing commits: %w", err)
		}
		for _, sha := range strings.Fields(string(commitsOut)) {
			if !seen[sha] {
				seen[sha] = true
				commits = append(commits, sha)
			}
		}
	}
	report.Commits = len(commits)
	err := repo.CatFileBatchEach(context.Background(), commits, "commit", func(_ int, sha string, raw []byte) error {
		obj, err := parseCommit(raw)
		if err != nil {
			return err
		}
		matched := false
		for _, trailer := range MatchingEvidence(obj.Message, matcher) {
			report.Trailers++
			matched = true
			identity := trailer.Line
			if trailer.Name != "" || trailer.Email != "" {
				identity = trailer.Name + " <" + trailer.Email + ">"
			}
			addPushFinding(&report, PushFinding{Commit: sha, Field: trailer.Field, Identity: identity})
		}
		for _, h := range obj.Headers {
			if h.Key != "author" && h.Key != "committer" {
				continue
			}
			if len(h.Lines) != 1 {
				return fmt.Errorf("invalid %s header in commit %s", h.Key, sha)
			}
			parts := gitIdentityHeader.FindStringSubmatch(h.Lines[0])
			if len(parts) != 6 {
				return fmt.Errorf("invalid %s header in commit %s", h.Key, sha)
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
		return nil
	})
	if err != nil {
		return report, err
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
