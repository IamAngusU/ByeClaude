package batch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListOwnedFiltersVisibilityAndOwner(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("authorization=%q", got)
		}
		if r.URL.Path != "/user/repos" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"full_name":"alice/public","private":false,"clone_url":"https://github.com/alice/public.git","owner":{"login":"alice"}},
			{"full_name":"alice/private","private":true,"clone_url":"https://github.com/alice/private.git","owner":{"login":"alice"}},
			{"full_name":"other/shared","private":true,"clone_url":"https://github.com/other/shared.git","owner":{"login":"other"}}
		]`))
	}))
	defer server.Close()

	client := GitHubClient{BaseURL: server.URL, HTTPClient: server.Client(), Token: "secret"}
	private, err := client.ListOwned(context.Background(), "alice", "private")
	if err != nil {
		t.Fatal(err)
	}
	if len(private) != 1 || private[0].Name != "alice/private" || private[0].Visibility != "private" {
		t.Fatalf("private=%#v", private)
	}
	all, err := client.ListOwned(context.Background(), "alice", "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].Name != "alice/private" || all[1].Name != "alice/public" {
		t.Fatalf("all=%#v", all)
	}
}

func TestListOwnedPublicWorksWithoutToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("unexpected authorization header")
		}
		if !strings.HasPrefix(r.URL.Path, "/users/alice/repos") {
			t.Fatalf("path=%q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"full_name":"alice/public","private":false,"clone_url":"https://github.com/alice/public.git","owner":{"login":"alice"}}]`))
	}))
	defer server.Close()

	client := GitHubClient{BaseURL: server.URL, HTTPClient: server.Client()}
	specs, err := client.ListOwned(context.Background(), "alice", "public")
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || specs[0].Name != "alice/public" {
		t.Fatalf("specs=%#v", specs)
	}
}

func TestPrivateDiscoveryRequiresToken(t *testing.T) {
	client := GitHubClient{BaseURL: "http://127.0.0.1:1"}
	_, err := client.ListOwned(context.Background(), "alice", "private")
	if err == nil || !strings.Contains(err.Error(), "GH_TOKEN") {
		t.Fatalf("expected token error, got %v", err)
	}
}

func TestUserIDResolvesPublicLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/malay-jeavio" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"login":"malay-jeavio","id":113889733}`))
	}))
	defer server.Close()

	client := GitHubClient{BaseURL: server.URL, HTTPClient: server.Client()}
	id, err := client.UserID(context.Background(), "malay-jeavio")
	if err != nil {
		t.Fatal(err)
	}
	if id != "113889733" {
		t.Fatalf("id=%q", id)
	}
}
