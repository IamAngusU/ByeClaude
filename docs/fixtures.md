# Fixture repository suite

ByeClaude ships a reproducible suite of real local Git repositories for manually exercising the product without touching an important repository.

Create the suite:

```sh
scripts/create-fixture-suite.sh
```

It creates six repositories covering:

| Fixture | Purpose |
| --- | --- |
| `clean` | Normal history with no matching attribution. |
| `claude-tip` | Matching trailer on the tip plus a human co-author that must survive. |
| `claude-old` | Old matching commit with descendants, forcing parent-ID propagation. |
| `human-claude` | A human named Claude with a non-Anthropic email, which must not match. |
| `branch-and-tag` | Match reachable from a non-current branch/tag. |
| `merge` | Merge DAG whose feature history contains a match. |

The script prints a ready-to-copy `byeclaude batch scan` command for the generated paths.

## Automated smoke verification

Run:

```sh
scripts/verify-fixture-suite.sh
```

That script builds the current checkout, generates the six repositories, audits them through the real batch command and verifies the expected aggregate result:

```text
6 repositories scanned
4 repositories with matches
4 matching trailers
0 failed repositories
```

The Go test suite separately covers rewrite, restore, tags, SHA-256 repositories, shallow clones, replace refs, Git notes, linked worktrees, concurrent remote movement, atomic multi-ref push behavior, custom attribution matchers and the batch engine itself.
