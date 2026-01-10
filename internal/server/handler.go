// Package server provides the HTTP/WebSocket server for AERO.
// handler.go: High-performance HTTP upload handler
//
// Term-Phase 3 & 4: Uses buffer pool, bandwidth tracker, and state manager.
// Speed Branch: Buffered disk I/O for 20+ MB/s throughput.
// Features:
//   - Context cancellation for "Panic Button" functionality
//   - Partial file cleanup on cancel
//   - Collision-safe filename resolution
//   - Buffered disk writes (256KB) for reduced syscalls

package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/username/aero/internal/state"
	"github.com/username/aero/internal/storage"
)

// ═══════════════════════════════════════════════════════════════
// FORTRESS UPLOAD HANDLER
// ═══════════════════════════════════════════════════════════════

// handleStreamUploadWithState handles uploads with full state management.
// This is the "Fortress" handler that integrates:
//   - TransferManager for state control
//   - Context cancellation (the "Panic Button")
//   - Pooled buffers for zero allocation
//   - Collision-safe filename resolution
//   - Partial file cleanup on cancel
func handleStreamUploadWithState(
	w http.ResponseWriter,
	r *http.Request,
	tm *state.TransferManager,
	uploadDir string,
	ctx context.Context,
) {
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

	// Get file size
	var totalSize int64
	if sizeStr := r.Header.Get("X-Filesize"); sizeStr != "" {
		totalSize, _ = strconv.ParseInt(sizeStr, 10, 64)
	}
	if totalSize == 0 {
		totalSize = r.ContentLength
	}

	// Generate unique transfer ID
	transferID := fmt.Sprintf("upload-%d", os.Getpid())

	// ═══════════════════════════════════════════════════════════
	// STATE: Attempt to start transfer
	// ═══════════════════════════════════════════════════════════

	transferCtx, err := tm.StartTransfer(transferID, filename, totalSize, "receive")
	if err != nil {
		if err == state.ErrTransferInProgress {
			http.Error(w, "Another transfer is in progress", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to start transfer", http.StatusInternalServerError)
		return
	}

	// Ensure we finish the transfer (success or failure)
	var transferSuccess bool
	defer func() {
		tm.FinishTransfer(transferSuccess)
	}()

	log.Printf("[AERO] 📥 Receiving: %s (%d bytes)", filename, totalSize)

	// ═══════════════════════════════════════════════════════════
	// FILE: Resolve collision and create partial file
	// ═══════════════════════════════════════════════════════════

	// Get unique filename (thread-safe collision resolution)
	finalFilename := storage.ResolveFilename(uploadDir, filename)
	partPath := fmt.Sprintf("%s/%s.part", uploadDir, finalFilename)
	finalPath := fmt.Sprintf("%s/%s", uploadDir, finalFilename)

	// Record partial file for cleanup on cancel
	tm.SetPartialFile(partPath)

	// Create partial file
	partFile, err := os.Create(partPath)
	if err != nil {
		log.Printf("[AERO] ❌ Failed to create file: %v", err)
		http.Error(w, "Failed to create file", http.StatusInternalServerError)
		return
	}

	// Wrap with buffered writer for faster disk I/O (256KB buffer)
	// This batches multiple small writes into larger disk operations
	bufferedWriter := bufio.NewWriterSize(partFile, BufferSize)

	// Cleanup on any exit
	defer func() {
		bufferedWriter.Flush() // Flush remaining data
		partFile.Close()
		if !transferSuccess {
			// Delete partial file on failure/cancel
			storage.DeletePartialFile(partPath)
		}
	}()

	// ═══════════════════════════════════════════════════════════
	// I/O: Copy with context cancellation (THE PANIC BUTTON)
	// ═══════════════════════════════════════════════════════════

	// Get pooled buffer
	buf := GetBuffer()
	defer PutBuffer(buf)

	var written int64
	var checkCounter int64 // For periodic cancellation checks
	const checkInterval = 1024 * 1024 // Check cancellation every 1MB

	tracker := NewTransferTracker(ctx, r.Body, totalSize, finalFilename, "receive")
	tracker.EmitStart()

	for {
		// ── CHECK CANCELLATION (every 1MB to reduce overhead) ───
		// This is the "Panic Button" check. At 20 MB/s, 1MB = 50ms latency.
		if written-checkCounter >= checkInterval {
			checkCounter = written
			select {
			case <-transferCtx.Done():
				log.Printf("[AERO] ⚠️ Transfer cancelled: %s", finalFilename)
				tracker.EmitError()
				http.Error(w, "Transfer cancelled", http.StatusRequestTimeout)
				return
			default:
			}
		}

		// ── READ ────────────────────────────────────────────────
		n, readErr := tracker.Read(*buf)
		if n > 0 {
			// ── WRITE (buffered) ────────────────────────────────
			nw, writeErr := bufferedWriter.Write((*buf)[:n])
			if nw > 0 {
				written += int64(nw)
			}
			if writeErr != nil {
				log.Printf("[AERO] ❌ Write error: %v", writeErr)
				tracker.EmitError()
				http.Error(w, "Write error", http.StatusInternalServerError)
				return
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			log.Printf("[AERO] ❌ Read error: %v", readErr)
			tracker.EmitError()
			http.Error(w, "Read error", http.StatusInternalServerError)
			return
		}
	}

	// ═══════════════════════════════════════════════════════════
	// FINALIZE: Flush buffer, close file, rename .part to final name
	// ═══════════════════════════════════════════════════════════

	bufferedWriter.Flush() // Ensure all data written
	partFile.Close()       // Close before rename (required on Windows)

	if err := os.Rename(partPath, finalPath); err != nil {
		log.Printf("[AERO] ❌ Rename failed: %v", err)
		tracker.EmitError()
		http.Error(w, "Failed to finalize file", http.StatusInternalServerError)
		return
	}

	// SUCCESS!
	transferSuccess = true

	log.Printf("[AERO] ✅ Saved: %s (%.2f MB @ %.1f MB/s)",
		finalFilename,
		float64(written)/(1024*1024),
		tracker.SpeedMBps())

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"success","filename":"` + finalFilename + `"}`))
}

// ═══════════════════════════════════════════════════════════════
// LEGACY HANDLERS (Kept for backward compatibility)
// ═══════════════════════════════════════════════════════════════

// handleStreamUpload handles direct binary uploads (from sender.ts BULLET/TRAIN modes).
// Uses pooled buffers and bandwidth tracking.
// NOTE: This is the non-stateful version. Use handleStreamUploadWithState for full features.
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
	
	// Buffered writer for performance
	bufWriter := bufio.NewWriterSize(writer, BufferSize)
	
	defer func() {
		bufWriter.Flush()
		writer.Close()
	}()

	// Copy using pooled buffer - ZERO ALLOCATION
	buf := GetBuffer()
	defer PutBuffer(buf)

	var written int64
	for {
		n, readErr := tracker.Read(*buf)
		if n > 0 {
			nw, writeErr := bufWriter.Write((*buf)[:n])
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
