# Rewrite planning

`plan` is the read-only bridge between finding attribution and deciding whether a history rewrite is worth doing.

```sh
byeclaude plan
byeclaude plan --rules ./rules.json
```

For a remote repository through batch mode:

```sh
byeclaude batch plan --repo owner/repository
```

No refs or Git objects are changed.

## What is measured

A plan walks the same local heads/tags that a rewrite would use and reports:

| Metric | Meaning |
| --- | --- |
| matched commits | Unique commits whose final trailer block contains matching attribution. |
| matching trailers | Matching declared co-author entries. One commit can contain more than one. |
| commits to rewrite | Matching commits plus every reachable descendant whose parent ID would change. |
| descendant commits | Rewritten only because an ancestor changed. |
| parent links to rewrite | Parent pointers that must be reconnected to rewritten commit IDs. |
| refs to move | Local branch/tag refs whose final target changes. |
| annotated tags to rewrite | Tag objects that must be recreated because their target object changes. |
| signatures at risk | Commit signature/mergetag fields and affected signed tag objects that cannot retain their old signature. |
| object writes estimate | Approximate number of new commit + annotated-tag objects ByeClaude would write. |
| rewrite ready / blocker | Runs the normal rewrite preflight read-only and reports whether the current repository state would allow `--apply`. |

Example:

```text
repository   /work/project
commits      420
matched      3 (0.71%)
trailers     4
rewrite      87 commit(s)
descendants  84
connections  91 parent link(s)
refs         3 (2 branch(es), 1 tag ref(s))
tag objects  1 annotated tag(s)
signatures   0 at risk
objects      ~88 write(s)
ready        yes
```

## Why matches are not enough

An old match near the root of a long branch can be more expensive than ten recent matches.

```text
A -- B -- C -- D -- E
     ^
     one matching trailer
```

Removing the trailer changes `B`. That changes the parent ID stored in `C`, which changes `C`, and so on. One match can therefore require four object rewrites and several ref/tag updates.

That is why a hosted service should estimate work from the rewrite graph rather than price or schedule a job from raw match count alone.

## Capacity planning, not pricing

The current alpha exposes work metrics but does not attach money or proprietary compute-unit pricing to them.

A future service could use a transparent internal work score based on inputs such as:

```text
mirror preparation
+ commits inspected
+ objects rewritten
+ parent links reconnected
+ tag objects rewritten
+ remote publication / verification
```

Network transfer, provider latency and cache state should remain separate because they can dominate a remote audit even when the Git rewrite itself is small.

## Parallelism

Repositories are independent and batch mode scans them through a bounded worker pool.

Commits inside one rewrite DAG are different: a child's new commit ID depends on the rewritten ID of its parent. ByeClaude therefore processes commit history in topological order rather than racing dependent commits against each other.

Independent branches/topological layers could be parallelized later, but the current design prioritizes deterministic correctness. For large scan-only workloads, reducing Git process/object-I/O overhead with a persistent `git cat-file --batch` reader is likely a better optimization target before adding fine-grained commit concurrency.
