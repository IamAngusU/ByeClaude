# GitHub Ruleset integration

Local Git hooks are bypassable with --no-verify, and they do not run on
GitHub web/API commits. GitHub Rulesets can prevent a matching commit from
updating protected branches server-side, rather than rewriting metadata.

**Availability:** GitHub documents commit metadata restrictions as an
additional Rulesets feature for GitHub Enterprise organizations. Standard
GitHub Free/Pro repository rulesets do not necessarily support it. GitHub
may reject an installation with HTTP 422 or a permission error. The
export remains useful as a proposal even without access to this feature.

**Reachability:** GitHub rejects the ref update, but says commit objects
from a rejected update may still be retrievable by SHA. Never claim that
server-side rulesets guarantee "never uploaded" or erase old PR history.
Only a local pre-push hook can stop a compliant client before the upload
begins, and Git hooks can be bypassed. [GitHub ruleset documentation](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets#metadata-restrictions).

Export a **proposal** first:

```sh
byeclaude ruleset export --repo OWNER/REPO
byeclaude ruleset export --repo OWNER/REPO --rules ./rules.json
byeclaude ruleset export --repo OWNER/REPO --include-identities
byeclaude ruleset export --repo OWNER/REPO --all-branches
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
rulesets. The default targets only the repository default branch to avoid disrupting feature-branch work. Use `--all-branches` only when you deliberately want to enforce across every branch. Rules do not target tags. An installed ruleset
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
