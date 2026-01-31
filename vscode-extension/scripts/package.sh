#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

TARGETS=(
  "linux-x64:gohtml-lsp-linux-amd64"
  "linux-arm64:gohtml-lsp-linux-arm64"
  "darwin-x64:gohtml-lsp-darwin-amd64"
  "darwin-arm64:gohtml-lsp-darwin-arm64"
  "win32-x64:gohtml-lsp-windows-amd64.exe"
)

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# Move all binaries to temp
mv bin/* "$TMPDIR/" 2>/dev/null || true

for entry in "${TARGETS[@]}"; do
  target="${entry%%:*}"
  binary="${entry##*:}"

  if [[ ! -f "$TMPDIR/$binary" ]]; then
    echo "WARNING: $binary not found in bin/, skipping $target"
    continue
  fi

  cp "$TMPDIR/$binary" "bin/$binary"
  npx vsce package --no-dependencies --target "$target"
  rm "bin/$binary"
done

# Restore all binaries
cp "$TMPDIR"/* bin/ 2>/dev/null || true

echo "Done. Platform .vsix files created."
