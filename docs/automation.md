# Hooks and GitHub Actions

ByeClaude separates prevention from remediation.

- local hook: remove the matching trailer before a new commit is created;
- CI guard: detect matching attribution in reachable fetched history;
- explicit rewrite: repair history only when a human chooses to do so.

CI never force-pushes by default.

## Local setup

```sh
byeclaude setup
byeclaude setup --apply
byeclaude doctor
```

The first command only previews changes. `--apply` installs both Git hooks
without replacing externally managed hooks or an unrelated pre-existing hook.
Setup attempts to roll back newly installed hooks if later installation fails.
On linked Git worktrees, the shared hooks directory requires the explicit
`--shared-worktrees` acknowledgement before installation or removal, even
when invoking individual `hook` commands. Git for Windows,
macOS and Linux each invoke the same native Git hooks on ordinary pushes.
GitHub Desktop and other IDEs will use the hooks when they invoke Git for the
same local clone without disabling hooks. GitHub Desktop 3.5.5 (2026) improved hook support; review **Settings/Options > Git > Hooks**. On Windows, a [known Desktop pre-push issue](https://github.com/desktop/desktop/issues/22620) can fail before ByeClaude's hook even runs. If affected, push from a normal terminal until the Desktop version is fixed. [Official hook support](https://docs.github.com/en/desktop/making-changes-in-a-branch/working-with-git-hooks-in-github-desktop). Web/API commits never invoke local hooks.

The hook stores an absolute path to the installed ByeClaude executable.
`go run` is not suitable for installation because Go removes its temporary
binary after execution. Use a persistent release binary or `go install`.

## Pre-push attribution guard

```sh
byeclaude hook pre-push-install
byeclaude hook pre-push-remove
```

The optional hook rejects a push when commit ancestry still contains matching
Co-Authored-By trailers, configured exact message markers/trailer keys, **or**
matching Git author/committer headers. It also refuses to publish local
`refs/byeclaude/*` recovery refs, including through `git push --mirror`.
It does not silently rewrite history. Git permits --no-verify, so combine this
with a [GitHub Ruleset](github-rulesets.md) for branch enforcement.

## Local `commit-msg` hook

Install:

```sh
byeclaude hook install
```

The hook calls the installed ByeClaude executable and filters the commit message file before Git finalizes the commit.

Claude Code can also stop adding its current attribution at the source. In
`~/.claude/settings.json`, use the documented `attribution` setting (or `false`
on versions that support it), for example:

```json
{
  "attribution": {
    "commit": "",
    "pr": "",
    "sessionUrl": false
  }
}
```

Use `claude doctor` to inspect the effective Claude Code installation and
settings. ByeClaude remains useful for older history and for repositories where
you do not control every contributor's local settings. See the
[Claude Code settings reference](https://code.claude.com/docs/en/settings-reference).

The installer refuses to replace an unrelated `commit-msg` hook. It also refuses when `core.hooksPath` is configured because writing to `.git/hooks` would either be ineffective or could interfere with a shared hook directory.

Remove ByeClaude's own hook:

```sh
byeclaude hook remove
```

## GitHub Actions guard

**This checks all reachable fetched history, not only the new commit or pull-request diff.** A matching credit from months ago can fail a workflow triggered by an unrelated change. Before making this a required check, run `byeclaude check --include-remotes` locally and review existing matches. Cleanup stays an explicit decision; this action never rewrites or pushes.

The action downloads a pinned, precompiled release binary and verifies its SHA-256 against hashes committed with the action. It does not install Go or compile code. Checkout full history, then use the action:

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
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          fetch-depth: 0  # Required: old history is part of the check.
          persist-credentials: false
      - uses: IamAngusU/ByeClaude@v0.1.0-alpha.8
        # Optional: with: { include-identities: 'true' } also checks authors/committers.
```

The example is ready to copy and uses the built-in Claude/Anthropic rule; no extra rules file is required. Add `rules-file` only after committing your own reviewed JSON file.

This action revision pins the **alpha.7 audit engine** ([version and six digests](../action-release/README.md)); the CLI installer supplies alpha.8. Action revisions pin an already published engine so the checksums can be reviewed before consumption. For an immutable action and digest manifest, replace the action tag with its [full commit SHA](https://docs.github.com/en/actions/reference/security/secure-use#using-third-party-actions).

The action runs the equivalent of:

```sh
byeclaude check --include-remotes
```

A pull request can be checked while `HEAD` is detached because audit mode treats the current `HEAD` as an explicit scan root.

## Why `fetch-depth: 0` matters

A default shallow checkout does not contain the repository's full history. The action rejects shallow repositories with a `fetch-depth: 0` hint. It audits reachable objects in the fetched heads, tags, HEAD and (by default) remote-tracking refs. It cannot audit unfetched objects or every historical GitHub pull-request object.

The optional scheduled run is useful for quiet branches that may contain an old matching commit but do not trigger a new push or pull request.

## Machine-readable audit

For your own CI wrapper:

```sh
byeclaude check --include-remotes --json
```

The command writes a JSON report and exits non-zero when matches exist.


### Several attribution rules

Commit a reviewed structured rules file to the repository, for example `.byeclaude-rules.json`, then pass it to the reusable action:

```yaml
- uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
  with:
    fetch-depth: 0
- uses: IamAngusU/ByeClaude@v0.1.0-alpha.8
  with:
    rules-file: .byeclaude-rules.json
```

The action accepts a repository-relative rules path inside `GITHUB_WORKSPACE`; absolute paths and `..` traversal are rejected. Boolean inputs must be `true` or `false`. Linux, macOS and Windows runners on X64 or ARM64 are supported. Downloads go into a unique temporary directory and are removed after the check; PATH and metrics are left unchanged. Self-hosted runners need Bash, Git, curl and SHA-256 tooling (Git Bash on Windows). The default remains the built-in Claude/Anthropic rule when the input is empty.

For local prevention using the same policy:

```sh
byeclaude hook install --rules ./.byeclaude-rules.json
```

The installed hook records the absolute path to that validated file. If the file later becomes missing or invalid, commits fail closed instead of silently dropping the policy.
