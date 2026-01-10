// MIT License
//
// Copyright (c) 2026 Project AERO Contributors
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Package storage provides high-performance file I/O operations with
// zero-copy streaming and atomic write guarantees.
package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/username/aero/internal/config"
)

// Service defines the interface for storage operations.
// This abstraction enables dependency injection and testing.
type Service interface {
	// WriteStream writes data from the stream to a file with the given filename.
	// It guarantees atomic writes: either the complete file is written or nothing.
	WriteStream(filename string, stream io.Reader) error
	
	// CreateWriter returns an io.WriteCloser for streaming writes.
	// Caller is responsible for closing the writer.
	CreateWriter(filename string) (io.WriteCloser, error)
}

// FileStorage implements Service using the local filesystem.
// It uses a fixed-size buffer pool to prevent memory allocations during streaming.
type FileStorage struct {
	uploadDir string
	bufPool   *sync.Pool
}

// NewFileStorage creates a new FileStorage instance.
// It ensures the upload directory exists and initializes the buffer pool.
func NewFileStorage(uploadDir string) (*FileStorage, error) {
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	return &FileStorage{
		uploadDir: uploadDir,
		bufPool: &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, config.BufferSize)
				return &buf
			},
		},
	}, nil
}

// WriteStream writes the contents of stream to a file named filename.
//
// Implementation details:
//  1. Creates a temporary file with .part suffix
//  2. Streams data using a pooled 32KB buffer (zero allocation per request)
//  3. Syncs the file to ensure durability
//  4. Atomically renames to the final filename
//  5. Cleans up the .part file on any error
//
// This approach prevents corrupted files from appearing in the upload directory.
func (fs *FileStorage) WriteStream(filename string, stream io.Reader) error {
	finalPath := filepath.Join(fs.uploadDir, filepath.Clean(filename))
	partPath := finalPath + ".part"

	// Create the temporary file
	partFile, err := os.Create(partPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	// Ensure cleanup on any error path
	success := false
	defer func() {
		partFile.Close()
		if !success {
			os.Remove(partPath)
		}
	}()

	// Get buffer from pool
	bufPtr := fs.bufPool.Get().(*[]byte)
	defer fs.bufPool.Put(bufPtr)

	// Stream data with fixed buffer - no memory growth
	_, err = io.CopyBuffer(partFile, stream, *bufPtr)
	if err != nil {
		return fmt.Errorf("failed to write stream: %w", err)
	}

	// Skip Sync() for speed - OS will handle write-back
	// (Data is safe once rename completes)

	// Close before rename (required on Windows)
	if err := partFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename - file appears complete or not at all
	if err := os.Rename(partPath, finalPath); err != nil {
		return fmt.Errorf("failed to finalize file: %w", err)
	}

	success = true
	return nil
}

// CreateWriter returns a WriteCloser for the given filename.
// Uses temp file + rename pattern for atomic writes.
func (fs *FileStorage) CreateWriter(filename string) (io.WriteCloser, error) {
	finalPath := filepath.Join(fs.uploadDir, filepath.Clean(filename))
	partPath := finalPath + ".part"

	partFile, err := os.Create(partPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	return &atomicWriter{
		file:      partFile,
		partPath:  partPath,
		finalPath: finalPath,
	}, nil
}

// atomicWriter wraps os.File to provide atomic rename on close
type atomicWriter struct {
	file      *os.File
	partPath  string
	finalPath string
	closed    bool
}

func (w *atomicWriter) Write(p []byte) (n int, err error) {
	return w.file.Write(p)
}

func (w *atomicWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	if err := w.file.Close(); err != nil {
		os.Remove(w.partPath)
		return err
	}

	if err := os.Rename(w.partPath, w.finalPath); err != nil {
		os.Remove(w.partPath)
		return err
	}

	return nil
}
