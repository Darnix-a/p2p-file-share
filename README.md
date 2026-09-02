# p2p-drop

End-to-end encrypted peer-to-peer file and folder transfer CLI tool. Transfers data directly between computers over local networks (LAN) or across the internet (WAN) with zero third-party storage.

## Installation

### Linux / macOS
```bash
curl -fsSL https://raw.githubusercontent.com/Darnix-a/p2p-file-share/main/install.sh | bash
```

### Windows (PowerShell)
```powershell
irm https://raw.githubusercontent.com/Darnix-a/p2p-file-share/main/install.ps1 | iex
```

### Build from Source
```bash
git clone https://github.com/Darnix-a/p2p-file-share.git
cd p2p-file-share
go build -o p2p-drop ./cmd/p2p-drop
```

## Quick Start

### 1. Local Network (LAN) Transfer
No internet or relay server required.

Sender:
```bash
p2p-drop send --lan path/to/file.iso
# or send a whole directory:
p2p-drop send --lan path/to/folder/
```

Receiver:
```bash
p2p-drop receive --lan
```

---

### 2. Internet (WAN) Transfer

#### Step 1: Start the Relay & Cloudflare Tunnel (Sender PC)
Run in a separate terminal:
```bash
make tunnel
# or: ./scripts/start-relay-tunnel.sh
```
This prints your public address (e.g. `https://random-name.trycloudflare.com`).

#### Step 2: Send the File
```bash
p2p-drop send path/to/file.iso --relay https://random-name.trycloudflare.com
```
This prints a pairing code (e.g. `42-crystal-dragon-falcon`).

#### Step 3: Receive on Friend's PC
```bash
p2p-drop receive 42-crystal-dragon-falcon --relay https://random-name.trycloudflare.com
```

---

## Troubleshooting

### Connection is stuck on "Waiting for receiver" or "Negotiating connection"
1. **Check the relay URL:** Ensure both sender and receiver use the exact same `--relay` URL.
2. **Ensure the tunnel is active:** The `make tunnel` terminal must stay running during the transfer.
3. **No open ports / Strict NAT:** `p2p-drop` automatically uses WebRTC STUN and built-in TURN relays (port 443) to bypass symmetric NATs and firewalls without port forwarding.

### Transfer speed is capped or slow
1. **Local transfer:** If sender and receiver are on the same Wi-Fi, use `--lan` to bypass the internet entirely for maximum local network speed.
2. **Internet transfer:** Direct WebRTC P2P connects automatically after candidate gathering. If direct P2P is blocked by restrictive ISP routing, traffic falls back to the encrypted relay.

---

## Command Reference

| Command | Option | Description |
| --- | --- | --- |
| `send <path>` | `--lan` | Direct local network broadcast transfer |
| `send <path>` | `--code <code>` | Set custom pairing passphrase |
| `send <path>` | `--relay <url>` | Signaling relay server URL |
| `receive [code]` | `--lan` | Auto-discover LAN transfer |
| `receive [code]` | `-o, --output <dir>` | Destination directory (default: current directory) |
| `receive [code]` | `-y, --yes` | Automatically accept transfer without prompt |
| `receive [code]` | `--relay <url>` | Signaling relay server URL |
| `relay` | `-p, --port <port>` | Port to run signaling relay on (default: `8080`) |
