# Torture benchmark v1.4

ByeClaude was tested locally against the externally supplied
`ByeClaude-Benchmark-Complete-v1.4.zip` corpus. The archive contains 16,731
synthetic commits, of which 16,501 are reachable through ordinary local
branches and tags and 230 exist only through simulated pull-request refs.

The input archive was 27,605,784 bytes with SHA-256
`d471aad27c365cf05d48cb67f215e2ac22ccdfd7e84d53836c90ce6017192ef8`.
The packaged oracle targets source commit
`dbe0720ff2a4f6546c8a6bacefe9e19c5662507f` and benchmark runner v1.4.

## What the original alpha.10 run found

The unmodified oracle run against the attested alpha.10 Windows binary found:

- 4,311/4,311 implementation-contract matches and zero ordinary control hits;
- 154 separately labelled human-name collisions for synthetic
  `Claudette Example <claudette@anthropic.com>` credits;
- `clean --apply` incorrectly returned success for a shallow clone when its
  visible tip contained no selected metadata;
- an otherwise successful full cleanup encoded an empty JSON `matches` value
  as `null`, which caused the harness to report an error when it expected an
  array;
- the full rewrite started one `git hash-object` process per changed commit.

The original full report therefore ended with 26 passes, one failure, one
harness error and one known policy risk. The full cleanup still proved that
the old implementation preserved every tree, the parent topology and all
author/committer identities, and that its backup restored the exact refs.

## Post-fix validation profile

The fixture's original oracle intentionally describes alpha.10 substring
semantics, so it counts the 154 `Claudette` cases as expected matches. For the
post-fix run, a separate copy changed only that expectation: the 154 entries
listed by `potential_person_matches` were removed from the default expected
set and their `default_positive` field was set to false. The original archive
and baseline report remained unchanged. The explicit custom-provider rules
were not narrowed and continued to exercise their declared substring policy.

The final candidate passed all 29 full-suite checks:

| Check | Result |
| --- | ---: |
| Default preset | 4,157 TP, 0 FP, 0 FN |
| Human lookalikes | 154/154 protected |
| Explicit Claude/Codex/Gemini rules | 5,049 exact expected commits |
| Protected ordinary controls | 4,512 untouched |
| Full clean and exact restore | PASS |
| Dirty, detached, notes, replace refs, linked worktrees | blocked |
| Shallow repository with no visible match | blocked |
| Concurrent remote drift and stale upstream | blocked |
| Recovery refs through mirror push | blocked |
| SHA-256 repository rewrite | PASS |
| Guarded reviewed publish | PASS |

The destructive preservation case removed 4,634 metadata elements while
rewriting 16,501 commits. It then verified the full tree/parent graph and all
identities, scanned the result as clean, and restored the exact original refs.

## Measurements on this Windows host

These are warm-cache local measurements, not cross-machine performance
claims. The supplied runner could not record RSS because its optional
`psutil` dependency was absent; manual observation during the baseline stayed
around 25–27 MiB.

| Operation | alpha.10 | final candidate |
| --- | ---: | ---: |
| Default scan median | 0.891 s | 0.995 s |
| Default scan throughput | 18,512.7 commits/s | 16,582.3 commits/s |
| Full clean command sample | 456.4 s | 31.3 s |
| Full clean + proofs + scan + exact restore | harness stopped at 468.6 s | 48.4 s, PASS |

The rewrite speedup comes from staging rewritten commit bodies, calculating
their SHA-1 or SHA-256 Git object IDs in process, and asking one
`git hash-object --stdin-paths` process to write the batch. ByeClaude compares
every returned Git hash with its calculated ID before it updates any ref.

The same candidate passed `go vet ./...`, `go test ./...`, `go test -race
./...`, and 74.5% statement coverage locally. The benchmark is synthetic and
measures declared Git metadata; it does not detect AI-authored code.
