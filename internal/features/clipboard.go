// Package features provides advanced AERO capabilities.
// clipboard.go: Universal Clipboard ("The Beam")
//
// Term-Phase 6: MVP 1
// Enables instant text transfer between PC and Phone.
// Uses golang.design/x/clipboard for cross-platform system access.
//
// Security:
//   - Max 1MB text limit (prevent clipboard bombing)
//   - Control characters stripped
//   - No binary data allowed

package features

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"unicode"

	"golang.design/x/clipboard"
)

// ═══════════════════════════════════════════════════════════════
// CONSTANTS
// ═══════════════════════════════════════════════════════════════

const (
	// MaxClipboardSize prevents clipboard bombing (1MB)
	MaxClipboardSize = 1 * 1024 * 1024

	// ClipboardContentType for the API
	ClipboardContentType = "application/json"
)

// ═══════════════════════════════════════════════════════════════
// CLIPBOARD MANAGER
// ═══════════════════════════════════════════════════════════════

// ClipboardManager handles system clipboard operations.
type ClipboardManager struct {
	mu          sync.RWMutex
	initialized bool
	lastError   error
}

// clipboardResponse is the JSON response for clipboard API.
type clipboardResponse struct {
	Success bool   `json:"success"`
	Text    string `json:"text,omitempty"`
	Error   string `json:"error,omitempty"`
	Length  int    `json:"length,omitempty"`
}

// clipboardRequest is the JSON request for clipboard write.
type clipboardRequest struct {
	Text string `json:"text"`
}

// NewClipboardManager creates and initializes a ClipboardManager.
func NewClipboardManager() (*ClipboardManager, error) {
	cm := &ClipboardManager{}

	// Initialize clipboard access
	if err := clipboard.Init(); err != nil {
		cm.lastError = err
		log.Printf("[CLIPBOARD] ⚠️ Init failed: %v (clipboard features disabled)", err)
		return cm, nil // Return manager anyway, but degraded
	}

	cm.initialized = true
	log.Printf("[CLIPBOARD] ✅ System clipboard access enabled")
	return cm, nil
}

// ═══════════════════════════════════════════════════════════════
// CORE OPERATIONS
// ═══════════════════════════════════════════════════════════════

// Read gets the current text from the system clipboard.
func (cm *ClipboardManager) Read() (string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if !cm.initialized {
		return "", cm.lastError
	}

	// Read text from clipboard
	data := clipboard.Read(clipboard.FmtText)
	if data == nil {
		return "", nil // Empty clipboard
	}

	text := string(data)

	// Sanitize and limit
	text = sanitizeClipboardText(text)
	if len(text) > MaxClipboardSize {
		text = text[:MaxClipboardSize]
	}

	return text, nil
}

// Write sets text to the system clipboard.
func (cm *ClipboardManager) Write(text string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.initialized {
		return cm.lastError
	}

	// Sanitize and limit
	text = sanitizeClipboardText(text)
	if len(text) > MaxClipboardSize {
		text = text[:MaxClipboardSize]
	}

	// Write to clipboard
	clipboard.Write(clipboard.FmtText, []byte(text))

	log.Printf("[CLIPBOARD] 📋 Wrote %d bytes to clipboard", len(text))
	return nil
}

// IsAvailable returns true if clipboard access is working.
func (cm *ClipboardManager) IsAvailable() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.initialized
}

// ═══════════════════════════════════════════════════════════════
// SECURITY
// ═══════════════════════════════════════════════════════════════

// sanitizeClipboardText removes dangerous control characters.
// Preserves newlines and tabs (useful for code).
func sanitizeClipboardText(text string) string {
	var sb strings.Builder
	sb.Grow(len(text))

	for _, r := range text {
		// Allow printable characters, spaces, tabs, and newlines
		if unicode.IsPrint(r) || r == '\n' || r == '\r' || r == '\t' {
			sb.WriteRune(r)
		}
		// Skip control characters (except above)
	}

	return sb.String()
}

// ═══════════════════════════════════════════════════════════════
// HTTP HANDLER
// ═══════════════════════════════════════════════════════════════

// HandleClipboard is the HTTP handler for /api/clipboard.
// Methods:
//   - GET: Read current clipboard content
//   - POST: Write text to clipboard
func (cm *ClipboardManager) HandleClipboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", ClipboardContentType)

	switch r.Method {
	case http.MethodGet:
		cm.handleRead(w, r)
	case http.MethodPost:
		cm.handleWrite(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRead serves the current clipboard content.
func (cm *ClipboardManager) handleRead(w http.ResponseWriter, r *http.Request) {
	text, err := cm.Read()
	if err != nil {
		json.NewEncoder(w).Encode(clipboardResponse{
			Success: false,
			Error:   "Clipboard not available",
		})
		return
	}

	json.NewEncoder(w).Encode(clipboardResponse{
		Success: true,
		Text:    text,
		Length:  len(text),
	})
}

// handleWrite sets text to the clipboard.
func (cm *ClipboardManager) handleWrite(w http.ResponseWriter, r *http.Request) {
	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, MaxClipboardSize+1024) // Allow for JSON overhead

	// Parse request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		json.NewEncoder(w).Encode(clipboardResponse{
			Success: false,
			Error:   "Request too large",
		})
		return
	}

	var req clipboardRequest
	if err := json.Unmarshal(body, &req); err != nil {
		// Try plain text fallback
		req.Text = string(body)
	}

	if len(req.Text) > MaxClipboardSize {
		json.NewEncoder(w).Encode(clipboardResponse{
			Success: false,
			Error:   "Text exceeds 1MB limit",
		})
		return
	}

	if err := cm.Write(req.Text); err != nil {
		json.NewEncoder(w).Encode(clipboardResponse{
			Success: false,
			Error:   "Failed to write to clipboard",
		})
		return
	}

	json.NewEncoder(w).Encode(clipboardResponse{
		Success: true,
		Length:  len(req.Text),
	})
}
