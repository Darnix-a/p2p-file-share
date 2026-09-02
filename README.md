# p2p-drop

End-to-end encrypted peer-to-peer file and folder transfer CLI tool. Transfers data directly between machines over local networks (LAN) or the internet (WAN) with zero third-party storage.

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

Sender:
```bash
p2p-drop send --lan path/to/file.zip
# or send a whole directory:
p2p-drop send --lan path/to/folder/
```

Receiver:
```bash
p2p-drop receive --lan
```

### 2. Internet (WAN) Transfer

Start relay server (or run with Cloudflare tunnel):
```bash
p2p-drop relay --port 8080
```

Sender:
```bash
p2p-drop send path/to/file.zip --relay ws://YOUR_RELAY_IP:8080
```

Receiver:
```bash
p2p-drop receive <room-code> --relay ws://YOUR_RELAY_IP:8080
```

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
