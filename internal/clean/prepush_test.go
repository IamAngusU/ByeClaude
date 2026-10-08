package clean

import (
	"fmt"
	"strings"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/preset"
)

func TestPrePushRejectsMatchingTrailersAndAllowsDeletion(t *testing.T) {
	dir := t.TempDir()
	git(t,dir,"init","-q")
	git(t,dir,"config","user.name","Human")
	git(t,dir,"config","user.email","human@example.org")
	git(t,dir,"commit","--allow-empty","-m","Base")
	git(t,dir,"commit","--allow-empty","-m","Feature\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	sha := strings.TrimSpace(git(t,dir,"rev-parse","HEAD"))
	repo,err := gitx.Open(dir)
	if err!=nil {t.Fatal(err)}
	input := fmt.Sprintf("refs/heads/test %s refs/heads/test %s\n",sha,strings.Repeat("0",40))
	rep,err := CheckPushInput(repo, strings.NewReader(input),preset.Claude())
	if err!=nil {t.Fatal(err)}
	if rep.Trailers != 1 || rep.CommitsWithMatch != 1 {t.Fatalf("unexpected scan: %+v",rep)}
	del := fmt.Sprintf("refs/heads/test %s refs/heads/test %s\n",strings.Repeat("0",40),sha)
	rep,err=CheckPushInput(repo,strings.NewReader(del),preset.Claude())
	if err!=nil || rep.Commits != 0 {t.Fatalf("delete rejected: %+v, %v",rep,err)}
}
func TestPrePushFailsClosedForMalformedInput(t *testing.T) {
	dir := t.TempDir()
	git(t,dir,"init","-q")
	repo,err:=gitx.Open(dir)
	if err!=nil{t.Fatal(err)}
	for _,in:=range []string{"refs/heads/main bad-id refs/heads/main deadbeef", "refs/heads/main 123"} {
		if _,err:=CheckPushInput(repo,strings.NewReader(in),preset.Claude());err==nil {t.Errorf("accepted malformed update %q",in)}
	}
}

func TestPrePushOnlyNewCommitsOnExistingRemoteBranch(t *testing.T) {
	dir:=t.TempDir()
	git(t,dir,"init","-q")
	git(t,dir,"config","user.name","Human")
	git(t,dir,"config","user.email","human@example.org")
	git(t,dir,"commit","--allow-empty","-m","Already published\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	old:=strings.TrimSpace(git(t,dir,"rev-parse","HEAD"))
	git(t,dir,"commit","--allow-empty","-m","New clean commit")
	current:=strings.TrimSpace(git(t,dir,"rev-parse","HEAD"))
	repo,err:=gitx.Open(dir)
	if err!=nil {t.Fatal(err)}
	input:=fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n",current,old)
	out,err:=CheckPushInput(repo,strings.NewReader(input),preset.Claude())
	if err!=nil {t.Fatal(err)}
	if out.Commits!=1 || out.Trailers!=0 {t.Fatalf("prior remote history incorrectly blocked new push: %+v",out)}
	git(t,dir,"commit","--allow-empty","-m","Fresh bad commit\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	bad:=strings.TrimSpace(git(t,dir,"rev-parse","HEAD"))
	input=fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n",bad,current)
	out,err=CheckPushInput(repo,strings.NewReader(input),preset.Claude())
	if err!=nil {t.Fatal(err)}
	if out.CommitsWithMatch!=1 || out.Trailers!=1 {t.Fatalf("new attribution was missed: %+v",out)}
}

func TestPrePushMissingRemoteTipFailsClosed(t *testing.T) {
	dir:=t.TempDir()
	git(t,dir,"init","-q")
	git(t,dir,"config","user.name","Human")
	git(t,dir,"config","user.email","human@example.org")
	git(t,dir,"commit","--allow-empty","-m","Initial")
	repo,err:=gitx.Open(dir)
	if err!=nil {t.Fatal(err)}
	local:=strings.TrimSpace(git(t,dir,"rev-parse","HEAD"))
	remote:=strings.Repeat("a",40)
	input:=fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n",local,remote)
	_,err=CheckPushInput(repo,strings.NewReader(input),preset.Claude())
	if err==nil || !strings.Contains(err.Error(),"fetch") {t.Fatalf("unknown remote must fail closed: %v",err)}
}
