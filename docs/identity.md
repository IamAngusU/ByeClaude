# GitHub identity enrichment

ByeClaude's Git engine works from Git objects. Git stores author/committer names and email addresses, not a durable GitHub account ID.

That means a hosted service should treat "GitHub user X" as an enrichment problem rather than silently equating one email string with one account.

## Two identity layers

```text
Git identity                         GitHub identity
name + email in commit object   ->   login + durable numeric/node ID
```

A single GitHub account may have commits under several Git identities:

- a private email;
- a work email;
- a GitHub noreply address;
- old usernames/names;
- commits created through another tool.

Conversely, an arbitrary Git email string is not proof that it belongs to a particular GitHub account.

## Suggested hosted flow

For an authenticated GitHub user:

1. use GitHub OAuth / GitHub App authentication server-side;
2. resolve and store the durable GitHub account ID alongside the current login;
3. query repository/commit metadata through GitHub when an account association is needed;
4. keep the raw Git author identity in the result;
5. record the method that linked that identity to the GitHub account.

A result can then distinguish:

```json
{
  "git_author": {
    "name": "Example",
    "email": "123+example@users.noreply.github.com"
  },
  "github_identity": {
    "login": "example",
    "id": 123,
    "resolution": "github_commit_association"
  }
}
```

## Co-authors are separate

A normal commit author association and a `Co-Authored-By` trailer are different pieces of evidence.

ByeClaude already parses declared co-authors from Git trailers. A GitHub-specific service may additionally enrich those identities with GitHub account data where GitHub exposes a reliable association.

Do not infer a GitHub account purely from a similar display name.

## User-level AI attribution metric

Once commit-author identity resolution is explicit, the service can calculate a useful metric such as:

> 47 of the 312 reachable commits associated with GitHub account X also carry declared co-author attribution matching the selected AI/tool rules.

That is more defensible than saying "15% of user X's code is AI", because the denominator is commits and the evidence source remains visible.


## Residual GitHub identity audit

GitHub's modern noreply commit address can include a durable numeric account ID:

```text
ID+USERNAME@users.noreply.github.com
```

That numeric ID matters more than the visible username fragment when GitHub associates the commit with an account. A historical commit can therefore display one name while still resolving to an unexpected GitHub user because its embedded numeric ID belongs to that account.

ByeClaude can inspect those identities without changing history:

```sh
byeclaude identity --repo owner/repository
```

That lists distinct Git author identities and extracts numeric GitHub IDs where the modern noreply format is present.

Search for a known GitHub account directly by login:

```sh
byeclaude identity \
  --repo owner/repository \
  --github-user suspicious-user
```

Or use a numeric ID when it is already known:

```sh
byeclaude identity \
  --repo owner/repository \
  --github-id 123456789
```

By default, identity audit includes fetched remote refs and `refs/pull/*` when present. JSON output distinguishes:

- `managed_ref: true` when the commit is still reachable through a normal branch or tag;
- `pull_ref_only: true` when the evidence is reachable only through GitHub-style pull-request refs;
- the exact refs that keep the commit reachable.

The identity command is **read-only**. It does not rewrite normal commit authors or attempt to mutate GitHub-managed pull refs.

### Why pull refs matter

A repository can be perfectly clean on all branches and tags while an old object remains reachable through:

```text
refs/pull/1/head
refs/pull/2/head
...
```

Those refs are created and managed by GitHub. They are intentionally outside ByeClaude's normal rewrite/push scope. The audit exposes them so a stale GitHub contributor/account association is not mistaken for a failed normal-history rewrite.
