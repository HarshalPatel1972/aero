// Package server provides the HTTP/WebSocket server for AERO.
// trinity_handlers.go: Handlers for Term-Phase 6 Trinity Features
//
// Features:
//   - Clipboard API (The Beam)
//   - Folder streaming (Pack & Go)
//   - Resumable file download (Iron-Grip)

package server

import (
	"log"
	"net/http"
	"sync"

	"github.com/username/aero/internal/features"
	"github.com/username/aero/internal/storage"
)

// ═══════════════════════════════════════════════════════════════
// CLIPBOARD SINGLETON
// ═══════════════════════════════════════════════════════════════

var (
	clipboardManager     *features.ClipboardManager
	clipboardManagerOnce sync.Once
)

// getClipboardManager returns the shared clipboard manager.
func getClipboardManager() *features.ClipboardManager {
	clipboardManagerOnce.Do(func() {
		var err error
		clipboardManager, err = features.NewClipboardManager()
		if err != nil {
			log.Printf("[CLIPBOARD] ⚠️ Failed to initialize: %v", err)
		}
	})
	return clipboardManager
}

// ═══════════════════════════════════════════════════════════════
// HANDLERS
// ═══════════════════════════════════════════════════════════════

// handleClipboard handles /api/clipboard requests.
// GET: Read current clipboard content
// POST: Write text to clipboard
func (s *Server) handleClipboard(w http.ResponseWriter, r *http.Request) {
	cm := getClipboardManager()
	if cm == nil {
		http.Error(w, "Clipboard not available", http.StatusServiceUnavailable)
		return
	}
	cm.HandleClipboard(w, r)
}

// handleFolderDownload handles /api/folder?path=/path/to/folder requests.
// Streams folder as zip without creating temp file.
func (s *Server) handleFolderDownload(w http.ResponseWriter, r *http.Request) {
	storage.HandleFolderDownload(w, r)
}

// handleFileDownload handles /api/file?path=/path/to/file requests.
// Supports Range headers for resumable downloads.
func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "Missing path parameter", http.StatusBadRequest)
		return
	}

	log.Printf("[DOWNLOAD] 📄 Serving: %s", filePath)

	if err := storage.ServeFileResumable(w, r, filePath); err != nil {
		log.Printf("[DOWNLOAD] ❌ Error: %v", err)
		http.Error(w, "File not found", http.StatusNotFound)
	}
}
