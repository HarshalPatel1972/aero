// MIT License
// Copyright (c) 2026 Project AERO Contributors

package transfer

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/username/aero/internal/security"
)

// download is a file the desktop user explicitly chose to send to the phone.
// Only files registered here can be read by the phone; there is no endpoint
// that accepts an arbitrary path.
type download struct {
	id        string
	name      string
	path      string
	size      int64
	modTime   time.Time
	chunkSize int
	total     int
	started   time.Time

	mu        sync.Mutex
	served    []bool
	count     int
	bytes     int64
	active    time.Time
	lastEmit  time.Time
	completed bool
}

func (d *download) lastActivity() time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.active
}

// ErrNoPhone is returned by OfferFile when no phone is connected.
var ErrNoPhone = errors.New("no phone connected")

// OfferFile registers path for download and tells every connected phone
// about it. The phone then fetches it chunk by chunk.
func (s *Server) OfferFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	if s.PhoneCount() == 0 {
		return ErrNoPhone
	}

	now := time.Now()
	total := int(max(1, (info.Size()+downloadChunkSize-1)/downloadChunkSize))
	d := &download{
		id: newID(), name: filepath.Base(path), path: path, size: info.Size(), modTime: info.ModTime(),
		chunkSize: downloadChunkSize, total: total, started: now,
		served: make([]bool, total), active: now,
	}

	s.mu.Lock()
	s.downloads[d.id] = d
	s.mu.Unlock()

	s.broadcast(map[string]any{
		"type": "offer", "id": d.id, "name": d.name, "size": d.size,
		"chunkSize": d.chunkSize, "totalChunks": d.total,
	})
	log.Printf("[AERO] Offered %q to phone (%d bytes)", d.name, d.size)
	s.emit(Event{Filename: d.name, Status: "started", Direction: "send"})
	return nil
}

// handleDownloadChunk returns one sealed chunk of an offered file.
func (s *Server) handleDownloadChunk(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	d := s.downloads[r.PathValue("id")]
	s.mu.Unlock()
	if d == nil {
		http.Error(w, "unknown or expired download", http.StatusNotFound)
		return
	}
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || index < 0 || index >= d.total {
		http.Error(w, "invalid chunk index", http.StatusBadRequest)
		return
	}

	f, err := os.Open(d.path)
	if err != nil {
		http.Error(w, "file no longer available", http.StatusGone)
		return
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || info.Size() != d.size || !info.ModTime().Equal(d.modTime) {
		http.Error(w, "file changed since it was offered", http.StatusConflict)
		return
	}

	offset := int64(index) * int64(d.chunkSize)
	n := min(int64(d.chunkSize), d.size-offset)
	buf := make([]byte, n)
	if _, err := io.ReadFull(io.NewSectionReader(f, offset, n), buf); err != nil {
		http.Error(w, "read failed", http.StatusInternalServerError)
		return
	}
	box, err := s.session.Seal(buf, security.DownloadChunkAAD(d.id, index, d.total))
	if err != nil {
		http.Error(w, "encryption failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(box)))
	if _, err := w.Write(box); err != nil {
		return // phone went away; it will retry
	}

	d.mu.Lock()
	if !d.served[index] {
		d.served[index] = true
		d.count++
		d.bytes += n
	}
	now := time.Now()
	d.active = now
	complete := d.count == d.total && !d.completed
	if complete {
		d.completed = true
	}
	emitProgress := !d.completed && now.Sub(d.lastEmit) > 200*time.Millisecond
	if emitProgress {
		d.lastEmit = now
	}
	progress := float64(d.bytes) / float64(max(1, d.size)) * 100
	speed := formatSpeed(d.bytes, d.started)
	d.mu.Unlock()

	switch {
	case complete:
		s.emit(Event{Filename: d.name, Status: "completed", Progress: 100, Speed: speed, Direction: "send"})
	case emitProgress:
		s.emit(Event{Filename: d.name, Status: "progress", Progress: progress, Speed: speed, Direction: "send"})
	}
}
