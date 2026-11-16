#!/usr/bin/env bash
# Copyright (c) 2026 Lark Technologies Pte. Ltd.
# SPDX-License-Identifier: MIT

set -euo pipefail

REPO="${LARKX_REPO:-Octane0411/larkx}"
VERSION="${LARKX_VERSION:-latest}"
BIN_DIR="${LOCAL_BIN:-$HOME/.local/bin}"
DO_TAKEOVER="${LARKX_TAKEOVER:-0}"
RELEASE_BASE_URL="${LARKX_RELEASE_BASE_URL:-}"

usage() {
  cat <<'EOF'
Install prebuilt larkx binary from GitHub Releases.

Usage:
  install.sh [options]

Options:
  --takeover            Enable ~/.local/bin/lark-cli -> larkx symlink after install
  --version <ver>       Install specific release tag (e.g. v0.1.0) or "latest"
  --bin-dir <dir>       Install destination directory (default: $HOME/.local/bin)
  -h, --help            Show this help message

Environment variables:
  LARKX_REPO              GitHub repository (default: Octane0411/larkx)
  LARKX_VERSION           Release version tag (default: latest)
  LOCAL_BIN               Destination directory (default: $HOME/.local/bin)
  LARKX_TAKEOVER          Set to 1 or true to enable takeover
  LARKX_RELEASE_BASE_URL  Override release download base URL
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --takeover)
      DO_TAKEOVER=1
      shift
      ;;
    --version)
      [ $# -ge 2 ] || { echo "Error: --version requires a value" >&2; exit 2; }
      VERSION="$2"
      shift 2
      ;;
    --bin-dir)
      [ $# -ge 2 ] || { echo "Error: --bin-dir requires a value" >&2; exit 2; }
      BIN_DIR="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

case "$(uname -s)" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux" ;;
  *)
    echo "Unsupported OS: $(uname -s). Build from source with Go >= 1.23 instead." >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  arm64|aarch64) ARCH="arm64" ;;
  x86_64|amd64)  ARCH="amd64" ;;
  *)
    echo "Unsupported architecture: $(uname -m). Build from source with Go >= 1.23 instead." >&2
    exit 1
    ;;
esac

if [ "$VERSION" = "latest" ]; then
  if [ -n "$RELEASE_BASE_URL" ]; then
    echo "Error: --version <tag> is required when LARKX_RELEASE_BASE_URL is set" >&2
    exit 2
  fi
  API_URL="https://api.github.com/repos/${REPO}/releases/latest"
  VERSION="$(
    curl -fsSL "$API_URL" \
      | grep -m1 '"tag_name"' \
      | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/'
  )"
  if [ -z "$VERSION" ]; then
    echo "Error: failed to resolve latest release tag from ${API_URL}" >&2
    exit 1
  fi
fi

BASE_URL="${RELEASE_BASE_URL:-https://github.com/${REPO}/releases/download/${VERSION}}"
BASE_URL="${BASE_URL%/}"
ARCHIVE="larkx_${VERSION}_${OS}_${ARCH}.tar.gz"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading larkx ${VERSION} (${OS}/${ARCH})..."
curl -fsSL "${BASE_URL}/${ARCHIVE}" -o "${TMP_DIR}/${ARCHIVE}"
curl -fsSL "${BASE_URL}/checksums.txt" -o "${TMP_DIR}/checksums.txt"

verify_sha256() {
  local archive="$1"
  local checksums="$2"
  local expected actual
  expected="$(awk -v target="$archive" '$2 == target || $2 == ("./" target) { print $1; exit }' "$checksums")"
  if [ -z "$expected" ]; then
    echo "Error: checksum entry for ${archive} not found in checksums.txt" >&2
    exit 1
  fi
  if command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "${TMP_DIR}/${archive}" | awk '{print $1}')"
  elif command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "${TMP_DIR}/${archive}" | awk '{print $1}')"
  else
    echo "Error: neither shasum nor sha256sum is installed" >&2
    exit 1
  fi
  if [ "$expected" != "$actual" ]; then
    echo "Error: SHA-256 mismatch for ${archive} (expected ${expected}, got ${actual})" >&2
    exit 1
  fi
}

verify_sha256 "$ARCHIVE" "${TMP_DIR}/checksums.txt"

tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "$TMP_DIR"
[ -f "${TMP_DIR}/larkx" ] || { echo "Error: extracted archive missing larkx binary" >&2; exit 1; }

mkdir -p "$BIN_DIR"
TMP_BIN="$(mktemp "${BIN_DIR}/.larkx.tmp.XXXXXX")"
cp "${TMP_DIR}/larkx" "$TMP_BIN"
chmod 0755 "$TMP_BIN"
mv -f "$TMP_BIN" "${BIN_DIR}/larkx"

echo "Installed: $("${BIN_DIR}/larkx" --version) -> ${BIN_DIR}/larkx"

case "$DO_TAKEOVER" in
  1|true|TRUE|yes|YES)
    "${BIN_DIR}/larkx" takeover enable --bin-dir "$BIN_DIR"
    ;;
esac

case ":$PATH:" in
  *":${BIN_DIR}:"*) ;;
  *)
    echo ""
    echo "Hint: add ${BIN_DIR} to your shell PATH (after brew shellenv on macOS):"
    echo "  export PATH=\"${BIN_DIR}:\$PATH\""
    ;;
esac

if [ -x /opt/homebrew/bin/lark-cli ]; then
  RESOLVED_LARK="$(PATH="$PATH" command -v lark-cli 2>/dev/null || true)"
  if [ "$RESOLVED_LARK" = "/opt/homebrew/bin/lark-cli" ]; then
    echo ""
    echo "Hint: /opt/homebrew/bin currently precedes ${BIN_DIR} in PATH."
    echo "Ensure 'export PATH=\"\$HOME/.local/bin:\$PATH\"' runs AFTER 'eval \"\$(/opt/homebrew/bin/brew shellenv)\"' in ~/.zprofile / ~/.bash_profile."
  fi
fi
