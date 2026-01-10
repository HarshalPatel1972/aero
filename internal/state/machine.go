// Package state provides thread-safe transfer state management.
// machine.go: The "Fortress" State Manager
//
// Term-Phase 4: Central Source of Truth for transfer state.
// Philosophy: "Share memory by communicating, don't communicate by sharing memory."
//
// Thread Safety:
// - All state access protected by sync.RWMutex
// - Never hold mutex during blocking I/O operations
// - Context propagation for cancellation

package state

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════
// TYPES
// ═══════════════════════════════════════════════════════════════

// Status represents the current transfer state.
type Status int

const (
	StatusIdle Status = iota
	StatusReceiving
	StatusSending
	StatusFinished
	StatusError
	StatusCancelled
)

func (s Status) String() string {
	return [...]string{"Idle", "Receiving", "Sending", "Finished", "Error", "Cancelled"}[s]
}

// TransferInfo holds metadata about the current transfer.
type TransferInfo struct {
	ID        string
	Filename  string
	TotalSize int64
	Direction string // "send" or "receive"
	StartedAt time.Time
}

// ═══════════════════════════════════════════════════════════════
// ERRORS
// ═══════════════════════════════════════════════════════════════

var (
	ErrTransferInProgress = errors.New("transfer already in progress")
	ErrNoActiveTransfer   = errors.New("no active transfer to cancel")
	ErrInvalidState       = errors.New("invalid state transition")
)

// ═══════════════════════════════════════════════════════════════
// TRANSFER MANAGER
// ═══════════════════════════════════════════════════════════════

// TransferManager is the thread-safe state machine for transfers.
// It is the Single Source of Truth for transfer state.
//
// Usage:
//   1. Call StartTransfer() to begin - returns context for I/O
//   2. I/O loop uses the context, checks ctx.Done()
//   3. Call FinishTransfer() or CancelTransfer() when done
//
// Thread Safety:
//   - All public methods are safe to call concurrently
//   - Mutex is released before any blocking operations
type TransferManager struct {
	// mu protects all fields below
	// RULE: Lock -> Read/Write state -> Unlock -> Do I/O
	mu sync.RWMutex

	status     Status
	info       *TransferInfo
	cancelFunc context.CancelFunc
	cancelCtx  context.Context

	// For cleanup
	partialFile string
}

// NewTransferManager creates a new TransferManager in Idle state.
func NewTransferManager() *TransferManager {
	return &TransferManager{
		status: StatusIdle,
	}
}

// ═══════════════════════════════════════════════════════════════
// STATE TRANSITIONS
// ═══════════════════════════════════════════════════════════════

// StartTransfer attempts to begin a new transfer.
// Returns a context that will be cancelled if CancelTransfer() is called.
//
// Thread Safety:
//   - Acquires write lock to check and set state atomically
//   - Lock is released before returning
//
// Returns:
//   - Context for the transfer (check ctx.Done() in I/O loop)
//   - Error if a transfer is already in progress
func (tm *TransferManager) StartTransfer(
	id string,
	filename string,
	totalSize int64,
	direction string,
) (context.Context, error) {
	// LOCK: Check current state and update atomically
	tm.mu.Lock()

	// Reject if not idle
	if tm.status != StatusIdle && tm.status != StatusFinished &&
		tm.status != StatusError && tm.status != StatusCancelled {
		tm.mu.Unlock() // UNLOCK before returning
		return nil, ErrTransferInProgress
	}

	// Create cancellable context for this transfer
	ctx, cancel := context.WithCancel(context.Background())

	// Update state
	tm.status = StatusReceiving
	if direction == "send" {
		tm.status = StatusSending
	}

	tm.info = &TransferInfo{
		ID:        id,
		Filename:  filename,
		TotalSize: totalSize,
		Direction: direction,
		StartedAt: time.Now(),
	}
	tm.cancelFunc = cancel
	tm.cancelCtx = ctx
	tm.partialFile = ""

	tm.mu.Unlock() // UNLOCK: State is set, no blocking ops here

	return ctx, nil
}

// SetPartialFile records the path to the partial (.part) file.
// This allows cleanup on cancellation.
func (tm *TransferManager) SetPartialFile(path string) {
	tm.mu.Lock()
	tm.partialFile = path
	tm.mu.Unlock()
}

// GetPartialFile returns the current partial file path.
func (tm *TransferManager) GetPartialFile() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.partialFile
}

// FinishTransfer marks the current transfer as complete.
// Thread Safety: Acquires write lock briefly.
func (tm *TransferManager) FinishTransfer(success bool) {
	tm.mu.Lock()

	if success {
		tm.status = StatusFinished
	} else {
		tm.status = StatusError
	}

	// Clear the cancel func (transfer is done, no need to cancel)
	tm.cancelFunc = nil
	tm.info = nil
	tm.partialFile = ""

	tm.mu.Unlock()
}

// CancelTransfer cancels the active transfer.
// This invokes the context's CancelFunc, causing any I/O loop
// that checks ctx.Done() to terminate immediately.
//
// Thread Safety:
//   - Acquires write lock to get cancel func
//   - Releases lock before calling cancel (cancel may block briefly)
//
// Returns the partial file path for cleanup (caller should delete it).
func (tm *TransferManager) CancelTransfer() (partialFile string, err error) {
	tm.mu.Lock()

	// Check if there's an active transfer
	if tm.status != StatusReceiving && tm.status != StatusSending {
		tm.mu.Unlock()
		return "", ErrNoActiveTransfer
	}

	// Capture values before unlock
	cancel := tm.cancelFunc
	partialFile = tm.partialFile

	// Update state
	tm.status = StatusCancelled
	tm.cancelFunc = nil
	tm.info = nil
	tm.partialFile = ""

	tm.mu.Unlock() // UNLOCK before calling cancel

	// Cancel the context (may unblock I/O operations)
	if cancel != nil {
		cancel()
	}

	return partialFile, nil
}

// ═══════════════════════════════════════════════════════════════
// STATE QUERIES (Read-only, thread-safe)
// ═══════════════════════════════════════════════════════════════

// Status returns the current transfer status.
func (tm *TransferManager) Status() Status {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.status
}

// IsIdle returns true if no transfer is in progress.
func (tm *TransferManager) IsIdle() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.status == StatusIdle || tm.status == StatusFinished ||
		tm.status == StatusError || tm.status == StatusCancelled
}

// IsActive returns true if a transfer is in progress.
func (tm *TransferManager) IsActive() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.status == StatusReceiving || tm.status == StatusSending
}

// Info returns a copy of the current transfer info.
// Returns nil if no transfer is active.
func (tm *TransferManager) Info() *TransferInfo {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if tm.info == nil {
		return nil
	}

	// Return a copy to prevent data races
	infoCopy := *tm.info
	return &infoCopy
}

// Context returns the current transfer's context.
// Returns nil if no transfer is active.
func (tm *TransferManager) Context() context.Context {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.cancelCtx
}

// Reset forces the state back to Idle.
// Use with caution - primarily for error recovery.
func (tm *TransferManager) Reset() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.cancelFunc != nil {
		tm.cancelFunc()
	}

	tm.status = StatusIdle
	tm.info = nil
	tm.cancelFunc = nil
	tm.cancelCtx = nil
	tm.partialFile = ""
}
