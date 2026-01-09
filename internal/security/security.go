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

// Package security provides cryptographic primitives for the AERO E2EE protocol.
// Implements the "Optical-Key" Protocol using AES-256-CTR for streaming encryption.
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// KeySize defines the AES-256 key size in bytes.
// 32 bytes = 256 bits for AES-256.
const KeySize = 32

// IVSize defines the initialization vector size for AES-CTR.
// 16 bytes = 128 bits (AES block size).
const IVSize = 16

// GenerateSessionKey creates a cryptographically secure 32-byte session key.
// Returns URL-safe base64 encoding (no padding) suitable for URL hash fragments.
//
// Security: Uses crypto/rand which reads from the OS CSPRNG
// (CryptGenRandom on Windows, /dev/urandom on Unix).
func GenerateSessionKey() (string, []byte, error) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return "", nil, fmt.Errorf("failed to generate random key: %w", err)
	}

	// URL-safe base64, no padding - safe for hash fragments
	encoded := base64.RawURLEncoding.EncodeToString(key)
	return encoded, key, nil
}

// DecodeSessionKey decodes a URL-safe base64 session key back to bytes.
func DecodeSessionKey(encoded string) ([]byte, error) {
	key, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode session key: %w", err)
	}
	if len(key) != KeySize {
		return nil, fmt.Errorf("invalid key size: got %d, expected %d", len(key), KeySize)
	}
	return key, nil
}

// DecryptReader wraps an io.Reader to provide streaming AES-256-CTR decryption.
// The encrypted stream must have the IV prepended as the first 16 bytes.
//
// Architecture:
//   - First Read() call extracts the 16-byte IV from the stream
//   - Subsequent reads decrypt data in a streaming fashion
//   - No buffering of ciphertext - true stream cipher behavior
//
// Why AES-CTR over AES-GCM:
//
//	GCM provides authentication but requires the auth tag at stream end,
//	forcing full buffering for verification. CTR mode provides true
//	stream-cipher behavior with zero buffering - essential for 10GB+ files
//	on memory-constrained mobile devices.
//
// Threat Model Note:
//
//	CTR does not provide integrity/authentication. An attacker could flip
//	bits in transit. For LAN transfers with visual confirmation (QR scan),
//	this is acceptable. Phase 3 could add HMAC-based integrity if required.
type DecryptReader struct {
	source      io.Reader
	stream      cipher.Stream
	key         []byte
	initialized bool
}

// NewDecryptReader creates a new streaming decryptor.
// The key must be exactly 32 bytes (AES-256).
// The source reader must contain: [16-byte IV][encrypted data...]
func NewDecryptReader(key []byte, source io.Reader) (*DecryptReader, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("invalid key size: got %d, expected %d", len(key), KeySize)
	}
	return &DecryptReader{
		source: source,
		key:    key,
	}, nil
}

// Read implements io.Reader with streaming decryption.
// First call reads the IV from the stream and initializes the cipher.
// Subsequent calls decrypt data directly into the provided buffer.
func (dr *DecryptReader) Read(p []byte) (int, error) {
	// Lazy initialization: read IV on first Read() call
	if !dr.initialized {
		if err := dr.initialize(); err != nil {
			return 0, err
		}
	}

	// Read ciphertext and decrypt in-place
	n, err := dr.source.Read(p)
	if n > 0 {
		// XORKeyStream decrypts in-place - no additional allocation
		dr.stream.XORKeyStream(p[:n], p[:n])
	}
	return n, err
}

// initialize reads the IV from the stream and sets up the AES-CTR cipher.
func (dr *DecryptReader) initialize() error {
	// Read exactly IVSize bytes for the initialization vector
	iv := make([]byte, IVSize)
	if _, err := io.ReadFull(dr.source, iv); err != nil {
		if err == io.EOF {
			return fmt.Errorf("stream too short: missing IV")
		}
		return fmt.Errorf("failed to read IV: %w", err)
	}

	// Create AES cipher block
	block, err := aes.NewCipher(dr.key)
	if err != nil {
		return fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Create CTR stream cipher
	// CTR mode turns a block cipher into a stream cipher
	dr.stream = cipher.NewCTR(block, iv)
	dr.initialized = true

	return nil
}
