<div align="center">

```
     _    _____ ____   ___  
    / \  | ____|  _ \ / _ \ 
   / _ \ |  _| | |_) | | | |
  / ___ \| |___|  _ <| |_| |
 /_/   \_\_____|_| \_\\___/ 
                            
 Encrypted LAN File Transfer
```

**Phone → PC file transfer with military-grade encryption.**
**No cloud. No tracking. No history.**

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev)
[![Platform](https://img.shields.io/badge/Platform-Windows%20|%20macOS%20|%20Linux-lightgrey)](https://github.com/username/aero/releases)

</div>

---

## ⚡ Quick Start

```
1. Download → Run AERO.exe
2. Scan the QR code with your phone
3. Drop files — they appear on your PC instantly
```

No installation. No account. No configuration.

---

## 🔐 Zero-Trust Security Model

AERO assumes your network is **hostile**. Here's how we protect your files:

### The Optical-Key Protocol

```
┌─────────────────────────────────────────────────────────────────┐
│                        YOUR LOCAL NETWORK                       │
│                     (Assumed Compromised)                       │
│                                                                 │
│   ┌─────────┐                              ┌─────────┐          │
│   │  PHONE  │  ═══════════════════════════>│   PC    │          │
│   │         │       Encrypted Stream       │  (AERO) │          │
│   └────┬────┘       AES-256-CTR            └────┬────┘          │
│        │                                        │               │
│        │  ┌──────────────────────────────┐     │               │
│        └──│  Attacker sees: 0x8a3f...    │─────┘               │
│           │  (Meaningless ciphertext)    │                     │
│           └──────────────────────────────┘                     │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

Key Exchange: QR Code (never transmitted over network)
```

### How It Works

1. **Key Generation**: When AERO starts, it generates a cryptographically secure 256-bit session key using your OS's CSPRNG.

2. **Out-of-Band Key Exchange**: The key is embedded in the QR code URL as a `hash fragment`:
   ```
   http://192.168.1.5:8080/#<BASE64_SESSION_KEY>
   ```
   
   > **Critical**: Hash fragments are **never sent to the server** in HTTP requests. This is defined in [RFC 3986](https://tools.ietf.org/html/rfc3986#section-3.5). An attacker sniffing your network sees only `http://192.168.1.5:8080/upload` — the key stays on your devices.

3. **Client-Side Encryption**: Your phone encrypts files **before** they leave the browser using the [WebCrypto API](https://developer.mozilla.org/en-US/docs/Web/API/Web_Crypto_API). We use AES-256-CTR (Counter Mode) for true streaming encryption — even 10GB files never load into memory.

4. **Server-Side Decryption**: AERO decrypts the stream on-the-fly as it writes to disk. The server already knows the key (it generated it), so no key exchange occurs over the network.

### Why Not Just Use HTTPS?

Self-signed certificates on local networks cause:
- ❌ Browser warnings that scare users
- ❌ Mobile Safari rejecting connections entirely
- ❌ Complex certificate distribution

AERO's approach:
- ✅ Works on any browser, any device
- ✅ No certificate warnings
- ✅ Same cryptographic strength as TLS 1.3

---

## 🛡️ Privacy Promise

| What AERO Does | What AERO Does NOT Do |
|----------------|----------------------|
| Encrypts your files | Send data to the cloud |
| Runs 100% locally | Track usage or analytics |
| Deletes session keys on exit | Keep any transfer history |
| Open source for audit | Phone home for updates |

**Your files. Your network. Your control.**

---

## 🏗️ Build From Source

### Prerequisites

- [Go 1.21+](https://go.dev/dl/)
- [Node.js 18+](https://nodejs.org/)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)

```bash
# Install Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Clone repository
git clone https://github.com/username/aero.git
cd aero

# Development mode (hot reload)
wails dev

# Production build
wails build
# Binary: ./build/bin/AERO.exe
```

### Release Build (Optimized)

Run the release build script for a smaller, optimized binary:

```powershell
.\build_release.ps1
```

This strips debug symbols and compresses with UPX (~15MB → ~4MB).

---

## 📁 Project Structure

```
aero/
├── app.go                    # Wails bridge (Go ↔ React)
├── main.go                   # Application entry point
├── wails.json                # Wails configuration
├── frontend/                 # React UI
│   ├── src/App.tsx          # Main component
│   └── tailwind.config.js   # Theme
├── internal/
│   ├── config/              # Configuration
│   ├── security/            # AES-256-CTR encryption
│   ├── server/              # HTTP ingestion engine
│   └── storage/             # Atomic file writer
└── pkg/
    └── networking/          # Smart IP detection
```

---

## 🔧 Technical Specifications

| Component | Technology |
|-----------|------------|
| Encryption | AES-256-CTR (streaming) |
| Key Size | 256 bits (32 bytes) |
| IV Size | 128 bits (16 bytes) |
| Key Exchange | QR code hash fragment |
| Frontend Crypto | WebCrypto API |
| Backend Crypto | Go `crypto/cipher` |
| Transfer | HTTP multipart/form-data |
| Memory | O(32KB) buffer, not O(file size) |

---

## ⚠️ Security Considerations

**What AERO protects against:**
- ✅ Passive network sniffing (coffee shop WiFi)
- ✅ Router-level traffic inspection
- ✅ ISP/corporate network monitoring

**What AERO does NOT protect against:**
- ❌ Physical access to your PC (files are stored unencrypted)
- ❌ Compromised phone or PC (malware)
- ❌ Active MITM attacks (attacker could swap QR code)

For defense against active MITM, verify the QR code visually appears on your PC before scanning.

---

## 📄 License

MIT License — See [LICENSE](LICENSE) for details.

---

<div align="center">

**Built for the paranoid. Trusted by the practical.**

</div>
