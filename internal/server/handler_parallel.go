// Package server provides the HTTP/WebSocket server for AERO.
// handler_parallel.go: High-speed parallel upload handler
//
// Speed Phase 3: Multi-stream architecture for 50+ MB/s transfers

package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// UploadSession manages a parallel chunked upload
type UploadSession struct {
	Filename     string
	TotalChunks  int
	TotalSize    int64
	File         *os.File
	FilePath     string
	ReceivedMask []bool // Track which chunks have arrived
	StartTime    time.Time
	mu           sync.Mutex
}

// SessionManager holds active upload sessions
type SessionManager struct {
	sessions map[string]*UploadSession
	mu       sync.RWMutex
}

var sessionManager = &SessionManager{
	sessions: make(map[string]*UploadSession),
}

// GetOrCreateSession retrieves an existing session or creates a new one
func (sm *SessionManager) GetOrCreateSession(sessionID, filename string, totalChunks int, totalSize int64, uploadDir string) (*UploadSession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, exists := sm.sessions[sessionID]; exists {
		return session, nil
	}

	// Create new session
	finalPath := filepath.Join(uploadDir, filepath.Clean(filename))
	partPath := finalPath + ".part"

	// Create sparse file (pre-allocated)
	file, err := os.Create(partPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	// Pre-allocate file size (creates sparse file on supported filesystems)
	if err := file.Truncate(totalSize); err != nil {
		file.Close()
		os.Remove(partPath)
		return nil, fmt.Errorf("failed to allocate file: %w", err)
	}

	session := &UploadSession{
		Filename:     filename,
		TotalChunks:  totalChunks,
		TotalSize:    totalSize,
		File:         file,
		FilePath:     finalPath,
		ReceivedMask: make([]bool, totalChunks),
		StartTime:    time.Now(),
	}

	sm.sessions[sessionID] = session
	log.Printf("[AERO] 🚀 New parallel session: %s (%d chunks, %d bytes)", filename, totalChunks, totalSize)
	
	return session, nil
}

// IsNewSession checks if this is the first chunk (for start event)
func (sm *SessionManager) IsNewSession(sessionID string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	_, exists := sm.sessions[sessionID]
	return !exists
}

// MarkChunkComplete marks a chunk as received
func (s *UploadSession) MarkChunkComplete(chunkIndex int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ReceivedMask[chunkIndex] = true
}

// IsComplete checks if all chunks have been received
func (s *UploadSession) IsComplete() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for i, received := range s.ReceivedMask {
		if !received {
			log.Printf("[AERO] ⚠️  Chunk %d not received yet", i)
			return false
		}
	}
	return true
}

// GetMissingChunks returns indices of chunks not yet received
func (s *UploadSession) GetMissingChunks() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	missing := []int{}
	for i, received := range s.ReceivedMask {
		if !received {
			missing = append(missing, i)
		}
	}
	return missing
}

// Finalize completes the upload (atomic rename)
func (s *UploadSession) Finalize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Sync to disk
	if err := s.File.Sync(); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	// Close file
	if err := s.File.Close(); err != nil {
		return fmt.Errorf("close failed: %w", err)
	}

	// Atomic rename
	partPath := s.FilePath + ".part"
	if err := os.Rename(partPath, s.FilePath); err != nil {
		return fmt.Errorf("rename failed: %w", err)
	}

	elapsed := time.Since(s.StartTime).Seconds()
	speedMBps := (float64(s.TotalSize) / (1024 * 1024)) / elapsed

	log.Printf("[AERO] ✅ Parallel upload complete: %s (%.2f MB @ %.1f MB/s)",
		s.Filename,
		float64(s.TotalSize)/(1024*1024),
		speedMBps)

	return nil
}

// Cleanup removes the session and cleans up files on error
func (s *UploadSession) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.File != nil {
		s.File.Close()
	}
	os.Remove(s.FilePath + ".part")
}

// RemoveSession removes a session from the manager
func (sm *SessionManager) RemoveSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, sessionID)
}

// handleParallelUpload handles chunked parallel uploads
func (s *Server) handleParallelUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract headers
	sessionID := r.Header.Get("X-Session-Id")
	filename := r.Header.Get("X-Filename")
	chunkIndexStr := r.Header.Get("X-Chunk-Index")
	totalChunksStr := r.Header.Get("X-Total-Chunks")
	offsetStr := r.Header.Get("X-Offset")
	totalSizeStr := r.Header.Get("X-Total-Size")

	// Validate headers
	if sessionID == "" || filename == "" || chunkIndexStr == "" || totalChunksStr == "" || offsetStr == "" || totalSizeStr == "" {
		http.Error(w, "Missing required headers", http.StatusBadRequest)
		return
	}

	// Parse values
	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil {
		http.Error(w, "Invalid chunk index", http.StatusBadRequest)
		return
	}

	totalChunks, err := strconv.Atoi(totalChunksStr)
	if err != nil {
		http.Error(w, "Invalid total chunks", http.StatusBadRequest)
		return
	}

	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid offset", http.StatusBadRequest)
		return
	}

	totalSize, err := strconv.ParseInt(totalSizeStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid total size", http.StatusBadRequest)
		return
	}

	// Get or create session
	session, err := sessionManager.GetOrCreateSession(sessionID, filename, totalChunks, totalSize, s.config.UploadDir)
	if err != nil {
		log.Printf("[AERO] ❌ Session creation failed: %v", err)
		http.Error(w, "Session creation failed", http.StatusInternalServerError)
		return
	}

	// Emit start event on first chunk
	if chunkIndex == 0 {
		s.emitTransferEvent(TransferEvent{
			Filename:  filename,
			Status:    "started",
			Progress:  0,
			Direction: "receive",
		})
	}

	// Write chunk at offset using io.NewOffsetWriter (Go 1.20+)
	offsetWriter := &offsetWriter{file: session.File, offset: offset}
	
	// Use pooled buffer for copy
	buf := GetBuffer()
	defer PutBuffer(buf)

	written, err := io.CopyBuffer(offsetWriter, r.Body, *buf)
	if err != nil {
		log.Printf("[AERO] ❌ Chunk write failed: %v", err)
		http.Error(w, "Chunk write failed", http.StatusInternalServerError)
		return
	}

	// CRITICAL VALIDATION: Check if all bytes were written
	if chunkIndex < totalChunks-1 && written != 5*1024*1024 {
		log.Printf("[AERO] ⚠️  WARNING: Chunk %d incomplete! Expected 5MB, got %d bytes", chunkIndex, written)
	}

	// Mark chunk as complete
	session.MarkChunkComplete(chunkIndex)

	log.Printf("[AERO] 📦 Chunk %d/%d received (%d bytes)", chunkIndex+1, totalChunks, written)

	// Emit progress event for desktop UI
	progress := (float64(chunkIndex+1) / float64(totalChunks)) * 100
	s.emitTransferEvent(TransferEvent{
		Filename:  filename,
		Status:    "progress",
		Progress:  progress,
		Direction: "receive",
	})

	// Check if upload is complete
	if session.IsComplete() {
		// FINAL VALIDATION: Check all chunks before finalize
		missingChunks := session.GetMissingChunks()
		if len(missingChunks) > 0 {
			log.Printf("[AERO] ❌ CRITICAL: IsComplete=true but missing chunks: %v", missingChunks)
			http.Error(w, "Missing chunks", http.StatusInternalServerError)
			return
		}

		log.Printf("[AERO] ✅ All %d chunks received. Finalizing...", totalChunks)

		if err := session.Finalize(); err != nil {
			log.Printf("[AERO] ❌ Finalization failed: %v", err)
			session.Cleanup()
			sessionManager.RemoveSession(sessionID)
			http.Error(w, "Finalization failed", http.StatusInternalServerError)
			return
		}

		sessionManager.RemoveSession(sessionID)

		// Emit completion event
		s.emitTransferEvent(TransferEvent{
			Filename:  filename,
			Status:    "completed",
			Progress:  100,
			Direction: "receive",
		})
	}

	// Send success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "success",
		"chunk":    chunkIndex,
		"complete": session.IsComplete(),
	})
}

// offsetWriter implements io.Writer with offset support (for Go < 1.20 compatibility)
type offsetWriter struct {
	file   *os.File
	offset int64
}

func (ow *offsetWriter) Write(p []byte) (n int, err error) {
	n, err = ow.file.WriteAt(p, ow.offset)
	ow.offset += int64(n) // CRITICAL: Increment offset to prevent overwrites
	return n, err
}
