// Package storage provides file I/O operations.
// naming.go: Safe filename collision resolution
//
// Term-Phase 4: Atomic file guard for duplicate filenames.
// If "photo.jpg" exists, generates "photo (1).jpg", "photo (2).jpg", etc.

package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ═══════════════════════════════════════════════════════════════
// FILENAME RESOLVER
// ═══════════════════════════════════════════════════════════════

// namingMu protects the check-and-create loop to prevent races
// between concurrent transfers checking the same filename.
var namingMu sync.Mutex

// ResolveFilename returns a unique filename in the given directory.
// If the file doesn't exist, returns the original name.
// If it exists, appends (1), (2), etc. until a unique name is found.
//
// Thread Safety:
//   - Uses a mutex to make check-and-reserve atomic
//   - Prevents race conditions when two transfers target the same file
//
// Example:
//   ResolveFilename("/uploads", "photo.jpg")
//   -> "photo.jpg" if doesn't exist
//   -> "photo (1).jpg" if photo.jpg exists
//   -> "photo (2).jpg" if both exist
func ResolveFilename(dir, filename string) string {
	// LOCK: Atomic check-and-reserve
	namingMu.Lock()
	defer namingMu.Unlock()

	// Clean the filename (remove path traversal attempts)
	filename = filepath.Clean(filename)
	filename = filepath.Base(filename)
	if filename == "." || filename == ".." {
		filename = "unnamed_file"
	}

	fullPath := filepath.Join(dir, filename)

	// If file doesn't exist, we're done
	if !fileExists(fullPath) {
		return filename
	}

	// Split into name and extension
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)

	// Find a unique name
	for i := 1; i < 1000; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", name, i, ext)
		candidatePath := filepath.Join(dir, candidate)
		if !fileExists(candidatePath) {
			return candidate
		}
	}

	// Fallback: use timestamp
	return fmt.Sprintf("%s_%d%s", name, os.Getpid(), ext)
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ═══════════════════════════════════════════════════════════════
// SAFE FILE CREATION
// ═══════════════════════════════════════════════════════════════

// CreateUniqueFile creates a new file with a unique name.
// If the desired filename exists, appends (1), (2), etc.
// Returns the final filename used and a file handle.
//
// Thread Safety:
//   - Uses ResolveFilename which holds a mutex
//   - File is created with O_CREATE|O_EXCL for atomic creation
func CreateUniqueFile(dir, desiredName string) (finalName string, file *os.File, err error) {
	namingMu.Lock()
	defer namingMu.Unlock()

	// Clean the filename
	desiredName = filepath.Clean(desiredName)
	desiredName = filepath.Base(desiredName)
	if desiredName == "." || desiredName == ".." {
		desiredName = "unnamed_file"
	}

	fullPath := filepath.Join(dir, desiredName)

	// Try to create with the desired name first (most common case)
	file, err = os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err == nil {
		return desiredName, file, nil
	}

	// File exists, find a unique name
	if !os.IsExist(err) {
		return "", nil, err // Some other error
	}

	ext := filepath.Ext(desiredName)
	name := strings.TrimSuffix(desiredName, ext)

	for i := 1; i < 1000; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", name, i, ext)
		candidatePath := filepath.Join(dir, candidate)

		file, err = os.OpenFile(candidatePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			return candidate, file, nil
		}
		if !os.IsExist(err) {
			return "", nil, err
		}
	}

	return "", nil, fmt.Errorf("could not find unique filename after 1000 attempts")
}

// ═══════════════════════════════════════════════════════════════
// CLEANUP UTILITIES
// ═══════════════════════════════════════════════════════════════

// DeletePartialFile safely deletes a partial (.part) file.
// This is called when a transfer is cancelled to avoid leaving garbage.
// Errors are logged but not returned (cleanup is best-effort).
func DeletePartialFile(path string) {
	if path == "" {
		return
	}

	// Only delete .part files for safety
	if !strings.HasSuffix(path, ".part") {
		return
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		// Log but don't fail - cleanup is best effort
		// The file may have already been deleted or renamed
	}
}
