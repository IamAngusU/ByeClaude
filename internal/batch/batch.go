package batch

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
	"github.com/IamAngusU/ByeClaude/internal/clean"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

type Spec struct {
	Name       string `json:"name"`
	Source     string `json:"source"`
	Visibility string `json:"visibility"`
	Local      bool   `json:"local"`
}

type RepoMetrics struct {
	Repository             string         `json:"repository"`
	Visibility             string         `json:"visibility"`
	Source                 string         `json:"source"`
	Commits                int            `json:"commits"`
	MatchedCommits         int            `json:"matched_commits"`
	CommitMatchPct         float64        `json:"commit_match_pct"`
	Matches                int            `json:"matches"`
	RuleMatches            map[string]int `json:"rule_matches"`
	CommitsToRewrite       int            `json:"commits_to_rewrite,omitempty"`
	DescendantCommits      int            `json:"descendant_commits_to_rewrite,omitempty"`
	ParentLinksToRewrite   int            `json:"parent_links_to_rewrite,omitempty"`
	RefsToMove             int            `json:"refs_to_move,omitempty"`
	AnnotatedTagsToRewrite int            `json:"annotated_tags_to_rewrite,omitempty"`
	SignaturesAtRisk       int            `json:"signatures_at_risk,omitempty"`
	ObjectWritesEstimate   int            `json:"object_writes_estimate,omitempty"`
	RewriteReady           *bool          `json:"rewrite_ready,omitempty"`
	RewriteBlocker         string         `json:"rewrite_blocker,omitempty"`
	PrepareMS              int64          `json:"prepare_ms"`
	ScanMS                 int64          `json:"scan_ms"`
	TotalMS                int64          `json:"total_ms"`
	Error                  string         `json:"error,omitempty"`
}

type Report struct {
	Selection                string         `json:"selection"`
	Operation                string         `json:"operation"`
	Jobs                     int            `json:"jobs"`
	Repositories             int            `json:"repositories"`
	Scanned                  int            `json:"scanned"`
	CleanRepositories        int            `json:"clean_repositories"`
	MatchedRepositories      int            `json:"matched_repositories"`
	RepositoryMatchPct       float64        `json:"repository_match_pct"`
	FailedRepositories       int            `json:"failed_repositories"`
	Commits                  int            `json:"commits"`
	MatchedCommits           int            `json:"matched_commits"`
	CommitMatchPct           float64        `json:"commit_match_pct"`
	Matches                  int            `json:"matches"`
	RuleMatches              map[string]int `json:"rule_matches"`
	CommitsToRewrite         int            `json:"commits_to_rewrite,omitempty"`
	DescendantCommits        int            `json:"descendant_commits_to_rewrite,omitempty"`
	ParentLinksToRewrite     int            `json:"parent_links_to_rewrite,omitempty"`
	RefsToMove               int            `json:"refs_to_move,omitempty"`
	AnnotatedTagsToRewrite   int            `json:"annotated_tags_to_rewrite,omitempty"`
	SignaturesAtRisk         int            `json:"signatures_at_risk,omitempty"`
	ObjectWritesEstimate     int            `json:"object_writes_estimate,omitempty"`
	RewriteReadyRepositories int            `json:"rewrite_ready_repositories,omitempty"`
	BlockedRepositories      int            `json:"blocked_repositories,omitempty"`
	PrepareMS                int64          `json:"prepare_ms_sum"`
	ScanMS                   int64          `json:"scan_ms_sum"`
	WallMS                   int64          `json:"wall_ms"`
	Results                  []RepoMetrics  `json:"results"`
}

type Options struct {
	Jobs               int
	Token              string
	Matcher            attribution.Matcher
	IncludeRemotes     bool
	Selection          string
	Plan               bool
	DisableCredentials bool
}

func DefaultJobs() int {
	jobs := runtime.NumCPU()
	if jobs < 1 {
		return 1
	}
	if jobs > 4 {
		return 4
	}
	return jobs
}

func Run(ctx context.Context, specs []Spec, opts Options) (Report, error) {
	if opts.Matcher == nil {
		return Report{}, fmt.Errorf("attribution matcher is required")
	}
	if opts.Jobs <= 0 {
		opts.Jobs = DefaultJobs()
	}
	if opts.Jobs > 32 {
		return Report{}, fmt.Errorf("jobs must be between 1 and 32")
	}

	started := time.Now()
	report := Report{
		Selection:    opts.Selection,
		Operation:    map[bool]string{true: "plan", false: "scan"}[opts.Plan],
		Jobs:         opts.Jobs,
		Repositories: len(specs),
		RuleMatches:  map[string]int{},
	}
	if len(specs) == 0 {
		report.WallMS = time.Since(started).Milliseconds()
		return report, nil
	}

	workspace, err := os.MkdirTemp("", "byeclaude-batch-")
	if err != nil {
		return Report{}, fmt.Errorf("create batch workspace: %w", err)
	}
	defer os.RemoveAll(workspace)

	type job struct {
		index int
		spec  Spec
	}
	jobs := make(chan job)
	results := make(chan RepoMetrics, len(specs))

	var wg sync.WaitGroup
	workers := opts.Jobs
	if workers > len(specs) {
		workers = len(specs)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				results <- scanOne(ctx, workspace, j.index, j.spec, opts)
			}
		}()
	}

	go func() {
		for i, spec := range specs {
			jobs <- job{index: i, spec: spec}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	for result := range results {
		report.Results = append(report.Results, result)
	}
	sort.Slice(report.Results, func(i, j int) bool {
		return report.Results[i].Repository < report.Results[j].Repository
	})

	for _, result := range report.Results {
		report.PrepareMS += result.PrepareMS
		report.ScanMS += result.ScanMS
		if result.Error != "" {
			report.FailedRepositories++
			continue
		}
		report.Scanned++
		report.Commits += result.Commits
		report.MatchedCommits += result.MatchedCommits
		report.Matches += result.Matches
		report.CommitsToRewrite += result.CommitsToRewrite
		report.DescendantCommits += result.DescendantCommits
		report.ParentLinksToRewrite += result.ParentLinksToRewrite
		report.RefsToMove += result.RefsToMove
		report.AnnotatedTagsToRewrite += result.AnnotatedTagsToRewrite
		report.SignaturesAtRisk += result.SignaturesAtRisk
		report.ObjectWritesEstimate += result.ObjectWritesEstimate
		if opts.Plan {
			if result.RewriteReady != nil && *result.RewriteReady {
				report.RewriteReadyRepositories++
			} else {
				report.BlockedRepositories++
			}
		}
		for ruleID, count := range result.RuleMatches {
			report.RuleMatches[ruleID] += count
		}
		if result.Matches == 0 {
			report.CleanRepositories++
		} else {
			report.MatchedRepositories++
		}
	}
	if report.Scanned > 0 {
		report.RepositoryMatchPct = (float64(report.MatchedRepositories) / float64(report.Scanned)) * 100
	}
	if report.Commits > 0 {
		report.CommitMatchPct = (float64(report.MatchedCommits) / float64(report.Commits)) * 100
	}
	report.WallMS = time.Since(started).Milliseconds()
	return report, nil
}

func scanOne(ctx context.Context, workspace string, index int, spec Spec, opts Options) RepoMetrics {
	started := time.Now()
	result := RepoMetrics{
		Repository: spec.Name,
		Visibility: spec.Visibility,
		Source:     spec.Source,
	}

	path := spec.Source
	if !spec.Local {
		prepareStarted := time.Now()
		dest := filepath.Join(workspace, fmt.Sprintf("%04d-%s.git", index, safeName(spec.Name)))
		if err := cloneMirror(ctx, spec.Source, dest, opts.Token, opts.DisableCredentials); err != nil {
			result.PrepareMS = time.Since(prepareStarted).Milliseconds()
			result.TotalMS = time.Since(started).Milliseconds()
			result.Error = err.Error()
			return result
		}
		result.PrepareMS = time.Since(prepareStarted).Milliseconds()
		path = dest
	}

	scanStarted := time.Now()
	repo, err := gitx.OpenContext(ctx, path)
	if err != nil {
		result.ScanMS = time.Since(scanStarted).Milliseconds()
		result.TotalMS = time.Since(started).Milliseconds()
		result.Error = err.Error()
		return result
	}
	if opts.Plan {
		plan, err := clean.PlanContext(ctx, repo, opts.Matcher)
		result.ScanMS = time.Since(scanStarted).Milliseconds()
		result.TotalMS = time.Since(started).Milliseconds()
		if err != nil {
			result.Error = err.Error()
			return result
		}
		result.Commits = plan.Commits
		result.MatchedCommits = plan.MatchedCommits
		result.CommitMatchPct = plan.CommitMatchPct
		result.Matches = len(plan.Matches)
		result.RuleMatches = plan.RuleMatches
		result.CommitsToRewrite = plan.CommitsToRewrite
		result.DescendantCommits = plan.DescendantCommits
		result.ParentLinksToRewrite = plan.ParentLinksToRewrite
		result.RefsToMove = plan.RefsToMove
		result.AnnotatedTagsToRewrite = plan.AnnotatedTagsToRewrite
		result.SignaturesAtRisk = plan.SignaturesAtRisk
		result.ObjectWritesEstimate = plan.ObjectWritesEstimate
		ready := plan.RewriteReady
		result.RewriteReady = &ready
		result.RewriteBlocker = plan.RewriteBlocker
		return result
	}

	scan, err := clean.ScanIncludingRemotesContext(ctx, repo, opts.IncludeRemotes, opts.Matcher)
	result.ScanMS = time.Since(scanStarted).Milliseconds()
	result.TotalMS = time.Since(started).Milliseconds()
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Commits = scan.Commits
	result.MatchedCommits = scan.MatchedCommits
	result.CommitMatchPct = scan.CommitMatchPct
	result.Matches = len(scan.Matches)
	result.RuleMatches = scan.RuleMatches
	return result
}

func cloneMirror(ctx context.Context, source, dest, token string, disableCredentials bool) error {
	args := []string{"clone", "--mirror", "--filter=blob:none", "--quiet", source, dest}
	if disableCredentials {
		args = append([]string{"-c", "credential.helper="}, args...)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = cloneGitEnvironment(os.Environ(), source, token, disableCredentials)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("clone %s: %w", source, err)
		}
		return fmt.Errorf("clone %s: %w: %s", source, err, msg)
	}
	return nil
}

func cloneGitEnvironment(env []string, source, token string, disableCredentials bool) []string {
	if disableCredentials {
		// Public demo mode is a hard credential boundary. Even if a future caller
		// accidentally supplies a token, do not forward it into Git.
		return isolatedPublicGitEnvironment(env)
	}
	out := append([]string(nil), env...)
	if token != "" && isGitHubHTTPS(source) {
		out = append(out,
			"GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=http.https://github.com/.extraheader",
			"GIT_CONFIG_VALUE_0=Authorization: basic "+base64.StdEncoding.EncodeToString([]byte("x-access-token:"+token)),
			"GIT_TERMINAL_PROMPT=0",
		)
	}
	return out
}

func isolatedPublicGitEnvironment(env []string) []string {
	blockedExact := map[string]bool{
		"GIT_CONFIG_COUNT":                  true,
		"GIT_CONFIG_GLOBAL":                 true,
		"GIT_CONFIG_SYSTEM":                 true,
		"GIT_CONFIG_NOSYSTEM":               true,
		"GIT_DIR":                           true,
		"GIT_WORK_TREE":                     true,
		"GIT_INDEX_FILE":                    true,
		"GIT_OBJECT_DIRECTORY":              true,
		"GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
		"GIT_COMMON_DIR":                    true,
	}
	out := make([]string, 0, len(env)+3)
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if blockedExact[key] || strings.HasPrefix(key, "GIT_CONFIG_KEY_") || strings.HasPrefix(key, "GIT_CONFIG_VALUE_") {
			continue
		}
		out = append(out, entry)
	}
	return append(out,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0",
	)
}

func isGitHubHTTPS(source string) bool {
	s := strings.ToLower(strings.TrimSpace(source))
	return strings.HasPrefix(s, "https://github.com/")
}

func safeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "repo"
	}
	return b.String()
}

func JSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

// PrepareRepository resolves a Spec into a local repository path. Remote specs
// are cloned into an isolated temporary mirror and returned with a cleanup
// function. Callers should always defer cleanup.
func PrepareRepository(ctx context.Context, spec Spec, token string, disableCredentials bool) (string, func(), error) {
	if spec.Local {
		return spec.Source, func() {}, nil
	}
	workspace, err := os.MkdirTemp("", "byeclaude-repo-")
	if err != nil {
		return "", func() {}, fmt.Errorf("create repository workspace: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(workspace) }
	dest := filepath.Join(workspace, safeName(spec.Name)+".git")
	if err := cloneMirror(ctx, spec.Source, dest, token, disableCredentials); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return dest, cleanup, nil
}
