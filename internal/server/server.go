// MIT License
// Copyright (c) 2026 Project AERO Contributors

// Package server - Bi-directional file transfer via WebSocket
package server

import (
	"context"
	"embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/username/aero/internal/config"
	"github.com/username/aero/internal/security"
	"github.com/username/aero/internal/storage"
)

//go:embed templates/*
var templateFS embed.FS

var upgrader = websocket.Upgrader{
	ReadBufferSize:  2 * 1024 * 1024,
	WriteBufferSize: 2 * 1024 * 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type TransferEvent struct {
	Filename  string  `json:"filename"`
	Progress  float64 `json:"progress"`
	Speed     string  `json:"speed"`
	Status    string  `json:"status"`
	Direction string  `json:"direction"` // "receive" or "send"
}

type TransferCallback func(event TransferEvent)

type TransferMeta struct {
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	TotalChunks int    `json:"totalChunks"`
}

// PhoneClient represents a connected phone browser
type PhoneClient struct {
	conn     *websocket.Conn
	mu       sync.Mutex
	id       string
	sendChan chan []byte
}

// Server represents the HTTP/WebSocket server with bi-directional transfer
type Server struct {
	httpServer       *http.Server
	storageService   storage.Service
	config           config.Config
	sessionKey       []byte
	sessionKeyBase64 string
	onTransfer       TransferCallback
	tempDir          string
	ctx              context.Context // Wails context for event emission

	// Term-Phase 5: Multi-peer hub
	hub *Hub

	// Connected phone clients
	clients   map[string]*PhoneClient
	clientsMu sync.RWMutex
	
	// For PC -> Phone transfers
	pendingSend    *os.File
	pendingSendMu  sync.Mutex
}

func NewServer(cfg config.Config, storageService storage.Service) (*Server, error) {
	return NewServerWithCallback(cfg, storageService, nil)
}

func NewServerWithCallback(cfg config.Config, storageService storage.Service, onTransfer TransferCallback) (*Server, error) {
	keyBase64, keyBytes, err := security.GenerateSessionKey()
	if err != nil {
		return nil, err
	}
	return NewServerWithKey(cfg, storageService, keyBytes, keyBase64, onTransfer)
}

func NewServerWithKey(cfg config.Config, storageService storage.Service, key []byte, keyBase64 string, onTransfer TransferCallback) (*Server, error) {
	if len(key) != security.KeySize {
		return nil, fmt.Errorf("invalid key size")
	}

	tempDir := filepath.Join(cfg.UploadDir, ".tmp")
	os.MkdirAll(tempDir, 0755)
	os.MkdirAll(cfg.UploadDir, 0755)

	s := &Server{
		storageService:   storageService,
		config:           cfg,
		sessionKey:       key,
		sessionKeyBase64: keyBase64,
		onTransfer:       onTransfer,
		tempDir:          tempDir,
		clients:          make(map[string]*PhoneClient),
		hub:              NewHub(), // Term-Phase 5: Initialize hub
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/upload", s.handleUpload)
	mux.HandleFunc("/stream", s.handleStreamUpload) // Term-Phase 3: pooled buffer handler
	mux.HandleFunc("/transfer", s.handleTransfer)
	mux.HandleFunc("/health", s.handleHealth)
	// Term-Phase 5: Hub routes
	mux.HandleFunc("/hub", s.handleHubWebSocket)
	mux.HandleFunc("/relay", s.handleRelay)
	mux.HandleFunc("/download/", s.handleRelayDownload)

	s.httpServer = &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  0,
		WriteTimeout: 0,
		IdleTimeout:  300 * time.Second,
	}

	return s, nil
}

func (s *Server) Start() error {
	log.Printf("[AERO] Server starting on port %s", s.config.Port)
	return s.httpServer.ListenAndServe()
}

func (s *Server) StartWithContext(ctx context.Context, ip string) error {
	s.ctx = ctx // Store for handlers to emit events
	addr := fmt.Sprintf("%s:%s", ip, s.config.Port)
	s.httpServer.Addr = addr
	log.Printf("[AERO] Starting on %s", addr)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		s.httpServer.Close()
	}()

	return s.httpServer.Serve(listener)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) emitTransferEvent(event TransferEvent) {
	if s.onTransfer != nil {
		s.onTransfer(event)
	}
}

// GetConnectedClients returns list of connected phone client IDs
func (s *Server) GetConnectedClients() []string {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	ids := make([]string, 0, len(s.clients))
	for id := range s.clients {
		ids = append(ids, id)
	}
	return ids
}

// SendFileToPhone sends a file to the first connected phone
func (s *Server) SendFileToPhone(filePath string) error {
	s.clientsMu.RLock()
	if len(s.clients) == 0 {
		s.clientsMu.RUnlock()
		return fmt.Errorf("no phone connected")
	}
	
	// Get first client
	var client *PhoneClient
	for _, c := range s.clients {
		client = c
		break
	}
	s.clientsMu.RUnlock()

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	fileName := filepath.Base(filePath)
	fileSize := info.Size()
	chunkSize := int64(1024 * 1024) // 1MB
	totalChunks := int((fileSize + chunkSize - 1) / chunkSize)

	log.Printf("[AERO] 📤 Sending to phone: %s (%d chunks)", fileName, totalChunks)
	s.emitTransferEvent(TransferEvent{
		Filename:  fileName,
		Progress:  0,
		Status:    "started",
		Direction: "send",
	})

	// Send file metadata
	client.mu.Lock()
	err = client.conn.WriteJSON(map[string]interface{}{
		"type":   "file_incoming",
		"name":   fileName,
		"size":   fileSize,
		"chunks": totalChunks,
	})
	client.mu.Unlock()
	if err != nil {
		return err
	}

	// Send chunks
	buf := make([]byte, chunkSize)
	for i := 0; i < totalChunks; i++ {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}

		// Build message: 4 byte idx + data
		header := make([]byte, 4)
		binary.LittleEndian.PutUint32(header, uint32(i))
		msg := append(header, buf[:n]...)

		client.mu.Lock()
		err = client.conn.WriteMessage(websocket.BinaryMessage, msg)
		client.mu.Unlock()
		if err != nil {
			return err
		}

		progress := float64(i+1) / float64(totalChunks) * 100
		s.emitTransferEvent(TransferEvent{
			Filename:  fileName,
			Progress:  progress,
			Status:    "sending",
			Direction: "send",
		})
	}

	// Send completion
	client.mu.Lock()
	client.conn.WriteJSON(map[string]interface{}{
		"type": "file_complete",
		"name": fileName,
	})
	client.mu.Unlock()

	log.Printf("[AERO] ✓ Sent to phone: %s", fileName)
	s.emitTransferEvent(TransferEvent{
		Filename:  fileName,
		Progress:  100,
		Status:    "completed",
		Direction: "send",
	})

	return nil
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	tmpl, _ := template.ParseFS(templateFS, "templates/index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, nil)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"status":"ok"}`))
}

// handleTransfer - Bi-directional WebSocket handler
func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[AERO] Upgrade failed: %v", err)
		return
	}
	
	// Register client
	clientID := fmt.Sprintf("%d", time.Now().UnixNano())
	client := &PhoneClient{
		conn:     conn,
		id:       clientID,
		sendChan: make(chan []byte, 100),
	}
	
	s.clientsMu.Lock()
	s.clients[clientID] = client
	s.clientsMu.Unlock()
	
	log.Printf("[AERO] 📱 Phone connected: %s", clientID)

	defer func() {
		conn.Close()
		s.clientsMu.Lock()
		delete(s.clients, clientID)
		s.clientsMu.Unlock()
		log.Printf("[AERO] 📱 Phone disconnected: %s", clientID)
	}()

	conn.SetReadDeadline(time.Time{})

	var meta TransferMeta
	var transferDir string
	var initialized bool

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("[AERO] Read error: %v", err)
			}
			break
		}

		if msgType == websocket.TextMessage {
			var msg map[string]interface{}
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}

			// Handle phone upload init
			if name, ok := msg["name"].(string); ok {
				meta.Filename = name
				meta.Size = int64(msg["size"].(float64))
				meta.TotalChunks = int(msg["chunks"].(float64))

				transferDir = filepath.Join(s.tempDir, fmt.Sprintf("%d", time.Now().UnixNano()))
				os.MkdirAll(transferDir, 0755)

				metaPath := filepath.Join(transferDir, "meta.json")
				metaBytes, _ := json.Marshal(meta)
				os.WriteFile(metaPath, metaBytes, 0644)

				initialized = true
				log.Printf("[AERO] 📥 Receiving from phone: %s (%d chunks)", name, meta.TotalChunks)
				s.emitTransferEvent(TransferEvent{Filename: name, Progress: 0, Status: "started", Direction: "receive"})

				conn.WriteJSON(map[string]interface{}{"status": "ready"})
				continue
			}

			// Handle upload completion
			if _, ok := msg["done"]; ok {
				if !initialized {
					conn.WriteJSON(map[string]interface{}{"error": "not initialized"})
					continue
				}

				entries, _ := os.ReadDir(transferDir)
				received := 0
				for _, e := range entries {
					if strings.HasSuffix(e.Name(), ".chunk") {
						received++
					}
				}

				if received < meta.TotalChunks {
					missing := []int{}
					for i := 0; i < meta.TotalChunks; i++ {
						chunkPath := filepath.Join(transferDir, fmt.Sprintf("%d.chunk", i))
						if _, err := os.Stat(chunkPath); os.IsNotExist(err) {
							missing = append(missing, i)
						}
					}
					conn.WriteJSON(map[string]interface{}{"status": "incomplete", "missing": missing})
					continue
				}

				// Assemble
				finalPath := filepath.Join(s.config.UploadDir, filepath.Clean(meta.Filename))
				finalFile, err := os.Create(finalPath)
				if err != nil {
					conn.WriteJSON(map[string]interface{}{"error": err.Error()})
					continue
				}

				var totalWritten int64
				for i := 0; i < meta.TotalChunks; i++ {
					chunkPath := filepath.Join(transferDir, fmt.Sprintf("%d.chunk", i))
					chunkData, _ := os.ReadFile(chunkPath)
					n, _ := finalFile.Write(chunkData)
					totalWritten += int64(n)
				}
				finalFile.Close()
				os.RemoveAll(transferDir)

				log.Printf("[AERO] ✓ Received: %s (%d bytes)", meta.Filename, totalWritten)
				s.emitTransferEvent(TransferEvent{Filename: meta.Filename, Progress: 100, Status: "completed", Direction: "receive"})
				conn.WriteJSON(map[string]interface{}{"status": "success", "size": totalWritten})
				
				// Reset for next transfer
				meta = TransferMeta{}
				transferDir = ""
				initialized = false
			}
			
			// Handle ACK from phone (for PC->Phone transfers)
			if ack, ok := msg["ack"]; ok {
				_ = ack // ACK received, continue sending
			}

		} else if msgType == websocket.BinaryMessage {
			if !initialized || len(data) < 4 {
				continue
			}

			chunkIdx := int(binary.LittleEndian.Uint32(data[:4]))
			chunkData := data[4:]

			chunkPath := filepath.Join(transferDir, fmt.Sprintf("%d.chunk", chunkIdx))
			os.WriteFile(chunkPath, chunkData, 0644)

			conn.WriteJSON(map[string]interface{}{"ack": chunkIdx})
		}
	}
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reader, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "Invalid", http.StatusBadRequest)
		return
	}

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil || part.FormName() != "file" {
			if part != nil {
				part.Close()
			}
			continue
		}

		filename := part.FileName()
		if filename == "" {
			part.Close()
			continue
		}

		s.storageService.WriteStream(filename, part)
		part.Close()
		log.Printf("[AERO] ✓ %s", filename)
		break
	}

	w.Write([]byte(`{"status":"success"}`))
}

func getLocalIP() string {
	addrs, _ := net.InterfaceAddrs()
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "localhost"
}

func getChunkFiles(dir string) []int {
	entries, _ := os.ReadDir(dir)
	chunks := []int{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".chunk") {
			idx, _ := strconv.Atoi(strings.TrimSuffix(e.Name(), ".chunk"))
			chunks = append(chunks, idx)
		}
	}
	sort.Ints(chunks)
	return chunks
}
