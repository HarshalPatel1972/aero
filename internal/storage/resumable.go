// Package storage provides file I/O operations.
// resumable.go: Range header support ("Iron-Grip" Resumability)
//
// Term-Phase 6: MVP 3
// Implements RFC 7233 Range header support for resumable downloads.
// If WiFi flickers, the browser automatically resumes from where it left off.

package storage

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ═══════════════════════════════════════════════════════════════
// RESUMABLE FILE SERVER
// ═══════════════════════════════════════════════════════════════

// ServeFileResumable serves a file with Range header support.
// Uses http.ServeContent which handles all Range parsing automatically.
//
// This is the recommended way for file downloads as it:
//   - Handles Range: bytes=X-Y headers
//   - Supports If-Range conditional requests
//   - Sets proper Content-Type based on extension
//   - Handles HEAD requests
func ServeFileResumable(w http.ResponseWriter, r *http.Request, filePath string) error {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get file info for Content-Length and ModTime
	info, err := file.Stat()
	if err != nil {
		return err
	}

	// Get filename for Content-Disposition
	filename := filepath.Base(filePath)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	// Accept-Ranges tells the client we support partial content
	w.Header().Set("Accept-Ranges", "bytes")

	// ServeContent handles everything:
	// - Range header parsing
	// - 206 Partial Content response
	// - Content-Range header
	// - If-Range conditional requests
	http.ServeContent(w, r, filename, info.ModTime(), file)

	return nil
}

// ═══════════════════════════════════════════════════════════════
// MANUAL RANGE SUPPORT (For custom streaming scenarios)
// ═══════════════════════════════════════════════════════════════

// RangeInfo represents a parsed Range header.
type RangeInfo struct {
	Start  int64
	End    int64
	Length int64
}

// ParseRangeHeader parses a Range header like "bytes=1024-" or "bytes=0-1023".
// Returns nil if no valid range is found.
func ParseRangeHeader(rangeHeader string, fileSize int64) *RangeInfo {
	if rangeHeader == "" {
		return nil
	}

	// Must start with "bytes="
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return nil
	}

	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")

	// Handle "bytes=START-" (from START to end)
	if strings.HasSuffix(rangeSpec, "-") {
		startStr := strings.TrimSuffix(rangeSpec, "-")
		start, err := strconv.ParseInt(startStr, 10, 64)
		if err != nil || start < 0 || start >= fileSize {
			return nil
		}
		return &RangeInfo{
			Start:  start,
			End:    fileSize - 1,
			Length: fileSize - start,
		}
	}

	// Handle "bytes=START-END"
	parts := strings.Split(rangeSpec, "-")
	if len(parts) != 2 {
		return nil
	}

	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 {
		return nil
	}

	end, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || end < start || end >= fileSize {
		end = fileSize - 1
	}

	return &RangeInfo{
		Start:  start,
		End:    end,
		Length: end - start + 1,
	}
}

// ServeRangeManual serves a file with manual Range handling.
// Use this when you need custom streaming logic (e.g., with pooled buffers).
func ServeRangeManual(w http.ResponseWriter, r *http.Request, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	fileSize := info.Size()
	filename := filepath.Base(filePath)

	// Set common headers
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	// Check for Range header
	rangeHeader := r.Header.Get("Range")
	rangeInfo := ParseRangeHeader(rangeHeader, fileSize)

	if rangeInfo == nil {
		// No range - serve entire file
		w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))
		w.WriteHeader(http.StatusOK)
		io.Copy(w, file)
		return nil
	}

	// Seek to start position
	_, err = file.Seek(rangeInfo.Start, io.SeekStart)
	if err != nil {
		return err
	}

	// Set range response headers
	w.Header().Set("Content-Length", strconv.FormatInt(rangeInfo.Length, 10))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d",
		rangeInfo.Start, rangeInfo.End, fileSize))

	// 206 Partial Content
	w.WriteHeader(http.StatusPartialContent)

	// Copy only the requested range
	io.CopyN(w, file, rangeInfo.Length)

	return nil
}
