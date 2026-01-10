// Package server provides the HTTP/WebSocket server for AERO.
// handler.go: High-performance HTTP upload handler
//
// Term-Phase 3: Uses buffer pool + bandwidth tracker
// Memory efficient: Uses sync.Pool buffers, never exceeds 50MB RAM

package server

import (
	"io"
	"log"
	"net/http"
	"strconv"
)

// handleStreamUpload handles direct binary uploads (from sender.ts BULLET/TRAIN modes).
// Uses pooled buffers and bandwidth tracking.
func (s *Server) handleStreamUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get metadata from headers
	filename := r.Header.Get("X-Filename")
	if filename == "" {
		http.Error(w, "Missing X-Filename header", http.StatusBadRequest)
		return
	}

	// URL decode filename
	if decoded, err := strconv.Unquote(`"` + filename + `"`); err == nil {
		filename = decoded
	}

	// Get file size for progress tracking
	var totalSize int64
	if sizeStr := r.Header.Get("X-Filesize"); sizeStr != "" {
		totalSize, _ = strconv.ParseInt(sizeStr, 10, 64)
	}
	if totalSize == 0 {
		totalSize = r.ContentLength
	}

	log.Printf("[AERO] 📥 Receiving: %s (%d bytes)", filename, totalSize)

	// Emit start event
	s.emitTransferEvent(TransferEvent{
		Filename:  filename,
		Status:    "started",
		Progress:  0,
		Direction: "receive",
	})

	// Wrap body with bandwidth tracker
	tracker := NewTransferTracker(s.ctx, r.Body, totalSize, filename, "receive")
	tracker.EmitStart()

	// Create destination file
	writer, err := s.storageService.CreateWriter(filename)
	if err != nil {
		log.Printf("[AERO] ❌ Failed to create file: %v", err)
		http.Error(w, "Failed to create file", http.StatusInternalServerError)
		tracker.EmitError()
		return
	}
	defer writer.Close()

	// Copy using pooled buffer - ZERO ALLOCATION
	buf := GetBuffer()
	defer PutBuffer(buf)

	var written int64
	for {
		n, readErr := tracker.Read(*buf)
		if n > 0 {
			nw, writeErr := writer.Write((*buf)[:n])
			if nw > 0 {
				written += int64(nw)
			}
			if writeErr != nil {
				log.Printf("[AERO] ❌ Write error: %v", writeErr)
				http.Error(w, "Write error", http.StatusInternalServerError)
				tracker.EmitError()
				return
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			log.Printf("[AERO] ❌ Read error: %v", readErr)
			http.Error(w, "Read error", http.StatusInternalServerError)
			tracker.EmitError()
			return
		}
	}

	// Success
	log.Printf("[AERO] ✅ Saved: %s (%.2f MB @ %.1f MB/s)",
		filename,
		float64(written)/(1024*1024),
		tracker.SpeedMBps())

	s.emitTransferEvent(TransferEvent{
		Filename:  filename,
		Status:    "completed",
		Progress:  100,
		Speed:     formatSpeedMBps(tracker.SpeedMBps()),
		Direction: "receive",
	})

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"success"}`))
}

// formatSpeedMBps formats speed as human readable
func formatSpeedMBps(mbps float64) string {
	if mbps >= 1 {
		return strconv.FormatFloat(mbps, 'f', 1, 64) + " MB/s"
	}
	return strconv.FormatFloat(mbps*1024, 'f', 1, 64) + " KB/s"
}

// handleStreamUploadAdaptive is a variant that uses adaptive buffer sizing.
// It starts with medium buffers and adjusts based on throughput.
func (s *Server) handleStreamUploadAdaptive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filename := r.Header.Get("X-Filename")
	if filename == "" {
		http.Error(w, "Missing X-Filename header", http.StatusBadRequest)
		return
	}

	var totalSize int64
	if sizeStr := r.Header.Get("X-Filesize"); sizeStr != "" {
		totalSize, _ = strconv.ParseInt(sizeStr, 10, 64)
	}

	log.Printf("[AERO] 📥 Adaptive receiving: %s", filename)

	tracker := NewTransferTracker(s.ctx, r.Body, totalSize, filename, "receive")
	tracker.EmitStart()

	writer, err := s.storageService.CreateWriter(filename)
	if err != nil {
		http.Error(w, "Failed to create file", http.StatusInternalServerError)
		return
	}
	defer writer.Close()

	// Adaptive buffer management
	currentSpeed := 0.0
	var written int64

	for {
		// Get appropriately sized buffer based on current speed
		buf, bufSize := GetAdaptiveBuffer(currentSpeed)

		n, readErr := tracker.Read(*buf)
		if n > 0 {
			nw, writeErr := writer.Write((*buf)[:n])
			written += int64(nw)
			if writeErr != nil {
				PutAdaptiveBuffer(buf, bufSize)
				tracker.EmitError()
				http.Error(w, "Write error", http.StatusInternalServerError)
				return
			}
		}

		// Return buffer to pool ASAP
		PutAdaptiveBuffer(buf, bufSize)

		// Update speed measurement
		currentSpeed = tracker.SpeedMBps()

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			tracker.EmitError()
			http.Error(w, "Read error", http.StatusInternalServerError)
			return
		}
	}

	log.Printf("[AERO] ✅ Saved: %s (%.2f MB)", filename, float64(written)/(1024*1024))
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"success"}`))
}
