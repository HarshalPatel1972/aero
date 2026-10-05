// MIT License
// Copyright (c) 2026 Project AERO Contributors

package transfer

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/username/aero/internal/security"
)

const (
	wsWriteTimeout = 10 * time.Second
	wsPingInterval = 25 * time.Second
	wsPongTimeout  = 60 * time.Second
)

// phone is a connected phone's control channel. The server uses it to tell the
// phone about files offered for download; every message is sealed.
type phone struct {
	conn *websocket.Conn
	send chan []byte
	once sync.Once
}

func (p *phone) close() {
	p.once.Do(func() {
		close(p.send)
		p.conn.Close()
	})
}

// PhoneCount reports how many phones are connected.
func (s *Server) PhoneCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.phones)
}

func (s *Server) notifyPhones(count int) {
	if s.onPhones != nil {
		s.onPhones(count)
	}
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		// Only the page this server served may open the socket.
		CheckOrigin: func(r *http.Request) bool {
			return r.Header.Get("Origin") == "http://"+s.hostport
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade already wrote the HTTP error
	}

	p := &phone{conn: conn, send: make(chan []byte, 16)}
	s.mu.Lock()
	s.phones[p] = struct{}{}
	count := len(s.phones)
	s.mu.Unlock()
	log.Printf("[AERO] Phone connected")
	s.notifyPhones(count)

	defer func() {
		s.mu.Lock()
		delete(s.phones, p)
		count := len(s.phones)
		s.mu.Unlock()
		p.close()
		log.Printf("[AERO] Phone disconnected")
		s.notifyPhones(count)
	}()

	go s.writePump(p)

	// The phone sends nothing but pongs; reading keeps the deadline fresh and
	// detects disconnects.
	conn.SetReadLimit(1024)
	conn.SetReadDeadline(time.Now().Add(wsPongTimeout))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongTimeout))
	})
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (s *Server) writePump(p *phone) {
	ping := time.NewTicker(wsPingInterval)
	defer ping.Stop()
	for {
		select {
		case msg, ok := <-p.send:
			if !ok {
				return
			}
			p.conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := p.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				p.conn.Close()
				return
			}
		case <-ping.C:
			p.conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := p.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				p.conn.Close()
				return
			}
		}
	}
}

// broadcast seals msg and queues it for every connected phone.
func (s *Server) broadcast(msg any) {
	plain, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[AERO] Marshal failed: %v", err)
		return
	}
	box, err := s.session.Seal(plain, security.WSServerAAD())
	if err != nil {
		log.Printf("[AERO] Seal failed: %v", err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for p := range s.phones {
		select {
		case p.send <- box:
		default:
			log.Printf("[AERO] Phone not keeping up; dropping message")
		}
	}
}
