# ByeClaude documentation

ByeClaude is intentionally small, but the boundary around a Git history rewrite is not. These notes explain the parts that deserve more than a one-line command example.

| Guide | Use it when… |
| --- | --- |
| [How the rewrite works](how-it-works.md) | You want to understand exactly which Git objects change and why descendant commit IDs move. |
| [Safety and recovery](safety.md) | You are preparing to rewrite a shared repository or need to restore a local backup. |
| [Hooks and GitHub Actions](automation.md) | You want Claude attribution blocked before merge or future commit creation. |
| [Troubleshooting](troubleshooting.md) | GitHub still shows a contributor, a push is rejected, or ByeClaude refuses to rewrite. |

The shortest safe workflow is still:

```sh
byeclaude scan
byeclaude clean --apply
# review the rewritten graph and note the printed backup ID
byeclaude push --backup BACKUP_ID
```

If you already ran the local rewrite, do not run `clean --apply` a second time just to push. Publish the reviewed rewritten refs with normal Git.
