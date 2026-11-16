#!/usr/bin/env bash
# Copyright (c) 2026 Lark Technologies Pte. Ltd.
# SPDX-License-Identifier: MIT

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOCAL_BIN="${LOCAL_BIN:-$HOME/.local/bin}"
BINARY_NAME="larkx"
MODULE="github.com/larksuite/cli"
VERSION="$(git -C "$ROOT_DIR" describe --tags --always --dirty 2>/dev/null || echo dev)"
DATE="$(date +%Y-%m-%d)"
LDFLAGS="-s -w -X ${MODULE}/internal/build.Version=${VERSION} -X ${MODULE}/internal/build.Date=${DATE}"

mkdir -p "$LOCAL_BIN"
TMP_BIN="$(mktemp "${LOCAL_BIN}/.${BINARY_NAME}.tmp.XXXXXX")"
trap 'rm -f "$TMP_BIN"' EXIT

(
  cd "$ROOT_DIR"
  go build -trimpath -ldflags "$LDFLAGS" -o "$TMP_BIN" .
)

chmod 0755 "$TMP_BIN"
mv -f "$TMP_BIN" "${LOCAL_BIN}/${BINARY_NAME}"

echo "Installed ${BINARY_NAME} (${VERSION}) to ${LOCAL_BIN}/${BINARY_NAME}"
