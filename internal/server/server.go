// MIT License
// Copyright (c) 2026 Project AERO Contributors

// Package server - Ultra-reliable chunked file transfer
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
	"time"

	"github.com/gorilla/websocket"
	"github.com/username/aero/internal/config"
	"github.com/username/aero/internal/security"
	"github.com/username/aero/internal/storage"
)

//go:embed templates/*
var templateFS embed.FS

var upgrader = websocket.Upgrader{
	ReadBufferSize:  2 * 1024 * 1024, // 2MB
	WriteBufferSize: 64 * 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type TransferEvent struct {
	Filename string  `json:"filename"`
	Progress float64 `json:"progress"`
	Speed    string  `json:"speed"`
	Status   string  `json:"status"`
}

type TransferCallback func(event TransferEvent)

type TransferMeta struct {
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	TotalChunks int    `json:"totalChunks"`
}

type Server struct {
	httpServer       *http.Server
	storageService   storage.Service
	config           config.Config
	sessionKey       []byte
	sessionKeyBase64 string
	onTransfer       TransferCallback
	tempDir          string
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
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/upload", s.handleUpload)
	mux.HandleFunc("/transfer", s.handleTransfer)
	mux.HandleFunc("/health", s.handleHealth)

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

// handleTransfer - Single WebSocket, sequential chunks, bulletproof
func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[AERO] Upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Increase timeouts for large files
	conn.SetReadDeadline(time.Time{}) // No timeout

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
				log.Printf("[AERO] JSON error: %v", err)
				continue
			}

			// Init message
			if name, ok := msg["name"].(string); ok {
				meta.Filename = name
				meta.Size = int64(msg["size"].(float64))
				meta.TotalChunks = int(msg["chunks"].(float64))

				// Create unique transfer directory
				transferDir = filepath.Join(s.tempDir, fmt.Sprintf("%d", time.Now().UnixNano()))
				os.MkdirAll(transferDir, 0755)

				// Save metadata
				metaPath := filepath.Join(transferDir, "meta.json")
				metaBytes, _ := json.Marshal(meta)
				os.WriteFile(metaPath, metaBytes, 0644)

				initialized = true
				log.Printf("[AERO] 📥 Started: %s (%d chunks, %d bytes)", name, meta.TotalChunks, meta.Size)
				s.emitTransferEvent(TransferEvent{Filename: name, Progress: 0, Status: "started"})

				conn.WriteJSON(map[string]interface{}{"status": "ready"})
				continue
			}

			// Done message
			if _, ok := msg["done"]; ok {
				if !initialized {
					conn.WriteJSON(map[string]interface{}{"error": "not initialized"})
					continue
				}

				// Count received chunks
				entries, _ := os.ReadDir(transferDir)
				received := 0
				for _, e := range entries {
					if strings.HasSuffix(e.Name(), ".chunk") {
						received++
					}
				}

				log.Printf("[AERO] Done signal: %d/%d chunks", received, meta.TotalChunks)

				if received < meta.TotalChunks {
					// Find missing
					missing := []int{}
					for i := 0; i < meta.TotalChunks; i++ {
						chunkPath := filepath.Join(transferDir, fmt.Sprintf("%d.chunk", i))
						if _, err := os.Stat(chunkPath); os.IsNotExist(err) {
							missing = append(missing, i)
						}
					}
					conn.WriteJSON(map[string]interface{}{
						"status":  "incomplete",
						"missing": missing,
					})
					continue
				}

				// All chunks received - assemble
				log.Printf("[AERO] Assembling %d chunks...", meta.TotalChunks)

				finalPath := filepath.Join(s.config.UploadDir, filepath.Clean(meta.Filename))
				finalFile, err := os.Create(finalPath)
				if err != nil {
					log.Printf("[AERO] Create failed: %v", err)
					conn.WriteJSON(map[string]interface{}{"error": err.Error()})
					continue
				}

				var totalWritten int64
				assemblyOK := true

				for i := 0; i < meta.TotalChunks; i++ {
					chunkPath := filepath.Join(transferDir, fmt.Sprintf("%d.chunk", i))
					chunkData, err := os.ReadFile(chunkPath)
					if err != nil {
						log.Printf("[AERO] Read chunk %d failed: %v", i, err)
						assemblyOK = false
						break
					}
					n, err := finalFile.Write(chunkData)
					if err != nil {
						log.Printf("[AERO] Write chunk %d failed: %v", i, err)
						assemblyOK = false
						break
					}
					totalWritten += int64(n)
				}

				finalFile.Close()

				if !assemblyOK || totalWritten != meta.Size {
					os.Remove(finalPath)
					log.Printf("[AERO] Assembly failed: wrote %d, expected %d", totalWritten, meta.Size)
					conn.WriteJSON(map[string]interface{}{"error": "assembly failed"})
					continue
				}

				// Cleanup temp
				os.RemoveAll(transferDir)

				log.Printf("[AERO] ✓ Complete: %s (%d bytes)", meta.Filename, totalWritten)
				s.emitTransferEvent(TransferEvent{Filename: meta.Filename, Progress: 100, Status: "completed"})

				conn.WriteJSON(map[string]interface{}{
					"status": "success",
					"size":   totalWritten,
				})
				return
			}
		} else if msgType == websocket.BinaryMessage {
			if !initialized || len(data) < 4 {
				continue
			}

			chunkIdx := int(binary.LittleEndian.Uint32(data[:4]))
			chunkData := data[4:]

			// Write chunk to file
			chunkPath := filepath.Join(transferDir, fmt.Sprintf("%d.chunk", chunkIdx))
			err := os.WriteFile(chunkPath, chunkData, 0644)
			if err != nil {
				log.Printf("[AERO] Chunk %d write failed: %v", chunkIdx, err)
			}

			// ACK the chunk
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

// Helper functions
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
