# Batch repository audit

ByeClaude can audit many repositories without turning account-wide history rewriting into a default behavior.

`batch scan` and `batch check` are **read-only**. They create temporary mirror clones for remote repositories, scan the reachable Git history, report metrics, and remove the temporary workspace when finished. There is deliberately no `batch clean` command in this alpha.

## Select repositories explicitly

One repository:

```sh
byeclaude batch scan --repo IamAngusU/ContextBridge
```

Several repositories:

```sh
byeclaude batch scan \
  --repo IamAngusU/ContextBridge \
  --repo IamAngusU/ByeClaude
```

`--repo` may be repeated and accepts:

- a GitHub `OWNER/NAME` slug;
- a clone URL;
- an existing local repository path.

Public GitHub slugs need no token. Private clone access uses `GH_TOKEN` or `GITHUB_TOKEN` when available. ByeClaude does not accept a token on the command line, so the credential does not need to be placed in shell history.

## Select an account or organization

Public repositories for an owner:

```sh
byeclaude batch scan --owner IamAngusU --public
```

`--public` is the default visibility when an owner selection does not name one explicitly.

Private repositories:

```sh
GH_TOKEN=... byeclaude batch scan --owner IamAngusU --private
```

Public and private repositories:

```sh
GH_TOKEN=... byeclaude batch scan --owner IamAngusU --all
```

With a token, `--owner` may be omitted. ByeClaude resolves the authenticated GitHub login:

```sh
GH_TOKEN=... byeclaude batch scan --all
```

For organization repositories, the token must be able to see the organization repositories being requested. Discovery is filtered back to repositories actually owned by the selected owner.

## CI / guard mode

`batch scan` reports matches but exits successfully when every repository could be audited.

`batch check` uses the same scan but also exits non-zero when any matching trailer exists:

```sh
byeclaude batch check --owner IamAngusU --public
```

Both modes exit non-zero when one or more repositories could not be scanned, because a partial account audit must not be reported as complete.

## Metrics are on by default

Normal text output includes one line per repository plus an aggregate summary:

```text
selection   IamAngusU:public
repos       3
jobs        4

clean  IamAngusU/a       218 commits    0 matches  prep 1.2s  scan 180ms  total 1.4s
match  IamAngusU/b       891 commits    2 matches  prep 2.0s  scan 640ms  total 2.7s
clean  IamAngusU/c        44 commits    0 matches  prep 900ms  scan  40ms  total 940ms

summary     3 scanned · 2 clean · 1 with matches · 0 failed
history     1153 commits · 2 matching trailers
timing      2.8s wall · 4.1s prepare sum · 860ms scan sum
```

The exact numbers above are illustrative.

Metrics mean:

| Metric | Meaning |
| --- | --- |
| `prepare` | Time to create the temporary remote mirror. Local paths normally report zero. |
| `scan` | Time spent opening and auditing Git objects for that repository. |
| `total` | End-to-end time for that repository. |
| `wall` | End-to-end wall time for the whole batch. |
| `prepare sum` / `scan sum` | Sum across repositories. These can exceed wall time because scans run concurrently. |
| `commits` | Unique reachable commits audited by ByeClaude. |
| `matches` | Matching Claude/Anthropic `Co-Authored-By` trailers. |

Use `--json` for stable machine-readable fields measured in milliseconds:

```sh
byeclaude batch scan --owner IamAngusU --public --json
```

## Concurrency and network behavior

The default parallelism is the smaller of the machine's CPU count and 4. Override it with:

```sh
byeclaude batch scan --owner IamAngusU --public --jobs 8
```

The accepted range is 1–32.

Remote audits use temporary mirror clones with `--filter=blob:none`. ByeClaude needs commit/ref metadata for attribution scanning, not working-tree blobs, so this reduces unnecessary transfer on servers that support partial clone filtering.

The current alpha does not keep a persistent mirror cache. Re-running a remote batch audit can therefore transfer repository metadata again. That keeps the first implementation easy to reason about and leaves cache lifecycle/credential boundaries explicit for a later version.

## Why batch is read-only

Account-wide discovery is convenient; account-wide force-pushing is a different risk class.

ByeClaude therefore keeps remediation per repository:

```sh
byeclaude clean --repo ./reviewed-clone --apply
byeclaude push --repo ./reviewed-clone --backup BACKUP_ID
```

The local backup/result snapshots and force-with-lease checks remain tied to one repository and one reviewed rewrite.
