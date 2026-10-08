package githubverify

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/gitx"
	"github.com/IamAngusU/ByeClaude/internal/preset"
)

func runGitCmd(t *testing.T,dir string,args ...string)string{
	t.Helper()
	cmd:=exec.Command("git",args...)
	cmd.Dir=dir
	out,err:=cmd.CombinedOutput()
	if err!=nil{t.Fatalf("git %s: %s (%v)",strings.Join(args," "),out,err)}
	return strings.TrimSpace(string(out))
}
func TestPullRefScanFindsResidualHistory(t *testing.T) {
	dir:=t.TempDir()
	remote:=filepath.Join(dir,"remote.git")
	work:=filepath.Join(dir,"work")
	mirror:=filepath.Join(dir,"mirror.git")
	if err:=os.MkdirAll(work,0755);err!=nil{t.Fatal(err)}
	runGitCmd(t,dir,"init","--bare","-q",remote)
	runGitCmd(t,work,"init","-q")
	runGitCmd(t,work,"config","user.name","Human")
	runGitCmd(t,work,"config","user.email","human@example.org")
	runGitCmd(t,work,"commit","--allow-empty","-m","initial\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	runGitCmd(t,work,"remote","add","origin",remote)
	runGitCmd(t,work,"push","-q","origin","HEAD:refs/pull/1/head")
	runGitCmd(t,dir,"clone","--mirror","-q",remote,mirror)
	repo,err:=gitx.Open(mirror)
	if err!=nil{t.Fatal(err)}
	result:=scanPullRefs(context.Background(),repo,preset.Claude(),200)
	if result.Status!="matches_found" || result.MatchingTrailers!=1 || result.RefsAdvertised!=1 {
		t.Fatalf("pull ref scan missing residual commit: %+v",result)
	}
}
func TestPullRefScanReportsCapInsteadOfClean(t *testing.T) {
	dir:=t.TempDir()
	remote:=filepath.Join(dir,"remote.git")
	work:=filepath.Join(dir,"work")
	mirror:=filepath.Join(dir,"mirror.git")
	if err:=os.MkdirAll(work,0755);err!=nil{t.Fatal(err)}
	runGitCmd(t,dir,"init","--bare","-q",remote)
	runGitCmd(t,work,"init","-q")
	runGitCmd(t,work,"config","user.name","Human")
	runGitCmd(t,work,"config","user.email","human@example.org")
	runGitCmd(t,work,"commit","--allow-empty","-m","clean commit")
	runGitCmd(t,work,"remote","add","origin",remote)
	runGitCmd(t,work,"push","-q","origin","HEAD:refs/pull/1/head")
	runGitCmd(t,work,"push","-q","origin","HEAD:refs/pull/2/head")
	runGitCmd(t,dir,"clone","--mirror","-q",remote,mirror)
	repo,err:=gitx.Open(mirror)
	if err!=nil{t.Fatal(err)}
	result:=scanPullRefs(context.Background(),repo,preset.Claude(),1)
	if result.Status!="partial" || result.RefsSkipped!=1 || result.RefsAdvertised!=2{
		t.Fatalf("missing partial result: %+v",result)
	}
}
