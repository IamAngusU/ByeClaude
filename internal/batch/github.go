package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type githubRepo struct {
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
	CloneURL string `json:"clone_url"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type githubUser struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
}

type GitHubClient struct {
	BaseURL    string
	HTTPClient *http.Client
	Token      string
}

func TokenFromEnv() string {
	if token := strings.TrimSpace(os.Getenv("GH_TOKEN")); token != "" {
		return token
	}
	return strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
}

func (c GitHubClient) baseURL() string {
	if strings.TrimSpace(c.BaseURL) == "" {
		return "https://api.github.com"
	}
	return strings.TrimRight(c.BaseURL, "/")
}

func (c GitHubClient) client() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c GitHubClient) CurrentLogin(ctx context.Context) (string, error) {
	if strings.TrimSpace(c.Token) == "" {
		return "", fmt.Errorf("GH_TOKEN or GITHUB_TOKEN is required when --owner is omitted")
	}
	var user githubUser
	if err := c.getJSON(ctx, c.baseURL()+"/user", &user); err != nil {
		return "", err
	}
	if strings.TrimSpace(user.Login) == "" {
		return "", fmt.Errorf("GitHub API returned an empty login")
	}
	return user.Login, nil
}

func (c GitHubClient) UserID(ctx context.Context, login string) (string, error) {
	login = strings.TrimSpace(login)
	if login == "" {
		return "", fmt.Errorf("GitHub login is required")
	}
	var user githubUser
	if err := c.getJSON(ctx, c.baseURL()+"/users/"+url.PathEscape(login), &user); err != nil {
		return "", err
	}
	if user.ID <= 0 {
		return "", fmt.Errorf("GitHub API returned an invalid account ID for %q", login)
	}
	return fmt.Sprintf("%d", user.ID), nil
}

func (c GitHubClient) ListOwned(ctx context.Context, owner, visibility string) ([]Spec, error) {
	owner = strings.TrimSpace(owner)
	visibility = strings.ToLower(strings.TrimSpace(visibility))
	if owner == "" {
		var err error
		owner, err = c.CurrentLogin(ctx)
		if err != nil {
			return nil, err
		}
	}
	if visibility == "" {
		visibility = "public"
	}
	if visibility != "public" && visibility != "private" && visibility != "all" {
		return nil, fmt.Errorf("visibility must be public, private, or all")
	}
	if visibility != "public" && strings.TrimSpace(c.Token) == "" {
		return nil, fmt.Errorf("GH_TOKEN or GITHUB_TOKEN is required for %s repository discovery", visibility)
	}

	var repos []githubRepo
	if strings.TrimSpace(c.Token) == "" {
		endpoint := fmt.Sprintf("%s/users/%s/repos?type=owner&sort=full_name&direction=asc&per_page=100", c.baseURL(), url.PathEscape(owner))
		var err error
		repos, err = c.listPages(ctx, endpoint)
		if err != nil {
			return nil, err
		}
	} else {
		endpoint := c.baseURL() + "/user/repos?affiliation=owner,organization_member&sort=full_name&direction=asc&per_page=100"
		var err error
		repos, err = c.listPages(ctx, endpoint)
		if err != nil {
			return nil, err
		}
	}

	var specs []Spec
	for _, repo := range repos {
		if !strings.EqualFold(repo.Owner.Login, owner) {
			continue
		}
		if visibility == "public" && repo.Private {
			continue
		}
		if visibility == "private" && !repo.Private {
			continue
		}
		repoVisibility := "public"
		if repo.Private {
			repoVisibility = "private"
		}
		specs = append(specs, Spec{
			Name:       repo.FullName,
			Source:     repo.CloneURL,
			Visibility: repoVisibility,
		})
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
	return specs, nil
}

func ResolveExplicit(values []string) ([]Spec, error) {
	seen := map[string]bool{}
	var specs []Spec
	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		if info, err := os.Stat(raw); err == nil {
			if !info.IsDir() {
				return nil, fmt.Errorf("local repository path is not a directory: %s", raw)
			}
			abs, err := filepath.Abs(raw)
			if err != nil {
				return nil, err
			}
			key := "local:" + abs
			if !seen[key] {
				seen[key] = true
				specs = append(specs, Spec{Name: filepath.Base(abs), Source: abs, Visibility: "local", Local: true})
			}
			continue
		}

		if strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "ssh://") || strings.HasPrefix(raw, "git@") || strings.HasPrefix(raw, "file://") {
			key := "url:" + raw
			if !seen[key] {
				seen[key] = true
				specs = append(specs, Spec{Name: repoNameFromURL(raw), Source: raw, Visibility: "unknown"})
			}
			continue
		}

		parts := strings.Split(raw, "/")
		if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
			full := strings.TrimSuffix(raw, ".git")
			key := "github:" + strings.ToLower(full)
			if !seen[key] {
				seen[key] = true
				specs = append(specs, Spec{Name: full, Source: "https://github.com/" + full + ".git", Visibility: "unknown"})
			}
			continue
		}
		return nil, fmt.Errorf("repo %q is neither an existing local path, clone URL, nor OWNER/NAME", raw)
	}
	return specs, nil
}

func repoNameFromURL(raw string) string {
	trimmed := strings.TrimSuffix(strings.TrimSuffix(raw, "/"), ".git")
	if i := strings.LastIndex(trimmed, "/"); i >= 0 && i+1 < len(trimmed) {
		return trimmed[i+1:]
	}
	if i := strings.LastIndex(trimmed, ":"); i >= 0 && i+1 < len(trimmed) {
		return trimmed[i+1:]
	}
	return trimmed
}

func (c GitHubClient) listPages(ctx context.Context, endpoint string) ([]githubRepo, error) {
	var all []githubRepo
	for page := 1; ; page++ {
		sep := "&"
		if !strings.Contains(endpoint, "?") {
			sep = "?"
		}
		var chunk []githubRepo
		if err := c.getJSON(ctx, fmt.Sprintf("%s%spage=%d", endpoint, sep, page), &chunk); err != nil {
			return nil, err
		}
		all = append(all, chunk...)
		if len(chunk) < 100 {
			break
		}
	}
	return all, nil
}

func (c GitHubClient) getJSON(ctx context.Context, endpoint string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ByeClaude")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if strings.TrimSpace(c.Token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return fmt.Errorf("GitHub API request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub API %s returned %s", endpoint, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode GitHub API response: %w", err)
	}
	return nil
}
