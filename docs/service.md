# Service / API integration

ByeClaude is a CLI today, but its JSON output is deliberately suitable as a backend engine for a later web service.

A website can accept a repository selection, run a read-only audit in an isolated worker, and store the JSON result. The service should preserve the evidence class rather than turning Git metadata into stronger authorship claims.

## Suggested service boundary

A future service could expose something like:

```http
POST /v1/audits
```

with a request such as:

```json
{
  "repositories": ["owner/repo-a", "owner/repo-b"],
  "visibility": "explicit",
  "ruleset": "declared-ai-v1"
}
```

The worker can translate that into:

```sh
byeclaude batch scan \
  --repo owner/repo-a \
  --repo owner/repo-b \
  --rules ./declared-ai-v1.json \
  --json
```

Account-wide public audits can use `--owner OWNER --public`; authenticated private audits should use short-lived server-side credentials rather than exposing a GitHub token to browser JavaScript.

## Useful product metrics

GitHub username is not the same thing as a Git author identity. One person can commit with several names/emails, and a GitHub account association may be unavailable offline. If the website wants a statistic for "user X", treat account-to-author mapping as an explicit enrichment step and show how identities were grouped.


From declared Git attribution, a service can calculate and display:

- repositories with at least one matching declared co-author;
- percentage of successfully scanned repositories with a match;
- reachable commits scanned;
- unique commits with matching attribution;
- percentage of reachable commits with matching attribution;
- trailer counts grouped by rule/provider;
- per-repository and aggregate scan timings;
- failed/inaccessible repositories separately from clean repositories.

Do not label these as "percentage of code written by AI." They measure commits carrying evidence that matched the configured attribution rules.

## Isolation

### Do not expose arbitrary clone URLs

The CLI accepts clone URLs because that is useful on a trusted developer machine. A public web service should **not** pass an arbitrary user-supplied URL directly to Git.

For a GitHub-focused service, accept a canonical `owner/name` or a validated `https://github.com/owner/name` URL, normalize it server-side, and allow-list the destination host. Reject local paths, `file://`, arbitrary SSH hosts, loopback/link-local/private-network targets, redirects to unapproved hosts, and credentials embedded in URLs.

This keeps the CLI flexible while preventing the hosted wrapper from becoming an SSRF or internal-network reachability primitive.


A public website that scans arbitrary repositories should treat repository contents and Git metadata as untrusted input.

Recommended worker boundaries:

- temporary per-job workspace;
- no execution of repository hooks or checked-in scripts;
- no checkout required for attribution scanning;
- no arbitrary shell arguments derived from repository content;
- network credentials scoped only to repositories the user authorized;
- resource/time limits;
- delete temporary mirrors after the job unless a deliberately designed cache is used.

ByeClaude's remote batch scanner already uses temporary mirror clones and does not check out a worktree.

## Future evidence layers

A future code/style or "AI slop" detector can be added to the service as a separate analysis stage:

```text
Git declared attribution  -> factual metadata evidence
Tool metadata             -> factual when provenance is understood
Code/style heuristics     -> probabilistic signal
Text watermark detector   -> provider-specific probabilistic/cryptographic signal
```

Keep each result's provenance and confidence visible. Do not average incompatible evidence classes into a single unexplained "AI percentage."
