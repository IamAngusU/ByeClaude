#!/bin/sh
set -eu

case "${1:-}" in
  '') ;; --no-start) BYECLAUDE_NO_START=1 ;; *) echo 'Usage: sh install.sh [--no-start]' >&2; exit 1 ;;
esac
[ "$#" -le 1 ] || { echo 'Usage: sh install.sh [--no-start]' >&2; exit 1; }

repo="IamAngusU/ByeClaude"
os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$os" in linux|darwin) ;; *) echo "Unsupported OS: $os" >&2; exit 1 ;; esac
case "$arch" in x86_64|amd64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) echo "Unsupported architecture: $arch" >&2; exit 1 ;; esac

asset="byeclaude_${os}_${arch}"
version=${BYECLAUDE_VERSION:-v0.1.0-alpha.6}
if [ "$version" != latest ] && ! printf '%s\n' "$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?$'; then
  echo "BYECLAUDE_VERSION must be 'latest' or a semantic v-prefixed tag" >&2
  exit 1
fi
if [ -n "${BYECLAUDE_DOWNLOAD_BASE:-}" ]; then
  base=${BYECLAUDE_DOWNLOAD_BASE%/}
  case "$base" in https://*|file://*) ;; *) echo "BYECLAUDE_DOWNLOAD_BASE must use https:// or file://" >&2; exit 1 ;; esac
else
  case "$version" in
    latest) base="https://github.com/${repo}/releases/latest/download" ;;
    *) base="https://github.com/${repo}/releases/download/${version}" ;;
  esac
fi

tmp=$(mktemp -d "${TMPDIR:-/tmp}/byeclaude-install.XXXXXX")
cleanup() {
  case "$tmp" in "${TMPDIR:-/tmp}"/byeclaude-install.*) rm -rf -- "$tmp" ;; *) echo "Refusing to remove unexpected temporary directory: $tmp" >&2 ;; esac
}
trap cleanup EXIT INT TERM

curl -fsSL "$base/$asset" -o "$tmp/byeclaude"
curl -fsSL "$base/SHA256SUMS.txt" -o "$tmp/SHA256SUMS.txt"
expected=$(awk -v f="$asset" '$2 == f || $2 == "*" f {print $1; exit}' "$tmp/SHA256SUMS.txt")
[ -n "$expected" ] || { echo "Checksum entry not found for $asset" >&2; exit 1; }
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/byeclaude" | awk '{print $1}')
else
  actual=$(shasum -a 256 "$tmp/byeclaude" | awk '{print $1}')
fi
[ "$actual" = "$expected" ] || { echo "Checksum mismatch for $asset" >&2; exit 1; }
chmod 0755 "$tmp/byeclaude"
"$tmp/byeclaude" version >/dev/null

if [ -n "${BYECLAUDE_INSTALL_DIR:-}" ]; then
  prefix=$BYECLAUDE_INSTALL_DIR
else
  prefix="$HOME/.local/bin"
fi

if ! mkdir -p "$prefix" 2>/dev/null || ! stage=$(mktemp "$prefix/.byeclaude.XXXXXX" 2>/dev/null); then
  echo "Cannot write to $prefix. Trying your user directory; no sudo is requested."
  prefix="$HOME/.local/bin"
  mkdir -p "$prefix"
  stage=$(mktemp "$prefix/.byeclaude.XXXXXX")
fi
install -m 0755 "$tmp/byeclaude" "$stage"
mv -f "$stage" "$prefix/byeclaude"

echo "Installed byeclaude to $prefix/byeclaude"
if [ "${BYECLAUDE_NO_PATH:-}" = 1 ]; then
  "$prefix/byeclaude" path skip || echo "PATH reminder preference could not be saved. The binary remains usable."
else
  "$prefix/byeclaude" path setup || echo "PATH setup can wait. Start $prefix/byeclaude directly and use path setup to retry."
fi
echo 'Local activity counters stay on this computer. Use byeclaude metrics or metrics off.'
echo 'Powered by angusu.de | Angus Uelsmann'
case "${CI:-}" in ''|false|0) interactive_ci=false ;; *) interactive_ci=true ;; esac
if [ "${BYECLAUDE_NO_START:-}" != 1 ] && [ "$interactive_ci" = false ] && [ -t 0 ] && [ -t 1 ]; then
  if command -v git >/dev/null 2>&1; then
    echo 'Opening guided setup. History and hooks change only after your confirmation.'
    "$prefix/byeclaude" guide || echo "Setup can wait. Run $prefix/byeclaude guide when ready."
  else
    echo 'ByeClaude is installed. Install Git from https://git-scm.com/downloads, then run byeclaude.'
  fi
else
  echo 'Next: open a new terminal and run byeclaude for guided setup.'
fi
