# Changelog

All notable changes to ByeClaude are documented here.

## Unreleased

### Product

- Add dry-run scanning for Claude/Anthropic `Co-Authored-By` trailers across reachable local history.
- Add `byeclaude check` for CI-safe attribution enforcement, including optional fetched remote-tracking refs.
- Add transactional local history rewriting with backup refs, annotated-tag retargeting, and explicit signature handling.
- Add guarded remote updates behind `--push` using atomic force-with-lease semantics.
- Add `push --backup ID` for review-then-publish workflows, bound to the exact recorded rewrite result.
- Use collision-resistant, ref-safe backup IDs instead of second-resolution identifiers.
- Separate the vendor-neutral Git rewrite engine from the Claude/Anthropic identity preset, with custom-matcher unit and integration coverage.
- Add a reproducible synthetic-history benchmark fixture and document a dated 5,000-commit development sanity measurement.
- Add read-only multi-repository batch audit with explicit repo selection, public/private/all GitHub discovery, bounded concurrency, partial mirror clones, default timing metrics, and JSON output.
- Add structured multi-rule attribution files for scanning, batch auditing, reviewed rewrites and local hook enforcement while keeping Claude/Anthropic as the default rule.
- Add matched-commit percentages, repository-match percentages, declared co-author identity and per-rule counts to JSON reports.
- Add read-only rewrite planning for single repositories and batch targets, including descendant propagation, parent-link reconnections, affected refs/tags, signature risk and estimated object writes.
- Add GitHub identity-enrichment guidance so account-level metrics keep Git author strings separate from durable GitHub identities.
- Add an embedded read-only demo server/UI for public GitHub repositories with strict owner/name validation, bounded in-flight work, per-audit timeouts and no mutation endpoint.
- Add demo Docker/Caddy deployment examples and disable Git credential helpers inside public demo clone jobs.
- Add evidence/API documentation that keeps declared Git metadata separate from heuristic AI-involvement signals.
- Add a six-repository disposable fixture suite plus automated batch smoke verification and a synthetic batch benchmark.
- Report scan and rewrite duration in normal single-repository output and JSON reports.
- Add a conservative local `commit-msg` hook, backup listing, and local ref restore.

### Safety and correctness

- Restrict matches to the real final trailer block so body-text examples are not rewritten.
- Preserve non-Claude co-authors, commit trees, author/committer identity and timestamps, and merge parent order.
- Refuse unsafe rewrite states including shallow clones, dirty worktrees, detached `HEAD`, replace refs, Git notes, multiple linked worktrees, and active Git sequencer operations.
- Refuse to overwrite an unrelated `commit-msg` hook or silently ignore an external `core.hooksPath`.
- Avoid publishing local-only refs during built-in remote rewrites and reject concurrent remote movement.
- Cover SHA-1 and SHA-256 repositories, merge DAGs, annotated tags, restore, remote lease races, and atomic multi-ref pushes in integration tests.

### Project

- Add the first ByeClaude brand mark with light and dark WebP assets.
- Rebuild the README around scan, prevention, rewrite semantics, recovery, and installation.
- Add focused documentation for safety, automation, troubleshooting, and rewrite internals.
- Add a reusable GitHub Action and a self-guard workflow.
- Expand CI with Linux race tests plus native macOS and Windows test/build jobs.
- Add checksum-verifying installers and cross-platform release builds for Linux, macOS, and Windows on amd64 and arm64.
