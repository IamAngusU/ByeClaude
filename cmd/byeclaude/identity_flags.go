package main

import (
	"flag"

	"github.com/IamAngusU/ByeClaude/internal/clean"
)

func identityRewriteFlags(fs *flag.FlagSet) (*string,*string) {
	author:=fs.String("replace-author","","explicit correction of matching Git author to 'Name <email>'")
	committer:=fs.String("replace-committer","","explicit correction of matching Git committer to 'Name <email>'")
	return author,committer
}
func parseIdentityRewriteOptions(author,committer string) (clean.IdentityRewriteOptions,error) {
	opts:=clean.IdentityRewriteOptions{}
	if author!="" {
		identity,err:=clean.ParseIdentityReplacement(author)
		if err!=nil{return opts,err}
		opts.Author=&identity
	}
	if committer!="" {
		identity,err:=clean.ParseIdentityReplacement(committer)
		if err!=nil{return opts,err}
		opts.Committer=&identity
	}
	return opts,nil
}
