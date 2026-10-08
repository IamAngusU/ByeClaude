package clean

import (
	"context"
	"fmt"
	"strings"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

type HeaderMatch struct {
	Commit string `json:"commit"`
	Role string `json:"role"`
	Name string `json:"name"`
	Email string `json:"email"`
}
type HeaderScanReport struct {
	Commits int `json:"commits_scanned"`
	MatchedCommits int `json:"matched_commits"`
	Authors int `json:"matching_authors"`
	Committers int `json:"matching_committers"`
	Examples []HeaderMatch `json:"examples,omitempty"`
}

// ScanMatchingHeadersContext checks author and committer Git object headers
// against an attribution matcher without modifying the original commits.
// Caller supplies exact ref namespaces, for example heads+tags or pull only.
func ScanMatchingHeadersContext(ctx context.Context, repo *gitx.Repo, matcher attribution.Matcher, namespaces []string, includeHEAD bool) (HeaderScanReport,error) {
	var report HeaderScanReport
	if matcher==nil{return report,fmt.Errorf("attribution matcher is required")}
	refs,err:=refsFromNamespacesContext(ctx,repo,namespaces...)
	if err!=nil{return report,err}
	if includeHEAD {
		if out,err:=repo.RunContext(ctx,"rev-parse","--verify","HEAD");err==nil {
			refs=append(refs,Ref{Name:"HEAD",SHA:strings.TrimSpace(string(out)),Type:"commit"})
		}
	}
	return ScanMatchingHeadersForRefsContext(ctx,repo,matcher,refs)
}

// ScanMatchingHeadersForRefsContext checks exactly the supplied roots, not
// unrelated PR refs that may already exist in a metadata mirror.
func ScanMatchingHeadersForRefsContext(ctx context.Context,repo *gitx.Repo,matcher attribution.Matcher,refs []Ref)(HeaderScanReport,error){
	var report HeaderScanReport
	if matcher==nil{return report,fmt.Errorf("attribution matcher is required")}
	commits,err:=commitsForRefsContext(ctx,repo,refs)
	if err!=nil{return report,err}
	raw,err:=repo.CatFileBatch(ctx,commits,"commit")
	if err!=nil{return report,err}
	report.Commits=len(commits)
	for i,sha:=range commits {
		obj,err:=parseCommit(raw[i])
		if err!=nil{return report,err}
		matched:=false
		for _,h:=range obj.Headers{
			if h.Key!="author" && h.Key!="committer"{continue}
			if len(h.Lines)!=1{return report,fmt.Errorf("malformed %s header in %s",h.Key,sha)}
			parts:=gitIdentityHeader.FindStringSubmatch(h.Lines[0])
			if len(parts)!=6{return report,fmt.Errorf("invalid %s header in %s",h.Key,sha)}
			if !matcher.Match(strings.TrimSpace(parts[2]),parts[3]){continue}
			matched=true
			if h.Key=="author"{report.Authors++}else{report.Committers++}
			if len(report.Examples)<15 {
				report.Examples=append(report.Examples,HeaderMatch{Commit:sha,Role:h.Key,Name:strings.TrimSpace(parts[2]),Email:parts[3]})
			}
		}
		if matched{report.MatchedCommits++}
	}
	return report,nil
}
