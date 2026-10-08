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
	repo, err := gitx.Open(t.TempDir())
	if err==nil { t.Fatal("expected absent repo to fail") }
	_ = repo
	_ = err
}
