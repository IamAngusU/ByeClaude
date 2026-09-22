# How ByeClaude rewrites history

## The important Git property

A commit hash is derived from the complete commit object: tree, parents, author/committer fields, message and other headers. Removing one `Co-Authored-By` line therefore changes the commit ID.

If commit `B` points to commit `A`, and `A` changes to `A'`, then `B` must point to `A'`. That changes `B` even when `B`'s own message and file tree stay identical. The effect continues through descendants.

This is why ByeClaude rewrites the commit DAG instead of treating contributor attribution as detachable GitHub metadata.

## What is and is not scanned

Normal `scan` and `check` start from:

- `refs/heads/*`;
- `refs/tags/*`;
- the current `HEAD`.

`--include-remotes` adds fetched remote-tracking refs under `refs/remotes/*`.

The walk follows reachable commit ancestry from those roots. It does not invent access to objects that are not present locally, and it does not use dangling/unreachable objects as rewrite roots. Unfetched GitHub-internal pull-request refs and external forks are outside that scope.

Backup refs under `refs/byeclaude/backups/*` are deliberately excluded from a normal cleanliness check. They exist to keep the pre-rewrite objects recoverable locally.

## Matching model

ByeClaude does not perform a free-form text replacement.

The default matcher only removes a `Co-authored-by` entry from the final Git trailer block when both conditions hold:

1. the co-author name contains `Claude`, case-insensitively;
2. the email belongs to Anthropic, including the common `noreply@anthropic.com` form.

A prose example in the commit body remains untouched. So does a human such as `Claude Shannon <shannon@example.org>`.

## Rewrite algorithm

1. Enumerate local branch and tag refs.
2. Walk reachable commits from oldest to newest.
3. Parse each raw commit object.
4. Remove only matching Claude/Anthropic co-author trailers.
5. Replace parent IDs with already-rewritten parent IDs.
6. Write a new commit object only when the message or a parent ID changed.
7. Rewrite annotated tags whose target changed.
8. Create backup refs for the original local heads/tags.
9. Record the exact rewritten heads/tags under a private result-ref namespace.
10. Update normal branch/tag refs in one ref transaction.
11. Rescan rewritten history and require zero matches.

The tree line is never changed, so the checked-in file snapshot for each logical commit stays the same.

## Why descendants change

Consider this history:

```text
A -- B -- C -- D
     ^
     Claude trailer
```

Removing the trailer creates `B'`:

```text
A -- B' -- C' -- D'
```

`C` did not need a message edit, but it pointed at `B`. To preserve the graph it must point at `B'`, making it `C'`. The same propagates to `D`.

File trees for those logical commits remain unchanged.

## Signatures

Cryptographic signatures cover object content. A history rewrite invalidates them by definition. ByeClaude drops `gpgsig`, `gpgsig-sha256` and `mergetag` headers when a commit object must be rewritten. A rewritten signed annotated tag also loses its embedded OpenPGP, SSH, or X.509 signature.

This is visible in the command summary. If retaining signed history matters more than removing the attribution, do not rewrite that repository.

## Backups

Before ref updates, ByeClaude copies every local branch/tag tip to a private ref namespace:

```text
refs/byeclaude/backups/<UTC timestamp>-<random suffix>/heads/...
refs/byeclaude/backups/<UTC timestamp>-<random suffix>/tags/...
```

Those refs keep the original objects reachable locally. ByeClaude also records the corresponding rewritten tips under:

```text
refs/byeclaude/results/<UTC timestamp>-<random suffix>/heads/...
refs/byeclaude/results/<UTC timestamp>-<random suffix>/tags/...
```

Both private namespaces are excluded from normal scans and remote publication. The paired snapshots are what make `byeclaude push --backup ID` able to prove which reviewed rewrite it is about to publish.

## Remote update

A remote history rewrite is a coordination event. The recommended flow is `clean --apply`, review the local graph, then `push --backup ID`. Before publishing, ByeClaude verifies that the current local heads/tags still match the recorded rewrite result. It then inspects the remote and only targets branch/tag refs that already exist there, using an atomic push with explicit force-with-lease expectations based on the pre-rewrite backup refs.

A changed remote tip causes the whole push to fail instead of being overwritten, and local-only refs are not published as a side effect.

Other clones still need to re-fetch and deliberately realign their branches. Open pull requests and forks can also continue to refer to old commit IDs.
