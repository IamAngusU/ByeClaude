package githubverify

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestContributorAPIUsesNumericIDAndReportsStaleSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/users/claude":
			_, _ = fmt.Fprint(w, `{"login":"claude","id":123}`)
		case r.URL.Path == "/repos/owner/repo/contributors":
			if r.URL.Query().Get("per_page") != "100" {
				t.Error("missing pagination")
			}
			_, _ = fmt.Fprint(w, `[{"login":"different-name","id":123}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := checkContributor(context.Background(), Options{Repo: "owner/repo", GitHubUser: "claude", Token: "fake", APIBaseURL: server.URL, HTTPClient: server.Client()})
	if c.Status != "listed" || c.GitHubID != "123" || c.NumberOfEntries != 1 {
		t.Fatalf("numeric identity check failed: %+v", c)
	}
}
func TestContributorAPIPendingRemainsUnverified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/users/") {
			_, _ = fmt.Fprint(w, `{"id":44,"login":"claude"}`)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	c := checkContributor(context.Background(), Options{Repo: "owner/repo", GitHubUser: "claude", APIBaseURL: server.URL, HTTPClient: server.Client()})
	if c.Status != "pending" {
		t.Fatalf("expected pending, got %+v", c)
	}
}

func TestContributorAPIDistinguishesAbsenceFromFailures(t *testing.T) {
	for _, tc := range []struct {
		name         string
		code         int
		body, status string
	}{
		{"empty", http.StatusOK, `[]`, "not_listed"},
		{"no-content", http.StatusNoContent, "", "not_listed"},
		{"different-id", http.StatusOK, `[{"login":"claude","id":999}]`, "not_listed"},
		{"forbidden", http.StatusForbidden, `{}`, "error"},
		{"bad-json", http.StatusOK, `broken`, "error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/users/") {
					_, _ = fmt.Fprint(w, `{"id":44,"login":"claude"}`)
					return
				}
				w.WriteHeader(tc.code)
				if tc.code != http.StatusNoContent {
					_, _ = fmt.Fprint(w, tc.body)
				}
			}))
			defer server.Close()
			got := checkContributor(context.Background(), Options{Repo: "owner/repo", GitHubUser: "claude", APIBaseURL: server.URL, HTTPClient: server.Client()})
			if got.Status != tc.status {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestContributorPaginationFindsLaterIDAndCapsIncompleteChecks(t *testing.T) {
	for _, findLater := range []bool{true, false} {
		pages := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/users/") {
				_, _ = fmt.Fprint(w, `{"id":44,"login":"claude"}`)
				return
			}
			pages++
			if findLater && pages == 2 {
				_, _ = fmt.Fprint(w, `[{"id":44,"login":"renamed"}]`)
				return
			}
			_, _ = fmt.Fprint(w, "["+strings.Repeat(`{"id":1,"login":"other"},`, 99)+`{"id":1}]`)
		}))
		got := checkContributor(context.Background(), Options{Repo: "owner/repo", GitHubUser: "claude", APIBaseURL: server.URL, HTTPClient: server.Client()})
		server.Close()
		if findLater {
			if got.Status != "listed" || pages != 2 || got.NumberOfEntries != 101 {
				t.Fatalf("later match: %+v, pages=%d", got, pages)
			}
		} else if got.Status != "partial" || pages != 50 || got.NumberOfEntries != 5000 {
			t.Fatalf("cap: %+v, pages=%d", got, pages)
		}
	}
}
