package githubpolicy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
)

func TestInstallCreatesAndReadsBackOnlyAfterExplicitCallerApproval(t *testing.T) {
	var created bool
	var policy Ruleset
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		w.Header().Set("Content-Type","application/json")
		switch r.Method+" "+r.URL.Path {
		case "GET /repos/owner/repo/rulesets":
			_,_=fmt.Fprint(w,"[]")
		case "POST /repos/owner/repo/rulesets":
			if r.Header.Get("Authorization")!="Bearer scoped-secret" {t.Errorf("authorization omitted")}
			if err:=json.NewDecoder(r.Body).Decode(&policy);err!=nil{t.Error(err)}
			created=true
			policy.ID=27
			w.WriteHeader(http.StatusCreated)
			_ =json.NewEncoder(w).Encode(policy)
		case "GET /repos/owner/repo/rulesets/27":
			_ =json.NewEncoder(w).Encode(policy)
		default:
			http.NotFound(w,r)
		}
	}))
	defer server.Close()
	want,err:=GenerateRuleset(attribution.Rule{RuleID:"ai",NameContains:[]string{"claude"},EmailDomains:[]string{"anthropic.com"}},false)
	if err!=nil{t.Fatal(err)}
	c:=Client{Token:"scoped-secret",BaseURL:server.URL,HTTPClient:server.Client()}
	got,err:=c.InstallRuleset(context.Background(),"owner/repo",want)
	if err!=nil{t.Fatal(err)}
	if !created || got.ID!=27 || len(got.Rules)!=1 || got.Enforcement!="active" {t.Fatalf("incorrect policy: %+v",got)}
}
