# Troubleshooting

## GitHub still shows Claude after the rewrite

First verify the local Git history rather than the GitHub UI:

```sh
byeclaude check --include-remotes
```

If that passes, inspect whether the rewritten default branch and relevant tags were actually pushed. GitHub contributor/statistics views can lag behind rewritten history, and old commit IDs can remain referenced by pull requests, forks, caches or other clones.

ByeClaude does not remove those external references.

## `push --backup ID` says the remote moved

That is the intended force-with-lease protection.

Fetch and inspect what changed before attempting another rewrite or push:

```sh
git fetch origin --prune --tags
git log --oneline --decorate --graph --all --max-count=40
```

Do not replace the guarded push with an unconditional `git push --force` just to make the error disappear.

If ByeClaude says a **local** ref moved since the recorded rewrite, that is a different guard: you changed local history after reviewing it. Re-run the audit and make a fresh rewrite decision instead of publishing an older snapshot implicitly.

## ByeClaude refuses because the repository is shallow

Fetch enough history to make the clone complete. On GitHub Actions, use:

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0
```

For an existing clone, the exact Git command depends on how it was cloned. A typical GitHub clone can be unshallowed with `git fetch --unshallow --tags`.

## Hook installation refuses an existing hook

ByeClaude does not chain or overwrite an unknown `commit-msg` hook automatically.

That is deliberate because hook ordering and failure semantics are repository-specific. Integrate ByeClaude into your existing hook manager explicitly, or keep the CI guard as the enforcement layer.

## `core.hooksPath` is configured

ByeClaude refuses to pretend `.git/hooks/commit-msg` will run when Git is configured to use a different hook directory.

Inspect it with:

```sh
git config --show-origin --get core.hooksPath
```

If that path is managed by another tool, integrate ByeClaude there deliberately rather than having the CLI mutate it automatically.

## A signed commit or tag is no longer signed

Expected when that object was rewritten. The original signature covered the old object bytes and cannot remain valid on a new object ID.

See [Safety and recovery](safety.md#signatures).
