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
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		w.Header().Set("Content-Type","application/json")
		switch {
		case r.URL.Path=="/users/claude":
			_,_=fmt.Fprint(w,`{"login":"claude","id":123}`)
		case r.URL.Path=="/repos/owner/repo/contributors":
			if r.URL.Query().Get("per_page")!="100" {t.Error("missing pagination")}
			_,_=fmt.Fprint(w,`[{"login":"different-name","id":123}]`)
		default:
			http.NotFound(w,r)
		}
	}))
	defer server.Close()
	c:=checkContributor(context.Background(),Options{Repo:"owner/repo",GitHubUser:"claude",Token:"fake",APIBaseURL:server.URL,HTTPClient:server.Client()})
	if c.Status!="listed" || c.GitHubID!="123" || c.NumberOfEntries!=1{
		t.Fatalf("numeric identity check failed: %+v",c)
	}
}
func TestContributorAPIPendingRemainsUnverified(t *testing.T) {
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		if strings.HasPrefix(r.URL.Path,"/users/"){_,_=fmt.Fprint(w,`{"id":44,"login":"claude"}`);return}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	c:=checkContributor(context.Background(),Options{Repo:"owner/repo",GitHubUser:"claude",APIBaseURL:server.URL,HTTPClient:server.Client()})
	if c.Status!="pending"{t.Fatalf("expected pending, got %+v",c)}
}
