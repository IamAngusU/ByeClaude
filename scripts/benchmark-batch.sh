#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_root=$(CDPATH= cd -- "$script_dir/.." && pwd)

repo_count=${BYECLAUDE_BATCH_REPOS:-8}
commit_count=${BYECLAUDE_BATCH_COMMITS:-500}
match_every=${BYECLAUDE_BATCH_MATCH_EVERY:-3}
jobs=${BYECLAUDE_BATCH_JOBS:-4}

for value in "$repo_count" "$commit_count" "$match_every" "$jobs"; do
  case "$value" in ''|*[!0-9]*) echo "batch benchmark variables must be positive integers" >&2; exit 2 ;; esac
  [ "$value" -gt 0 ] || { echo "batch benchmark variables must be positive integers" >&2; exit 2; }
done

tmp=$(mktemp -d "${TMPDIR:-/tmp}/byeclaude-batch-bench.XXXXXX")
trap 'rm -rf "$tmp"' EXIT INT TERM

bin="$tmp/byeclaude"
(
  cd "$project_root"
  go build -trimpath -o "$bin" ./cmd/byeclaude
)

set -- "$bin" batch scan --jobs "$jobs"
i=1
while [ "$i" -le "$repo_count" ]; do
  repo="$tmp/repo-$i"
  mkdir -p "$repo"
  git -C "$repo" init -q
  git -C "$repo" config user.name "ByeClaude Batch Benchmark"
  git -C "$repo" config user.email "benchmark@example.invalid"
  empty_tree=$(git -C "$repo" mktree </dev/null)
  parent=
  j=1
  while [ "$j" -le "$commit_count" ]; do
    message="synthetic repo $i commit $j"
    if [ "$j" -eq 1 ] && [ $((i % match_every)) -eq 0 ]; then
      message=$(printf '%s\n\nCo-Authored-By: Claude Benchmark <noreply@anthropic.com>\n' "$message")
    fi
    if [ -z "$parent" ]; then
      parent=$(printf '%s\n' "$message" | git -C "$repo" commit-tree "$empty_tree")
    else
      parent=$(printf '%s\n' "$message" | git -C "$repo" commit-tree "$empty_tree" -p "$parent")
    fi
    j=$((j + 1))
  done
  git -C "$repo" update-ref refs/heads/main "$parent"
  git -C "$repo" symbolic-ref HEAD refs/heads/main
  set -- "$@" --repo "$repo"
  i=$((i + 1))
done

echo "Synthetic batch: $repo_count repos x $commit_count commits; jobs=$jobs; every $match_every repo(s) contains one match"
"$@"
