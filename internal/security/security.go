// MIT License
// Copyright (c) 2026 Project AERO Contributors

// Package security implements Aero's session cryptography.
//
// Protocol (v1)
//
// The desktop app generates a random 32-byte session key K each time the
// server starts. K reaches the phone only optically, inside the QR code's URL
// fragment (http://ip:port/#k=<base64url K>). Browsers never send the fragment
// over the network, so K never crosses the LAN.
//
// Every payload is sealed with XChaCha20-Poly1305 under K, using a fresh random
// 24-byte nonce:
//
//	box = nonce(24) || ciphertext || tag(16)
//
// Each kind of message uses its own associated data (AAD), which binds a box to
// its purpose, transfer ID and chunk index. A box cannot be replayed into a
// different slot, and any tampering makes decryption fail.
//
// Requests that carry no sealed body (WebSocket connect, downloads, cancel)
// carry an auth token instead: a box with an empty plaintext whose AAD is the
// request method and path. The server rejects tokens it has already seen, so a
// captured token cannot be replayed.
//
// The phone uses @noble/ciphers (audited, MIT) for the same construction,
// because the browser's WebCrypto API is unavailable on plain-HTTP LAN pages.
package security

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/chacha20poly1305"
)

// KeySize is the session key length in bytes.
const KeySize = chacha20poly1305.KeySize

// Overhead is the number of bytes a sealed box adds to its plaintext.
const Overhead = chacha20poly1305.NonceSizeX + chacha20poly1305.Overhead

// maxSeenTokens bounds the replay cache. At 4 MiB per chunk this allows
// roughly 2 TB of transfers before the phone must rescan the QR code.
const maxSeenTokens = 500_000

var (
	// ErrAuth means a box or token failed authentication.
	ErrAuth = errors.New("authentication failed")
	// ErrReplay means a token was presented twice.
	ErrReplay = errors.New("token already used")
	// ErrSessionExhausted means the replay cache is full and the session must be restarted.
	ErrSessionExhausted = errors.New("session exhausted; restart the server and rescan")
)

// AAD domain-separation labels. They must match the phone client exactly.
const (
	aadAuth          = "aero/v1/auth"
	aadWSServer      = "aero/v1/ws/s2c"
	aadUploadInit    = "aero/v1/upload-init"
	aadUploadChunk   = "aero/v1/upload-chunk"
	aadDownloadChunk = "aero/v1/download-chunk"
)

func joinAAD(parts ...string) []byte {
	return []byte(strings.Join(parts, "\x00"))
}

// AuthAAD is the associated data for a request auth token.
func AuthAAD(method, path string) []byte { return joinAAD(aadAuth, method, path) }

// WSServerAAD is the associated data for server-to-phone WebSocket messages.
func WSServerAAD() []byte { return joinAAD(aadWSServer) }

// UploadInitAAD is the associated data for an upload's metadata.
func UploadInitAAD() []byte { return joinAAD(aadUploadInit) }

// UploadChunkAAD is the associated data for one uploaded chunk.
func UploadChunkAAD(id string, index, total int) []byte {
	return joinAAD(aadUploadChunk, id, fmt.Sprint(index), fmt.Sprint(total))
}

// DownloadChunkAAD is the associated data for one downloaded chunk.
func DownloadChunkAAD(id string, index, total int) []byte {
	return joinAAD(aadDownloadChunk, id, fmt.Sprint(index), fmt.Sprint(total))
}

// GenerateSessionKey returns a fresh random key and its URL-safe encoding.
func GenerateSessionKey() (string, []byte, error) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return "", nil, fmt.Errorf("failed to generate random key: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(key), key, nil
}

// Session holds the key for one server run and the replay cache for auth tokens.
type Session struct {
	key []byte

	mu   sync.Mutex
	seen map[[chacha20poly1305.NonceSizeX]byte]struct{}
}

// NewSession creates a Session for key, which must be KeySize bytes.
func NewSession(key []byte) (*Session, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("invalid key size: got %d, expected %d", len(key), KeySize)
	}
	k := make([]byte, KeySize)
	copy(k, key)
	return &Session{key: k, seen: make(map[[chacha20poly1305.NonceSizeX]byte]struct{})}, nil
}

// Seal encrypts plaintext and returns nonce||ciphertext||tag.
func (s *Session) Seal(plaintext, aad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(s.key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, chacha20poly1305.NonceSizeX, chacha20poly1305.NonceSizeX+len(plaintext)+aead.Overhead())
	if _, err := rand.Read(out); err != nil {
		return nil, err
	}
	return aead.Seal(out, out, plaintext, aad), nil
}

// Open authenticates and decrypts a box produced by Seal (or the phone client).
// The plaintext is decrypted in place and aliases box.
func (s *Session) Open(box, aad []byte) ([]byte, error) {
	if len(box) < Overhead {
		return nil, ErrAuth
	}
	aead, err := chacha20poly1305.NewX(s.key)
	if err != nil {
		return nil, err
	}
	nonce, ct := box[:chacha20poly1305.NonceSizeX], box[chacha20poly1305.NonceSizeX:]
	pt, err := aead.Open(ct[:0], nonce, ct, aad)
	if err != nil {
		return nil, ErrAuth
	}
	return pt, nil
}

// VerifyToken checks a base64url auth token for the given method and path,
// rejecting any token that has been used before.
func (s *Session) VerifyToken(token, method, path string) error {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != Overhead {
		return ErrAuth
	}
	var nonce [chacha20poly1305.NonceSizeX]byte
	copy(nonce[:], raw)

	if _, err := s.Open(raw, AuthAAD(method, path)); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, dup := s.seen[nonce]; dup {
		return ErrReplay
	}
	if len(s.seen) >= maxSeenTokens {
		return ErrSessionExhausted
	}
	s.seen[nonce] = struct{}{}
	return nil
}

// Destroy zeroes the key. The Session must not be used afterwards.
func (s *Session) Destroy() {
	for i := range s.key {
		s.key[i] = 0
	}
}
