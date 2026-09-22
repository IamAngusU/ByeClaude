<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/brand/byeclaude-logo-dark.webp">
    <img src="assets/brand/byeclaude-logo.webp" width="220" alt="ByeClaude">
  </picture>
</p>

<h1 align="center">Clean Claude co-author attribution from Git history. Safely.</h1>

<p align="center">Scan the history you actually have. Rewrite only when you explicitly ask. Keep the trailer out with a local hook or a read-only CI guard.</p>

<p align="center">
  <a href="#30-second-flow"><img src="docs/assets/readme/badge-default.svg" height="34" alt="Read-only by default"></a>
  <a href="#safety-first"><img src="docs/assets/readme/badge-safety.svg" height="34" alt="Backup and guarded leases"></a>
  <a href="#install"><img src="docs/assets/readme/badge-platforms.svg" height="34" alt="Windows, Linux and macOS"></a>
  <a href="LICENSE"><img src="docs/assets/readme/badge-license.svg" height="34" alt="MIT license"></a>
</p>

<p align="center"><a href="#30-second-flow">Quick start</a> · <a href="#keep-it-clean">Keep it clean</a> · <a href="#what-the-rewrite-changes">Rewrite model</a> · <a href="#safety-first">Safety</a> · <a href="#install">Install</a> · <a href="docs/README.md">Docs</a></p>

Claude Code can add a `Co-Authored-By: Claude … <noreply@anthropic.com>` trailer to a commit. GitHub recognizes `Co-authored-by` trailers as additional authors, so that attribution can surface in repository history and contributor data.

**ByeClaude** is a small Git-aware CLI for removing that specific attribution without pretending a history rewrite is harmless.

| You want to… | ByeClaude does… |
| --- | --- |
| Find old Claude co-author trailers | Scans every commit reachable from local heads, tags and `HEAD`; optionally fetched remotes too. |
| Remove them | Rewrites matching commit messages and the descendants whose parent IDs must change. File trees stay untouched. |
| Keep them from coming back | Installs a conservative `commit-msg` hook and ships a read-only GitHub Actions guard. |
| Avoid clobbering shared work | Creates local backup refs, blocks unsafe repository states, and uses atomic force-with-lease for remote updates. |

## 30-second flow

Start read-only:

```sh
byeclaude scan
```

```text
repository  /work/project
commits     184
matches     2
  8a51b8b9dbd1  Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
  c0b7bd296ec4  Co-Authored-By: Claude Sonnet 4 <noreply@anthropic.com>
```

Nothing changed. Review the result, then rewrite locally:

```sh
byeclaude clean --apply
```

ByeClaude creates a local backup under `refs/byeclaude/backups/...`, rewrites the necessary commit DAG, updates local heads/tags in one ref transaction, then rescans the result.

When the rewritten history is exactly what you expect:

```sh
byeclaude clean --apply --push
```

That last command is intentionally explicit. A remote rewrite changes commit IDs for everyone using those refs.

> [!TIP]
> Already ran `clean --apply` and reviewed the result? Do not run it again just to push. Use normal Git to publish the already rewritten refs.

## It scans more than the current branch

`byeclaude scan` audits commits reachable from:

- every local branch under `refs/heads/*`;
- every local tag under `refs/tags/*`;
- the current `HEAD`, including detached `HEAD` during read-only audit.

Add fetched remote-tracking refs when you want a wider audit:

```sh
byeclaude scan --include-remotes
```

For scripts and CI, `check` performs the same audit but exits non-zero when a match exists:

```sh
byeclaude check --include-remotes
```

Dangling objects, unfetched GitHub-internal PR refs and external forks are not silently treated as rewrite targets. [Exact scope](docs/how-it-works.md#what-is-and-is-not-scanned).

## Keep it clean

### Local commits

```sh
byeclaude hook install
```

The installed `commit-msg` hook removes only matching Claude + Anthropic co-author trailers before the commit is created. Other co-authors remain intact.

ByeClaude refuses to overwrite an unrelated existing `commit-msg` hook. It also refuses installation when `core.hooksPath` points somewhere else instead of pretending a hook was installed successfully.

Remove only the hook owned by ByeClaude:

```sh
byeclaude hook remove
```

### GitHub Actions

Use the repository itself as a read-only attribution guard:

```yaml
name: ByeClaude

on:
  push:
  pull_request:

permissions:
  contents: read

jobs:
  attribution:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: IamAngusU/ByeClaude@v0.1.0
```

The action checks reachable history and fails CI when the trailer appears. It **does not** rewrite or force-push from CI.

For old quiet branches as well as active PRs, add a scheduled run. [CI and hook guide](docs/automation.md).

## What the rewrite changes

A Git commit ID covers the commit message and its parent IDs. Removing one trailer changes that commit ID. Every descendant must then point at the new parent ID, which changes those descendant IDs too.

That is why this cannot be a cosmetic GitHub toggle.

| Commit property | ByeClaude behavior |
| --- | --- |
| File tree | Preserved exactly. |
| Author / committer | Preserved. |
| Author / committer timestamps | Preserved. |
| Merge parent order | Preserved. |
| Commit message | Only matching Claude/Anthropic co-author trailers are removed. |
| Descendant IDs | Rewritten when a parent ID changed. |
| Signed rewritten objects | Signature fields are removed because the original signature cannot remain valid. |
| Annotated tags | Retargeted when needed; an embedded signature is removed if the tag object itself changes. |

The matcher is intentionally narrow. A prose mention of Claude is untouched. So is `Co-authored-by: Claude Shannon <shannon@example.org>`.

[Read the rewrite algorithm](docs/how-it-works.md).

## Safety first

History rewriting is a coordination event, so the dangerous cases fail closed.

Before `--apply`, ByeClaude blocks a rewrite when it sees a shallow repository, dirty working tree, detached `HEAD`, active replace refs, Git notes, multiple linked worktrees, or an in-progress merge/rebase/cherry-pick/revert/bisect/sequencer operation.

During a rewrite it:

1. creates local backup refs before moving normal refs;
2. writes new commit objects without touching file trees;
3. updates heads and tags in one ref transaction;
4. verifies that the matching trailers are gone;
5. leaves GitHub unchanged unless `--push` was requested.

A built-in remote update only touches refs that already exist on the selected remote. It uses an **atomic push** plus an explicit **force-with-lease** expectation for every changed remote ref. If somebody moved a branch after your local copy, Git rejects the update instead of letting ByeClaude overwrite their work.

```sh
byeclaude backups
byeclaude restore --backup 20260922T183653Z --apply
```

Restore is local only. It never republishes the old history for you.

[Safety and recovery](docs/safety.md).

## Install

### Linux / macOS

```sh
curl -fsSL https://raw.githubusercontent.com/IamAngusU/ByeClaude/main/install.sh | sh
```

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/IamAngusU/ByeClaude/main/install.ps1 | iex
```

The installers download the matching release binary and verify it against the published SHA-256 checksum file before installation.

### Go

Requires Git and Go 1.23+:

```sh
go install github.com/IamAngusU/ByeClaude/cmd/byeclaude@latest
```

### Build from source

```sh
git clone https://github.com/IamAngusU/ByeClaude.git
cd ByeClaude
go test ./...
go build -o byeclaude ./cmd/byeclaude
```

Release builds are published for Linux, macOS and Windows on amd64 and arm64.

## Command desk

<p align="center">
  <img src="docs/assets/readme/command-desk.svg" width="800" alt="ByeClaude command desk with audit, prevention, local rewrite, backup and guarded push commands.">
</p>

<details>
<summary><strong>Copy commands and recovery path</strong></summary>

```sh
byeclaude scan
byeclaude check --include-remotes
byeclaude hook install
byeclaude clean
byeclaude clean --apply
byeclaude backups
byeclaude restore --backup ID --apply
byeclaude clean --apply --push
```

| Intent | Behavior |
| --- | --- |
| `scan` | Read-only audit of reachable local history. |
| `check --include-remotes` | CI-friendly audit including fetched remote-tracking refs. |
| `hook install` | Prevent matching trailers in future local commits. |
| `clean` | Preview only; no refs move. |
| `clean --apply` | Rewrite locally after preflight checks and create backup refs. |
| `restore --backup ID --apply` | Restore local heads/tags from a ByeClaude backup. |
| `clean --apply --push` | Explicit guarded rewrite of already-existing remote refs. |

Every command accepts `--repo PATH` where applicable. GitHub stays untouched unless `--push` is present.

</details>

## Scope and limitations

ByeClaude cleans **Git commit attribution**. It does not edit pull-request text, issues, comments, external forks, GitHub caches, or repository objects you never fetched.

GitHub contributor statistics can lag behind a force-push or rewritten default branch. Old commit IDs may also remain referenced by forks, pull requests, caches or other clones even after your normal branches are clean.

This is pre-1.0 software. The repository includes integration coverage for linear and merge histories, branches, tags, backups/restores, remote lease races, atomic push behavior, shallow clones, worktrees, replace refs, notes and SHA-256 repositories. Platform CI runs the Go test suite on Linux, macOS and Windows, with the race detector additionally exercised on Linux.

## Documentation

- [How the rewrite works](docs/how-it-works.md)
- [Safety, backups and recovery](docs/safety.md)
- [Hooks and GitHub Actions](docs/automation.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Contributing](CONTRIBUTING.md)
- [Security policy](SECURITY.md)

## License

MIT. See [LICENSE](LICENSE).

<p align="center"><sub>ByeClaude is an independent open-source project and is not affiliated with Anthropic.</sub></p>
