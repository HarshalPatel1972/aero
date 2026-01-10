// Package server provides the HTTP/WebSocket server for AERO.
// protocol.go: Protocol Negotiation Handler
//
// Term-Phase 10: Future-proofing for QUIC/HTTP3
// When clients connect, they can query supported protocols.

package server

import (
	"encoding/json"
	"net/http"

	"github.com/username/aero/pkg/transport"
)

// handleProtocol responds with supported protocols and capabilities.
// Endpoint: GET /api/protocol
func (s *Server) handleProtocol(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	caps := transport.DefaultCapabilities()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(caps)
}

// handleHello is the initial handshake endpoint.
// Used for protocol version negotiation.
// Endpoint: POST /api/hello
func (s *Server) handleHello(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse client hello
	var clientHello struct {
		Version   string   `json:"version"`
		Protocols []string `json:"protocols"`
		Features  []string `json:"features"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&clientHello); err != nil {
		// If no body, just use defaults
		clientHello.Version = "1"
		clientHello.Protocols = []string{"http"}
	}

	// Server response
	serverHello := struct {
		Version      string                     `json:"version"`
		Protocol     string                     `json:"protocol"`
		Capabilities transport.Capabilities     `json:"capabilities"`
		SessionKey   string                     `json:"sessionKey,omitempty"`
	}{
		Version:      "1",
		Protocol:     "http/1.1",
		Capabilities: transport.DefaultCapabilities(),
	}

	// Include session key if available (for E2E encryption)
	// Note: Key is also in QR URL hash for redundancy
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(serverHello)
}
