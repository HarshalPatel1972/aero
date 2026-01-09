# AERO Development Journey

> **"We might fail but I wanna find a solution at any cost"** — The spirit of this project

## 🎯 Project Goal
Build a **free, fast, and 100% safe** wireless file transfer application that works from phone to PC via QR code scanning.

---

## 📅 Development Timeline

### Phase 1: Foundation (Initial Setup)
**Status:** ✅ Complete

- Set up Wails v2 desktop application framework
- Created Go backend with HTTP server
- Built React + TypeScript frontend
- Implemented QR code generation for easy connection

---

### Phase 2: End-to-End Encryption Attempt
**Status:** ❌ Failed → Pivoted

#### What We Tried
- Implemented AES-256-CTR encryption in pure JavaScript
- Key exchange via URL hash fragment (optical-key protocol)
- Server-side decryption with Go crypto

#### Why It Failed
| Issue | Cause |
|-------|-------|
| WebCrypto API blocked | HTTP on LAN IPs = not a "secure context" |
| Pure JS AES too slow | 300KB/s-1MB/s — unusable for large files |
| Key mismatch bugs | Different keys on encrypt vs decrypt |

#### Lesson Learned
> Browser-based encryption over HTTP is fundamentally limited. For LAN transfers on trusted networks, encryption adds overhead without meaningful security benefit.

---

### Phase 3: Speed Optimization Journey
**Status:** 🔄 Iterative Improvements

#### Attempt 1: Simple HTTP Upload
- **Speed:** 10-15 Mbps
- **Problem:** Single connection, multipart overhead

#### Attempt 2: Parallel WebSockets (6 connections)
- **Speed:** 18-30 Mbps
- **Problem:** Race conditions, chunk corruption

#### Attempt 3: Parallel WebSockets (8 connections) + Larger Chunks
- **Speed:** 30 Mbps peak
- **Problem:** `.part` files not renamed, data corruption on large files

#### Attempt 4: WriteAt() Direct Disk
- **Speed:** Fast
- **Problem:** Offset calculation errors, missing chunks, corrupted video files

#### Root Cause Analysis
```
Why parallel writes failed:
1. Multiple goroutines writing to same file → race conditions
2. WriteAt(offset) math errors → bytes in wrong positions
3. Pre-allocated files (Truncate) → empty .part files on failure
4. WebSocket "done" message unreliable → finalization never triggered
```

---

### Phase 4: Chunk File Assembly Architecture
**Status:** ✅ Working

#### The Fix
Instead of writing directly to one file:
```
uploads/.tmp/{fileId}/
  ├── 0.chunk
  ├── 1.chunk
  ├── 2.chunk
  └── meta.json

→ Assemble on completion → final file
```

#### Why This Works
- Each chunk is a separate file (no race conditions)
- Assembly is sequential and verified
- Missing chunks detected before finalization

---

### Phase 5: ACK-Based Reliability
**Status:** ✅ Working

#### Sequential ACK (Baseline)
- **Speed:** 2-3 Mbps
- **Reliability:** 100%
- **Method:** Send chunk → wait for ACK → send next

#### Windowed ACK (Current)
- **Speed:** Target 16-24 Mbps
- **Reliability:** 100%
- **Method:** Send 8 chunks → collect ACKs → continue

---

## 🏆 Achievements

| Milestone | Date | Notes |
|-----------|------|-------|
| First successful transfer | Day 1 | Small files only |
| 30 Mbps peak speed | Day 1 | But corrupted files |
| 100% reliable transfers | Day 1 | Via ACK protocol |
| Chunk assembly working | Day 1 | Any file size |

---

## 📊 Speed Comparison

| Approach | Speed | Reliability |
|----------|-------|-------------|
| HTTP Multipart | 10-15 Mbps | Medium |
| Parallel WS (broken) | 30 Mbps | Low |
| Sequential ACK | 2-3 Mbps | 100% |
| Windowed ACK | TBD | 100% |
| Target | 100 Mbps | 100% |

---

## 🔧 Technical Decisions

### Why WebSocket over HTTP?
- Bidirectional communication for ACKs
- Lower overhead for many small messages
- Persistent connection reduces latency

### Why Chunk Files over WriteAt?
- No concurrent write issues
- Easy verification (count files)
- Resumable (chunks persist on failure)

### Why Not Encryption?
- LAN = trusted network
- Speed is priority
- Can add HTTPS later for untrusted networks

---

## 🎓 Lessons Learned

1. **Start simple, add complexity carefully**
   - Sequential ACK worked first try
   - Parallel attempts failed multiple times

2. **Verify data integrity at every step**
   - Chunk counting before assembly
   - Size verification after assembly

3. **Browser limitations are real**
   - WebCrypto needs HTTPS
   - Memory limits for large files
   - Single-threaded JS

4. **The best code is boring code**
   - Simple file-per-chunk vs clever WriteAt()
   - Explicit ACKs vs fire-and-forget

---

## 🚀 Future Goals

- [ ] Reach 50+ Mbps sustained
- [ ] Reach 100 Mbps target
- [ ] Add optional HTTPS for untrusted networks
- [ ] Multi-file batch transfers
- [ ] Resume interrupted transfers
- [ ] Cross-platform testing (macOS, Linux)

---

## 📁 Project Structure

```
aero/
├── app.go              # Wails app bridge
├── main.go             # Entry point
├── internal/
│   ├── server/         # HTTP/WebSocket server
│   │   ├── server.go   # Transfer logic
│   │   └── templates/  # Web UI
│   ├── storage/        # File I/O
│   ├── security/       # Crypto (unused for now)
│   └── config/         # Configuration
├── pkg/
│   └── networking/     # IP detection
└── frontend/           # React UI for desktop
```

---

*"We failed many times, but we kept building. That's what matters."*
