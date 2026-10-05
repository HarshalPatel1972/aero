// MIT License
// Copyright (c) 2026 Project AERO Contributors

package transfer

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/HarshalPatel1972/aero/internal/security"
)

const (
	minChunkSize = 64 << 10
	maxChunkSize = 16 << 20
	maxChunks    = 1 << 20

	// maxBuffered bounds the out-of-order chunks held in memory per upload.
	// Beyond it, chunks are written in place instead (slower on Windows, but
	// memory stays bounded).
	maxBuffered = 64 << 20
)

// upload is one phone-to-PC file in progress.
type upload struct {
	id        string
	name      string
	size      int64
	chunkSize int
	total     int
	part      string
	file      *os.File
	started   time.Time

	mu       sync.Mutex
	received []bool
	count    int
	bytes    int64
	active   time.Time
	lastEmit time.Time
	done     bool

	// Chunks are appended to the file strictly in order. Writing past the
	// end of a file makes NTFS zero-fill the gap first, which halved real
	// throughput, so early arrivals wait in `early` until their turn.
	next         int            // next chunk index to append
	early        map[int][]byte // decrypted chunks that arrived ahead of `next`
	earlyBytes   int
	writtenAhead map[int]bool // chunks written in place because the buffer was full
}

// store records chunk i and appends every chunk that is now contiguous.
// The caller must hold u.mu.
func (u *upload) store(i int, plain []byte) error {
	if i != u.next {
		if u.earlyBytes+len(plain) <= maxBuffered {
			u.early[i] = plain
			u.earlyBytes += len(plain)
			return nil
		}
		if _, err := u.file.WriteAt(plain, int64(i)*int64(u.chunkSize)); err != nil {
			return err
		}
		u.writtenAhead[i] = true
		return nil
	}
	if _, err := u.file.WriteAt(plain, int64(i)*int64(u.chunkSize)); err != nil {
		return err
	}
	u.next++
	for u.next < u.total {
		if buf, ok := u.early[u.next]; ok {
			if _, err := u.file.WriteAt(buf, int64(u.next)*int64(u.chunkSize)); err != nil {
				return err
			}
			delete(u.early, u.next)
			u.earlyBytes -= len(buf)
		} else if u.writtenAhead[u.next] {
			delete(u.writtenAhead, u.next)
		} else {
			break
		}
		u.next++
	}
	return nil
}

func (u *upload) lastActivity() time.Time {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.active
}

// chunkLen is the expected plaintext length of chunk i.
func (u *upload) chunkLen(i int) int64 {
	start := int64(i) * int64(u.chunkSize)
	return min(int64(u.chunkSize), u.size-start)
}

// abort closes and deletes the partial file. Callers must hold Server.mu or
// otherwise own the upload.
func (u *upload) abort() {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.done {
		return
	}
	u.done = true
	u.file.Close()
	os.Remove(u.part)
}

type uploadInitRequest struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	ChunkSize int    `json:"chunkSize"`
}

// handleUploadInit starts an upload. The body is a sealed JSON
// uploadInitRequest, so the file name never crosses the network in clear.
func (s *Server) handleUploadInit(w http.ResponseWriter, r *http.Request) {
	box, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	plain, err := s.session.Open(box, security.UploadInitAAD())
	if err != nil {
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}
	var req uploadInitRequest
	if err := json.Unmarshal(plain, &req); err != nil {
		http.Error(w, "invalid metadata", http.StatusBadRequest)
		return
	}
	if req.Size < 0 || req.ChunkSize < minChunkSize || req.ChunkSize > maxChunkSize {
		http.Error(w, "invalid size", http.StatusBadRequest)
		return
	}
	total := int(max(1, (req.Size+int64(req.ChunkSize)-1)/int64(req.ChunkSize)))
	if total > maxChunks {
		http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}

	s.mu.Lock()
	if len(s.uploads) >= maxActiveUploads {
		s.mu.Unlock()
		http.Error(w, "too many active uploads", http.StatusTooManyRequests)
		return
	}
	id := newID()
	part := s.partPath(id)
	f, err := os.OpenFile(part, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
	if err != nil {
		s.mu.Unlock()
		log.Printf("[AERO] Cannot create upload file: %v", err)
		http.Error(w, "cannot create file (is the disk full?)", http.StatusInsufficientStorage)
		return
	}
	now := time.Now()
	u := &upload{
		id: id, name: SanitizeFilename(req.Name), size: req.Size, chunkSize: req.ChunkSize,
		total: total, part: part, file: f, started: now,
		received: make([]bool, total), active: now,
		early: make(map[int][]byte), writtenAhead: make(map[int]bool),
	}
	s.uploads[id] = u
	s.mu.Unlock()

	log.Printf("[AERO] Receiving %q (%d bytes, %d chunks)", u.name, u.size, u.total)
	s.emit(Event{ID: u.id, Filename: u.name, Size: u.size, Status: "started", Direction: "receive"})
	writeJSON(w, map[string]any{"id": id, "totalChunks": total})
}

// handleUploadChunk writes one sealed chunk. Chunks may arrive in any order
// and in parallel; the file is finalized when the last one lands.
func (s *Server) handleUploadChunk(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	u := s.uploads[r.PathValue("id")]
	s.mu.Unlock()
	if u == nil {
		// Gone, not 404: the phone must stop rather than retry.
		http.Error(w, "upload cancelled or expired", http.StatusGone)
		return
	}
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || index < 0 || index >= u.total {
		http.Error(w, "invalid chunk index", http.StatusBadRequest)
		return
	}

	box, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(u.chunkSize)+security.Overhead))
	if err != nil {
		http.Error(w, "chunk too large", http.StatusRequestEntityTooLarge)
		return
	}
	plain, err := s.session.Open(box, security.UploadChunkAAD(u.id, index, u.total))
	if err != nil {
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}
	if int64(len(plain)) != u.chunkLen(index) {
		http.Error(w, "wrong chunk length", http.StatusBadRequest)
		return
	}

	u.mu.Lock()
	if u.done {
		u.mu.Unlock()
		http.Error(w, "upload cancelled", http.StatusGone)
		return
	}
	if !u.received[index] {
		if err := u.store(index, plain); err != nil {
			u.mu.Unlock()
			log.Printf("[AERO] Write failed for %q: %v", u.name, err)
			s.mu.Lock()
			delete(s.uploads, u.id)
			s.mu.Unlock()
			u.abort()
			s.emit(Event{ID: u.id, Filename: u.name, Size: u.size, Status: "error", Direction: "receive"})
			http.Error(w, "write failed (is the disk full?)", http.StatusInsufficientStorage)
			return
		}
		u.received[index] = true
		u.count++
		u.bytes += int64(len(plain))
	}
	now := time.Now()
	u.active = now
	complete := u.count == u.total
	if complete {
		u.done = true // claim finalization; later chunks are rejected
	}
	emitProgress := !complete && now.Sub(u.lastEmit) > 200*time.Millisecond
	if emitProgress {
		u.lastEmit = now
	}
	progress := float64(u.bytes) / float64(max(1, u.size)) * 100
	speed := formatSpeed(u.bytes, u.started)
	u.mu.Unlock()

	if emitProgress {
		s.emit(Event{ID: u.id, Filename: u.name, Size: u.size, Status: "progress", Progress: progress, Speed: speed, Direction: "receive"})
	}
	if !complete {
		writeJSON(w, map[string]any{"complete": false})
		return
	}

	s.mu.Lock()
	delete(s.uploads, u.id)
	s.mu.Unlock()

	finalName, err := s.finalize(u)
	if err != nil {
		log.Printf("[AERO] Finalize failed for %q: %v", u.name, err)
		os.Remove(u.part)
		s.emit(Event{ID: u.id, Filename: u.name, Size: u.size, Status: "error", Direction: "receive"})
		http.Error(w, "could not save file", http.StatusInternalServerError)
		return
	}
	log.Printf("[AERO] Saved %q (%s)", finalName, speed)
	s.emit(Event{ID: u.id, Filename: u.name, Size: u.size, Status: "completed", Progress: 100, Speed: speed, Direction: "receive"})
	writeJSON(w, map[string]any{"complete": true})
}

func (s *Server) finalize(u *upload) (string, error) {
	if err := u.file.Sync(); err != nil {
		u.file.Close()
		return "", err
	}
	if err := u.file.Close(); err != nil {
		return "", err
	}
	return moveToUniqueName(u.part, s.dir, u.name)
}

// handleUploadCancel discards an upload the phone gave up on.
func (s *Server) handleUploadCancel(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	u := s.uploads[r.PathValue("id")]
	delete(s.uploads, r.PathValue("id"))
	s.mu.Unlock()
	if u == nil {
		http.Error(w, "unknown upload", http.StatusNotFound)
		return
	}
	u.abort()
	s.emit(Event{ID: u.id, Filename: u.name, Size: u.size, Status: "cancelled", Direction: "receive"})
	log.Printf("[AERO] Upload %q cancelled on the phone; partial data deleted", u.name)
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil && !errors.Is(err, http.ErrHandlerTimeout) {
		log.Printf("[AERO] Response write failed: %v", err)
	}
}
