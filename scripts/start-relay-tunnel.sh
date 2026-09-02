#!/usr/bin/env bash
set -e

RELAY_PORT=8080
CLOUDFLARED_BIN="$HOME/.local/bin/cloudflared"

# Kill any existing stale process on port 8080
fuser -k "${RELAY_PORT}/tcp" 2>/dev/null || true

if ! command -v "$CLOUDFLARED_BIN" >/dev/null 2>&1 && ! command -v cloudflared >/dev/null 2>&1; then
  echo "cloudflared not found. Installing..."
  mkdir -p "$HOME/.local/bin"
  curl -fsSL https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o "$CLOUDFLARED_BIN"
  chmod +x "$CLOUDFLARED_BIN"
fi

if command -v cloudflared >/dev/null 2>&1; then
  CLOUDFLARED_CMD="cloudflared"
else
  CLOUDFLARED_CMD="$CLOUDFLARED_BIN"
fi

echo "Starting p2p-drop relay server on port $RELAY_PORT..."
./p2p-drop relay --port "$RELAY_PORT" &
RELAY_PID=$!

trap 'echo -e "\nStopping relay and tunnel..."; kill $RELAY_PID 2>/dev/null || true; exit 0' INT TERM EXIT

sleep 1

echo "Starting Cloudflare tunnel for port $RELAY_PORT..."
echo "Your public address is the URL ending with .trycloudflare.com"
echo ""

$CLOUDFLARED_CMD tunnel --url "http://127.0.0.1:$RELAY_PORT"
