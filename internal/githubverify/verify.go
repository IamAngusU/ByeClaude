package githubverify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
	"github.com/IamAngusU/ByeClaude/internal/batch"
	"github.com/IamAngusU/ByeClaude/internal/clean"
	"github.com/IamAngusU/ByeClaude/internal/githubpolicy"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/model"
)

type RemoteHistory struct {
	Status             string `json:"status"`
	Commits            int    `json:"commits_scanned"`
	MatchedCommits     int    `json:"matched_commits"`
	MatchingTrailers   int    `json:"matching_trailers"`
	MatchingAuthors    int    `json:"matching_authors"`
	MatchingCommitters int    `json:"matching_committers"`
}
type PullHistory struct {
	Status             string   `json:"status"`
	RefsAdvertised     int      `json:"refs_advertised"`
	RefsSelected       int      `json:"refs_selected"`
	RefsSkipped        int      `json:"refs_skipped"`
	MatchedCommits     int      `json:"matched_commits"`
	MatchingTrailers   int      `json:"matching_trailers"`
	MatchingAuthors    int      `json:"matching_authors"`
	MatchingCommitters int      `json:"matching_committers"`
	ExampleCommits     []string `json:"example_commits,omitempty"`
	Error              string   `json:"error,omitempty"`
}
type ContributorCheck struct {
	Status          string `json:"status"`
	GitHubUser      string `json:"github_user,omitempty"`
	GitHubID        string `json:"github_id,omitempty"`
	NumberOfEntries int    `json:"entries_inspected,omitempty"`
	Errors          string `json:"error,omitempty"`
}
type Report struct {
	Repository   string           `json:"repository"`
	VerifiedAt   string           `json:"verified_at"`
	Overall      string           `json:"overall"`
	Remote       RemoteHistory    `json:"remote_history"`
	PullRefs     PullHistory      `json:"pull_refs"`
	Contributors ContributorCheck `json:"contributors"`
	Limitations  []string         `json:"limitations"`
}
type Options struct {
	Repo        string
	Token       string
	Matcher     attribution.Matcher
	GitHubUser  string
	MaxPullRefs int
	HTTPClient  *http.Client
	APIBaseURL  string
}

func Audit(ctx context.Context, opts Options) (Report, error) {
	return audit(ctx, opts, batch.PrepareRepository)
}

func audit(ctx context.Context, opts Options, prepare func(context.Context, batch.Spec, string, bool) (string, func(), error)) (Report, error) {
	r := Report{
		Repository:   opts.Repo,
		VerifiedAt:   time.Now().UTC().Format(time.RFC3339),
		Overall:      "incomplete",
		Remote:       RemoteHistory{Status: "unavailable"},
		PullRefs:     PullHistory{Status: "unavailable"},
		Contributors: ContributorCheck{Status: "not_requested", GitHubUser: opts.GitHubUser},
		Limitations: []string{
			"GitHub PR refs, forks, cached commit URLs, and other clones are not writable by ByeClaude.",
			"Contributor API data can be cached for hours; GitHub says graphs can take about 24 hours after a history rewrite.",
			"The contributor REST API reports commit authors and may not represent all GitHub UI co-author attributions.",
		},
	}
	if err := githubpolicy.ValidateRepo(opts.Repo); err != nil {
		return r, err
	}
	if opts.Matcher == nil {
		return r, fmt.Errorf("attribution matcher is required")
	}
	if err := ValidateGitHubUser(opts.GitHubUser); err != nil {
		return r, err
	}
	if opts.MaxPullRefs < 1 || opts.MaxPullRefs > 1000 {
		return r, fmt.Errorf("max-pull-refs must be between 1 and 1000")
	}
	specs, err := batch.ResolveExplicit([]string{opts.Repo})
	if err != nil {
		return r, err
	}
	repoPath, cleanup, err := prepare(ctx, specs[0], opts.Token, false)
	if err != nil {
		return r, fmt.Errorf("cannot clone current GitHub refs: %w", err)
	}
	defer cleanup()
	gitRepo, err := gitx.OpenContext(ctx, repoPath)
	if err != nil {
		return r, err
	}
	normal, err := clean.ScanIncludingRemotesContext(ctx, gitRepo, false, opts.Matcher)
	if err != nil {
		return r, err
	}
	remoteHeads, err := clean.ScanMatchingHeadersContext(ctx, gitRepo, opts.Matcher, []string{"refs/heads", "refs/tags"}, true)
	if err != nil {
		return r, err
	}
	r.Remote = remoteResult(normal, remoteHeads)
	r.PullRefs = scanPullRefs(ctx, gitRepo, opts.Matcher, opts.MaxPullRefs)
	if opts.GitHubUser != "" {
		r.Contributors = checkContributor(ctx, opts)
	}
	r.Overall = Classify(r)
	return r, nil
}
func remoteResult(scan model.ScanReport, headers clean.HeaderScanReport) RemoteHistory {
	status := "clean"
	if scan.MatchedCommits > 0 || headers.MatchedCommits > 0 {
		status = "matches_found"
	}
	return RemoteHistory{Status: status, Commits: scan.Commits, MatchedCommits: scan.MatchedCommits, MatchingTrailers: len(scan.Matches), MatchingAuthors: headers.Authors, MatchingCommitters: headers.Committers}
}
func Classify(r Report) string {
	if r.Remote.Status == "matches_found" || r.PullRefs.Status == "matches_found" || r.Contributors.Status == "listed" {
		return "residual_evidence"
	}
	if r.Remote.Status != "clean" || r.PullRefs.Status != "clean" && r.PullRefs.Status != "no_advertised_refs" ||
		r.Contributors.Status == "error" || r.Contributors.Status == "pending" || r.Contributors.Status == "partial" {
		return "incomplete"
	}
	return "clean_in_checked_scopes"
}

type gitRemoteRef struct{ ref, sha string }

func scanPullRefs(ctx context.Context, repo *gitx.Repo, matcher attribution.Matcher, limit int) PullHistory {
	out := PullHistory{Status: "unavailable"}
	raw, err := repo.RunContext(ctx, "ls-remote", "origin", "refs/pull/*")
	if err != nil {
		out.Error = "could not list advertised pull request refs"
		return out
	}
	var entries []gitRemoteRef
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		name := f[1]
		if !strings.HasPrefix(name, "refs/pull/") || !(strings.HasSuffix(name, "/head") || strings.HasSuffix(name, "/merge")) {
			continue
		}
		entries = append(entries, gitRemoteRef{ref: name, sha: f[0]})
	}
	sort.Slice(entries, func(i, j int) bool {
		num := func(ref string) int {
			parts := strings.Split(ref, "/")
			if len(parts) != 4 {
				return 0
			}
			n, _ := strconv.Atoi(parts[2])
			return n
		}
		if num(entries[i].ref) != num(entries[j].ref) {
			return num(entries[i].ref) > num(entries[j].ref)
		}
		return entries[i].ref < entries[j].ref
	})
	out.RefsAdvertised = len(entries)
	if len(entries) == 0 {
		out.Status = "no_advertised_refs"
		return out
	}
	count := len(entries)
	if count > limit {
		count = limit
	}
	out.RefsSelected = count
	out.RefsSkipped = len(entries) - count
	args := []string{"fetch", "--no-tags", "origin"}
	for _, e := range entries[:count] {
		args = append(args, "+"+e.ref+":"+e.ref)
	}
	if _, err := repo.RunContext(ctx, args...); err != nil {
		out.Error = "could not fetch all selected PR refs; verification is incomplete"
		out.Status = "partial"
		return out
	}
	selected := make([]clean.Ref, 0, count)
	for _, e := range entries[:count] {
		selected = append(selected, clean.Ref{Name: e.ref, SHA: e.sha, Type: "commit"})
	}
	scan, err := clean.ScanSelectedPullRefsContext(ctx, repo, matcher, selected)
	if err != nil {
		out.Error = "could not scan fetched PR commits"
		out.Status = "partial"
		return out
	}
	matchingHeaders, err := clean.ScanMatchingHeadersForRefsContext(ctx, repo, matcher, selected)
	if err != nil {
		out.Error = "could not scan PR author/committer fields"
		out.Status = "partial"
		return out
	}
	out.MatchedCommits = scan.MatchedCommits
	out.MatchingTrailers = len(scan.Matches)
	out.MatchingAuthors = matchingHeaders.Authors
	out.MatchingCommitters = matchingHeaders.Committers
	for _, m := range scan.Matches {
		if len(out.ExampleCommits) >= 8 {
			break
		}
		out.ExampleCommits = append(out.ExampleCommits, m.Commit)
	}
	out.Status = "clean"
	if out.RefsSkipped > 0 {
		out.Status = "partial"
	}
	if out.MatchedCommits > 0 || out.MatchingAuthors+out.MatchingCommitters > 0 {
		out.Status = "matches_found"
	}
	return out
}

type contributor struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

func checkContributor(ctx context.Context, opts Options) ContributorCheck {
	out := ContributorCheck{Status: "error", GitHubUser: opts.GitHubUser}
	lookup := batch.GitHubClient{Token: opts.Token, BaseURL: opts.APIBaseURL, HTTPClient: opts.HTTPClient}
	id, err := lookup.UserID(ctx, opts.GitHubUser)
	if err != nil {
		out.Errors = "unable to resolve GitHub user: " + err.Error()
		return out
	}
	out.GitHubID = id
	var client *http.Client = opts.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	api := strings.TrimRight(opts.APIBaseURL, "/")
	if api == "" {
		api = "https://api.github.com"
	}
	for page := 1; page <= 50; page++ {
		endpoint := api + "/repos/" + opts.Repo + "/contributors?anon=true&per_page=100&page=" + strconv.Itoa(page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			out.Errors = "cannot build contributor request"
			return out
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Set("User-Agent", "ByeClaude")
		if opts.Token != "" {
			req.Header.Set("Authorization", "Bearer "+opts.Token)
		}
		resp, err := client.Do(req)
		if err != nil {
			out.Errors = "contributor API request failed"
			return out
		}
		if resp.StatusCode == http.StatusAccepted {
			_ = resp.Body.Close()
			out.Status = "pending"
			return out
		}
		if resp.StatusCode == http.StatusNoContent {
			_ = resp.Body.Close()
			out.Status = "not_listed"
			return out
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			out.Errors = fmt.Sprintf("GitHub contributor API returned HTTP %d", resp.StatusCode)
			return out
		}
		var entries []contributor
		err = json.NewDecoder(resp.Body).Decode(&entries)
		_ = resp.Body.Close()
		if err != nil {
			out.Errors = "invalid contributor API response"
			return out
		}
		out.NumberOfEntries += len(entries)
		for _, entry := range entries {
			if entry.ID > 0 && strconv.FormatInt(entry.ID, 10) == id {
				out.Status = "listed"
				return out
			}
		}
		if len(entries) < 100 {
			out.Status = "not_listed"
			return out
		}
	}
	out.Status = "partial"
	out.Errors = "contributor pagination limit reached"
	return out
}

var githubLoginRE = regexp.MustCompile("^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$")

func ValidateGitHubUser(login string) error {
	if login == "" {
		return nil
	}
	if !githubLoginRE.MatchString(login) {
		return fmt.Errorf("invalid GitHub username")
	}
	return nil
}
