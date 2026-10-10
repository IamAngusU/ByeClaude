# GitHub Support request after an authorized rewrite

Use this only after the normal branch/tag history is clean and a later
`byeclaude verify --strict --json` still reports pull-request refs or cached
contributor data. GitHub controls those server-side surfaces; ByeClaude does
not rewrite them.

Replace every bracketed value, attach the JSON report, and send the request
through the official GitHub Support portal for the account that owns the
repository.

```text
Subject: Review retained pull-request refs and contributor data after an authorized history rewrite

Hello GitHub Support,

I administer [OWNER/REPOSITORY]. We intentionally rewrote its Git commit
messages to remove selected attribution metadata while preserving every
committed file tree. The normal branch and tag refs have been updated and a
fresh remote verification reports them clean.

Rewrite details:
- Repository: https://github.com/[OWNER]/[REPOSITORY]
- Completed at: [UTC DATE/TIME]
- Default branch old tip: [OLD SHA]
- Default branch current tip: [NEW SHA]
- Local recovery backup ID: [BACKUP ID; the refs were not pushed]

Residual server-side evidence reported by a fresh check:
- Pull refs: [LIST refs/pull/... AND SHAs, OR "none"]
- Contributor account/login: [LOGIN AND NUMERIC ACCOUNT ID, OR "not applicable"]
- First observed after rewrite: [UTC DATE/TIME]
- Still present after GitHub's documented refresh period: [YES/NO]

Could you inspect whether retained pull-request refs or contributor caches for
this repository can be refreshed or removed? I understand that forks, other
clones, comments, external references and objects GitHub must retain may remain
available. I am asking specifically about the server-managed refs and cached
repository views listed in the attached ByeClaude JSON report.

Thank you.
```

Do not include secrets, access tokens or private commit contents. Keep the
local ByeClaude backup until the remote state has been reviewed; then remove
that one backup with `byeclaude backups --prune ID --confirm`.
