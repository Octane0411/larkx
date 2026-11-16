#!/usr/bin/env bash
# Copyright (c) 2026 Lark Technologies Pte. Ltd.
# SPDX-License-Identifier: MIT

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

case "$(uname -s)" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux" ;;
  *) echo "skip unsupported OS $(uname -s)"; exit 0 ;;
esac

case "$(uname -m)" in
  arm64|aarch64) ARCH="arm64" ;;
  x86_64|amd64)  ARCH="amd64" ;;
  *) echo "skip unsupported arch $(uname -m)"; exit 0 ;;
esac

MOCK_DIR="${WORK_DIR}/release"
STAGE_DIR="${WORK_DIR}/stage"
DEST_BIN="${WORK_DIR}/bin"
mkdir -p "$MOCK_DIR" "$STAGE_DIR" "$DEST_BIN"

(
  cd "$ROOT_DIR"
  go build -trimpath -ldflags "-s -w -X github.com/larksuite/cli/internal/build.Version=v0.1.0-test" -o "${STAGE_DIR}/larkx" .
)
cp "${ROOT_DIR}/LICENSE" "${STAGE_DIR}/LICENSE"
cp "${ROOT_DIR}/README.md" "${STAGE_DIR}/README.md"

ARCHIVE="larkx_v0.1.0_${OS}_${ARCH}.tar.gz"
tar -C "$STAGE_DIR" -czf "${MOCK_DIR}/${ARCHIVE}" larkx LICENSE README.md

if command -v shasum >/dev/null 2>&1; then
  (cd "$MOCK_DIR" && shasum -a 256 "$ARCHIVE" > checksums.txt)
else
  (cd "$MOCK_DIR" && sha256sum "$ARCHIVE" > checksums.txt)
fi

LARKX_RELEASE_BASE_URL="file://${MOCK_DIR}" \
  bash "${ROOT_DIR}/scripts/install.sh" \
  --version v0.1.0 \
  --bin-dir "$DEST_BIN" \
  --takeover

VER_OUT="$("${DEST_BIN}/larkx" --version)"
case "$VER_OUT" in
  *v0.1.0-test*) ;;
  *)
    echo "unexpected --version output: ${VER_OUT}" >&2
    exit 1
    ;;
esac

if [ ! -L "${DEST_BIN}/lark-cli" ]; then
  echo "expected ${DEST_BIN}/lark-cli symlink to exist" >&2
  exit 1
fi

LINK_TARGET="$(readlink "${DEST_BIN}/lark-cli")"
if [ "$LINK_TARGET" != "${DEST_BIN}/larkx" ]; then
  echo "unexpected symlink target: ${LINK_TARGET}, want ${DEST_BIN}/larkx" >&2
  exit 1
fi

echo "PASS: scripts/install.sh verified (${OS}/${ARCH})"
