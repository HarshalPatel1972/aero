// Package server provides the HTTP/WebSocket server for AERO.
// ws_handler.go: WebSocket upgrade handler for the Hub
//
// Term-Phase 5: Multi-Peer Relay
// Handles WebSocket connection upgrades and integrates with the Hub.

package server

import (
	"log"
	"net/http"
)

// handleHubWebSocket upgrades HTTP to WebSocket and registers with the Hub.
func (s *Server) handleHubWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check if hub exists
	if s.hub == nil {
		http.Error(w, "Hub not initialized", http.StatusInternalServerError)
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[HUB] ❌ WebSocket upgrade failed: %v", err)
		return
	}

	// Determine if this is the host (PC has a special query param)
	isHost := r.URL.Query().Get("host") == "true"

	// Register with hub
	peer := s.hub.Register(conn, isHost)

	// Send welcome message with peer info and current peer list
	peer.SendEvent("welcome", map[string]interface{}{
		"you":   peer.toInfo(),
		"peers": s.hub.GetPeerList(),
	})

	// Broadcast to others that a new peer joined
	s.hub.BroadcastPeerJoined(peer)

	// Start read/write pumps
	go peer.WritePump()
	peer.ReadPump() // Blocks until disconnect
}

// handleRelay handles peer-to-peer file relay through the PC.
// Flow: Phone A -> POST /relay?target=B -> PC saves to temp -> Phone B notified
func (s *Server) handleRelay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	targetID := r.URL.Query().Get("target")
	if targetID == "" {
		http.Error(w, "Missing target parameter", http.StatusBadRequest)
		return
	}

	// Get target peer
	if s.hub == nil {
		http.Error(w, "Hub not initialized", http.StatusInternalServerError)
		return
	}

	target := s.hub.GetPeer(targetID)
	if target == nil {
		http.Error(w, "Target peer not found", http.StatusNotFound)
		return
	}

	// Get file metadata from headers
	filename := r.Header.Get("X-Filename")
	if filename == "" {
		filename = "relayed_file"
	}

	filesize := r.Header.Get("X-Filesize")

	log.Printf("[RELAY] 📤 Relaying %s to peer %s (%s)", filename, target.Name, target.ID)

	// Save to temp directory
	tempFilename := "relay_" + targetID + "_" + filename
	writer, err := s.storageService.CreateWriter(tempFilename)
	if err != nil {
		http.Error(w, "Failed to create temp file", http.StatusInternalServerError)
		return
	}

	// Use pooled buffer for copy
	buf := GetBuffer()
	defer PutBuffer(buf)

	var written int64
	for {
		n, readErr := r.Body.Read(*buf)
		if n > 0 {
			nw, writeErr := writer.Write((*buf)[:n])
			written += int64(nw)
			if writeErr != nil {
				writer.Close()
				http.Error(w, "Write error", http.StatusInternalServerError)
				return
			}
		}
		if readErr != nil {
			break
		}
	}
	writer.Close()

	// Notify target peer
	target.SendEvent("incoming_relay", map[string]interface{}{
		"filename": filename,
		"size":     filesize,
		"url":      "/download/" + tempFilename,
	})

	log.Printf("[RELAY] ✅ Saved %s (%.2f MB), notifying %s", tempFilename, float64(written)/(1024*1024), target.Name)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"relayed","target":"` + target.Name + `"}`))
}

// handleRelayDownload serves relayed files to the target peer.
func (s *Server) handleRelayDownload(w http.ResponseWriter, r *http.Request) {
	// Extract filename from path: /download/{filename}
	filename := r.URL.Path[len("/download/"):]
	if filename == "" {
		http.Error(w, "Missing filename", http.StatusBadRequest)
		return
	}

	// Serve the file from upload directory
	filePath := s.config.UploadDir + "/" + filename
	http.ServeFile(w, r, filePath)

	// Note: In production, delete the temp file after download
	// For now, leave it for debugging
}
