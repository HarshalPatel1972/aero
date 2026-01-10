// Package server provides the HTTP/WebSocket server for AERO.
// tracker.go: Bandwidth monitoring with atomic counters
//
// Term-Phase 3: Telemetry Hook ("The Speedometer")
// Uses atomic operations for lock-free performance in the hot Read path.

package server

import (
	"context"
	"io"
	"sync/atomic"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// TransferTracker wraps an io.Reader to monitor bandwidth and progress.
// It uses atomic operations for thread-safe, lock-free updates.
type TransferTracker struct {
	reader       io.Reader
	totalSize    int64
	bytesRead    int64 // atomic
	startTime    time.Time
	lastEmitTime int64 // atomic, Unix nano
	ctx          context.Context
	filename     string
	direction    string // "send" or "receive"
	emitInterval time.Duration
}

// TransferProgress is the event payload sent to the frontend.
type TransferProgress struct {
	Filename    string  `json:"filename"`
	Status      string  `json:"status"` // "started", "progress", "completed", "error"
	Progress    float64 `json:"progress"` // 0-100
	SpeedMBps   float64 `json:"speedMBps"`
	Direction   string  `json:"direction"`
	BytesTotal  int64   `json:"bytesTotal"`
	BytesDone   int64   `json:"bytesDone"`
	ElapsedMs   int64   `json:"elapsedMs"`
}

// NewTransferTracker creates a new bandwidth tracker.
// emitInterval controls how often progress events are sent (default 500ms).
func NewTransferTracker(
	ctx context.Context,
	reader io.Reader,
	totalSize int64,
	filename string,
	direction string,
) *TransferTracker {
	return &TransferTracker{
		reader:       reader,
		totalSize:    totalSize,
		bytesRead:    0,
		startTime:    time.Now(),
		lastEmitTime: 0,
		ctx:          ctx,
		filename:     filename,
		direction:    direction,
		emitInterval: 500 * time.Millisecond,
	}
}

// Read implements io.Reader with bandwidth tracking.
// Uses atomic operations for zero-lock performance.
func (t *TransferTracker) Read(p []byte) (n int, err error) {
	n, err = t.reader.Read(p)
	if n > 0 {
		// Atomic add to bytes counter
		newTotal := atomic.AddInt64(&t.bytesRead, int64(n))
		
		// Check if we should emit (debounced)
		now := time.Now().UnixNano()
		lastEmit := atomic.LoadInt64(&t.lastEmitTime)
		
		if now-lastEmit >= t.emitInterval.Nanoseconds() {
			// Try to claim this emit slot (CAS)
			if atomic.CompareAndSwapInt64(&t.lastEmitTime, lastEmit, now) {
				t.emitProgress(newTotal, "progress")
			}
		}
	}
	
	// Emit completion on EOF
	if err == io.EOF && t.totalSize > 0 {
		t.emitProgress(atomic.LoadInt64(&t.bytesRead), "completed")
	}
	
	return n, err
}

// emitProgress sends a transfer:progress event to the Wails frontend.
func (t *TransferTracker) emitProgress(bytesRead int64, status string) {
	elapsed := time.Since(t.startTime)
	elapsedSeconds := elapsed.Seconds()
	
	// Calculate speed in MB/s
	var speedMBps float64
	if elapsedSeconds > 0 {
		speedMBps = (float64(bytesRead) / (1024 * 1024)) / elapsedSeconds
	}
	
	// Calculate percentage
	var progress float64
	if t.totalSize > 0 {
		progress = (float64(bytesRead) / float64(t.totalSize)) * 100
		if progress > 100 {
			progress = 100
		}
	}
	
	event := TransferProgress{
		Filename:   t.filename,
		Status:     status,
		Progress:   progress,
		SpeedMBps:  speedMBps,
		Direction:  t.direction,
		BytesTotal: t.totalSize,
		BytesDone:  bytesRead,
		ElapsedMs:  elapsed.Milliseconds(),
	}
	
	// Emit to Wails (non-blocking)
	if t.ctx != nil {
		wailsruntime.EventsEmit(t.ctx, "transfer:progress", event)
	}
}

// EmitStart sends the initial "started" event.
func (t *TransferTracker) EmitStart() {
	t.emitProgress(0, "started")
}

// EmitError sends an "error" event.
func (t *TransferTracker) EmitError() {
	t.emitProgress(atomic.LoadInt64(&t.bytesRead), "error")
}

// BytesTransferred returns the current total bytes read (atomic).
func (t *TransferTracker) BytesTransferred() int64 {
	return atomic.LoadInt64(&t.bytesRead)
}

// SpeedMBps returns the current average speed in MB/s.
func (t *TransferTracker) SpeedMBps() float64 {
	bytesRead := atomic.LoadInt64(&t.bytesRead)
	elapsed := time.Since(t.startTime).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return (float64(bytesRead) / (1024 * 1024)) / elapsed
}

// Progress returns the current progress percentage (0-100).
func (t *TransferTracker) Progress() float64 {
	if t.totalSize <= 0 {
		return 0
	}
	bytesRead := atomic.LoadInt64(&t.bytesRead)
	return (float64(bytesRead) / float64(t.totalSize)) * 100
}
