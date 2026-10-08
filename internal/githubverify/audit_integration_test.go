package githubverify

import (
	"context"
	"fmt"
	"github.com/IamAngusU/ByeClaude/internal/batch"
	"github.com/IamAngusU/ByeClaude/internal/preset"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestAuditCombinesActualGitHistoryPullRefsAndAPI(t *testing.T) {
	for _, scope := range []string{"clean", "trailer", "author", "pull", "contributor", "partial"} {
		t.Run(scope, func(t *testing.T) {
			work := t.TempDir()
			remote := filepath.Join(t.TempDir(), "source.git")
			mirror := filepath.Join(t.TempDir(), "audit.git")
			runGitCmd(t, work, "init", "-q", "-b", "main")
			runGitCmd(t, work, "config", "user.name", "Human")
			runGitCmd(t, work, "config", "user.email", "human@example.org")
			message := "Initial"
			if scope == "trailer" {
				message += "\n\nCo-authored-by: Claude <noreply@anthropic.com>"
			}
			args := []string{"commit", "--allow-empty", "-m", message}
			if scope == "author" {
				args = append(args, "--author=Claude <noreply@anthropic.com>")
			}
			runGitCmd(t, work, args...)
			runGitCmd(t, work, "clone", "--bare", "-q", work, remote)
			if scope == "pull" || scope == "partial" {
				if scope == "pull" {
					runGitCmd(t, work, "commit", "--allow-empty", "-m", "PR\n\nCo-authored-by: Claude <noreply@anthropic.com>")
				}
				for _, ref := range []string{"refs/pull/1/head", "refs/pull/2/head"} {
					runGitCmd(t, work, "push", "-q", remote, "HEAD:"+ref)
				}
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/users/helper":
					fmt.Fprint(w, `{"id":123,"login":"helper"}`)
				case "/repos/owner/project/contributors":
					if scope == "contributor" {
						fmt.Fprint(w, `[{"id":123,"login":"helper"}]`)
					} else {
						fmt.Fprint(w, `[]`)
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			cleaned := false
			prepare := func(ctx context.Context, spec batch.Spec, token string, disable bool) (string, func(), error) {
				runGitCmd(t, work, "clone", "--bare", "-q", remote, mirror)
				return mirror, func() { cleaned = true }, nil
			}
			report, err := audit(context.Background(), Options{Repo: "owner/project", Matcher: preset.Claude(), MaxPullRefs: 1, GitHubUser: "helper", HTTPClient: server.Client(), APIBaseURL: server.URL}, prepare)
			if err != nil {
				t.Fatal(err)
			}
			want := "residual_evidence"
			if scope == "clean" {
				want = "clean_in_checked_scopes"
			}
			if scope == "partial" {
				want = "incomplete"
			}
			if !cleaned || report.Overall != want {
				t.Fatalf("cleanup=%v report=%+v", cleaned, report)
			}
			if scope == "author" && report.Remote.MatchingAuthors != 1 {
				t.Fatalf("author missing: %+v", report.Remote)
			}
			if scope == "pull" && report.PullRefs.MatchingTrailers != 1 {
				t.Fatalf("PR evidence missing: %+v", report.PullRefs)
			}
		})
	}
}

func TestAuditRejectsInvalidInputsBeforeNetwork(t *testing.T) {
	for _, opts := range []Options{
		{Repo: "../invalid"}, {Repo: "owner/project"},
		{Repo: "owner/project", Matcher: preset.Claude(), MaxPullRefs: 0},
		{Repo: "owner/project", Matcher: preset.Claude(), MaxPullRefs: 1, GitHubUser: "bad/user"},
	} {
		_, err := audit(context.Background(), opts, func(context.Context, batch.Spec, string, bool) (string, func(), error) {
			t.Fatal("invalid input reached network")
			return "", nil, nil
		})
		if err == nil {
			t.Fatal("accepted invalid options")
		}
	}
}
