package main

import (
    "fmt"
    "flag"
    "strings"

    "github.com/IamAngusU/ByeClaude/internal/clean"
    "github.com/IamAngusU/ByeClaude/internal/gitx"
)

func identityRewriteFlags(fs *flag.FlagSet) (*string, *string, *bool) {
    author := fs.String("replace-author", "", "replace matching Git author with 'Name <email>'")
    committer := fs.String("replace-committer", "", "replace matching Git committer with 'Name <email>'")
    fromGit := fs.Bool("identity-from-git", false, "use this clone's configured user.name and user.email for matching author and committer fields (explicit opt-in)")
    return author, committer, fromGit
}

func parseIdentityRewriteOptions(author, committer string) (clean.IdentityRewriteOptions, error) {
    opts := clean.IdentityRewriteOptions{}
    if author != "" {
        identity, err := clean.ParseIdentityReplacement(author)
        if err != nil { return opts, fmt.Errorf("--replace-author: %w", err) }
        opts.Author = &identity
    }
    if committer != "" {
        identity, err := clean.ParseIdentityReplacement(committer)
        if err != nil { return opts, fmt.Errorf("--replace-committer: %w", err) }
        opts.Committer = &identity
    }
    return opts, nil
}

// ResolveIdentityRewriteOptions never assumes a repository owner authored a commit.
// Git config is only a proposed identity, and must be explicitly opted into.
// An authenticated GitHub owner is not necessarily a Git author.
func resolveIdentityRewriteOptions(repo *gitx.Repo, author, committer string, fromGit bool) (clean.IdentityRewriteOptions, error) {
    if !fromGit { return parseIdentityRewriteOptions(author, committer) }
    if strings.TrimSpace(author) != "" || strings.TrimSpace(committer) != "" {
        return clean.IdentityRewriteOptions{}, fmt.Errorf("--identity-from-git cannot be combined with --replace-author or --replace-committer; choose one method")
    }
    nameOut, nameFound, err := repo.RunOptional("config", "--get", "user.name")
    if err != nil { return clean.IdentityRewriteOptions{}, err }
    emailOut, emailFound, err := repo.RunOptional("config", "--get", "user.email")
    if err != nil { return clean.IdentityRewriteOptions{}, err }
    if !nameFound || !emailFound {
        return clean.IdentityRewriteOptions{}, fmt.Errorf("Git user.name or user.email is missing; configure both in this repository (git config user.name / user.email), or pass explicit --replace-author/--replace-committer")
    }
    identity, err := clean.ParseIdentityReplacement(strings.TrimSpace(string(nameOut))+" <"+strings.TrimSpace(string(emailOut))+">")
    if err != nil {
        return clean.IdentityRewriteOptions{}, fmt.Errorf("configured Git identity is not valid: %w", err)
    }
    return clean.IdentityRewriteOptions{Author:&identity, Committer:&identity}, nil
}

func printIdentityTargets(opts clean.IdentityRewriteOptions) {
    if opts.Author != nil {
        fmt.Printf("new author    %s <%s> (only matching existing author fields)\n", opts.Author.Name, opts.Author.Email)
    }
    if opts.Committer != nil {
        fmt.Printf("new committer %s <%s> (only matching existing committer fields)\n", opts.Committer.Name, opts.Committer.Email)
    }
    if opts.Author != nil || opts.Committer != nil {
        fmt.Println("attribution   check this identity is the correct contributor before applying")
    }
}
