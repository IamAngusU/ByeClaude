# Correcting Git author and committer identities

Git author and committer headers are part of the content-addressed commit
object. They cannot simply be deleted while preserving a normal valid Git
commit. A verified correction replaces the selected name and email, while
preserving each header's original timestamp and timezone.

By default, ByeClaude **does not change either identity**. Only use these
options when correcting genuinely misattributed metadata.

Preview the exact impacted commit DAG without changing anything:

```sh
byeclaude plan \
  --replace-author "Actual Author <author@example.com>" \
  --replace-committer "Actual Committer <committer@example.com>"
```

The two roles are independent: pass one flag if you want to correct only
author or committer. A role is changed only when the existing name **and**
email match your current ByeClaude rules.

Apply the reviewed local rewrite:

```sh
byeclaude clean \
  --replace-author "Actual Author <author@example.com>" \
  --replace-committer "Actual Committer <committer@example.com>" \
  --apply

byeclaude check --include-identities
byeclaude push --backup BACKUP_ID --verify-github
```

The replacement must contain a valid complete name/email and cannot itself
match the active rule. Invalid Git headers fail closed. Preserving timestamps
does not preserve commit hashes or cryptographic signatures: every rewritten
commit and affected descendant gets a new SHA.

GitHub's contributor associations use author email; changing only the
committer will not necessarily remove a contributor. Historical forks, old
PR refs, local backups and caches remain out of scope. For a GitHub-facing
audit see [GitHub verification](github-verification.md).

## Use your Git identity as a suggested replacement

If and **only if** the configured Git identity correctly identifies the real contributor,
`--identity-from-git` reads `git config user.name` and `git config user.email` from
this clone and replaces **matching** author and committer identities. It does not
change unrelated authors and does not read a repository owner's username.

```sh
byeclaude plan --identity-from-git
byeclaude clean --identity-from-git --apply
```

Both commands display the proposed replacement identity. If the Git config is
missing or invalid, they refuse to rewrite and explain how to fix it.
The flag is mutually exclusive with the manual replacement flags. Git configuration
is not proof of GitHub account ownership or authorship, so double-check the
attribution before applying.
