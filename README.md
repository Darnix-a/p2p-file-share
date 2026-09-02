# 🔒 p2p-drop

**p2p-drop** is a high-performance, end-to-end encrypted peer-to-peer file transfer CLI written in Go. It enables secure, direct transfers of single files and entire directory trees between computers across the **Internet (WAN)** using WebRTC NAT traversal and over the **local network (LAN)** with zero third-party cloud storage.

---

## ✨ Features

* 🔐 **End-to-End Encryption (E2EE)**:
  * Ephemeral **X25519** elliptic-curve Diffie-Hellman key exchange.
  * **HKDF-SHA256** session key derivation with room salt.
  * Streaming **ChaCha20-Poly1305 AEAD** chunk encryption (64 KB chunk size with nonce counter sequence).
  * Real-time **SHA-256** hash verification for file integrity.
* 🌐 **Direct Internet Transfer (WAN)**:
  * Uses **WebRTC DataChannels** with public STUN servers (`stun.l.google.com:19302`) for automatic NAT hole-punching through home routers and firewalls.
  * Pair with friends using short, human-friendly 3-word passphrase codes (e.g. `42-crystal-dragon-falcon`).
* ⚡ **Zero-Config LAN Mode**:
  * Local UDP broadcast discovery (`255.255.255.255:9002`) and direct high-speed TCP socket streaming.
* 📁 **Folder & Directory Support**:
  * Streams directories on-the-fly using `tar.gz` compression pipeline without creating huge intermediate archive files on disk.
* 📊 **Terminal UI**:
  * Live animated progress bar with transfer rate (MB/s), ETA, and interactive acceptance prompts.
* 🚀 **Built-in Self-Hostable Relay**:
  * Single command to run your own signaling server (`p2p-drop relay`).

---

## 📦 Installation & Build

```bash
cd p2p-drop
go build -o p2p-drop ./cmd/p2p-drop
```

## 📦 Easy 1-Line Installation for Friends

### 🍎 Mac & 🐧 Linux
```bash
curl -fsSL https://raw.githubusercontent.com/Darnix-a/p2p-file-share/main/install.sh | bash
```

### 🪟 Windows (Run in PowerShell)
```powershell
irm https://raw.githubusercontent.com/Darnix-a/p2p-file-share/main/install.ps1 | iex
```

---

## 🚀 Usage Guide

### 1. Sending a File or Folder (WAN / Internet to Friends)

Start the sender:
```bash
./p2p-drop send path/to/large-video.mp4
# Or send a whole folder:
./p2p-drop send path/to/project-folder/
```

This will output a human-friendly pairing code:
```
======================================================
🔑 Pairing Code: 42-crystal-dragon-falcon
======================================================
Tell your friend to run:
  p2p-drop receive 42-crystal-dragon-falcon
```

On your friend's machine:
```bash
./p2p-drop receive 42-crystal-dragon-falcon
```

Once connected, both peers perform an encrypted key exchange and stream bytes directly between machines!

---

### 2. Fast LAN Transfer (Same Wi-Fi / Local Network)

On the sender machine:
```bash
./p2p-drop send --lan path/to/file.iso
```

On the receiver machine:
```bash
./p2p-drop receive --lan
```
*The receiver will auto-discover the sender's beacon on the local network and connect directly over LAN TCP!*

---

### 3. Running a Custom Signaling Relay

If you want to host a private signaling rendezvous server on a VPS or home server:
```bash
./p2p-drop relay --port 8080
```

Then point clients to your relay:
```bash
./p2p-drop send my-file.zip --relay ws://your-server-ip:8080
./p2p-drop receive 42-crystal-dragon-falcon --relay ws://your-server-ip:8080
```

---

## 🛠️ Commands & Flags

| Command | Flag | Description |
| :--- | :--- | :--- |
| `send <path>` | `--code <code>` | Use a custom pairing room code |
| `send <path>` | `--lan` | Broadcast on local LAN instead of WebRTC |
| `send <path>` | `--relay <url>` | Custom WebSocket signaling server URL |
| `receive [code]`| `-o, --output <dir>`| Destination directory (defaults to `.`) |
| `receive [code]`| `-y, --yes` | Auto-accept incoming transfer without prompt |
| `receive [code]`| `--lan` | Discover and receive from LAN beacon |
| `relay` | `-p, --port <port>` | Port to run signaling WebSocket server on |

---

## 🧪 Running Tests

```bash
go test -v ./...
```
