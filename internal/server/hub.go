// Package server provides the HTTP/WebSocket server for AERO.
// hub.go: The "Aero Hub" - WebSocket presence manager
//
// Term-Phase 5: Multi-Peer Relay
// Manages real-time presence of connected phones.
// Features:
//   - Client registration/unregistration with cleanup
//   - Avatar name generation (privacy-first, no IPs exposed)
//   - Broadcast peer_joined/peer_left events
//   - Peer list for lobby UI

package server

import (
	"encoding/json"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ═══════════════════════════════════════════════════════════════
// AVATAR NAMES (Privacy-first identifiers)
// ═══════════════════════════════════════════════════════════════

var avatarAdjectives = []string{
	"Neon", "Cyber", "Quantum", "Plasma", "Nova",
	"Sonic", "Hyper", "Turbo", "Pulse", "Storm",
	"Phantom", "Shadow", "Crystal", "Cosmic", "Stellar",
	"Blazing", "Frozen", "Thunder", "Mystic", "Astral",
}

var avatarNouns = []string{
	"Fox", "Wolf", "Hawk", "Panther", "Phoenix",
	"Dragon", "Tiger", "Eagle", "Shark", "Falcon",
	"Viper", "Raven", "Lynx", "Bear", "Cobra",
	"Jaguar", "Oryx", "Mantis", "Griffin", "Hydra",
}

func generateAvatarName() string {
	rand.Seed(time.Now().UnixNano())
	adj := avatarAdjectives[rand.Intn(len(avatarAdjectives))]
	noun := avatarNouns[rand.Intn(len(avatarNouns))]
	return adj + " " + noun
}

// ═══════════════════════════════════════════════════════════════
// PEER CLIENT
// ═══════════════════════════════════════════════════════════════

// Peer represents a connected phone/device.
type Peer struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"` // Avatar name (e.g., "Neon Fox")
	conn     *websocket.Conn // WebSocket connection
	hub      *Hub            // Reference to parent hub
	send     chan []byte     // Outbound message queue
	isHost   bool            `json:"isHost"` // True if this is the PC
}

// PeerInfo is the public-safe representation of a peer.
type PeerInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	IsHost bool   `json:"isHost"`
}

// toInfo returns a privacy-safe representation (no IP, no conn).
func (p *Peer) toInfo() PeerInfo {
	return PeerInfo{
		ID:     p.ID,
		Name:   p.Name,
		IsHost: p.isHost,
	}
}

// ═══════════════════════════════════════════════════════════════
// HUB (Central Presence Manager)
// ═══════════════════════════════════════════════════════════════

// Hub maintains the set of active peers and broadcasts messages.
// Thread-safe via single mutex.
type Hub struct {
	// mu protects all map access
	mu sync.RWMutex

	// Active peers: map[peerID]*Peer
	peers map[string]*Peer

	// Counter for unique IDs
	idCounter int
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		peers: make(map[string]*Peer),
	}
}

// ═══════════════════════════════════════════════════════════════
// REGISTRATION
// ═══════════════════════════════════════════════════════════════

// Register adds a new peer to the hub.
// Returns the assigned peer ID and avatar name.
func (h *Hub) Register(conn *websocket.Conn, isHost bool) *Peer {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.idCounter++
	id := string(rune('A' + h.idCounter - 1))
	if h.idCounter > 26 {
		id = string(rune('A'+((h.idCounter-1)/26)-1)) + string(rune('A'+((h.idCounter-1)%26)))
	}

	name := "Host PC"
	if !isHost {
		name = generateAvatarName()
	}

	peer := &Peer{
		ID:     id,
		Name:   name,
		conn:   conn,
		hub:    h,
		send:   make(chan []byte, 256),
		isHost: isHost,
	}

	h.peers[id] = peer

	log.Printf("[HUB] ✅ Peer joined: %s (%s)", name, id)

	return peer
}

// Unregister removes a peer from the hub.
// Triggers peer_left broadcast to remaining peers.
func (h *Hub) Unregister(peerID string) {
	h.mu.Lock()

	peer, exists := h.peers[peerID]
	if !exists {
		h.mu.Unlock()
		return
	}

	delete(h.peers, peerID)
	close(peer.send)

	// Capture peer info before unlock
	peerInfo := peer.toInfo()

	h.mu.Unlock() // UNLOCK before broadcast

	log.Printf("[HUB] 👋 Peer left: %s (%s)", peerInfo.Name, peerInfo.ID)

	// Broadcast peer_left to remaining peers
	h.broadcastEvent("peer_left", peerInfo)
}

// ═══════════════════════════════════════════════════════════════
// BROADCAST
// ═══════════════════════════════════════════════════════════════

// HubEvent is the structure for all hub events.
type HubEvent struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

// broadcastEvent sends an event to all connected peers.
func (h *Hub) broadcastEvent(eventName string, data interface{}) {
	event := HubEvent{
		Event: eventName,
		Data:  data,
	}

	msg, err := json.Marshal(event)
	if err != nil {
		log.Printf("[HUB] ❌ Failed to marshal event: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, peer := range h.peers {
		select {
		case peer.send <- msg:
		default:
			// Channel full, skip (peer is slow)
			log.Printf("[HUB] ⚠️ Peer %s send buffer full, skipping", peer.ID)
		}
	}
}

// BroadcastPeerJoined notifies all peers that a new peer joined.
func (h *Hub) BroadcastPeerJoined(peer *Peer) {
	h.broadcastEvent("peer_joined", peer.toInfo())
}

// ═══════════════════════════════════════════════════════════════
// QUERIES
// ═══════════════════════════════════════════════════════════════

// GetPeerList returns a list of all connected peers (privacy-safe).
func (h *Hub) GetPeerList() []PeerInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	list := make([]PeerInfo, 0, len(h.peers))
	for _, peer := range h.peers {
		list = append(list, peer.toInfo())
	}
	return list
}

// GetPeer returns a peer by ID.
func (h *Hub) GetPeer(id string) *Peer {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.peers[id]
}

// PeerCount returns the number of connected peers.
func (h *Hub) PeerCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.peers)
}

// ═══════════════════════════════════════════════════════════════
// PEER METHODS
// ═══════════════════════════════════════════════════════════════

// SendMessage queues a message to be sent to this peer.
func (p *Peer) SendMessage(msg []byte) {
	select {
	case p.send <- msg:
	default:
		log.Printf("[HUB] ⚠️ Peer %s send buffer full", p.ID)
	}
}

// SendEvent sends a structured event to this peer.
func (p *Peer) SendEvent(eventName string, data interface{}) error {
	event := HubEvent{
		Event: eventName,
		Data:  data,
	}
	msg, err := json.Marshal(event)
	if err != nil {
		return err
	}
	p.SendMessage(msg)
	return nil
}

// WritePump pumps messages from the send channel to the WebSocket.
// Run as a goroutine.
func (p *Peer) WritePump() {
	defer func() {
		p.conn.Close()
	}()

	for msg := range p.send {
		err := p.conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			log.Printf("[HUB] ❌ Write error for %s: %v", p.ID, err)
			return
		}
	}
}

// ReadPump reads messages from the WebSocket.
// Run as a goroutine. Unregisters peer on disconnect.
func (p *Peer) ReadPump() {
	defer func() {
		p.hub.Unregister(p.ID)
		p.conn.Close()
	}()

	for {
		_, message, err := p.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[HUB] ❌ Read error for %s: %v", p.ID, err)
			}
			return
		}

		// Handle incoming messages (relay commands, etc.)
		p.handleMessage(message)
	}
}

// handleMessage processes incoming messages from a peer.
func (p *Peer) handleMessage(message []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}

	action, _ := msg["action"].(string)

	switch action {
	case "get_peers":
		// Send current peer list
		p.SendEvent("peer_list", p.hub.GetPeerList())

	case "relay_to":
		// Relay a message to another peer
		targetID, _ := msg["target"].(string)
		payload := msg["payload"]
		if target := p.hub.GetPeer(targetID); target != nil {
			target.SendEvent("relay_from", map[string]interface{}{
				"from":    p.toInfo(),
				"payload": payload,
			})
		}
	}
}
