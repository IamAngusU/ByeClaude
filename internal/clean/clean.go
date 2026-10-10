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
	"regexp"
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
	report := model.ScanReport{
		Repository:  repo.Root,
		Commits:     len(commits),
		RuleMatches: map[string]int{},
	}
	matchedCommits := map[string]bool{}
	err = repo.CatFileBatchEach(ctx, commits, "commit", func(i int, sha string, raw []byte) error {
		obj, err := parseCommit(raw)
		if err != nil {
			return err
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
				AttributionField: evidence.Field,
				Rules:            append([]string(nil), evidence.RuleIDs...),
				Line:             evidence.Line,
			})
		}
		return nil
	})
	if err != nil {
		return report, err
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

var backupIDPattern = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}Z-[0-9a-f]{12}$`)

func createBackups(repo *gitx.Repo, refs []Ref, id string) error {
	return createBackupsContext(context.Background(), repo, refs, id, "")
}

func createBackupsContext(ctx context.Context, repo *gitx.Repo, refs []Ref, id, upstreamSHA string) error {
	var b strings.Builder
	b.WriteString("start\n")
	for _, r := range refs {
		suffix := strings.TrimPrefix(r.Name, "refs/")
		fmt.Fprintf(&b, "create refs/byeclaude/backups/%s/%s %s\n", id, suffix, r.SHA)
		if upstreamSHA != "" {
			fmt.Fprintf(&b, "create refs/byeclaude/upstreams/%s/%s %s\n", id, suffix, upstreamSHA)
		}
	}
	b.WriteString("prepare\ncommit\n")
	_, err := repo.RunInputContext(ctx, []byte(b.String()), "update-ref", "--stdin")
	return err
}

func Rewrite(repo *gitx.Repo, matcher attribution.Matcher) (model.RewriteReport, map[string]string, error) {
	return RewriteWithIdentity(repo, matcher, IdentityRewriteOptions{})
}

func RewriteWithIdentity(repo *gitx.Repo, matcher attribution.Matcher, opts IdentityRewriteOptions) (model.RewriteReport, map[string]string, error) {
	return RewriteWithIdentityContext(context.Background(), repo, matcher, opts)
}

func RewriteWithIdentityContext(ctx context.Context, repo *gitx.Repo, matcher attribution.Matcher, opts IdentityRewriteOptions) (model.RewriteReport, map[string]string, error) {
	if err := opts.Validate(matcher); err != nil {
		return model.RewriteReport{}, nil, err
	}
	if matcher == nil {
		return model.RewriteReport{}, nil, fmt.Errorf("attribution matcher is required")
	}
	if err := PreflightContext(ctx, repo); err != nil {
		return model.RewriteReport{}, nil, err
	}
	selection, err := allRewriteSelectionContext(ctx, repo)
	if err != nil {
		return model.RewriteReport{}, nil, err
	}
	return rewriteWithSelectionContext(ctx, repo, matcher, opts, selection)
}

func RewriteUnpushedWithIdentityContext(ctx context.Context, repo *gitx.Repo, matcher attribution.Matcher, opts IdentityRewriteOptions) (model.RewriteReport, map[string]string, error) {
	if err := opts.Validate(matcher); err != nil {
		return model.RewriteReport{}, nil, err
	}
	if matcher == nil {
		return model.RewriteReport{}, nil, fmt.Errorf("attribution matcher is required")
	}
	if err := PreflightContext(ctx, repo); err != nil {
		return model.RewriteReport{}, nil, err
	}
	selection, err := unpushedRewriteSelectionContext(ctx, repo)
	if err != nil {
		return model.RewriteReport{}, nil, err
	}
	return rewriteWithSelectionContext(ctx, repo, matcher, opts, selection)
}

func rewriteWithSelectionContext(ctx context.Context, repo *gitx.Repo, matcher attribution.Matcher, opts IdentityRewriteOptions, selection rewriteSelection) (model.RewriteReport, map[string]string, error) {
	started := time.Now()
	if err := opts.Validate(matcher); err != nil {
		return model.RewriteReport{}, nil, err
	}
	if matcher == nil {
		return model.RewriteReport{}, nil, fmt.Errorf("attribution matcher is required")
	}
	refs := selection.refs
	commits := selection.commits
	id, err := backupID()
	if err != nil {
		return model.RewriteReport{}, nil, err
	}
	if err := createBackupsContext(ctx, repo, refs, id, selection.upstreamSHA); err != nil {
		return model.RewriteReport{}, nil, fmt.Errorf("create backup refs: %w", err)
	}

	report := model.RewriteReport{
		Repository:     repo.Root,
		Backup:         id,
		Scope:          selection.scope,
		Upstream:       selection.upstream,
		UpstreamCommit: selection.upstreamSHA,
		Remote:         selection.remote,
		RemoteVerified: selection.remoteVerified,
		CommitsVisited: len(commits),
	}
	mapping := make(map[string]string, len(commits))
	err = repo.CatFileBatchEach(ctx, commits, "commit", func(_ int, sha string, raw []byte) error {
		obj, err := parseCommit(raw)
		if err != nil {
			return err
		}
		newMsg, removed := StripMatchingTrailers(obj.Message, matcher)
		rewrittenObj, authors, committers, err := ReplaceMatchingCommitIdentities(obj, matcher, opts)
		if err != nil {
			return err
		}
		parentChanged := false
		for _, p := range obj.parents() {
			if n, ok := mapping[p]; ok && n != p {
				parentChanged = true
				break
			}
		}
		if len(removed) == 0 && authors == 0 && committers == 0 && !parentChanged {
			mapping[sha] = sha
			return nil
		}
		newRaw, dropped := rebuildCommit(rewrittenObj, mapping, newMsg)
		newObject, err := parseCommit(newRaw)
		if err != nil || obj.tree() == "" || newObject.tree() != obj.tree() {
			return fmt.Errorf("tree verification failed before writing rewritten commit %s", sha)
		}
		newSHAOut, err := repo.RunInputContext(ctx, newRaw, "hash-object", "-t", "commit", "-w", "--stdin")
		if err != nil {
			return err
		}
		newSHA := strings.TrimSpace(string(newSHAOut))
		mapping[sha] = newSHA
		report.CommitsRewritten++
		report.CreditsRemoved += len(removed)
		report.SignaturesDropped += dropped
		report.AuthorsReplaced += authors
		report.CommittersReplaced += committers
		return nil
	})
	if err != nil {
		return report, mapping, err
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
			newSHA, err = rewriteTagContext(ctx, repo, r.SHA, mapping, tagMemo, &report)
			if err != nil {
				return report, mapping, err
			}
		}
		newRefs[r.Name] = newSHA
	}

	var tx strings.Builder
	tx.WriteString("start\n")
	var updatedRefs []Ref
	for _, r := range refs {
		newSHA := newRefs[r.Name]
		suffix := strings.TrimPrefix(r.Name, "refs/")
		fmt.Fprintf(&tx, "create refs/byeclaude/results/%s/%s %s\n", id, suffix, newSHA)
		if newSHA == r.SHA {
			continue
		}
		fmt.Fprintf(&tx, "update %s %s %s\n", r.Name, newSHA, r.SHA)
		report.RefsUpdated++
		updatedRefs = append(updatedRefs, r)
	}
	tx.WriteString("prepare\ncommit\n")
	if _, err := repo.RunInputContext(ctx, []byte(tx.String()), "update-ref", "--stdin"); err != nil {
		return report, mapping, fmt.Errorf("update refs: %w", err)
	}
	verified, err := verifyUpdatedRefTreesContext(ctx, repo, updatedRefs, newRefs)
	if err != nil {
		rollbackErr := rollbackUpdatedRefs(repo, updatedRefs, newRefs)
		if rollbackErr != nil {
			return report, mapping, fmt.Errorf("post-update tree proof failed: %w; automatic rollback also failed: %v (backup %s remains available)", err, rollbackErr, id)
		}
		return report, mapping, fmt.Errorf("post-update tree proof failed: %w; updated refs were rolled back (backup %s remains available)", err, id)
	}
	report.TreesVerified = verified
	report.DurationMS = time.Since(started).Milliseconds()
	return report, mapping, nil
}

func verifyUpdatedRefTreesContext(ctx context.Context, repo *gitx.Repo, oldRefs []Ref, newRefs map[string]string) (int, error) {
	verified := 0
	for _, ref := range oldRefs {
		newSHA := newRefs[ref.Name]
		out, err := repo.RunContext(ctx, "rev-parse", ref.SHA+"^{tree}", ref.Name+"^{tree}")
		if err != nil {
			return verified, fmt.Errorf("read tree tips for %s: %w", ref.Name, err)
		}
		trees := strings.Fields(string(out))
		if len(trees) != 2 {
			return verified, fmt.Errorf("read tree tips for %s: expected two object IDs, got %q", ref.Name, strings.TrimSpace(string(out)))
		}
		if trees[0] != trees[1] {
			return verified, fmt.Errorf("%s changed tree %s -> %s (new object %s)", ref.Name, trees[0], trees[1], newSHA)
		}
		verified++
	}
	return verified, nil
}

func rollbackUpdatedRefs(repo *gitx.Repo, oldRefs []Ref, newRefs map[string]string) error {
	if len(oldRefs) == 0 {
		return nil
	}
	var tx strings.Builder
	tx.WriteString("start\n")
	for _, ref := range oldRefs {
		fmt.Fprintf(&tx, "update %s %s %s\n", ref.Name, ref.SHA, newRefs[ref.Name])
	}
	tx.WriteString("prepare\ncommit\n")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := repo.RunInputContext(ctx, []byte(tx.String()), "update-ref", "--stdin")
	return err
}

func rewriteTagContext(ctx context.Context, repo *gitx.Repo, sha string, commits map[string]string, memo map[string]string, report *model.RewriteReport) (string, error) {
	if v, ok := memo[sha]; ok {
		return v, nil
	}
	raw, err := repo.RunContext(ctx, "cat-file", "tag", sha)
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
		newTarget, err = rewriteTagContext(ctx, repo, target, commits, memo, report)
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
	out, err := repo.RunInputContext(ctx, newRaw, "hash-object", "-t", "tag", "-w", "--stdin")
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
	return pushRefs(repo, remote, oldRefs, current, nil)
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
	upstreamRefs, err := UpstreamLocalRefs(repo, id)
	if err != nil {
		return err
	}
	verifiedUpstreams := make(map[string]string, len(upstreamRefs))
	for _, ref := range upstreamRefs {
		verifiedUpstreams[ref.Name] = ref.SHA
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
	return pushRefs(repo, remote, oldRefs, desiredRefs, verifiedUpstreams)
}

func pushRefs(repo *gitx.Repo, remote string, oldRefs, desiredRefs []Ref, verifiedUpstreams map[string]string) error {
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
		lease := expected
		if remoteSHA != expected {
			verifiedUpstream, scoped := verifiedUpstreams[r.Name]
			if !scoped || remoteSHA != verifiedUpstream {
				return fmt.Errorf("remote ref %s is %s, expected pre-rewrite tip %s; refusing to overwrite it", r.Name, remoteSHA, expected)
			}
			if !strings.HasPrefix(r.Name, "refs/heads/") {
				return fmt.Errorf("remote ref %s is %s, expected pre-rewrite tip %s; refusing to overwrite it", r.Name, remoteSHA, expected)
			}
			remoteBeforeOld, err := isAncestorContext(context.Background(), repo, remoteSHA, expected)
			if err != nil {
				return fmt.Errorf("prove remote ancestry for %s against pre-rewrite tip: %w", r.Name, err)
			}
			remoteBeforeNew, err := isAncestorContext(context.Background(), repo, remoteSHA, r.SHA)
			if err != nil {
				return fmt.Errorf("prove remote ancestry for %s against rewrite result: %w", r.Name, err)
			}
			if !remoteBeforeOld || !remoteBeforeNew {
				return fmt.Errorf("remote ref %s moved outside the verified local lineage (%s); fetch and review before publishing", r.Name, remoteSHA)
			}
			lease = remoteSHA
		}
		updates = append(updates, update{ref: r, expected: lease})
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

func UpstreamLocalRefs(repo *gitx.Repo, id string) ([]Ref, error) {
	return snapshotLocalRefs(repo, "upstreams", id)
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

// PruneBackup removes both the recovery refs and the recorded rewrite-result
// refs for exactly one backup. Object collection remains Git's responsibility.
func PruneBackup(repo *gitx.Repo, id string) (int, error) {
	if !backupIDPattern.MatchString(id) {
		return 0, fmt.Errorf("invalid backup ID %q", id)
	}
	var refs []Ref
	for _, namespace := range []string{"backups", "results", "upstreams"} {
		prefix := "refs/byeclaude/" + namespace + "/" + id + "/"
		out, err := repo.Run("for-each-ref", "--format=%(refname)%00%(objectname)", prefix)
		if err != nil {
			return 0, err
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line == "" {
				continue
			}
			parts := strings.Split(line, "\x00")
			if len(parts) != 2 || !strings.HasPrefix(parts[0], prefix) {
				return 0, fmt.Errorf("unexpected backup ref while pruning %q", id)
			}
			refs = append(refs, Ref{Name: parts[0], SHA: parts[1]})
		}
	}
	if len(refs) == 0 {
		return 0, fmt.Errorf("backup %q not found", id)
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Name < refs[j].Name })
	var tx strings.Builder
	tx.WriteString("start\n")
	for _, ref := range refs {
		fmt.Fprintf(&tx, "delete %s %s\n", ref.Name, ref.SHA)
	}
	tx.WriteString("prepare\ncommit\n")
	if _, err := repo.RunInput([]byte(tx.String()), "update-ref", "--stdin"); err != nil {
		return 0, fmt.Errorf("prune backup refs: %w", err)
	}
	return len(refs), nil
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
	results, err := ResultLocalRefs(repo, id)
	if err != nil {
		return 0, err
	}
	expected := make(map[string]string, len(results))
	for _, ref := range results {
		expected[ref.Name] = ref.SHA
	}
	refs, err := LocalRefs(repo)
	if err != nil {
		return 0, err
	}
	currentByName := make(map[string]string, len(refs))
	for _, ref := range refs {
		currentByName[ref.Name] = ref.SHA
	}

	var tx strings.Builder
	tx.WriteString("start\n")
	for _, item := range items {
		current, exists := currentByName[item.Name]
		if !exists {
			fmt.Fprintf(&tx, "create %s %s\n", item.Name, item.SHA)
			continue
		}
		if current != item.SHA && current != expected[item.Name] {
			return 0, fmt.Errorf("local ref %s moved since rewrite %s; refusing to discard later work (preserve it on another branch before manual recovery)", item.Name, id)
		}
		fmt.Fprintf(&tx, "update %s %s %s\n", item.Name, item.SHA, current)
	}
	tx.WriteString("prepare\ncommit\n")
	if _, err := repo.RunInput([]byte(tx.String()), "update-ref", "--stdin"); err != nil {
		return 0, fmt.Errorf("restore refs: %w", err)
	}
	return len(items), nil
}
