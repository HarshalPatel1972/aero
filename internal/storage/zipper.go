// Package storage provides file I/O operations.
// zipper.go: On-the-fly folder streaming ("Pack & Go")
//
// Term-Phase 6: MVP 2
// Creates a zip stream directly to the HTTP response writer.
// No temporary file on disk - instant streaming.
//
// Performance:
//   - Store (no compression) for media files
//   - Deflate for text/code files
//   - Uses pooled buffers for zero-copy streaming

package storage

import (
	"archive/zip"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ═══════════════════════════════════════════════════════════════
// CONSTANTS
// ═══════════════════════════════════════════════════════════════

// mediaExtensions use Store (no compression) - already compressed
var mediaExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".webm": true,
	".mp3": true, ".flac": true, ".aac": true, ".ogg": true, ".wav": true,
	".zip": true, ".rar": true, ".7z": true, ".gz": true, ".xz": true,
	".pdf": true, // PDFs are already compressed internally
}

// Buffer pool for file reading
var zipBufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 32*1024) // 32KB chunks
		return &buf
	},
}

// ═══════════════════════════════════════════════════════════════
// FOLDER STREAMER
// ═══════════════════════════════════════════════════════════════

// StreamFolder creates a zip archive on-the-fly and streams to ResponseWriter.
// No temporary file is created - data flows directly to the network.
//
// Features:
//   - Instant streaming (no wait for compression)
//   - Smart compression (Store for media, Deflate for text)
//   - Handles large folders efficiently
func StreamFolder(w http.ResponseWriter, folderPath string) error {
	// Verify folder exists
	info, err := os.Stat(folderPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return os.ErrInvalid
	}

	folderName := filepath.Base(folderPath)

	// Set headers for zip download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+folderName+`.zip"`)
	w.Header().Set("Transfer-Encoding", "chunked") // Streaming

	// Create zip writer wrapping the response
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// Track stats
	var fileCount, totalBytes int64

	// Walk the folder
	err = filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the root folder itself
		if path == folderPath {
			return nil
		}

		// Get relative path for zip entry
		relPath, err := filepath.Rel(folderPath, path)
		if err != nil {
			return err
		}

		// Convert to forward slashes for zip compatibility
		relPath = strings.ReplaceAll(relPath, "\\", "/")

		// Handle directories
		if info.IsDir() {
			_, err := zipWriter.Create(relPath + "/")
			return err
		}

		// Choose compression method
		method := getCompressionMethod(path)

		// Create file header
		header := &zip.FileHeader{
			Name:   relPath,
			Method: method,
		}
		header.SetModTime(info.ModTime())

		// Create entry
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// Open source file
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		// Copy using pooled buffer
		bufPtr := zipBufferPool.Get().(*[]byte)
		defer zipBufferPool.Put(bufPtr)

		n, err := io.CopyBuffer(writer, file, *bufPtr)
		if err != nil {
			return err
		}

		fileCount++
		totalBytes += n

		return nil
	})

	if err != nil {
		log.Printf("[ZIPPER] ❌ Walk error: %v", err)
		return err
	}

	log.Printf("[ZIPPER] ✅ Streamed %s: %d files, %.2f MB",
		folderName, fileCount, float64(totalBytes)/(1024*1024))

	return nil
}

// getCompressionMethod returns Store for media, Deflate for text.
func getCompressionMethod(path string) uint16 {
	ext := strings.ToLower(filepath.Ext(path))
	if mediaExtensions[ext] {
		return zip.Store // No compression - already compressed
	}
	return zip.Deflate // Compress text/code files
}

// ═══════════════════════════════════════════════════════════════
// HTTP HANDLER
// ═══════════════════════════════════════════════════════════════

// HandleFolderDownload is the HTTP handler for folder streaming.
// Query param: path=/absolute/path/to/folder
func HandleFolderDownload(w http.ResponseWriter, r *http.Request) {
	folderPath := r.URL.Query().Get("path")
	if folderPath == "" {
		http.Error(w, "Missing path parameter", http.StatusBadRequest)
		return
	}

	// Security: Validate path is absolute and exists
	if !filepath.IsAbs(folderPath) {
		http.Error(w, "Path must be absolute", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(folderPath)
	if err != nil {
		http.Error(w, "Folder not found", http.StatusNotFound)
		return
	}
	if !info.IsDir() {
		http.Error(w, "Path is not a directory", http.StatusBadRequest)
		return
	}

	log.Printf("[ZIPPER] 📦 Streaming folder: %s", folderPath)

	if err := StreamFolder(w, folderPath); err != nil {
		log.Printf("[ZIPPER] ❌ Stream error: %v", err)
		// Can't send error after headers are written
	}
}

// ═══════════════════════════════════════════════════════════════
// FOLDER INFO
// ═══════════════════════════════════════════════════════════════

// FolderInfo holds metadata about a folder for pre-flight checks.
type FolderInfo struct {
	Name      string `json:"name"`
	FileCount int    `json:"fileCount"`
	TotalSize int64  `json:"totalSize"`
	Path      string `json:"path"`
}

// GetFolderInfo scans a folder and returns metadata.
// Useful for showing estimated size before streaming.
func GetFolderInfo(folderPath string) (*FolderInfo, error) {
	info, err := os.Stat(folderPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, os.ErrInvalid
	}

	result := &FolderInfo{
		Name: filepath.Base(folderPath),
		Path: folderPath,
	}

	filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		result.FileCount++
		result.TotalSize += info.Size()
		return nil
	})

	return result, nil
}
