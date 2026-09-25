# ByeClaude documentation

ByeClaude is intentionally small, but the boundary around a Git history rewrite is not. These notes explain the parts that deserve more than a one-line command example.

| Guide | Use it when… |
| --- | --- |
| [How the rewrite works](how-it-works.md) | You want to understand exactly which Git objects change and why descendant commit IDs move. |
| [Matching architecture](matching.md) | You want to see how the Claude preset is separated from the vendor-neutral rewrite engine. |
| [Multiple attribution rules](rules.md) | You want Claude plus other declared co-author identities in one validated rule set. |
| [Evidence model](evidence.md) | You want to distinguish factual Git attribution from heuristic AI-involvement signals. |
| [Service / API integration](service.md) | You want to use ByeClaude JSON as a backend engine for a website or service. |
| [Public VPS demo](demo-server.md) | You want a one-binary public-repository demo with an embedded UI, bounded work and no mutation endpoints. |
| [Rewrite planning](planning.md) | You want to know how much of the DAG, refs and signatures a cleanup would affect before changing anything. |
| [GitHub identity enrichment](identity.md) | You want account-level metrics or need to trace a stale GitHub user through numeric noreply IDs and pull refs. |
| [Batch repository audit](batch.md) | You want to audit named repositories, all public repos, private repos, or both with built-in metrics. |
| [Safety and recovery](safety.md) | You are preparing to rewrite a shared repository or need to restore a local backup. |
| [Hooks and GitHub Actions](automation.md) | You want Claude attribution blocked before merge or future commit creation. |
| [Troubleshooting](troubleshooting.md) | GitHub still shows a contributor, a push is rejected, or ByeClaude refuses to rewrite. |
| [Synthetic history benchmark](performance.md) | You want to reproduce scan/rewrite scaling on your own hardware. |
| [Fixture repository suite](fixtures.md) | You want disposable real Git repositories that exercise the product safely. |
| [Release integrity](supply-chain.md) | You want to verify checksums, SBOM contents, build provenance or reproducibility claims. |

The shortest safe workflow is still:

```sh
byeclaude scan
byeclaude plan
byeclaude clean --apply
# review the rewritten graph and note the printed backup ID
byeclaude push --backup BACKUP_ID
```

If you already ran the local rewrite, do not run `clean --apply` a second time just to push. Publish exactly that reviewed result with `byeclaude push --backup BACKUP_ID`.
