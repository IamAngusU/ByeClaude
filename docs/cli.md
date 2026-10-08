# ByeClaude command reference

Alpha.4 adds local activity metrics, recoverable user PATH setup and terminal
branding to the guided menu introduced in alpha.3.

## First 60 seconds

For an interactive terminal, run `byeclaude` or `byeclaude tui --repo PATH`.
Press Enter for the guided first-use flow, or choose a numbered action.
`byeclaude guide` opens the walkthrough directly. It works in cmd, Windows PowerShell,
PowerShell 7, and ordinary Unix terminals. `--no-color` or `NO_COLOR` disables
color without hiding any explanations. Redirected input/output and CI never
start an interactive session; use explicit commands in those environments.

The menu shows the current repository, active blacklist and hook status.
**m** opens local metrics and their controls. `byeclaude metrics` works outside
a repository too; see [counting semantics and privacy](metrics.md).
`byeclaude path setup` retries optional PATH configuration without elevation;
`path status` explains the current command and `path skip` disables reminders.
Cleanup previews are read-only until you type `CLEAN`; a change to refs,
configured identity or rules during review cancels the apply. Hook installation
requires `y` or `yes`. Adding/removing blacklist rules also asks before saving;
resetting all rules requires `RESET`. Publishing remains the separate `push`
command. Ctrl+C or EOF at a prompt cancels the pending choice; an incomplete
answer without Enter is never confirmation. Previously completed choices stay
saved. To undo an already completed cleanup,
use its backup ID with `restore --backup ID --apply`.

The guide walks through identities, a read-only audit, an optional cleanup
preview and hook setup. Default answers never rewrite history or install hooks.
Invalid choices, rule labels and emails get up to three attempts. Oversized
lines are drained before another answer is read, and invalid UTF-8, terminal
controls and invisible formatting characters are rejected. A bad folder does
not replace the current repository. Empty, already clean and blocked cleanup
plans do not offer an apply confirmation.

Menu command output is bounded to 1 MiB and terminal control characters are
removed from repository-supplied metadata. Exceeding the limit stops the menu
action with a diagnostic; use the explicit CLI for large reports.

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
but clients with their own disabled/custom hook settings will not. GitHub Desktop 3.5.5+ improved hook support (check Settings/Options > Git > Hooks), but its [Windows pre-push issue](https://github.com/desktop/desktop/issues/22620) can cause failures before the hook runs. If affected, push via terminal until fixed. GitHub's website/API does not run hooks installed on a developer computer.

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
| Open the terminal menu | `byeclaude tui` |
| Start the guided walkthrough | `byeclaude guide` |
| Inspect the saved blacklist | `byeclaude blacklist list` |
| Add a declared identity to the blacklist | `byeclaude blacklist add --id helper --email helper@example.org` |
| Test a name/email against active rules | `byeclaude blacklist test --name Helper --email helper@example.org` |
| Remove one blacklist entry | `byeclaude blacklist remove --id helper` |
| Restore the built-in blacklist | `byeclaude blacklist reset` |
| Export active rules for CI or batch scans | `byeclaude blacklist export` |
| Read-only local scan | `byeclaude scan` |
| Include fetched remote-tracking refs | `byeclaude scan --include-remotes` |
| Include matching Git author/committer identities | `byeclaude scan --include-identities` |
| Preview rewrite impact | `byeclaude plan` |
| Preview author-only correction from configured Git identity | `byeclaude plan --author-from-git` |
| Apply author-only correction with opt-in | `byeclaude clean --author-from-git --apply` |
| Preview committer-only correction | `byeclaude plan --committer-from-git` |
| Preview correcting both matching Git identities using your configured Git user | `byeclaude plan --identity-from-git` |
| Rewrite matching Git author/committer fields using your configured Git user | `byeclaude clean --identity-from-git --apply` |
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
  checks newly pushed history. For brand-new branches it checks full reachable ancestry; for existing branches it excludes the previously published remote ancestry.
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

### GitHub account identity

If you correct an author header, GitHub associates the resulting commit with an
account by its **author email**, not merely by the new display name. Before
applying `--author-from-git`, check `git config user.email` and confirm that
it belongs to the account that should actually receive the authorship credit.
