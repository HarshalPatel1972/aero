// Package security provides cryptographic primitives for AERO.
// vault.go: Ephemeral Session Key Vault ("The Optical Vault")
//
// Term-Phase 8: Zero-Trust Encryption
// This file adds the Vault struct which manages the ephemeral session key.
// Uses existing GenerateSessionKey() from security.go.

package security

import (
	"log"
	"sync"
)

// ═══════════════════════════════════════════════════════════════
// VAULT (Ephemeral Key Store)
// ═══════════════════════════════════════════════════════════════

// Vault holds the ephemeral session key.
// The key exists only in RAM and is destroyed when the process exits.
type Vault struct {
	mu         sync.RWMutex
	key        []byte // 32-byte AES-256 key
	keyBase64  string // URL-safe Base64 encoding
	rotationID int    // Increments on each key rotation
}

// NewVault creates a new Vault with a freshly generated session key.
// CRITICAL: This must be called once at server startup.
func NewVault() (*Vault, error) {
	v := &Vault{}
	if err := v.Rotate(); err != nil {
		return nil, err
	}
	log.Printf("[VAULT] 🔐 Ephemeral session key generated (RAM only)")
	return v, nil
}

// Rotate generates a new session key.
// This invalidates all existing encrypted sessions.
func (v *Vault) Rotate() error {
	// Use existing GenerateSessionKey from security.go
	keyBase64, keyBytes, err := GenerateSessionKey()
	if err != nil {
		return err
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	// Zero out old key before replacing (defense in depth)
	if v.key != nil {
		for i := range v.key {
			v.key[i] = 0
		}
	}

	v.key = keyBytes
	v.keyBase64 = keyBase64
	v.rotationID++

	return nil
}

// Key returns a copy of the current session key.
func (v *Vault) Key() []byte {
	v.mu.RLock()
	defer v.mu.RUnlock()

	keyCopy := make([]byte, len(v.key))
	copy(keyCopy, v.key)
	return keyCopy
}

// KeyBase64 returns the URL-safe Base64 encoding of the key.
// This is embedded in the QR code URL fragment.
func (v *Vault) KeyBase64() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.keyBase64
}

// RotationID returns the current key rotation counter.
func (v *Vault) RotationID() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.rotationID
}

// Destroy zeros out and clears the key.
// Called on graceful shutdown for defense in depth.
func (v *Vault) Destroy() {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.key != nil {
		for i := range v.key {
			v.key[i] = 0
		}
		v.key = nil
	}
	v.keyBase64 = ""
	log.Printf("[VAULT] 🔒 Session key destroyed")
}
