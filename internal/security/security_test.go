package security

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func newTestSession(t *testing.T) *Session {
	t.Helper()
	_, key, err := GenerateSessionKey()
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSession(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSealOpenRoundTrip(t *testing.T) {
	s := newTestSession(t)
	box, err := s.Seal([]byte("secret"), UploadInitAAD())
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Open(append([]byte(nil), box...), UploadInitAAD())
	if err != nil || string(got) != "secret" {
		t.Fatalf("Open = %q, %v", got, err)
	}
}

func TestOpenRejectsWrongContextAndTampering(t *testing.T) {
	s := newTestSession(t)
	box, _ := s.Seal([]byte("chunk"), UploadChunkAAD("id", 1, 4))

	if _, err := s.Open(append([]byte(nil), box...), UploadChunkAAD("id", 2, 4)); !errors.Is(err, ErrAuth) {
		t.Errorf("chunk moved to another index: err = %v, want ErrAuth", err)
	}
	if _, err := s.Open(append([]byte(nil), box...), UploadChunkAAD("other", 1, 4)); !errors.Is(err, ErrAuth) {
		t.Errorf("chunk moved to another transfer: err = %v, want ErrAuth", err)
	}
	tampered := append([]byte(nil), box...)
	tampered[len(tampered)-1] ^= 1
	if _, err := s.Open(tampered, UploadChunkAAD("id", 1, 4)); !errors.Is(err, ErrAuth) {
		t.Errorf("tampered box: err = %v, want ErrAuth", err)
	}
	other := newTestSession(t)
	if _, err := other.Open(append([]byte(nil), box...), UploadChunkAAD("id", 1, 4)); !errors.Is(err, ErrAuth) {
		t.Errorf("wrong key: err = %v, want ErrAuth", err)
	}
}

func makeToken(t *testing.T, s *Session, method, path string) string {
	t.Helper()
	box, err := s.Seal(nil, AuthAAD(method, path))
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(box)
}

func TestVerifyToken(t *testing.T) {
	s := newTestSession(t)
	tok := makeToken(t, s, "GET", "/api/ws")

	if err := s.VerifyToken(tok, "POST", "/api/ws"); !errors.Is(err, ErrAuth) {
		t.Errorf("wrong method: %v", err)
	}
	if err := s.VerifyToken(tok, "GET", "/api/other"); !errors.Is(err, ErrAuth) {
		t.Errorf("wrong path: %v", err)
	}
	if err := s.VerifyToken(tok, "GET", "/api/ws"); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if err := s.VerifyToken(tok, "GET", "/api/ws"); !errors.Is(err, ErrReplay) {
		t.Errorf("replayed token: %v, want ErrReplay", err)
	}
	for _, bad := range []string{"", "!!", "AAAA"} {
		if err := s.VerifyToken(bad, "GET", "/api/ws"); !errors.Is(err, ErrAuth) {
			t.Errorf("garbage token %q: %v", bad, err)
		}
	}
}

// TestJavaScriptInterop opens values sealed by the phone client's bundled
// cipher (testdata/gen_vectors.mjs), proving both sides speak the same protocol.
func TestJavaScriptInterop(t *testing.T) {
	raw, err := os.ReadFile("testdata/js_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Key   string
		Token struct{ Method, Path, Value string }
		UploadInit struct {
			Plain, Box string
		}
		UploadChunk struct {
			ID           string
			Index, Total int
			Plain, Box   string
		}
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	key, _ := hex.DecodeString(v.Key)
	s, err := NewSession(key)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.VerifyToken(v.Token.Value, v.Token.Method, v.Token.Path); err != nil {
		t.Errorf("JS token rejected: %v", err)
	}

	box, _ := hex.DecodeString(v.UploadInit.Box)
	if got, err := s.Open(box, UploadInitAAD()); err != nil || string(got) != v.UploadInit.Plain {
		t.Errorf("JS upload-init: %q, %v", got, err)
	}

	box, _ = hex.DecodeString(v.UploadChunk.Box)
	got, err := s.Open(box, UploadChunkAAD(v.UploadChunk.ID, v.UploadChunk.Index, v.UploadChunk.Total))
	if err != nil || !bytes.Equal(got, []byte(v.UploadChunk.Plain)) {
		t.Errorf("JS upload-chunk: %q, %v", got, err)
	}
}
