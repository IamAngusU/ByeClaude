# Local metrics

`byeclaude metrics` shows work performed by this Windows/macOS/Linux user account, across its local repositories. The menu's **m** action shows the same counters and offers on/off controls, a confirmed reset and an optional manual-effort estimate. Alpha.4 starts recording new operations; it does not reconstruct earlier usage.

## Dashboard and live view

`byeclaude metrics --watch` refreshes the display once per second. In the menu, choose **m**, then **w**. Leave it open while another terminal runs scans, cleanups or Git hooks: totals update in place, and cyan `+` values show work recorded since opening the view. Press **Enter** to return; Ctrl+C exits the application. Reading metrics never adds activity.

If another process resets counters, the live baseline restarts. Unavailable storage keeps the last known totals with an explicit stale-data notice and retries. Resizing below 72 columns or 30 rows replaces the dashboard with a resize hint; input still works. Starting in a smaller terminal, a pipe, CI, `TERM=dumb`, `NO_COLOR` or `--no-color` prints a static snapshot instead. Ordinary line input remains active; no raw input mode is used.

`--details` shows processing time and counting notes separately. `--json` keeps its existing machine-readable schema and cannot be combined with `--watch`.

## What counts

| Counter | Meaning |
| --- | --- |
| History credits removed | Matching co-author lines removed by a successfully applied local cleanup. |
| Commit-message credits / hook edits | Lines actually removed from commit messages, and messages changed by the commit hook. Git may still cancel the commit afterwards. |
| Commits rewritten | All rewritten commits, including descendants whose parent IDs changed. These are not separate credit removals. |
| Identity fields corrected | Actual author/committer fields replaced by an explicitly requested cleanup. |
| Push attempts blocked / checked | Valid pre-push checks and attempts refused because they contain matching metadata. Retrying counts as another attempt, not another unique incident. |
| Scans / checks / cleanups | Completed explicit scans/checks and successfully applied cleanups. A check that finds matches still counts as a completed check. |
| Commits inspected | Sum of explicit scan/check visits. Inspecting the same history again counts again. |
| Recorded processing time | Time reported by explicit scans/checks and rewrites. It is neither total program runtime nor time saved. |

Previews, setup's internal scans, verification and no-op cleanups do not inflate work counters. A local rewrite still counts if a later remote push fails, or if you later restore the original history. The totals describe activity, not the current state of GitHub or your repositories.

## Time estimates

Actual time savings cannot be measured from these counters. By default, no saved-time figure is claimed. You can supply your own assumption:

```sh
byeclaude metrics --seconds-per-credit 30
```

The estimate is `(history credits removed + commit-message credits removed) × your seconds per credit`. It models manual editing effort only. It excludes review time, verification, Git graph complexity and automatic tool runtime; it is not a benchmark or a net productivity claim. The assumption applies to that display only and is included in JSON output.

## Privacy and control

```sh
byeclaude metrics --json
byeclaude metrics off
byeclaude metrics on
byeclaude metrics reset --confirm
```

Only a schema version, on/off preference, first-record timestamp and numeric counters are stored. No network requests, analytics SDK, account identifier, repository name/path, commit hash/content, email or command arguments are used by the metrics store. Terminal and installer text disclose local collection.

`off` preserves existing totals. `reset --confirm` clears counters while preserving the enabled/disabled setting. If damaged state requires repair, an explicit reset clears it and leaves collection disabled until `metrics on`. It never deletes Git backups, hooks or history.

Storage is `metrics.json` in:

- Windows: `%LOCALAPPDATA%\ByeClaude`
- macOS: `~/Library/Application Support/ByeClaude`
- Linux: `$XDG_CONFIG_HOME/ByeClaude`, or `~/.config/ByeClaude`

Set an absolute `BYECLAUDE_STATE_DIR` to choose a different local directory. This also holds the small `path.json` retry preference. Keep it on a local filesystem with working file locks and atomic renames. A persistent `metrics.lock` file is normal; its operating-system lock is released when a process exits or crashes.

`BYECLAUDE_METRICS=off` pauses recording for that process. CI pauses collection by default; `BYECLAUDE_METRICS=on` explicitly permits recording there, but never overrides a saved `metrics off` preference. Reading metrics does not increment counters.

Updates are bounded, locked and atomically replaced. If storage is unavailable, damaged or busy beyond a short timeout, the event is omitted and the Git operation continues. These best-effort counters are not an audit log.
