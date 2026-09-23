#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_root=$(CDPATH= cd -- "$script_dir/.." && pwd)

commits=${BYECLAUDE_BENCH_COMMITS:-2000}
match_at=${BYECLAUDE_BENCH_MATCH_AT:-400}

case "$commits" in
  ''|*[!0-9]*) echo "BYECLAUDE_BENCH_COMMITS must be a positive integer" >&2; exit 2 ;;
esac
case "$match_at" in
  ''|*[!0-9]*) echo "BYECLAUDE_BENCH_MATCH_AT must be a positive integer" >&2; exit 2 ;;
esac
if [ "$commits" -lt 1 ] || [ "$match_at" -lt 1 ] || [ "$match_at" -gt "$commits" ]; then
  echo "require 1 <= BYECLAUDE_BENCH_MATCH_AT <= BYECLAUDE_BENCH_COMMITS" >&2
  exit 2
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

bin="$tmp/byeclaude"
repo="$tmp/repo"

(
  cd "$project_root"
  go build -trimpath -o "$bin" ./cmd/byeclaude
)

mkdir -p "$repo"
cd "$repo"
git init -q
git config user.name "ByeClaude Benchmark"
git config user.email "benchmark@example.invalid"

empty_tree=$(git mktree </dev/null)
parent=
i=1
while [ "$i" -le "$commits" ]; do
  if [ "$i" -eq "$match_at" ]; then
    message=$(printf 'synthetic commit %s\n\nCo-Authored-By: Claude Benchmark <noreply@anthropic.com>\n' "$i")
  else
    message="synthetic commit $i"
  fi

  if [ -z "$parent" ]; then
    parent=$(printf '%s\n' "$message" | git commit-tree "$empty_tree")
  else
    parent=$(printf '%s\n' "$message" | git commit-tree "$empty_tree" -p "$parent")
  fi
  i=$((i + 1))
done

git update-ref refs/heads/main "$parent"
git symbolic-ref HEAD refs/heads/main

echo "Synthetic history: $commits commits; matching trailer at commit $match_at"
echo
echo "== scan =="
"$bin" scan --repo "$repo"
echo
echo "== rewrite =="
"$bin" clean --apply --repo "$repo"
echo
echo "== verify =="
"$bin" check --repo "$repo"
echo
echo "Tip: prefix this script with your platform's time command for wall/RSS measurements."
