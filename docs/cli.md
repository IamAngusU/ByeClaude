# ByeClaude command reference

> **Release note:** The commands introduced by this development PR require
> building the updated source. They are not included in the existing
> `v0.1.0-alpha.1` binary. Install a tested newer release once published.

## First 60 seconds

Run these commands **inside the Git clone you intend to protect**:

```sh
byeclaude scan           # read-only: find matching co-author trailers
byeclaude plan           # read-only: inspect how much of Git history would change
byeclaude setup          # read-only: preview commit and push hook installation
byeclaude setup --apply  # install both Git hooks in this clone
byeclaude doctor         # check local protection
```

No daemon, IDE plugin or background service is necessary. Git executes the
hooks when a compatible client creates a commit or pushes the same local
repository. VS Code, JetBrains and GitHub Desktop normally use Git hooks,
but clients with their own disabled/custom hook settings will not. GitHub's
website/API does not run hooks installed on a developer computer.

## Clean existing history

Always check before publishing a rewritten graph:

```sh
byeclaude scan
byeclaude plan
byeclaude clean --apply        # local only: save the printed backup ID
git log --oneline --graph --decorate --all --max-count=40
byeclaude push                 # automatic if exactly one backup exists
byeclaude verify               # detect GitHub origin and check remote surfaces
```

If several backups exist, use `byeclaude backups`, then publish the exact
reviewed operation with `byeclaude push --backup BACKUP_ID`.
`byeclaude clean --apply` does not publish unless explicitly asked;
`byeclaude push` uses guarded remote ref expectations. Coordinate with
collaborators before any rewrite.

## Commands by purpose

| Task | Command |
| --- | --- |
| Read-only local scan | `byeclaude scan` |
| Include fetched remote-tracking refs | `byeclaude scan --include-remotes` |
| Include matching Git author/committer identities | `byeclaude scan --include-identities` |
| Preview rewrite impact | `byeclaude plan` |
| Preview correcting one misattributed Git author | `byeclaude plan --replace-author "Correct Name <correct@example.com>"` |
| Locally rewrite and back up | `byeclaude clean --apply` |
| Locally correct a matching Git author/committer | `byeclaude clean --replace-author "Correct Name <correct@example.com>" --apply` |
| List available backups | `byeclaude backups` |
| Restore one backup locally | `byeclaude restore --backup ID --apply` |
| Publish a reviewed rewrite | `byeclaude push --backup ID` |
| Verify fresh GitHub history and PR refs | `byeclaude verify` |
| Verify a specific GitHub repository | `byeclaude verify --repo OWNER/REPO` |
| Inspect a contributor ID through GitHub API | `byeclaude verify --repo OWNER/REPO --github-user LOGIN` |
| Fail an automation on incomplete/residual checks | `byeclaude verify --strict --json` |
| Install local commit and push guards | `byeclaude setup --apply` |
| Diagnose local hook installation | `byeclaude doctor` |
| Install only the commit-msg sanitizer | `byeclaude hook install` |
| Install only the pre-push blocker | `byeclaude hook pre-push-install` |
| Uninstall a ByeClaude-owned pre-push blocker | `byeclaude hook pre-push-remove` |
| Check history from CI, including identities | `byeclaude check --include-remotes --include-identities` |
| Audit all public repos owned by an account | `byeclaude batch scan --owner LOGIN --public` |
| Plan changes for one remote repo, read-only | `byeclaude batch plan --repo OWNER/REPO` |
| Use a custom set of AI/bot attribution rules | `byeclaude scan --rules ./rules.json` |
| Diagnose lingering GitHub user IDs in refs | `byeclaude identity --repo OWNER/REPO --github-user LOGIN` |
| Start the read-only public repository browser demo | `byeclaude serve` |
| Preview a GitHub server-side ruleset | `byeclaude ruleset export --repo OWNER/REPO` |
| Install a reviewed GitHub ruleset | `byeclaude ruleset install --repo OWNER/REPO --confirm` |

**Scope notes:** No regular CLI option can erase external forks, hidden
GitHub-managed historical PR refs, caches, or other users' local clones.
`verify` reports which surfaces were actually checked. GitHub metadata
rulesets may require Enterprise organization capabilities. A Ruleset blocks
branch ref updates but does not necessarily prevent upload of Git objects.
For full details see [verification](github-verification.md),
[safety](safety.md), [prevention](automation.md),
[identity correction](identity-correction.md),
and [GitHub rulesets](github-rulesets.md).

## Troubleshooting

- **Push was blocked:** `byeclaude scan --include-identities` and
  `byeclaude plan` show where the matching metadata is. The pre-push hook
  conservatively checks reachable history, not just the most recent commit.
- **Existing hooks:** ByeClaude refuses to overwrite foreign hooks or
  `core.hooksPath`. Keep your hook manager and integrate ByeClaude manually
  (see [automation](automation.md)).
- **Shared worktrees:** Their hooks directory is shared. Explicitly use
  `byeclaude setup --apply --shared-worktrees` only if all linked worktrees
  should be protected.
- **The CLI was installed through `go run`:** The executable is temporary;
  install a persistent binary before installing hooks.
- **Contributors are still visible:** Check
  `byeclaude verify --repo OWNER/REPO --github-user LOGIN`.
  Contributor data can lag, and old PR refs can remain reachable.
