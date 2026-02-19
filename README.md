# tailchat

End-to-end encrypted terminal chat over Tailscale.

```
 tailchat

  ● you  nathanbhanji-MacBook-Pro (100.125.202.126)

  Peers (4 online)
  ● andy-MacBook-Pro       100.94.1.38     🔒
  ● jonahlytle-MacBook-Pro 100.101.196.39
  ○ steve-MacBook-Pro      100.71.178.13

  j/k navigate • enter connect/open • g new group • q quit
```

No servers. No accounts. No cloud. Just your tailnet and math.

## Install

```bash
brew install NathanBhanji/tap/tailchat
```

Or with Go:

```bash
go install github.com/NathanBhanji/tail-chat/cmd/tailchat@latest
```

Or download a binary from [Releases](https://github.com/NathanBhanji/tail-chat/releases).

## Prerequisites

- [Tailscale](https://tailscale.com) installed and running (`tailscale up`)
- That's it

## Usage

Run `tailchat` on two or more machines on the same tailnet:

```bash
tailchat
```

### Controls

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate peer list |
| `enter` | Connect to peer / open chat |
| `g` | Create group chat |
| `esc` | Back to peer list |
| `r` | Refresh peers |
| `q` | Quit |

### 1:1 Chat

Select an online peer and press `enter`. A TCP connection is established over Tailscale, keys are exchanged via X25519 ECDH, and all messages are encrypted with XChaCha20-Poly1305.

### Group Chat

Press `g` to create a group. Enter a name, add members by hostname, then `ctrl+s` to create. Messages are encrypted per-recipient — no shared group key.

## How It Works

```
┌─────────┐         Tailscale (WireGuard)         ┌─────────┐
│  Alice   │◄────────────────────────────────────►│   Bob   │
│          │                                       │         │
│ X25519   │──── ECDH Key Exchange ───────────────│ X25519  │
│ Keypair  │                                       │ Keypair │
│          │──── XChaCha20-Poly1305 Messages ─────│         │
└─────────┘                                       └─────────┘
```

1. **Identity**: On first run, generates an X25519 keypair stored at `~/.tailchat/identity.json`
2. **Discovery**: Parses `tailscale status --json` to find peers on your tailnet
3. **Connection**: TCP on port 9377, binds to your Tailscale IP
4. **Key Exchange**: X25519 ECDH handshake derives a shared secret
5. **Encryption**: Every message encrypted with XChaCha20-Poly1305 (nonce || ciphertext)
6. **No relay**: Messages go directly peer-to-peer, never through a server

Even though Tailscale already encrypts traffic with WireGuard, tailchat adds E2E encryption so messages are unreadable even if Tailscale's coordination server were compromised.

## Building from Source

```bash
git clone https://github.com/NathanBhanji/tail-chat.git
cd tail-chat
go build -o tailchat ./cmd/tailchat
./tailchat
```

## License

MIT
