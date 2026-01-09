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

// Package main is the entry point for the AERO file transfer server.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/username/aero/internal/config"
	"github.com/username/aero/internal/server"
	"github.com/username/aero/internal/storage"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	// Load configuration
	cfg := config.Default()

	// Initialize storage layer (Dependency Injection)
	storageService, err := storage.NewFileStorage(cfg.UploadDir)
	if err != nil {
		log.Fatalf("[AERO] Failed to initialize storage: %v", err)
	}

	// Initialize HTTP server with E2EE (Dependency Injection)
	srv, err := server.NewServer(cfg, storageService)
	if err != nil {
		log.Fatalf("[AERO] Failed to initialize server: %v", err)
	}

	// Create context that cancels on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start server in a goroutine
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- srv.Start()
	}()

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErrors:
		if err != nil {
			log.Fatalf("[AERO] Server failed: %v", err)
		}
	case <-ctx.Done():
		log.Println("[AERO] Shutdown signal received")
	}

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(cfg.ShutdownTimeoutSeconds)*time.Second,
	)
	defer cancel()

	log.Printf("[AERO] Waiting up to %d seconds for active transfers to complete...",
		cfg.ShutdownTimeoutSeconds)

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[AERO] Forced shutdown: %v", err)
	} else {
		log.Println("[AERO] Graceful shutdown complete")
	}
}
