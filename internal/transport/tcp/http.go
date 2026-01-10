// Package tcp provides the HTTP/TCP transport adapter for AERO.
// http.go: Implementation of the Transport interface for HTTP/1.1
//
// Term-Phase 10: Hexagonal Architecture - HTTP Adapter
// This adapter wraps net/http to satisfy the Transport interface.
//
// Benefits:
//   - Can be swapped for QUIC/HTTP3 without changing server logic
//   - Testable via mock transports
//   - Clean separation of network concerns

package tcp

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/username/aero/pkg/transport"
)

// ═══════════════════════════════════════════════════════════════
// HTTP TRANSPORT
// ═══════════════════════════════════════════════════════════════

// HTTPTransport implements transport.Transport using net/http.
type HTTPTransport struct {
	mu       sync.RWMutex
	server   *http.Server
	handler  http.Handler
	listener net.Listener
	addr     string
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
}

// New creates a new HTTP transport.
func New() *HTTPTransport {
	return &HTTPTransport{}
}

// Name returns "http".
func (t *HTTPTransport) Name() string {
	return "http"
}

// Version returns "1.1".
func (t *HTTPTransport) Version() string {
	return "1.1"
}

// SetHandler sets the HTTP request handler.
func (t *HTTPTransport) SetHandler(handler http.Handler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handler = handler
}

// Listen starts the HTTP server on the given address.
func (t *HTTPTransport) Listen(addr string) error {
	return t.ListenWithContext(context.Background(), addr)
}

// ListenWithContext starts with cancellation support.
func (t *HTTPTransport) ListenWithContext(ctx context.Context, addr string) error {
	t.mu.Lock()
	
	if t.running {
		t.mu.Unlock()
		return fmt.Errorf("transport already running")
	}

	t.ctx, t.cancel = context.WithCancel(ctx)
	t.addr = addr

	// Create listener
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.mu.Unlock()
		return &transport.TransportError{
			Protocol: "http",
			Op:       "listen",
			Err:      err,
		}
	}
	t.listener = listener

	// Create HTTP server
	t.server = &http.Server{
		Addr:         addr,
		Handler:      t.handler,
		ReadTimeout:  0, // No timeout for large uploads
		WriteTimeout: 0, // No timeout for large downloads
		IdleTimeout:  300 * time.Second,
	}

	t.running = true
	t.mu.Unlock()

	log.Printf("[TRANSPORT] 🌐 HTTP/1.1 listening on %s", addr)

	// Handle graceful shutdown
	go func() {
		<-t.ctx.Done()
		t.server.Close()
	}()

	// Start serving (blocks)
	if err := t.server.Serve(listener); err != http.ErrServerClosed {
		return &transport.TransportError{
			Protocol: "http",
			Op:       "serve",
			Err:      err,
		}
	}

	return nil
}

// Close shuts down the transport.
func (t *HTTPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running {
		return nil
	}

	if t.cancel != nil {
		t.cancel()
	}

	if t.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		t.server.Shutdown(ctx)
	}

	t.running = false
	log.Printf("[TRANSPORT] 🔌 HTTP transport closed")
	return nil
}

// Addr returns the listening address.
func (t *HTTPTransport) Addr() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.addr
}

// IsRunning returns true if the transport is active.
func (t *HTTPTransport) IsRunning() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.running
}

// ═══════════════════════════════════════════════════════════════
// CAPABILITIES
// ═══════════════════════════════════════════════════════════════

// Capabilities returns the HTTP transport's feature set.
func (t *HTTPTransport) Capabilities() transport.Capabilities {
	return transport.Capabilities{
		Protocols: []transport.ProtocolInfo{
			{
				Name:     "http",
				Version:  "1.1",
				Features: []string{"upload", "download", "websocket", "range", "chunked"},
			},
		},
		MaxChunkSize: 10 * 1024 * 1024, // 10MB
		Encryption:   true,              // E2E via session key
		Resumable:    true,              // Range headers
		MultiPeer:    true,              // Hub relay
	}
}

// ═══════════════════════════════════════════════════════════════
// FACTORY
// ═══════════════════════════════════════════════════════════════

// Factory creates transport instances.
type Factory struct{}

// NewFactory creates a transport factory.
func NewFactory() *Factory {
	return &Factory{}
}

// Create returns a transport for the given protocol.
func (f *Factory) Create(protocol string) (transport.Transport, error) {
	switch protocol {
	case "http", "tcp", "":
		return New(), nil
	case "quic", "http3":
		// Future: return quic.New(), nil
		return nil, fmt.Errorf("QUIC transport not yet implemented")
	default:
		return nil, fmt.Errorf("unknown protocol: %s", protocol)
	}
}

// Available returns list of supported protocols.
func (f *Factory) Available() []string {
	return []string{"http"} // Add "quic" when implemented
}

// Default returns the default transport (HTTP).
func (f *Factory) Default() transport.Transport {
	return New()
}
