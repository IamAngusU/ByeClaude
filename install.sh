#!/bin/sh
set -eu

repo="IamAngusU/ByeClaude"
os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$os" in linux|darwin) ;; *) echo "Unsupported OS: $os" >&2; exit 1 ;; esac
case "$arch" in x86_64|amd64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) echo "Unsupported architecture: $arch" >&2; exit 1 ;; esac

asset="byeclaude_${os}_${arch}"
version=${BYECLAUDE_VERSION:-latest}
case "$version" in
  latest) base="https://github.com/${repo}/releases/latest/download" ;;
  v*) base="https://github.com/${repo}/releases/download/${version}" ;;
  *) echo "BYECLAUDE_VERSION must be 'latest' or a v-prefixed tag" >&2; exit 1 ;;
esac
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

curl -fsSL "$base/$asset" -o "$tmp/byeclaude"
curl -fsSL "$base/SHA256SUMS.txt" -o "$tmp/SHA256SUMS.txt"
expected=$(awk -v f="$asset" '$2 == f {print $1}' "$tmp/SHA256SUMS.txt")
[ -n "$expected" ] || { echo "Checksum entry not found" >&2; exit 1; }
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/byeclaude" | awk '{print $1}')
else
  actual=$(shasum -a 256 "$tmp/byeclaude" | awk '{print $1}')
fi
[ "$actual" = "$expected" ] || { echo "Checksum mismatch" >&2; exit 1; }
chmod +x "$tmp/byeclaude"

prefix=${BYECLAUDE_INSTALL_DIR:-/usr/local/bin}
if [ -w "$prefix" ]; then
  install -m 0755 "$tmp/byeclaude" "$prefix/byeclaude"
else
  prefix=${BYECLAUDE_INSTALL_DIR:-"$HOME/.local/bin"}
  mkdir -p "$prefix"
  install -m 0755 "$tmp/byeclaude" "$prefix/byeclaude"
fi

echo "Installed byeclaude to $prefix/byeclaude"
case ":$PATH:" in *":$prefix:"*) ;; *) echo "Add $prefix to PATH." ;; esac
