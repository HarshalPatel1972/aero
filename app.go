// MIT License
//
// Copyright (c) 2026 Project AERO Contributors
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/username/aero/internal/config"
	"github.com/username/aero/internal/security"
	"github.com/username/aero/internal/server"
	"github.com/username/aero/internal/storage"
	"github.com/username/aero/pkg/networking"
)

// NetworkInterface represents a network interface for the UI.
type NetworkInterface struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// ServerStatus represents the current server state.
type ServerStatus struct {
	Running bool   `json:"running"`
	URL     string `json:"url"`
	IP      string `json:"ip"`
	Port    string `json:"port"`
}

// Note: TransferEvent is defined in server package, we use that type directly.

// App struct serves as the bridge between Go backend and React frontend.
// It holds references to the server infrastructure and manages lifecycle.
type App struct {
	ctx    context.Context
	config config.Config

	// Server infrastructure
	server         *server.Server
	storage        storage.Service
	serverCancel   context.CancelFunc
	serverMu       sync.Mutex
	serverRunning  bool

	// Session data
	sessionKey       []byte
	sessionKeyBase64 string
	currentIP        string
}

// NewApp creates a new App instance with default configuration.
func NewApp() *App {
	return &App{
		config: config.Default(),
	}
}

// startup is called when the app starts. The context is saved for runtime calls.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// shutdown is called when the app is closing.
func (a *App) shutdown(ctx context.Context) {
	a.StopServer()
}

// GetLocalIPs returns all available network interfaces for the UI dropdown.
// Uses the smart networking package to filter virtual adapters and prioritize
// real LAN interfaces (WiFi, Ethernet) over Docker/VMware/WSL adapters.
func (a *App) GetLocalIPs() []NetworkInterface {
	_, candidates, err := networking.GetPreferredOutboundIP()
	if err != nil {
		return []NetworkInterface{}
	}

	// Convert networking.NetworkInterface to our local type
	result := make([]NetworkInterface, len(candidates))
	for i, c := range candidates {
		result[i] = NetworkInterface{
			Name: c.Name,
			IP:   c.IP,
		}
	}

	return result
}

// StartServer initializes and starts the HTTP server on the specified IP.
// Emits "server:started" event with the QR code URL.
func (a *App) StartServer(ip string) error {
	a.serverMu.Lock()
	defer a.serverMu.Unlock()

	if a.serverRunning {
		return fmt.Errorf("server is already running")
	}

	// Initialize storage
	storageService, err := storage.NewFileStorage(a.config.UploadDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	a.storage = storageService

	// Generate session key
	keyBase64, keyBytes, err := security.GenerateSessionKey()
	if err != nil {
		return fmt.Errorf("failed to generate session key: %w", err)
	}
	a.sessionKey = keyBytes
	a.sessionKeyBase64 = keyBase64
	a.currentIP = ip

	// Create server with OUR session key (critical: must match URL key)
	srv, err := server.NewServerWithKey(a.config, storageService, keyBytes, keyBase64, a.onTransferEvent)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	a.server = srv

	// Create cancellable context for server
	ctx, cancel := context.WithCancel(context.Background())
	a.serverCancel = cancel

	// Start server in goroutine
	go func() {
		a.server.SetWailsContext(a.ctx) // Ensure server handles Wails events
		if err := a.server.StartWithContext(ctx, ip); err != nil {
			wailsruntime.EventsEmit(a.ctx, "server:error", map[string]string{
				"error": err.Error(),
			})
		}
	}()

	a.serverRunning = true

	// Build URL with session key in hash fragment
	url := fmt.Sprintf("http://%s:%s/#%s", ip, a.config.Port, a.sessionKeyBase64)

	// Emit event to frontend
	wailsruntime.EventsEmit(a.ctx, "server:started", ServerStatus{
		Running: true,
		URL:     url,
		IP:      ip,
		Port:    a.config.Port,
	})

	return nil
}

// StopServer gracefully shuts down the HTTP server.
// Emits "server:stopped" event.
func (a *App) StopServer() error {
	a.serverMu.Lock()
	defer a.serverMu.Unlock()

	if !a.serverRunning {
		return nil
	}

	if a.serverCancel != nil {
		a.serverCancel()
	}

	if a.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5)
		defer cancel()
		a.server.Shutdown(ctx)
	}

	a.serverRunning = false
	a.server = nil

	wailsruntime.EventsEmit(a.ctx, "server:stopped", map[string]bool{
		"running": false,
	})

	return nil
}

// GetServerStatus returns the current server status.
func (a *App) GetServerStatus() ServerStatus {
	a.serverMu.Lock()
	defer a.serverMu.Unlock()

	if !a.serverRunning {
		return ServerStatus{Running: false}
	}

	url := fmt.Sprintf("http://%s:%s/#%s", a.currentIP, a.config.Port, a.sessionKeyBase64)
	return ServerStatus{
		Running: true,
		URL:     url,
		IP:      a.currentIP,
		Port:    a.config.Port,
	}
}

// OpenDownloadsFolder opens the uploads directory in the system file explorer.
func (a *App) OpenDownloadsFolder() error {
	// Get absolute path to uploads folder
	absPath, err := filepath.Abs(a.config.UploadDir)
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return err
	}

	// Open folder based on OS
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", absPath)
	case "darwin":
		cmd = exec.Command("open", absPath)
	default: // Linux and others
		cmd = exec.Command("xdg-open", absPath)
	}

	return cmd.Start()
}

// onTransferEvent is called by the server when transfer events occur.
func (a *App) onTransferEvent(event server.TransferEvent) {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "transfer:progress", event)
	}
}

// SendFileToPhone opens a file picker and sends the selected file to the connected phone.
func (a *App) SendFileToPhone() error {
	a.serverMu.Lock()
	if !a.serverRunning || a.server == nil {
		a.serverMu.Unlock()
		return fmt.Errorf("server not running")
	}
	a.serverMu.Unlock()

	// Open file dialog
	filePath, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select file to send",
	})
	if err != nil {
		return err
	}
	if filePath == "" {
		return nil // User cancelled
	}

	// Send via server
	return a.server.SendFileToPhone(filePath)
}

// IsPhoneConnected returns true if a phone is currently connected
func (a *App) IsPhoneConnected() bool {
	a.serverMu.Lock()
	defer a.serverMu.Unlock()
	
	if !a.serverRunning || a.server == nil {
		return false
	}
	
	clients := a.server.GetConnectedClients()
	return len(clients) > 0
}

// SetMiniMode toggles the application between Standard and Mini configurations.
func (a *App) SetMiniMode(enabled bool) {
	if a.ctx == nil {
		return
	}

	if enabled {
		// Get primary screen to calculate position relative to Taskbar
		screens, err := wailsruntime.ScreenGetAll(a.ctx)
		if err != nil || len(screens) == 0 {
			// Fallback: bottom-right 1080p
			wailsruntime.WindowSetSize(a.ctx, 600, 120)
			return
		}

		// Find primary screen
		var primary *wailsruntime.Screen
		for _, s := range screens {
			if s.IsPrimary {
				primary = &s
				break
			}
		}
		if primary == nil {
			primary = &screens[0]
		}

		// MINI MODE SPEC:
		// Width: 600, Height: 120
		// Docked Bottom-Right (using available screen info)
		width := 600
		height := 120

		// Use WorkArea to position (Respects Taskbar)
		// Rect struct usually has X, Y, Width, Height
		workArea := primary.WorkArea
		
		// If WorkArea is zero (unlikely on modern Wails), fallback to Bounds
		if workArea.Width == 0 || workArea.Height == 0 {
			workArea = primary.Bounds
		}

		// Calculate Position: Bottom-Right Dock position
		// Strict docking to the WorkArea edges
		x := workArea.X + workArea.Width - width
		y := workArea.Y + workArea.Height - height

		// ROBUSTNESS: Explicitly set size BEFORE position, then again after to ensure it sticks.
		// Wails/Windows sometimes ignores resize if moved simultaneously.
		wailsruntime.WindowSetSize(a.ctx, width, height)
		wailsruntime.WindowSetPosition(a.ctx, x, y)
		wailsruntime.WindowSetAlwaysOnTop(a.ctx, true)
		
		// "Double-tap" resize to force layout update if OS animation interfered
		go func() {
			goruntime.Gosched() // Yield
			wailsruntime.WindowSetSize(a.ctx, width, height)
		}()
		
	} else {
		// STANDARD MODE SPEC: 400x700 Centered
		wailsruntime.WindowSetSize(a.ctx, 400, 700)
		wailsruntime.WindowCenter(a.ctx)
		wailsruntime.WindowSetAlwaysOnTop(a.ctx, false)
	}
}
