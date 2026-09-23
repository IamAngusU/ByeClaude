package clean

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/model"
)

type Ref struct {
	Name string
	SHA  string
	Type string
}

func LocalRefs(repo *gitx.Repo) ([]Ref, error) {
	return LocalRefsContext(context.Background(), repo)
}

func LocalRefsContext(ctx context.Context, repo *gitx.Repo) ([]Ref, error) {
	return refsFromNamespacesContext(ctx, repo, "refs/heads", "refs/tags")
}

func refsFromNamespaces(repo *gitx.Repo, namespaces ...string) ([]Ref, error) {
	return refsFromNamespacesContext(context.Background(), repo, namespaces...)
}

func refsFromNamespacesContext(ctx context.Context, repo *gitx.Repo, namespaces ...string) ([]Ref, error) {
	args := []string{"for-each-ref", "--format=%(refname)%00%(objectname)%00%(objecttype)"}
	args = append(args, namespaces...)
	out, err := repo.RunContext(ctx, args...)
	if err != nil {
		return nil, err
	}
	var refs []Ref
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		p := strings.Split(line, "\x00")
		if len(p) != 3 {
			continue
		}
		refs = append(refs, Ref{Name: p[0], SHA: p[1], Type: p[2]})
	}
	return refs, nil
}

func commitsForRefs(repo *gitx.Repo, refs []Ref) ([]string, error) {
	return commitsForRefsContext(context.Background(), repo, refs)
}

func commitsForRefsContext(ctx context.Context, repo *gitx.Repo, refs []Ref) ([]string, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	args := []string{"rev-list", "--topo-order", "--reverse"}
	for _, r := range refs {
		peeled, err := repo.RunContext(ctx, "rev-parse", "--verify", r.Name+"^{commit}")
		if err != nil {
			// Tags that resolve to blobs/trees have no commit history to scan.
			continue
		}
		args = append(args, strings.TrimSpace(string(peeled)))
	}
	if len(args) == 3 {
		return nil, nil
	}
	out, err := repo.RunContext(ctx, args...)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var commits []string
	for _, sha := range strings.Fields(string(out)) {
		if !seen[sha] {
			seen[sha] = true
			commits = append(commits, sha)
		}
	}
	return commits, nil
}

func Scan(repo *gitx.Repo, matcher attribution.Matcher) (model.ScanReport, error) {
	return ScanContext(context.Background(), repo, matcher)
}

func ScanContext(ctx context.Context, repo *gitx.Repo, matcher attribution.Matcher) (model.ScanReport, error) {
	return ScanIncludingRemotesContext(ctx, repo, false, matcher)
}

func ScanIncludingRemotes(repo *gitx.Repo, includeRemotes bool, matcher attribution.Matcher) (model.ScanReport, error) {
	return ScanIncludingRemotesContext(context.Background(), repo, includeRemotes, matcher)
}

func ScanIncludingRemotesContext(ctx context.Context, repo *gitx.Repo, includeRemotes bool, matcher attribution.Matcher) (model.ScanReport, error) {
	started := time.Now()
	if matcher == nil {
		return model.ScanReport{}, fmt.Errorf("attribution matcher is required")
	}
	refs, err := LocalRefsContext(ctx, repo)
	if err != nil {
		return model.ScanReport{}, err
	}
	if includeRemotes {
		remoteRefs, err := refsFromNamespacesContext(ctx, repo, "refs/remotes")
		if err != nil {
			return model.ScanReport{}, err
		}
		refs = append(refs, remoteRefs...)
	}
	if headOut, err := repo.RunContext(ctx, "rev-parse", "--verify", "HEAD"); err == nil {
		refs = append(refs, Ref{Name: "HEAD", SHA: strings.TrimSpace(string(headOut)), Type: "commit"})
	}
	commits, err := commitsForRefsContext(ctx, repo, refs)
	if err != nil {
		return model.ScanReport{}, err
	}
	rawCommits, err := repo.CatFileBatch(ctx, commits, "commit")
	if err != nil {
		return model.ScanReport{}, err
	}

	report := model.ScanReport{
		Repository:  repo.Root,
		Commits:     len(commits),
		RuleMatches: map[string]int{},
	}
	matchedCommits := map[string]bool{}
	for i, sha := range commits {
		obj, err := parseCommit(rawCommits[i])
		if err != nil {
			return report, err
		}
		author, email := obj.author()
		for _, evidence := range MatchingEvidence(obj.Message, matcher) {
			matchedCommits[sha] = true
			for _, ruleID := range evidence.RuleIDs {
				report.RuleMatches[ruleID]++
			}
			report.Matches = append(report.Matches, model.Match{
				Commit:           sha,
				Author:           author,
				Email:            email,
				AttributionName:  evidence.Name,
				AttributionEmail: evidence.Email,
				Rules:            append([]string(nil), evidence.RuleIDs...),
				Line:             evidence.Line,
			})
		}
	}
	report.MatchedCommits = len(matchedCommits)
	if report.Commits > 0 {
		report.CommitMatchPct = (float64(report.MatchedCommits) / float64(report.Commits)) * 100
	}
	report.DurationMS = time.Since(started).Milliseconds()
	return report, nil
}

func Preflight(repo *gitx.Repo) error {
	return PreflightContext(context.Background(), repo)
}

func PreflightContext(ctx context.Context, repo *gitx.Repo) error {
	shallow, err := repo.RunContext(ctx, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(shallow)) == "true" {
		return fmt.Errorf("shallow repository: fetch full history before rewriting")
	}

	replaceRefs, err := repo.RunContext(ctx, "for-each-ref", "--format=%(refname)", "refs/replace")
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(replaceRefs)) != 0 {
		return fmt.Errorf("replace refs are active; remove refs/replace entries before rewriting")
	}

	noteRefs, err := repo.RunContext(ctx, "for-each-ref", "--format=%(refname)", "refs/notes")
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(noteRefs)) != 0 {
		return fmt.Errorf("git notes are present; migrate or remove refs/notes before rewriting commit IDs")
	}

	worktrees, err := repo.RunContext(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return err
	}
	count := 0
	for _, line := range strings.Split(string(worktrees), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			count++
		}
	}
	if count > 1 {
		return fmt.Errorf("multiple linked worktrees are present; remove or consolidate them before rewriting")
	}

	if !repo.Bare {
		if _, err := repo.RunContext(ctx, "symbolic-ref", "-q", "HEAD"); err != nil {
			return fmt.Errorf("detached HEAD: switch to a branch before rewriting")
		}
		if state := inProgressOperation(repo.GitDir); state != "" {
			return fmt.Errorf("git operation in progress (%s); finish or abort it before rewriting", state)
		}

		status, err := repo.RunContext(ctx, "status", "--porcelain=v1", "--untracked-files=all")
		if err != nil {
			return err
		}
		if len(bytes.TrimSpace(status)) != 0 {
			return fmt.Errorf("working tree is not clean; commit or stash changes first")
		}
	}
	return nil
}

func inProgressOperation(gitDir string) string {
	checks := []struct {
		name string
		path string
	}{
		{"merge", "MERGE_HEAD"},
		{"cherry-pick", "CHERRY_PICK_HEAD"},
		{"revert", "REVERT_HEAD"},
		{"bisect", "BISECT_LOG"},
		{"rebase", "rebase-merge"},
		{"rebase/am", "rebase-apply"},
		{"sequencer", "sequencer"},
	}
	for _, check := range checks {
		if _, err := os.Stat(filepath.Join(gitDir, check.path)); err == nil {
			return check.name
		}
	}
	return ""
}

func backupID() (string, error) {
	var nonce [6]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("generate backup ID: %w", err)
	}
	return time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(nonce[:]), nil
}

func createBackups(repo *gitx.Repo, refs []Ref, id string) error {
	var b strings.Builder
	b.WriteString("start\n")
	for _, r := range refs {
		suffix := strings.TrimPrefix(r.Name, "refs/")
		fmt.Fprintf(&b, "create refs/byeclaude/backups/%s/%s %s\n", id, suffix, r.SHA)
	}
	b.WriteString("prepare\ncommit\n")
	_, err := repo.RunInput([]byte(b.String()), "update-ref", "--stdin")
	return err
}

func Rewrite(repo *gitx.Repo, matcher attribution.Matcher) (model.RewriteReport, map[string]string, error) {
	started := time.Now()
	if matcher == nil {
		return model.RewriteReport{}, nil, fmt.Errorf("attribution matcher is required")
	}
	if err := Preflight(repo); err != nil {
		return model.RewriteReport{}, nil, err
	}
	refs, err := LocalRefs(repo)
	if err != nil {
		return model.RewriteReport{}, nil, err
	}
	commits, err := commitsForRefs(repo, refs)
	if err != nil {
		return model.RewriteReport{}, nil, err
	}
	id, err := backupID()
	if err != nil {
		return model.RewriteReport{}, nil, err
	}
	if err := createBackups(repo, refs, id); err != nil {
		return model.RewriteReport{}, nil, fmt.Errorf("create backup refs: %w", err)
	}

	report := model.RewriteReport{Repository: repo.Root, Backup: id, CommitsVisited: len(commits)}
	mapping := make(map[string]string, len(commits))
	rawCommits, err := repo.CatFileBatch(context.Background(), commits, "commit")
	if err != nil {
		return report, mapping, err
	}
	for i, sha := range commits {
		obj, err := parseCommit(rawCommits[i])
		if err != nil {
			return report, mapping, err
		}
		newMsg, removed := StripMatchingTrailers(obj.Message, matcher)
		parentChanged := false
		for _, p := range obj.parents() {
			if n, ok := mapping[p]; ok && n != p {
				parentChanged = true
				break
			}
		}
		if len(removed) == 0 && !parentChanged {
			mapping[sha] = sha
			continue
		}
		newRaw, dropped := rebuildCommit(obj, mapping, newMsg)
		newSHAOut, err := repo.RunInput(newRaw, "hash-object", "-t", "commit", "-w", "--stdin")
		if err != nil {
			return report, mapping, err
		}
		newSHA := strings.TrimSpace(string(newSHAOut))
		mapping[sha] = newSHA
		report.CommitsRewritten++
		report.SignaturesDropped += dropped
	}

	newRefs := make(map[string]string, len(refs))
	tagMemo := map[string]string{}
	for _, r := range refs {
		newSHA := r.SHA
		if r.Type == "commit" {
			if m, ok := mapping[r.SHA]; ok {
				newSHA = m
			}
		} else if r.Type == "tag" {
			newSHA, err = rewriteTag(repo, r.SHA, mapping, tagMemo, &report)
			if err != nil {
				return report, mapping, err
			}
		}
		newRefs[r.Name] = newSHA
	}

	var tx strings.Builder
	tx.WriteString("start\n")
	for _, r := range refs {
		newSHA := newRefs[r.Name]
		suffix := strings.TrimPrefix(r.Name, "refs/")
		fmt.Fprintf(&tx, "create refs/byeclaude/results/%s/%s %s\n", id, suffix, newSHA)
		if newSHA == r.SHA {
			continue
		}
		fmt.Fprintf(&tx, "update %s %s %s\n", r.Name, newSHA, r.SHA)
		report.RefsUpdated++
	}
	tx.WriteString("prepare\ncommit\n")
	if _, err := repo.RunInput([]byte(tx.String()), "update-ref", "--stdin"); err != nil {
		return report, mapping, fmt.Errorf("update refs: %w", err)
	}
	report.DurationMS = time.Since(started).Milliseconds()
	return report, mapping, nil
}

func rewriteTag(repo *gitx.Repo, sha string, commits map[string]string, memo map[string]string, report *model.RewriteReport) (string, error) {
	if v, ok := memo[sha]; ok {
		return v, nil
	}
	raw, err := repo.Run("cat-file", "tag", sha)
	if err != nil {
		return "", err
	}
	text := string(raw)
	sep := strings.Index(text, "\n\n")
	if sep < 0 {
		return "", fmt.Errorf("invalid tag object %s", sha)
	}
	head, msg := text[:sep], text[sep+2:]
	lines := strings.Split(head, "\n")
	var target, typ string
	for _, l := range lines {
		if strings.HasPrefix(l, "object ") {
			target = strings.TrimSpace(strings.TrimPrefix(l, "object "))
		}
		if strings.HasPrefix(l, "type ") {
			typ = strings.TrimSpace(strings.TrimPrefix(l, "type "))
		}
	}
	newTarget := target
	if typ == "commit" {
		if m, ok := commits[target]; ok {
			newTarget = m
		}
	} else if typ == "tag" {
		newTarget, err = rewriteTag(repo, target, commits, memo, report)
		if err != nil {
			return "", err
		}
	}
	if newTarget == target {
		memo[sha] = sha
		return sha, nil
	}
	for i, l := range lines {
		if strings.HasPrefix(l, "object ") {
			lines[i] = "object " + newTarget
			break
		}
	}
	if unsigned, dropped := stripTagSignature(msg); dropped {
		msg = unsigned
		report.SignaturesDropped++
	}
	newRaw := []byte(strings.Join(lines, "\n") + "\n\n" + msg)
	out, err := repo.RunInput(newRaw, "hash-object", "-t", "tag", "-w", "--stdin")
	if err != nil {
		return "", err
	}
	newSHA := strings.TrimSpace(string(out))
	memo[sha] = newSHA
	report.TagsRewritten++
	return newSHA, nil
}

func stripTagSignature(msg string) (string, bool) {
	markers := []string{
		"-----BEGIN PGP SIGNATURE-----",
		"-----BEGIN PGP MESSAGE-----",
		"-----BEGIN SSH SIGNATURE-----",
		"-----BEGIN SIGNED MESSAGE-----",
	}
	idx := -1
	for _, marker := range markers {
		if i := strings.Index(msg, marker); i >= 0 && (idx < 0 || i < idx) {
			idx = i
		}
	}
	if idx < 0 {
		return msg, false
	}
	return strings.TrimRight(msg[:idx], "\r\n") + "\n", true
}

func Push(repo *gitx.Repo, remote string, oldRefs []Ref) error {
	current, err := LocalRefs(repo)
	if err != nil {
		return err
	}
	return pushRefs(repo, remote, oldRefs, current)
}

// PushBackup publishes exactly the rewrite result recorded for one backup ID.
// It refuses if a local head/tag moved after that rewrite, so a review-then-push
// workflow cannot accidentally publish unrelated later local work.
func PushBackup(repo *gitx.Repo, remote, id string) error {
	oldRefs, err := BackupLocalRefs(repo, id)
	if err != nil {
		return err
	}
	desiredRefs, err := ResultLocalRefs(repo, id)
	if err != nil {
		return err
	}
	if len(oldRefs) == 0 {
		return fmt.Errorf("backup %q not found", id)
	}
	if len(desiredRefs) == 0 {
		return fmt.Errorf("rewrite result for backup %q not found; this backup predates review-then-push support", id)
	}

	current, err := LocalRefs(repo)
	if err != nil {
		return err
	}
	currentByName := make(map[string]string, len(current))
	for _, ref := range current {
		currentByName[ref.Name] = ref.SHA
	}
	for _, desired := range desiredRefs {
		got, ok := currentByName[desired.Name]
		if !ok {
			return fmt.Errorf("local ref %s no longer exists; refusing to publish rewrite %s", desired.Name, id)
		}
		if got != desired.SHA {
			return fmt.Errorf("local ref %s moved since rewrite %s; expected %s, now %s", desired.Name, id, desired.SHA, got)
		}
	}
	return pushRefs(repo, remote, oldRefs, desiredRefs)
}

func pushRefs(repo *gitx.Repo, remote string, oldRefs, desiredRefs []Ref) error {
	remoteRefs, err := remoteHeadsAndTags(repo, remote)
	if err != nil {
		return err
	}
	old := map[string]string{}
	for _, r := range oldRefs {
		old[r.Name] = r.SHA
	}
	sort.Slice(desiredRefs, func(i, j int) bool { return desiredRefs[i].Name < desiredRefs[j].Name })

	type update struct {
		ref      Ref
		expected string
	}
	var updates []update
	for _, r := range desiredRefs {
		expected, ok := old[r.Name]
		remoteSHA, existsRemote := remoteRefs[r.Name]
		if !ok || expected == r.SHA || !existsRemote || remoteSHA == r.SHA {
			continue
		}
		updates = append(updates, update{ref: r, expected: expected})
	}
	if len(updates) == 0 {
		return nil
	}

	args := []string{"push", "--atomic"}
	for _, u := range updates {
		args = append(args, "--force-with-lease="+u.ref.Name+":"+u.expected)
	}
	args = append(args, remote)
	for _, u := range updates {
		args = append(args, u.ref.SHA+":"+u.ref.Name)
	}
	_, err = repo.Run(args...)
	return err
}

func remoteHeadsAndTags(repo *gitx.Repo, remote string) (map[string]string, error) {
	out, err := repo.Run("ls-remote", "--refs", remote, "refs/heads/*", "refs/tags/*")
	if err != nil {
		return nil, fmt.Errorf("inspect remote refs: %w", err)
	}
	refs := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		refs[fields[1]] = fields[0]
	}
	return refs, nil
}

func JSON(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }

func snapshotLocalRefs(repo *gitx.Repo, namespace, id string) ([]Ref, error) {
	prefix := "refs/byeclaude/" + namespace + "/" + id + "/"
	out, err := repo.Run("for-each-ref", "--format=%(refname)%00%(objectname)", prefix)
	if err != nil {
		return nil, err
	}
	var refs []Ref
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\x00")
		if len(parts) != 2 {
			continue
		}
		suffix := strings.TrimPrefix(parts[0], prefix)
		if !strings.HasPrefix(suffix, "heads/") && !strings.HasPrefix(suffix, "tags/") {
			continue
		}
		refs = append(refs, Ref{Name: "refs/" + suffix, SHA: parts[1]})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Name < refs[j].Name })
	return refs, nil
}

func BackupLocalRefs(repo *gitx.Repo, id string) ([]Ref, error) {
	return snapshotLocalRefs(repo, "backups", id)
}

func ResultLocalRefs(repo *gitx.Repo, id string) ([]Ref, error) {
	return snapshotLocalRefs(repo, "results", id)
}

func BackupRefs(repo *gitx.Repo) ([]string, error) {
	out, err := repo.Run("for-each-ref", "--format=%(refname)", "refs/byeclaude/backups")
	if err != nil {
		return nil, err
	}
	var ids []string
	seen := map[string]bool{}
	s := bufio.NewScanner(bytes.NewReader(out))
	for s.Scan() {
		p := strings.Split(s.Text(), "/")
		if len(p) >= 4 && !seen[p[3]] {
			seen[p[3]] = true
			ids = append(ids, p[3])
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	return ids, s.Err()
}

func Restore(repo *gitx.Repo, id string) (int, error) {
	if err := Preflight(repo); err != nil {
		return 0, err
	}
	items, err := BackupLocalRefs(repo, id)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, fmt.Errorf("backup %q not found", id)
	}

	var tx strings.Builder
	tx.WriteString("start\n")
	for _, item := range items {
		currentOut, err := repo.Run("show-ref", "--verify", "--hash", item.Name)
		if err != nil {
			fmt.Fprintf(&tx, "create %s %s\n", item.Name, item.SHA)
			continue
		}
		current := strings.TrimSpace(string(currentOut))
		fmt.Fprintf(&tx, "update %s %s %s\n", item.Name, item.SHA, current)
	}
	tx.WriteString("prepare\ncommit\n")
	if _, err := repo.RunInput([]byte(tx.String()), "update-ref", "--stdin"); err != nil {
		return 0, fmt.Errorf("restore refs: %w", err)
	}
	return len(items), nil
}
