# GitHub Ruleset integration

Local Git hooks are bypassable with --no-verify, and they do not run on
GitHub web/API commits. GitHub Rulesets can reject a matching push to a
protected branch server-side, rather than rewriting its metadata.

Export a **proposal** first:

```sh
byeclaude ruleset export --repo OWNER/REPO
byeclaude ruleset export --repo OWNER/REPO --rules ./rules.json
byeclaude ruleset export --repo OWNER/REPO --include-identities
```

The default exports an active branch ruleset with a Co-Authored-By message
pattern. --include-identities additionally rejects matching **author and
committer emails**. The email rule is intentionally broader than the CLI
name+email matcher. Review exported JSON before installing.

GitHub evaluates the regex on the entire message, while ByeClaude's CLI
recognizes final trailer blocks only. A server-side rule can therefore
reject a line that the CLI would intentionally leave alone.

Explicit installation needs GH_TOKEN or GITHUB_TOKEN supplied securely through the environment with repository administration access:

```sh
byeclaude ruleset install --repo OWNER/REPO --confirm
byeclaude ruleset status --repo OWNER/REPO
```

Without --confirm, install only prints the candidate. It refuses to
overwrite an existing ByeClaude-named ruleset, and reads back newly created
rulesets. The default targets all branches, not tags. An installed ruleset
cannot retroactively sanitize past commits or old PR refs.

## Local protection

```sh
byeclaude hook install            # sanitize new commit messages
byeclaude hook pre-push-install   # block matching pushed ancestry
byeclaude hook pre-push-remove
```

Pre-push checks all commits reachable from pushed commit/tag tips, checks
co-author trailers **and Git author/committer fields**, and blocks a push
without rewriting history. Delete-only pushes are permitted. ByeClaude
does not overwrite unrelated hooks or externally configured hooksPath.

## CI

The GitHub Action can optionally check author/committer headers:

```yaml
- uses: IamAngusU/ByeClaude@RELEASE_OR_COMMIT_SHA
  with:
    include-identities: 'true'
    rules-file: .byeclaude-rules.json
```

New features will be present only in a release tagged after this work,
not retroactively in v0.1.0-alpha.1. Pin the new tested version.
