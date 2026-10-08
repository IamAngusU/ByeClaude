#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/byeclaude-installer-test.XXXXXX")
cleanup() {
  case "$tmp" in "${TMPDIR:-/tmp}"/byeclaude-installer-test.*) rm -rf -- "$tmp" ;; *) echo "Refusing unexpected test directory: $tmp" >&2 ;; esac
}
trap cleanup EXIT INT TERM

assets="$tmp/assets"
export BYECLAUDE_STATE_DIR="$tmp/state"
export BYECLAUDE_NO_PATH=1
install_dir="$tmp/install"
mkdir -p "$assets" "$install_dir"
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in linux|darwin) ;; *) echo "Shell installer test skipped on unsupported host: $os"; exit 0 ;; esac
arch=$(uname -m)
case "$arch" in x86_64|amd64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) echo "Unsupported test architecture: $arch" >&2; exit 1 ;; esac
asset="byeclaude_${os}_${arch}"
version=v0.0.0-test

(cd "$repo_root" && GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "-s -w -X main.version=$version" -o "$assets/$asset" ./cmd/byeclaude)
if command -v sha256sum >/dev/null 2>&1; then
  hash=$(sha256sum "$assets/$asset" | awk '{print $1}')
else
  hash=$(shasum -a 256 "$assets/$asset" | awk '{print $1}')
fi
printf '%s  %s\n' "$hash" "$asset" > "$assets/SHA256SUMS.txt"

BYECLAUDE_VERSION="$version" BYECLAUDE_DOWNLOAD_BASE="file://$assets" BYECLAUDE_INSTALL_DIR="$install_dir" sh "$repo_root/install.sh"
[ "$("$install_dir/byeclaude" version)" = "byeclaude $version" ]

# A managed shell profile makes optional PATH setup fail without breaking install.
mkdir -p "$tmp/home"
printf 'preserve\n' > "$tmp/home/managed-profile"
ln -s "$tmp/home/managed-profile" "$tmp/home/.bashrc"
HOME="$tmp/home" SHELL=/bin/bash BYECLAUDE_NO_PATH=0 BYECLAUDE_VERSION="$version" BYECLAUDE_DOWNLOAD_BASE="file://$assets" BYECLAUDE_INSTALL_DIR="$install_dir" sh "$repo_root/install.sh" > "$tmp/path-failure.log"
grep -q 'PATH setup deferred' "$tmp/path-failure.log"
[ "$(cat "$tmp/home/managed-profile")" = preserve ]
[ "$("$install_dir/byeclaude" version)" = "byeclaude $version" ]

# An unwritable custom destination falls back to a user directory without sudo.
printf 'keep' > "$tmp/blocked-destination"
HOME="$tmp/home" BYECLAUDE_VERSION="$version" BYECLAUDE_DOWNLOAD_BASE="file://$assets" BYECLAUDE_INSTALL_DIR="$tmp/blocked-destination" sh "$repo_root/install.sh"
[ "$("$tmp/home/.local/bin/byeclaude" version)" = "byeclaude $version" ]
[ "$(cat "$tmp/blocked-destination")" = keep ]
if command -v sha256sum >/dev/null 2>&1; then
  installed_hash=$(sha256sum "$install_dir/byeclaude" | awk '{print $1}')
else
  installed_hash=$(shasum -a 256 "$install_dir/byeclaude" | awk '{print $1}')
fi

printf 'corrupt' > "$assets/$asset"
if BYECLAUDE_VERSION="$version" BYECLAUDE_DOWNLOAD_BASE="file://$assets" BYECLAUDE_INSTALL_DIR="$install_dir" sh "$repo_root/install.sh" 2>/dev/null; then
  echo 'Installer accepted a corrupted binary.' >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  after_hash=$(sha256sum "$install_dir/byeclaude" | awk '{print $1}')
else
  after_hash=$(shasum -a 256 "$install_dir/byeclaude" | awk '{print $1}')
fi
[ "$installed_hash" = "$after_hash" ] || { echo 'Failed installer attempt replaced the existing binary.' >&2; exit 1; }

if BYECLAUDE_VERSION=not-a-version BYECLAUDE_INSTALL_DIR="$install_dir" sh "$repo_root/install.sh" 2>/dev/null; then
  echo 'Installer accepted an invalid version selector.' >&2
  exit 1
fi
echo 'Shell installer positive and failure-path tests passed.'
