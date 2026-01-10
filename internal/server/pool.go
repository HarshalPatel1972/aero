// Package server provides the HTTP/WebSocket server for AERO.
// pool.go: Zero-allocation buffer pool using sync.Pool
//
// Term-Phase 3: Memory Manager
// Goal: Keep Go process under 50MB RAM even for 50GB transfers.

package server

import (
	"sync"
)

// BufferSize is the standard chunk size for transfers.
// 32KB is optimal for most network conditions and disk I/O.
const BufferSize = 32 * 1024 // 32KB

// bufferPool is a global pool of reusable byte slices.
// This eliminates GC pressure from constant allocations in the hot path.
var bufferPool = sync.Pool{
	New: func() interface{} {
		// Allocate a new buffer only when pool is empty
		buf := make([]byte, BufferSize)
		return &buf
	},
}

// GetBuffer retrieves a buffer from the pool.
// The returned buffer is guaranteed to be at least BufferSize bytes.
// IMPORTANT: Always call PutBuffer when done to return it to the pool.
func GetBuffer() *[]byte {
	return bufferPool.Get().(*[]byte)
}

// PutBuffer returns a buffer to the pool for reuse.
// The buffer contents may be overwritten by future GetBuffer calls.
// It's safe to call with nil (no-op).
func PutBuffer(buf *[]byte) {
	if buf == nil {
		return
	}
	// Reset the slice length but keep capacity
	*buf = (*buf)[:BufferSize]
	bufferPool.Put(buf)
}

// SmallBufferSize for adaptive chunking on slow networks
const SmallBufferSize = 4 * 1024 // 4KB

// LargeBufferSize for adaptive chunking on fast networks
const LargeBufferSize = 16 * 1024 // 16KB

// smallBufferPool for slow/jittery connections
var smallBufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, SmallBufferSize)
		return &buf
	},
}

// largeBufferPool for fast connections (>20MB/s)
var largeBufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, LargeBufferSize)
		return &buf
	},
}

// GetAdaptiveBuffer returns a buffer sized for current network conditions.
// speedMBps is the current measured speed in MB/s.
func GetAdaptiveBuffer(speedMBps float64) (*[]byte, int) {
	if speedMBps > 20 {
		// Fast network: use larger chunks to reduce CPU overhead
		buf := largeBufferPool.Get().(*[]byte)
		return buf, LargeBufferSize
	} else if speedMBps < 5 {
		// Slow/jittery: use smaller chunks
		buf := smallBufferPool.Get().(*[]byte)
		return buf, SmallBufferSize
	}
	// Default: standard buffer
	buf := GetBuffer()
	return buf, BufferSize
}

// PutAdaptiveBuffer returns an adaptive buffer to the appropriate pool.
func PutAdaptiveBuffer(buf *[]byte, size int) {
	if buf == nil {
		return
	}
	switch size {
	case SmallBufferSize:
		*buf = (*buf)[:SmallBufferSize]
		smallBufferPool.Put(buf)
	case LargeBufferSize:
		*buf = (*buf)[:LargeBufferSize]
		largeBufferPool.Put(buf)
	default:
		PutBuffer(buf)
	}
}
