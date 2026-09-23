#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/byeclaude-fixture-test.XXXXXX")
trap 'rm -rf "$tmp"' EXIT INT TERM

fixtures="$tmp/fixtures"
mkdir -p "$fixtures"
"$script_dir/create-fixture-suite.sh" "$fixtures" >/dev/null

bin="$tmp/byeclaude"
(
  cd "$project_root"
  go build -trimpath -o "$bin" ./cmd/byeclaude
)

report="$tmp/report.json"
"$bin" batch scan \
  --repo "$fixtures/clean" \
  --repo "$fixtures/claude-tip" \
  --repo "$fixtures/claude-old" \
  --repo "$fixtures/human-claude" \
  --repo "$fixtures/branch-and-tag" \
  --repo "$fixtures/merge" \
  --json > "$report"

grep -q '"repositories": 6' "$report"
grep -q '"scanned": 6' "$report"
grep -q '"matched_repositories": 4' "$report"
grep -q '"matches": 4' "$report"
grep -q '"failed_repositories": 0' "$report"

cat "$report"
echo "fixture suite: PASS" >&2
