# GitHub verification after cleanup

A successful local rewrite says nothing about every historical copy on GitHub.
To audit GitHub using a **fresh remote clone**, run:

```sh
byeclaude verify  # detects origin from the current Git repository
byeclaude verify --repo OWNER/REPO
byeclaude verify --repo OWNER/REPO --github-user LOGIN --strict
byeclaude verify --repo OWNER/REPO --max-pull-refs 500 --json
```

Private repos need GH_TOKEN or GITHUB_TOKEN with repository read access.
The CLI also supports verification immediately after a guarded push:

```sh
byeclaude push --backup BACKUP_ID --verify-github --github-user LOGIN
byeclaude clean --apply --push --verify-github --github-user LOGIN
```

## What is verified?

1. **Managed history:** a fresh temporary Git mirror scans reachable branches
   and tags for matching co-author trailers and matching author/committer
   Git headers. This does not rely on your local clone.
2. **GitHub pull refs:** ByeClaude enumerates publicly advertised refs/pull/*
   head/merge refs, fetches up to --max-pull-refs (default: 200) and scans
   the metadata they reach. Unfetched/unadvertised refs and caches cannot be
   assumed clean. PR refs are never rewritten.
3. **Contributors:** with --github-user, the selected account is resolved
   through GitHub's API to a durable numeric ID, then the REST contributor
   listing is checked. This is a cached API snapshot of commit-author
   identities, not an exhaustive list of all co-author credits visible in UI.

## Status semantics

- remote_history.status = clean: no selected metadata matches in freshly
  fetched normal refs.
- pull_refs.status = clean: no selected metadata matches in the successfully
  scanned advertised PR refs. It is not a guarantee for all old Git objects.
- pull_refs.status = partial: a bounded subset was scanned or a fetch/scan failed.
- pull_refs.status = matches_found: a scanned PR ref reaches matching metadata.
- contributors.status = not_requested / listed / not_listed / pending /
  partial / error: these describe the contributor API, not durable removal.
- overall = residual_evidence: positive evidence remains in a checked scope.
- overall = incomplete: some required checks could not be completed.
- overall = clean_in_checked_scopes: no matches found in exactly the scopes
  listed in the report. **This does not mean all copies are gone.**

A clean branch/tag history can coexist with old pull refs, external forks,
backup refs in local clones, cached commit URLs, and cached contributor graphs.
GitHub documents that contributor displays may take **about 24 hours** to
refresh after a force-push. Use a later verification if needed.

The scan does not erase PR descriptions, comments, external forks, cached
views or unreachable objects. GitHub Support may be necessary when contributor
data remains stale after its refresh window.

See [safety](safety.md), [identity enrichment](identity.md),
[GitHub contributor guidance](https://docs.github.com/en/repositories/viewing-activity-and-data-for-your-repository/viewing-a-projects-contributors),
and [REST contributor semantics](https://docs.github.com/en/rest/repos/repos#list-repository-contributors).
