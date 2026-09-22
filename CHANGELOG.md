# Changelog

All notable changes to ByeClaude are documented here.

## Unreleased

### Product

- Add dry-run scanning for Claude/Anthropic `Co-Authored-By` trailers across reachable local history.
- Add `byeclaude check` for CI-safe attribution enforcement, including optional fetched remote-tracking refs.
- Add transactional local history rewriting with backup refs, annotated-tag retargeting, and explicit signature handling.
- Add guarded remote updates behind `--push` using atomic force-with-lease semantics.
- Add `push --backup ID` for review-then-publish workflows, bound to the exact recorded rewrite result.
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
