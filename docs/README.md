# ByeClaude documentation

ByeClaude is intentionally small, but the boundary around a Git history rewrite is not. These notes explain the parts that deserve more than a one-line command example.

| Guide | Use it when… |
| --- | --- |
| [How the rewrite works](how-it-works.md) | You want to understand exactly which Git objects change and why descendant commit IDs move. |
| [Matching architecture](matching.md) | You want to see how the Claude preset is separated from the vendor-neutral rewrite engine. |
| [Batch repository audit](batch.md) | You want to audit named repositories, all public repos, private repos, or both with built-in metrics. |
| [Safety and recovery](safety.md) | You are preparing to rewrite a shared repository or need to restore a local backup. |
| [Hooks and GitHub Actions](automation.md) | You want Claude attribution blocked before merge or future commit creation. |
| [Troubleshooting](troubleshooting.md) | GitHub still shows a contributor, a push is rejected, or ByeClaude refuses to rewrite. |
| [Synthetic history benchmark](performance.md) | You want to reproduce scan/rewrite scaling on your own hardware. |
| [Fixture repository suite](fixtures.md) | You want disposable real Git repositories that exercise the product safely. |

The shortest safe workflow is still:

```sh
byeclaude scan
byeclaude clean --apply
# review the rewritten graph and note the printed backup ID
byeclaude push --backup BACKUP_ID
```

If you already ran the local rewrite, do not run `clean --apply` a second time just to push. Publish exactly that reviewed result with `byeclaude push --backup BACKUP_ID`.
