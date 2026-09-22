# Safety and recovery

ByeClaude changes commit IDs. Treat that as a repository coordination event, not as a formatting operation.

## Before you rewrite

A good sequence for an important repository is:

```sh
git fetch --all --tags --prune
byeclaude scan --include-remotes
git status
```

Make sure the local clone contains the branches and tags you actually intend to reason about. ByeClaude cannot scan a remote ref that has never been fetched.

If the repository has collaborators, tell them before the remote rewrite. Their local branches can still point at the old DAG afterward.

## States that block `--apply`

ByeClaude refuses a rewrite when it cannot make a narrow, predictable change.

| Blocked state | Why it is blocked |
| --- | --- |
| Shallow repository | The visible DAG may not contain all ancestors needed for a correct rewrite. |
| Dirty non-bare worktree | Ref movement could make uncommitted work harder to reason about. |
| Detached `HEAD` | There is no normal checked-out branch to realign after a destructive operation. |
| `refs/replace/*` | Object identity is being virtually substituted, so the visible DAG is not the literal stored DAG. |
| Git notes | Notes remain keyed by old object IDs unless explicitly rewritten. |
| Multiple linked worktrees | One ref movement can affect several checked-out working trees at once. |
| Merge/rebase/cherry-pick/revert/bisect/sequencer in progress | Git is already in the middle of another history operation. |

Read-only `scan` and `check` remain useful in several of these states, including detached `HEAD` in CI.

## Local backup refs

Before normal branch or tag refs move, ByeClaude copies their tips under:

```text
refs/byeclaude/backups/<UTC timestamp>-<random suffix>/heads/...
refs/byeclaude/backups/<UTC timestamp>-<random suffix>/tags/...
```

List available backups:

```sh
byeclaude backups
```

Restore one locally:

```sh
byeclaude restore --backup 20260922T183653Z --apply
```

A restore only moves local refs. It does not force-push the old history to a remote.

## Remote update model

After a local rewrite, the recommended reviewed publish path is:

```sh
byeclaude push --backup BACKUP_ID
```

The backup ID identifies both the original ref tips and the exact rewritten result recorded by that operation. ByeClaude refuses this reviewed push if a local head or tag moved after the rewrite, so later local work cannot be published accidentally through the old review decision.

The publisher inspects the selected remote and only prepares updates for branches/tags that already exist there. Each update carries the expected old object ID from before the rewrite. Git then receives one atomic push using force-with-lease semantics. A remote ref that already equals the recorded rewrite result is treated as complete, making a retry idempotent.

That gives two useful properties:

1. a local-only branch does not get published accidentally;
2. if a collaborator moved a remote ref after your local copy, the whole atomic update fails instead of overwriting the newer tip.

This is still a force push. The guard makes it narrower, not harmless. `clean --apply --push` remains available as a one-shot shortcut when you deliberately do not need an inspection gap between rewrite and publish.

## Signatures

A cryptographic signature covers the object content. If a commit or annotated tag changes, its old signature cannot still be valid.

ByeClaude therefore removes commit signature headers and `mergetag` data from rewritten commit objects. A rewritten signed annotated tag also loses its embedded signature.

If signed history matters more than removing the attribution, do not rewrite that repository.

## After a shared rewrite

Other clones should fetch before doing anything else:

```sh
git fetch --all --tags --prune
```

How each collaborator realigns local work depends on whether they have local-only commits. Avoid telling everyone to blindly reset without first checking their branches.
