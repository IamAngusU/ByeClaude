#!/usr/bin/env bash
set -euo pipefail

action_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
temp_root=${RUNNER_TEMP:?RUNNER_TEMP is required}
if [ "${RUNNER_OS:-}" = Windows ]; then temp_root=$(cygpath -u "$temp_root"); fi
temp_root=$(CDPATH= cd -- "$temp_root" && pwd)
temporary=$(mktemp -d "$temp_root/byeclaude-history-test.XXXXXXXX")
cleanup() {
  case "$temporary" in "$temp_root"/byeclaude-history-test.*) rm -rf -- "$temporary" ;; *) exit 1 ;; esac
}
trap cleanup EXIT
repo="$temporary/history with spaces"
git init -q -b main "$repo"
git -C "$repo" config user.name 'Action test'
git -C "$repo" config user.email 'action-test@example.invalid'
git -C "$repo" config commit.gpgsign false
git -C "$repo" config core.hooksPath "$temporary/no-hooks"
git -C "$repo" commit --allow-empty -qm $'Old matching change\n\nCo-authored-by: Claude <noreply@anthropic.com>'
ancestor=$(git -C "$repo" rev-parse --short=12 HEAD)
git -C "$repo" commit --allow-empty -qm 'Unrelated clean new change'
if GITHUB_WORKSPACE="$repo" bash "$action_root/scripts/run-action.sh" > "$temporary/old.log" 2>&1; then
  echo 'Action accepted a matching ancestor behind a clean HEAD.' >&2; exit 1
fi
grep -q 'attribution guard failed' "$temporary/old.log"
grep -q "$ancestor" "$temporary/old.log"
git clone -q --depth 1 "file://$repo" "$temporary/shallow"
if GITHUB_WORKSPACE="$temporary/shallow" bash "$action_root/scripts/run-action.sh" > "$temporary/shallow.log" 2>&1; then
  echo 'Action accepted incomplete shallow history.' >&2; exit 1
fi
grep -q 'fetch-depth: 0' "$temporary/shallow.log"
echo 'Real pinned binary rejected an old matching ancestor and a shallow checkout.'
