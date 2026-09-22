# Changelog

All notable changes to ByeClaude are documented here.

## Unreleased

- Add the first ByeClaude brand mark with light and dark WebP assets.
- Rebuild the README around scan, prevention, rewrite semantics, recovery, and installation.
- Add focused documentation for safety, automation, troubleshooting, and rewrite internals.
- Expand CI with Linux race tests plus native macOS and Windows test/build jobs.
- Add a reusable GitHub Action with optional remote-tracking ref coverage.

- Add SHA-256 repository and merge-topology rewrite coverage.

- Refuse rewrites when Git notes are present so notes are not silently stranded on old commit IDs.

- Refuse hook installation when `core.hooksPath` is externally configured instead of pretending the default hook will run.

- Refuse rewrites while a Git sequencer/merge/rebase-style operation is in progress.

- Add `byeclaude check` for CI-safe attribution enforcement.
- Add a reusable GitHub Action and self-guard workflow.
- Restrict scan matches to the real final trailer block, avoiding body-text false positives.
- Refuse to overwrite an existing third-party `commit-msg` hook.
- Refuse rewrites with detached HEAD, replace refs, or multiple linked worktrees.
- Add integration coverage for all local branches/tags, detached HEAD audit, shallow clones, remote push leases, and stale-remote rejection.
- Make built-in multi-ref remote rewrites atomic in addition to force-with-lease protected.

### Added

- Dry-run scanning for Claude/Anthropic `Co-Authored-By` trailers.
- Transactional local history rewriting with backup refs.
- Annotated-tag rewriting and explicit signature handling.
- Force-with-lease remote updates behind an explicit `--push` flag.
- Local `commit-msg` hook for preventing future matching trailers.
- Backup listing and local ref restore.
- Cross-platform release builds and checksum-verifying installers.
