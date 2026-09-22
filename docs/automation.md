# Hooks and GitHub Actions

ByeClaude separates prevention from remediation.

- local hook: remove the matching trailer before a new commit is created;
- CI guard: detect matching attribution in reachable fetched history;
- explicit rewrite: repair history only when a human chooses to do so.

CI never force-pushes by default.

## Local `commit-msg` hook

Install:

```sh
byeclaude hook install
```

The hook calls the installed ByeClaude executable and filters the commit message file before Git finalizes the commit.

The installer refuses to replace an unrelated `commit-msg` hook. It also refuses when `core.hooksPath` is configured because writing to `.git/hooks` would either be ineffective or could interfere with a shared hook directory.

Remove ByeClaude's own hook:

```sh
byeclaude hook remove
```

## GitHub Actions guard

Checkout full history, then use the action:

```yaml
name: ByeClaude

on:
  push:
  pull_request:
  workflow_dispatch:
  schedule:
    - cron: '17 3 * * *'

permissions:
  contents: read

jobs:
  attribution:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: IamAngusU/ByeClaude@main
```

Until the first tagged release exists, the example above follows `main`. After a release, pin the action to a release tag or, for the strongest supply-chain stability, an exact commit SHA.

The action runs the equivalent of:

```sh
byeclaude check --include-remotes
```

A pull request can be checked while `HEAD` is detached because audit mode treats the current `HEAD` as an explicit scan root.

## Why `fetch-depth: 0` matters

A default shallow checkout does not contain the repository's full history. ByeClaude can only audit objects that are present locally, so the guard deliberately expects a full-history checkout.

The optional scheduled run is useful for quiet branches that may contain an old matching commit but do not trigger a new push or pull request.

## Machine-readable audit

For your own CI wrapper:

```sh
byeclaude check --include-remotes --json
```

The command writes a JSON report and exits non-zero when matches exist.
