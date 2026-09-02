#!/usr/bin/env bash
set -e

# Installer for p2p-drop
# Usage: curl -fsSL https://raw.githubusercontent.com/Darnix-a/p2p-file-share/main/install.sh | bash

REPO="Darnix-a/p2p-file-share"
BIN_NAME="p2p-drop"
INSTALL_DIR="/usr/local/bin"

if [ "$EUID" -ne 0 ]; then
  INSTALL_DIR="$HOME/.local/bin"
  mkdir -p "$INSTALL_DIR"
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Error: Unsupported architecture $ARCH"
    exit 1
    ;;
esac

case "$OS" in
  linux|darwin) ;;
  *)
    echo "Error: Unsupported operating system $OS"
    exit 1
    ;;
esac

TAG=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$TAG" ]; then
  TAG="v1.0.4"
fi

DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${BIN_NAME}-${OS}-${ARCH}.tar.gz"

echo "Downloading ${BIN_NAME} (${OS}/${ARCH})..."
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

if curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/${BIN_NAME}.tar.gz"; then
  tar -xzf "$TMP_DIR/${BIN_NAME}.tar.gz" -C "$TMP_DIR"
  mv "$TMP_DIR/${BIN_NAME}-${OS}-${ARCH}" "${INSTALL_DIR}/${BIN_NAME}"
  chmod +x "${INSTALL_DIR}/${BIN_NAME}"
  echo "${BIN_NAME} installed to ${INSTALL_DIR}/${BIN_NAME}"
  
  if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo ""
    echo "Note: '$INSTALL_DIR' is not in your current PATH."
    echo "  - For Fish: fish_add_path $INSTALL_DIR"
    echo "  - For Bash: export PATH=\"\$HOME/.local/bin:\$PATH\" >> ~/.bashrc"
    echo "  - For Zsh:  export PATH=\"\$HOME/.local/bin:\$PATH\" >> ~/.zshrc"
    echo ""
  fi

  echo "Run 'p2p-drop --help' to get started."
else
  echo ""
  echo "Error: Could not download release from:"
  echo "  $DOWNLOAD_URL"
  echo ""
  exit 1
fi
