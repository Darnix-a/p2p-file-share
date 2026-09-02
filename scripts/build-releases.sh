#!/usr/bin/env bash
set -e

VERSION="${1:-v1.0.0}"
DIST_DIR="dist"
APP_NAME="p2p-drop"

echo "🔨 Building releases for $APP_NAME ($VERSION)..."
mkdir -p "$DIST_DIR"
rm -rf "$DIST_DIR"/*

TARGETS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
)

for TARGET in "${TARGETS[@]}"; do
  OS=$(echo "$TARGET" | cut -d'/' -f1)
  ARCH=$(echo "$TARGET" | cut -d'/' -f2)
  OUTPUT_NAME="${APP_NAME}-${OS}-${ARCH}"
  
  if [ "$OS" = "windows" ]; then
    OUTPUT_BIN="${OUTPUT_NAME}.exe"
  else
    OUTPUT_BIN="${OUTPUT_NAME}"
  fi

  echo "  📦 Compiling $OS/$ARCH -> $OUTPUT_BIN..."
  CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -ldflags="-s -w" -o "${DIST_DIR}/${OUTPUT_BIN}" ./cmd/p2p-drop

  # Create archives for easy sharing
  cd "$DIST_DIR"
  if [ "$OS" = "windows" ]; then
    if command -v zip >/dev/null 2>&1; then
      zip -q "${OUTPUT_NAME}.zip" "${OUTPUT_BIN}"
    else
      python3 -m zipfile -c "${OUTPUT_NAME}.zip" "${OUTPUT_BIN}"
    fi
  else
    tar -czf "${OUTPUT_NAME}.tar.gz" "${OUTPUT_BIN}"
  fi
  cd ..
done

# Generate SHA256 checksums
cd "$DIST_DIR"
sha256sum *.{tar.gz,zip,exe} 2>/dev/null > checksums.txt || true
cd ..

echo "✅ All releases built successfully in ./dist/:"
ls -lh "$DIST_DIR"
