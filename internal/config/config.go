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

// Package config provides centralized configuration for the AERO server.
package config

// BufferSize defines the fixed buffer size for streaming operations.
// 256KB provides good balance of throughput and memory usage.
const BufferSize = 256 * 1024

// Config holds all configurable parameters for the AERO server.
type Config struct {
	// Port specifies the TCP port the HTTP server listens on.
	Port string

	// UploadDir specifies the directory where uploaded files are stored.
	UploadDir string

	// ShutdownTimeout specifies the maximum duration to wait for
	// active connections to complete during graceful shutdown.
	ShutdownTimeoutSeconds int
}

// Default returns a Config with sensible default values.
func Default() Config {
	return Config{
		Port:                   "8080",
		UploadDir:              "uploads",
		ShutdownTimeoutSeconds: 30,
	}
}
