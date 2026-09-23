#!/bin/sh
set -eu

root=${1:-}
if [ -z "$root" ]; then
  root=$(mktemp -d "${TMPDIR:-/tmp}/byeclaude-fixtures.XXXXXX")
else
  mkdir -p "$root"
  if [ -n "$(find "$root" -mindepth 1 -maxdepth 1 -print -quit)" ]; then
    echo "fixture directory must be empty: $root" >&2
    exit 2
  fi
fi
root=$(CDPATH= cd -- "$root" && pwd)

init_repo() {
  dir=$1
  mkdir -p "$dir"
  git -C "$dir" init -q
  git -C "$dir" config user.name "ByeClaude Fixture"
  git -C "$dir" config user.email "fixture@example.invalid"
}

commit_file() {
  dir=$1
  file=$2
  content=$3
  message=$4
  printf '%s\n' "$content" > "$dir/$file"
  git -C "$dir" add "$file"
  git -C "$dir" commit -q -m "$message"
}

dir="$root/clean"
init_repo "$dir"
commit_file "$dir" a.txt one "initial"
commit_file "$dir" a.txt two "clean follow-up"

dir="$root/claude-tip"
init_repo "$dir"
commit_file "$dir" a.txt one "initial"
printf 'two\n' > "$dir/a.txt"
git -C "$dir" add a.txt
git -C "$dir" commit -q -m "assistant change" -m "Co-Authored-By: Claude Opus <noreply@anthropic.com>
Co-Authored-By: Human Tester <human@example.com>"

dir="$root/claude-old"
init_repo "$dir"
commit_file "$dir" a.txt one "initial"
printf 'two\n' > "$dir/a.txt"
git -C "$dir" add a.txt
git -C "$dir" commit -q -m "old assistant change" -m "Co-Authored-By: Claude <noreply@anthropic.com>"
commit_file "$dir" a.txt three "descendant one"
commit_file "$dir" a.txt four "descendant two"

dir="$root/human-claude"
init_repo "$dir"
commit_file "$dir" a.txt one "initial"
printf 'two\n' > "$dir/a.txt"
git -C "$dir" add a.txt
git -C "$dir" commit -q -m "human coauthor" -m "Co-Authored-By: Claude Shannon <shannon@example.org>"

dir="$root/branch-and-tag"
init_repo "$dir"
commit_file "$dir" base.txt base "base"
main=$(git -C "$dir" branch --show-current)
git -C "$dir" checkout -q -b old-work
commit_file "$dir" old.txt old "old work

Co-Authored-By: Claude <noreply@anthropic.com>"
git -C "$dir" tag historical-assistant
old_tip=$(git -C "$dir" rev-parse HEAD)
git -C "$dir" checkout -q "$main"
: "$old_tip"

dir="$root/merge"
init_repo "$dir"
commit_file "$dir" base.txt base "base"
main=$(git -C "$dir" branch --show-current)
git -C "$dir" checkout -q -b feature
commit_file "$dir" feature.txt feature "feature

Co-Authored-By: Claude <noreply@anthropic.com>"
git -C "$dir" checkout -q "$main"
commit_file "$dir" main.txt main "main work"
git -C "$dir" merge -q --no-ff feature -m "merge feature"

cat <<OUT
Fixture suite created at:
  $root

Repositories:
  clean
  claude-tip
  claude-old
  human-claude
  branch-and-tag
  merge

Expected batch audit:
  6 repositories scanned
  4 repositories with matches
  4 matching trailers

Run against an installed ByeClaude:
  byeclaude batch scan \
    --repo "$root/clean" \
    --repo "$root/claude-tip" \
    --repo "$root/claude-old" \
    --repo "$root/human-claude" \
    --repo "$root/branch-and-tag" \
    --repo "$root/merge"
OUT
