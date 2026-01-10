// Package transport defines the Universal Transport Interface for AERO.
// contract.go: Interface definitions for protocol abstraction.
//
// Term-Phase 10: Hexagonal Architecture
// Core Philosophy: "The Interface is King"
//
// This abstraction allows swapping TCP/HTTP for QUIC/HTTP3 with zero
// changes to the core server logic. Future protocols must satisfy
// these interfaces to be compatible with AERO.
//
// Design Principles:
//   - Zero-cost abstraction (Go interfaces are cheap)
//   - io.ReadWriteCloser compatibility
//   - Metadata for protocol introspection
//   - Clean separation of concerns

package transport

import (
	"context"
	"io"
	"net/http"
)

// ═══════════════════════════════════════════════════════════════
// CORE INTERFACES
// ═══════════════════════════════════════════════════════════════

// Transport is the universal interface for network protocols.
// Implementations: TCP/HTTP, QUIC/HTTP3, WebSocket, etc.
type Transport interface {
	// Name returns the protocol identifier (e.g., "http", "quic", "ws")
	Name() string

	// Version returns the protocol version (e.g., "1.1", "3")
	Version() string

	// Listen starts accepting connections on the given address.
	Listen(addr string) error

	// ListenWithContext starts with cancellation support.
	ListenWithContext(ctx context.Context, addr string) error

	// Close shuts down the transport.
	Close() error

	// Handler sets the request handler (HTTP-compatible).
	SetHandler(handler http.Handler)
}

// Connection represents an active network connection.
// Must satisfy io.ReadWriteCloser for streaming.
type Connection interface {
	io.ReadWriteCloser

	// Metadata returns connection info (IP, protocol, etc.)
	Metadata() ConnectionMeta

	// Context returns the connection's context.
	Context() context.Context
}

// ═══════════════════════════════════════════════════════════════
// METADATA
// ═══════════════════════════════════════════════════════════════

// ConnectionMeta holds connection metadata.
type ConnectionMeta struct {
	// RemoteAddr is the client's network address
	RemoteAddr string

	// Protocol is the transport protocol name
	Protocol string

	// Version is the protocol version
	Version string

	// Encrypted indicates if the connection is TLS/encrypted
	Encrypted bool

	// Extra holds protocol-specific metadata
	Extra map[string]string
}

// ═══════════════════════════════════════════════════════════════
// PROTOCOL NEGOTIATION
// ═══════════════════════════════════════════════════════════════

// ProtocolInfo describes a supported protocol.
type ProtocolInfo struct {
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	Features []string `json:"features,omitempty"`
}

// Capabilities lists what a transport supports.
type Capabilities struct {
	Protocols    []ProtocolInfo `json:"protocols"`
	MaxChunkSize int64          `json:"maxChunkSize"`
	Encryption   bool           `json:"encryption"`
	Resumable    bool           `json:"resumable"`
	MultiPeer    bool           `json:"multiPeer"`
}

// DefaultCapabilities returns AERO's current feature set.
func DefaultCapabilities() Capabilities {
	return Capabilities{
		Protocols: []ProtocolInfo{
			{Name: "http", Version: "1.1", Features: []string{"upload", "download", "websocket"}},
		},
		MaxChunkSize: 10 * 1024 * 1024, // 10MB
		Encryption:   true,              // AES-256-CTR
		Resumable:    true,              // Range headers
		MultiPeer:    true,              // Hub relay
	}
}

// ═══════════════════════════════════════════════════════════════
// FACTORY
// ═══════════════════════════════════════════════════════════════

// Factory creates transport instances.
type Factory interface {
	// Create returns a transport for the given protocol name.
	Create(protocol string) (Transport, error)

	// Available returns list of supported protocols.
	Available() []string
}

// ═══════════════════════════════════════════════════════════════
// ERRORS
// ═══════════════════════════════════════════════════════════════

// TransportError wraps transport-specific errors.
type TransportError struct {
	Protocol string
	Op       string
	Err      error
}

func (e *TransportError) Error() string {
	return e.Protocol + " " + e.Op + ": " + e.Err.Error()
}

func (e *TransportError) Unwrap() error {
	return e.Err
}
