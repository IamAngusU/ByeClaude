#!/usr/bin/env bash
set -euo pipefail

fail() { printf 'ByeClaude: %s\n' "$1" >&2; exit 1; }
action_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
version=$(cat "$action_root/action-release/version")
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?$ ]] || fail 'Invalid pinned binary version in the action.'

case "${RUNNER_OS:-}" in
  Linux) os=linux; extension= ;;
  macOS) os=darwin; extension= ;;
  Windows) os=windows; extension=.exe ;;
  *) fail 'Supported runners: Linux, macOS and Windows.' ;;
esac
case "${RUNNER_ARCH:-}" in
  X64) arch=amd64 ;;
  ARM64) arch=arm64 ;;
  *) fail 'Supported runner architectures: X64 and ARM64.' ;;
esac
asset="byeclaude_${os}_${arch}${extension}"

args=()
case "${BYECLAUDE_INCLUDE_REMOTES:-true}" in
  true) args+=(--include-remotes) ;; false) ;; *) fail 'include-remotes must be true or false.' ;;
esac
case "${BYECLAUDE_INCLUDE_IDENTITIES:-false}" in
  true) args+=(--include-identities) ;; false) ;; *) fail 'include-identities must be true or false.' ;;
esac
workspace=${GITHUB_WORKSPACE:?GITHUB_WORKSPACE is required}
if [ -n "${BYECLAUDE_RULES_FILE:-}" ]; then
  case "$BYECLAUDE_RULES_FILE" in
    /*|\\*|[A-Za-z]:*) fail 'rules-file must be relative to the checked-out repository.' ;;
  esac
  case "/${BYECLAUDE_RULES_FILE//\\//}/" in */../*) fail 'rules-file must stay inside the checked-out repository.' ;; esac
  args+=(--rules "$workspace/$BYECLAUDE_RULES_FILE")
fi
git -C "$workspace" rev-parse --git-dir >/dev/null || fail 'Check out the repository before running this action.'
shallow=$(git -C "$workspace" rev-parse --is-shallow-repository)
[ "$shallow" = false ] || fail 'Full history is required. Set actions/checkout fetch-depth: 0. Older attribution can fail the guard.'

# The digest is part of the action revision, not fetched beside the binary.
# Do not source this file or allow consumer inputs to select another checksum.
expected=$(awk -v f="$asset" '$2 == f {n++; hash=$1} END {if (n == 1) print hash; else exit 1}' "$action_root/action-release/SHA256SUMS.txt") || fail 'Missing or duplicate pinned checksum.'
[[ "$expected" =~ ^[0-9a-f]{64}$ ]] || fail 'Invalid pinned checksum.'
temporary_root=${RUNNER_TEMP:?RUNNER_TEMP is required}
if [ "$os" = windows ]; then temporary_root=$(cygpath -u "$temporary_root"); fi
temporary_root=$(CDPATH= cd -- "$temporary_root" && pwd)
temporary=$(mktemp -d "$temporary_root/byeclaude-action.XXXXXXXX")
cleanup() {
  case "$temporary" in "$temporary_root"/byeclaude-action.*) rm -rf -- "$temporary" ;; *) printf 'Refusing unexpected temporary path.\n' >&2 ;; esac
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
binary="$temporary/$asset"
curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' --tlsv1.2 \
  --retry 3 --connect-timeout 20 --max-time 120 \
  "https://github.com/IamAngusU/ByeClaude/releases/download/$version/$asset" -o "$binary"
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$binary"); actual=${actual%% *}
else
  actual=$(shasum -a 256 "$binary"); actual=${actual%% *}
fi
[ "$actual" = "$expected" ] || fail 'Release binary checksum mismatch; the downloaded file was not executed.'
chmod 0755 "$binary"
export BYECLAUDE_METRICS=off
[ "$("$binary" version)" = "byeclaude $version" ] || fail 'Release binary version mismatch.'
printf 'ByeClaude %s: verified release binary; no Go setup or build.\n' "$version"
printf 'Checking all reachable fetched history. Older attribution can block this workflow.\n'
"$binary" check "${args[@]}" --repo "$workspace"
