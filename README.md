<div align="center">

<img src="assets/appicon.png" width="128" height="128" alt="Aero Logo">

# AERO

**Next-Gen Local File Transfer**
Phone ↔ PC • Encrypted • Blazing Fast

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev)
[![Platform](https://img.shields.io/badge/Platform-Windows-blue?logo=windows)](https://github.com/HarshalPatel1972/aero/releases)

</div>

---

## 🚀 Features

*   **⚡ Fast**: Parallel, chunked transfers over your local network (speed depends on your Wi-Fi and phone).
*   **🔒 End-to-End Encrypted**: Every file, file name and message is encrypted and authenticated with XChaCha20-Poly1305 using a one-time key shared only through the QR code.
*   **📱 Universal Client**: Works on **any** device (iOS, Android, Mac, Linux) via browser. No app install required on the phone.
*   **✨ Modern UI**: "Mini-Mode" for unobtrusive multitasking, and fluid animations.
*   **📦 Portable or Installed**: Available as a standard Windows Installer (`.exe`) or portable binary.

---

## 📥 Installation

### Windows (Recommended)
1.  Go to the [Releases Page](https://github.com/HarshalPatel1972/aero/releases).
2.  Download **`Aero_Setup.exe`**.
3.  Run the installer.
4.  Launch **Aero** from your desktop or start menu.

### Quick Start
1.  **Launch Aero** on your PC.
2.  **Scan the QR Code** with your phone's camera.
3.  **Drop files** on either device — they transfer instantly.

---

## 🛡️ Security Model

AERO assumes other devices on your network may be watching.

1.  **Session Key**: A fresh 256-bit key is generated every time you start the server, and wiped when you stop it.
2.  **Optical Exchange**: The key travels only inside the QR code, in the URL fragment (`#k=…`), which browsers never send over the network.
3.  **Encryption**: Every file chunk, file name and control message is sealed with XChaCha20-Poly1305 (via the audited [@noble/ciphers](https://github.com/paulmillr/noble-ciphers) library in the browser). Each chunk is bound to its transfer and position, so tampering, reordering or splicing is detected.
4.  **Authentication**: Every request carries a single-use token derived from the key. Devices without the QR code cannot connect, upload, or download anything, and captured requests cannot be replayed.
5.  **Least Access**: The phone can only download files you explicitly pick on the PC. Received files are saved to `Downloads\Aero` and never overwrite existing files.

**Limits, stated honestly:** the phone page is served over plain `http://` on your LAN, so an attacker who can actively intercept and modify your Wi-Fi traffic (not just observe it) could serve a tampered page. Use Aero on networks you trust, such as your home Wi-Fi, rather than public hotspots.

> **Privacy Promise**: No cloud. No accounts. No history on our side. Files go directly between your devices.

---

## 🏗️ Build From Source

### Prerequisites
*   [Go 1.24+](https://go.dev/dl/)
*   [Node.js 18+](https://nodejs.org/)
*   [Wails CLI](https://wails.io/) (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
*   *(Optional)* [NSIS](https://nsis.sourceforge.io/) (for building the installer)

### Build Commands

```powershell
# Clone repository
git clone https://github.com/HarshalPatel1972/aero.git
cd aero

# Development (Hot Reload)
wails dev

# Production Build (Auto-Magic)
# Builds Binary + Installer + Checksums
.\scripts\build_release.ps1
```

The version number lives in one place: `info.productVersion` in `wails.json`.

### Code Signing
Unsigned apps trigger Windows SmartScreen warnings. Buy a code-signing certificate, then set either `AERO_SIGN_PFX` + `AERO_SIGN_PASSWORD` or `AERO_SIGN_THUMBPRINT` before running the build script. See the header of `scripts/build_release.ps1`.

### Manual Artifact Generation
*   **Frontend** (required once in a fresh clone): `cd frontend; npm ci; npm run build`
*   **Binary Only**: `wails build -platform windows/amd64 -clean -ldflags "-s -w"`
*   **Binary + Installer**: `wails build -platform windows/amd64 -clean -ldflags "-s -w" -nsis`
*   **Icons**: `.\scripts\png2ico.ps1 -SourcePng "build\appicon.png" -DestIco "build\windows\icon.ico"`

---

## 🔧 Architecture

| Component | Tech Stack |
|-----------|------------|
| **Core** | Go (Golang) 1.24 |
| **GUI** | Wails v2 + React + TypeScript |
| **Styling** | TailwindCSS + Framer Motion |
| **Crypto** | XChaCha20-Poly1305: `@noble/ciphers` (phone) + `golang.org/x/crypto` (PC) |
| **Protocol** | Encrypted, authenticated chunked HTTP + WebSocket (see `internal/security/security.go`) |

---

## 📄 License

MIT License — See [LICENSE](LICENSE) for details.

<div align="center">
<b>Built for Speed. Designed for Privacy.</b>
</div>
