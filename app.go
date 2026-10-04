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
	goruntime "runtime"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/username/aero/internal/transfer"
	"github.com/username/aero/pkg/networking"
)

// preferredPort is tried first; the server falls back to any free port.
const preferredPort = 8080

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

// App struct serves as the bridge between Go backend and React frontend.
// It holds references to the server infrastructure and manages lifecycle.
type App struct {
	ctx         context.Context
	downloadDir string

	serverMu  sync.Mutex
	server    *transfer.Server
	currentIP string
}

// NewApp creates a new App instance with default configuration.
func NewApp() *App {
	return &App{
		downloadDir: transfer.DefaultDownloadDir(),
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

// StartServer starts the transfer server on the specified IP with a fresh
// session key. Emits "server:started" with the QR code URL.
func (a *App) StartServer(ip string) error {
	a.serverMu.Lock()
	defer a.serverMu.Unlock()

	if a.server != nil {
		return fmt.Errorf("server is already running")
	}

	srv, err := transfer.Start(transfer.Options{
		IP:            ip,
		PreferredPort: preferredPort,
		DownloadDir:   a.downloadDir,
		OnEvent:       a.onTransferEvent,
		OnError: func(err error) {
			wailsruntime.EventsEmit(a.ctx, "server:error", map[string]string{"error": err.Error()})
		},
	})
	if err != nil {
		return err
	}
	a.server = srv
	a.currentIP = ip

	wailsruntime.EventsEmit(a.ctx, "server:started", a.statusLocked())
	return nil
}

// StopServer shuts down the server, which also wipes the session key so the
// old QR code stops working. Emits "server:stopped".
func (a *App) StopServer() error {
	a.serverMu.Lock()
	defer a.serverMu.Unlock()

	if a.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := a.server.Close(ctx)
	a.server = nil

	wailsruntime.EventsEmit(a.ctx, "server:stopped", map[string]bool{
		"running": false,
	})
	return err
}

// GetServerStatus returns the current server status.
func (a *App) GetServerStatus() ServerStatus {
	a.serverMu.Lock()
	defer a.serverMu.Unlock()
	return a.statusLocked()
}

func (a *App) statusLocked() ServerStatus {
	if a.server == nil {
		return ServerStatus{Running: false}
	}
	return ServerStatus{
		Running: true,
		URL:     a.server.URL(),
		IP:      a.currentIP,
		Port:    a.server.Port(),
	}
}

// OpenDownloadsFolder opens the folder received files are saved to.
func (a *App) OpenDownloadsFolder() error {
	if err := os.MkdirAll(a.downloadDir, 0o755); err != nil {
		return err
	}

	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", a.downloadDir)
	case "darwin":
		cmd = exec.Command("open", a.downloadDir)
	default: // Linux and others
		cmd = exec.Command("xdg-open", a.downloadDir)
	}

	return cmd.Start()
}

// onTransferEvent is called by the server when transfer events occur.
func (a *App) onTransferEvent(event transfer.Event) {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "transfer:progress", event)
	}
}

// SendFileToPhone opens a file picker and offers the selected file to the
// connected phone. Only files picked here can ever be downloaded by the phone.
func (a *App) SendFileToPhone() error {
	a.serverMu.Lock()
	srv := a.server
	a.serverMu.Unlock()
	if srv == nil {
		return fmt.Errorf("server not running")
	}
	if srv.PhoneCount() == 0 {
		return transfer.ErrNoPhone
	}

	filePath, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select file to send",
	})
	if err != nil {
		return err
	}
	if filePath == "" {
		return nil // User cancelled
	}

	return srv.OfferFile(filePath)
}

// IsPhoneConnected returns true if a phone is currently connected
func (a *App) IsPhoneConnected() bool {
	a.serverMu.Lock()
	defer a.serverMu.Unlock()
	return a.server != nil && a.server.PhoneCount() > 0
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
		// Docked Bottom-Right (Approximate WorkArea due to Wails struct limitations)
		width := 600
		height := 120

		// Use detected Size
		screenWidth := 1920
		screenHeight := 1080
		
		if primary.Size.Width > 0 {
			screenWidth = primary.Size.Width
			screenHeight = primary.Size.Height
		}

	// Calculate Position: Bottom-Right Dock position
		// "Move to WorkArea.Right - 600, WorkArea.Bottom - 120"
		// Assuming taskbar is ~48px height on bottom
		
		x := screenWidth - width
		y := screenHeight - height - 48

		// CRITICAL: Do NOT use WindowToggleMaximise. Use WindowSetSize and WindowSetPosition.
		wailsruntime.WindowSetSize(a.ctx, width, height)
		wailsruntime.WindowSetPosition(a.ctx, x, y)
		wailsruntime.WindowSetAlwaysOnTop(a.ctx, true)
		
		// Robustness: Re-apply size to fight OS animations
		go func() {
			goruntime.Gosched()
			wailsruntime.WindowSetSize(a.ctx, width, height)
		}()
		
	} else {
		// STANDARD MODE SPEC: 400x700 Centered
		wailsruntime.WindowSetSize(a.ctx, 400, 700)
		wailsruntime.WindowCenter(a.ctx)
		wailsruntime.WindowSetAlwaysOnTop(a.ctx, false)
	}
}
