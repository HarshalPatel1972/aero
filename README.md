<div align="center">

<img src="build/appicon.png" width="128" height="128" alt="Aero Logo">

# AERO

**Next-Gen Local File Transfer**
Phone ↔ PC • Encrypted • Blazing Fast

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev)
[![Platform](https://img.shields.io/badge/Platform-Windows-blue?logo=windows)](https://github.com/HarshalPatel1972/aero/releases)

</div>

---

## 🚀 Features

*   **⚡ Blazing Fast**: 40-100+ MB/s over LAN (device dependent).
*   **🔒 Zero-Trust Security**: End-to-End Encryption (AES-256-CTR) with P-521 Curve.
*   **📱 Universal Client**: Works on **any** device (iOS, Android, Mac, Linux) via browser. No app install required on the phone.
*   **📎 Universal Clipboard**: Instantly share text/links between PC and Phone.
*   **✨ Modern UI**: Glassmorphism design, "Mini-Mode" for unobtrusive multitasking, and fluid animations.
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

AERO assumes your local network is **hostile**.

1.  **Session Key**: Generated locally on launch (never sent to any cloud).
2.  **Optical Exchange**: The key is transferred via QR code hash fragment (`#key`).
3.  **E2EE**: Files are encrypted **in the browser** before they leave your phone.
4.  **No Storage**: Keys are wiped from memory on exit.

> **Privacy Promise**: No cloud. No tracking. No history. Your data never leaves your local network.

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

### Manual Artifact Generation
*   **Binary Only**: `wails build -platform windows/amd64 -clean -ldflags "-s -w"`
*   **Installer**: `makensis build\windows\installer.nsi`
*   **Icons**: `.\scripts\png2ico.ps1 -SourcePng "build\appicon.png" -DestIco "build\windows\icon.ico"`

---

## 🔧 Architecture

| Component | Tech Stack |
|-----------|------------|
| **Core** | Go (Golang) 1.24 |
| **GUI** | Wails v2 + React + TypeScript |
| **Styling** | TailwindCSS + Framer Motion |
| **Crypto** | WebCrypto API (Frontend) + `crypto/cipher` (Backend) |
| **Protocol** | Custom HTTP Multipart Stream over TCP |

---

## 📄 License

MIT License — See [LICENSE](LICENSE) for details.

<div align="center">
<b>Built for Speed. Designed for Privacy.</b>
</div>
