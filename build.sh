#!/usr/bin/env bash
set -euo pipefail

OUTDIR="vscode-extension/bin"
MODULE="."

mkdir -p "$OUTDIR"

targets=(
  "darwin  arm64"
  "darwin  amd64"
  "linux   amd64"
  "linux   arm64"
  "windows amd64"
)

for t in "${targets[@]}"; do
  os=$(echo "$t" | awk '{print $1}')
  arch=$(echo "$t" | awk '{print $2}')
  name="gohtml-lsp-${os}-${arch}"
  if [ "$os" = "windows" ]; then
    name="${name}.exe"
  fi
  echo "Building ${name}..."
  GOOS="$os" GOARCH="$arch" go build -o "${OUTDIR}/${name}" "$MODULE"
done

echo "Done. Binaries in ${OUTDIR}/:"
ls -lh "$OUTDIR"/
