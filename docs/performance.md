# Synthetic history benchmark

ByeClaude includes a reproducible plumbing-level benchmark fixture for checking how scan and rewrite behavior scale with a longer Git history.

It deliberately uses `git commit-tree` and an unchanged empty file tree so the measurement emphasizes commit traversal and rewrite overhead rather than filesystem checkout work.

## Run it

From the repository root:

```sh
scripts/benchmark-history.sh
```

Defaults:

- 2,000 linear commits;
- one Claude/Anthropic co-author trailer at commit 400;
- the matching commit and every descendant must be rewritten.

Choose a larger fixture:

```sh
BYECLAUDE_BENCH_COMMITS=5000 \
BYECLAUDE_BENCH_MATCH_AT=1000 \
time scripts/benchmark-history.sh
```

The script builds the current checkout, creates an isolated temporary repository, runs `scan`, performs a local `clean --apply`, and finishes with `check`. It does not contact a remote.

## Development sanity measurement

A development run on **23 September 2026** used:

- Linux x86-64;
- Git 2.47.3;
- Go 1.23.2;
- a shared container exposing 5 vCPUs on an AMD EPYC 9V74 host;
- 5,000 linear commits;
- one match at commit 1,000, causing 4,001 commits to be rewritten.

Observed wall time was approximately:

| Operation | Wall time | Peak RSS |
| --- | ---: | ---: |
| Scan 5,000 commits | 5.14 s | ~9 MiB |
| Rewrite 4,001 commits | 20.19 s | ~9 MiB |
| Verify 5,000 commits | 5.45 s | ~9 MiB |

These numbers are a development sanity check, not an SLA or cross-platform comparison. Commit creation time was excluded. The fixture has one linear branch, identical trees, no network, no remote push, no large tag set, and no real-world storage contention.

Use the script on your own repositories' target hardware if performance matters to a rollout.
