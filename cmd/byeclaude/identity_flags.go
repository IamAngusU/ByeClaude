package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/IamAngusU/ByeClaude/internal/clean"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

type identitySelection struct {
	author *string
	committer *string
	bothFromGit *bool
	authorFromGit *bool
	committerFromGit *bool
}

func identityRewriteFlags(fs *flag.FlagSet) identitySelection {
	return identitySelection{
		author: fs.String("replace-author", "", "explicit replacement 'Name <email>' for a matching Git author"),
		committer: fs.String("replace-committer", "", "explicit replacement 'Name <email>' for a matching Git committer"),
		bothFromGit: fs.Bool("identity-from-git", false, "use configured Git identity for both matching author and committer fields"),
		authorFromGit: fs.Bool("author-from-git", false, "use configured Git identity for matching author fields only"),
		committerFromGit: fs.Bool("committer-from-git", false, "use configured Git identity for matching committer fields only"),
	}
}

func parseIdentityRewriteOptions(author, committer string) (clean.IdentityRewriteOptions, error) {
	opts := clean.IdentityRewriteOptions{}
	if author != "" {
		v, err := clean.ParseIdentityReplacement(author)
		if err != nil { return opts, fmt.Errorf("--replace-author: %w", err) }
		opts.Author = &v
	}
	if committer != "" {
		v, err := clean.ParseIdentityReplacement(committer)
		if err != nil { return opts, fmt.Errorf("--replace-committer: %w", err) }
		opts.Committer = &v
	}
	return opts, nil
}

func configuredGitIdentity(repo *gitx.Repo) (clean.IdentityReplacement, error) {
	nameOut, nameFound, err := repo.RunOptional("config", "--get", "user.name")
	if err != nil { return clean.IdentityReplacement{}, err }
	emailOut, emailFound, err := repo.RunOptional("config", "--get", "user.email")
	if err != nil { return clean.IdentityReplacement{}, err }
	if !nameFound || !emailFound {
		return clean.IdentityReplacement{}, fmt.Errorf("missing Git user.name or user.email; configure both or use an explicit replacement")
	}
	identity, err := clean.ParseIdentityReplacement(strings.TrimSpace(string(nameOut)) + " <" + strings.TrimSpace(string(emailOut)) + ">")
	if err != nil { return clean.IdentityReplacement{}, fmt.Errorf("configured Git identity is invalid: %w", err) }
	return identity, nil
}

func resolveIdentityFlags(repo *gitx.Repo, flags identitySelection) (clean.IdentityRewriteOptions, error) {
	author, committer := *flags.author, *flags.committer
	both := *flags.bothFromGit
	onlyAuthor, onlyCommitter := *flags.authorFromGit, *flags.committerFromGit
	if both && (onlyAuthor || onlyCommitter || author != "" || committer != "") {
		return clean.IdentityRewriteOptions{}, fmt.Errorf("--identity-from-git cannot be combined with other identity replacement options")
	}
	if onlyAuthor && author != "" {
		return clean.IdentityRewriteOptions{}, fmt.Errorf("--author-from-git conflicts with --replace-author")
	}
	if onlyCommitter && committer != "" {
		return clean.IdentityRewriteOptions{}, fmt.Errorf("--committer-from-git conflicts with --replace-committer")
	}
	opts, err := parseIdentityRewriteOptions(author, committer)
	if err != nil { return opts, err }
	if !both && !onlyAuthor && !onlyCommitter { return opts, nil }
	identity, err := configuredGitIdentity(repo)
	if err != nil { return opts, err }
	if both || onlyAuthor { opts.Author = &identity }
	if both || onlyCommitter { opts.Committer = &identity }
	return opts, nil
}

// Retained for compatible internal callers and regression tests.
func resolveIdentityRewriteOptions(repo *gitx.Repo, author, committer string, fromGit bool) (clean.IdentityRewriteOptions, error) {
	a, c, all, onlyA, onlyC := author, committer, fromGit, false, false
	return resolveIdentityFlags(repo, identitySelection{
		author: &a, committer: &c, bothFromGit: &all,
		authorFromGit: &onlyA, committerFromGit: &onlyC,
	})
}

func printIdentityTargets(opts clean.IdentityRewriteOptions) {
	if opts.Author != nil {
		fmt.Printf("new author    %s <%s> (only matching existing author fields)\n", opts.Author.Name, opts.Author.Email)
	}
	if opts.Committer != nil {
		fmt.Printf("new committer %s <%s> (only matching existing committer fields)\n", opts.Committer.Name, opts.Committer.Email)
	}
	if opts.Author != nil || opts.Committer != nil {
		fmt.Println("attribution   verify that the proposed identity is the actual contributor before applying")
		if opts.Author != nil {
			fmt.Println("github        the author email must be associated with the intended GitHub account to display that account")
		}
	}
}
