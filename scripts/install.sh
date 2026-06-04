#!/bin/sh
# Install miudb. Auto-detects OS/arch and fetches the matching release binary.
# Usage: curl -fsSL https://raw.githubusercontent.com/vanducng/miu-db/main/scripts/install.sh | sh
# Env:   MIUDB_VERSION=v0.2.4   MIUDB_INSTALL_DIR=/usr/local/bin
set -eu

REPO="vanducng/miu-db"
BIN="miudb"
: "${MIUDB_VERSION:=latest}"
: "${MIUDB_INSTALL_DIR:=/usr/local/bin}"

err() { printf 'error: %s\n' "$1" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

dl() {
  if have curl; then curl -fsSL "$1" -o "$2"
  elif have wget; then wget -qO "$2" "$1"
  else err "need curl or wget"; fi
}

os=$(uname -s)
case "$os" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) err "unsupported OS '$os' — on Windows use the PowerShell install" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch=x86_64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) err "unsupported architecture '$arch'" ;;
esac

asset="${BIN}_${os}_${arch}.tar.gz"
if [ "$MIUDB_VERSION" = latest ]; then
  base="https://github.com/${REPO}/releases/latest/download"
else
  base="https://github.com/${REPO}/releases/download/${MIUDB_VERSION}"
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

printf 'Downloading %s (%s)...\n' "$asset" "$MIUDB_VERSION"
dl "${base}/${asset}" "${tmp}/${asset}" ||
  err "download failed — check that release '${MIUDB_VERSION}' has ${asset}"

if dl "${base}/checksums.txt" "${tmp}/checksums.txt" 2>/dev/null; then
  sum=""
  if have sha256sum; then sum=$(sha256sum "${tmp}/${asset}" | awk '{print $1}')
  elif have shasum; then sum=$(shasum -a 256 "${tmp}/${asset}" | awk '{print $1}'); fi
  [ -z "$sum" ] || grep -q -- "$sum  $asset" "${tmp}/checksums.txt" ||
    err "checksum mismatch for $asset"
fi

tar -xzf "${tmp}/${asset}" -C "$tmp"
[ -f "${tmp}/${BIN}" ] || err "binary '$BIN' missing from archive"
chmod +x "${tmp}/${BIN}"

dir="$MIUDB_INSTALL_DIR"
[ -d "$dir" ] || mkdir -p "$dir" 2>/dev/null || true
if [ -w "$dir" ]; then
  mv "${tmp}/${BIN}" "${dir}/${BIN}"
elif have sudo; then
  printf 'Installing to %s (requires sudo)...\n' "$dir"
  sudo mv "${tmp}/${BIN}" "${dir}/${BIN}"
else
  dir="$HOME/.local/bin"
  mkdir -p "$dir"
  mv "${tmp}/${BIN}" "${dir}/${BIN}"
  printf 'Note: %s is not on your PATH by default — add it.\n' "$dir"
fi

printf 'Installed %s -> %s\n' "$BIN" "${dir}/${BIN}"
"${dir}/${BIN}" version 2>/dev/null || true
