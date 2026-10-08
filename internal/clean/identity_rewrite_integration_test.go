package clean

import (
	"strings"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/preset"
)

func TestMatchingGitHeaderReplacementIntegratedWithRewrite(t *testing.T) {
	dir:=t.TempDir()
	git(t,dir,"init","-q")
	git(t,dir,"config","user.name","Claude Bot")
	git(t,dir,"config","user.email","noreply@anthropic.com")
	git(t,dir,"commit","--allow-empty","-m","First commit\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	git(t,dir,"config","user.name","Human")
	git(t,dir,"config","user.email","human@example.org")
	git(t,dir,"commit","--allow-empty","-m","Later commit")
	old:=git(t,dir,"rev-parse","HEAD")
	repo,err:=gitx.Open(dir)
	if err!=nil{t.Fatal(err)}
	a:=IdentityReplacement{Name:"Actual Author",Email:"author@example.org"}
	c:=IdentityReplacement{Name:"Actual Committer",Email:"committer@example.org"}
	opts:=IdentityRewriteOptions{Author:&a,Committer:&c}
	plan,err:=PlanWithIdentity(repo,preset.Claude(),opts)
	if err!=nil{t.Fatal(err)}
	if plan.AuthorsToReplace!=1 || plan.CommittersToReplace!=1 || plan.CommitsToRewrite!=2 {
		t.Fatalf("unexpected identity impact: %+v",plan)
	}
	if git(t,dir,"rev-parse","HEAD")!=old {t.Fatal("preview moved refs")}
	out,_,err:=RewriteWithIdentity(repo,preset.Claude(),opts)
	if err!=nil{t.Fatal(err)}
	if out.AuthorsReplaced!=1 || out.CommittersReplaced!=1 || out.CommitsRewritten!=2 || out.Backup==""{
		t.Fatalf("unexpected rewrite: %+v",out)
	}
	if git(t,dir,"rev-parse","HEAD")==old {t.Fatal("HEAD did not move after rewrite")}
	before:=git(t,dir,"log","--format=%an <%ae> | %cn <%ce> | %B","--max-count=2")
	if !strings.Contains(before,"Actual Author <author@example.org>") || !strings.Contains(before,"Actual Committer <committer@example.org>") {
		t.Fatalf("corrected identities not present: %q",before)
	}
	if strings.Contains(before,"Co-Authored-By: Claude") {t.Fatalf("trailer survived: %q",before)}
	after,err:=PlanWithIdentity(repo,preset.Claude(),opts)
	if err!=nil || after.MatchedCommits!=0 {t.Fatalf("remaining selected attribution: %+v, %v",after,err)}
	headers,err:=ScanMatchingHeadersContext(t.Context(),repo,preset.Claude(),[]string{"refs/heads","refs/tags"},true)
	if err!=nil || headers.MatchedCommits!=0 {t.Fatalf("headers still match: %+v, %v",headers,err)}
	if len(git(t,dir,"for-each-ref","refs/byeclaude/backups"))==0 {t.Fatal("backup refs missing")}
}
