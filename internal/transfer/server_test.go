package transfer

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/username/aero/internal/security"
)

// client is a minimal Go implementation of the phone protocol.
type client struct {
	t    *testing.T
	s    *Server
	base string
}

func startTestServer(t *testing.T) (*Server, *client) {
	t.Helper()
	dir := t.TempDir()
	s, err := Start(Options{IP: "127.0.0.1", DownloadDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close(context.Background()) })
	return s, &client{t: t, s: s, base: "http://" + s.hostport}
}

func (c *client) token(method, path string) string {
	box, err := c.s.session.Seal(nil, security.AuthAAD(method, path))
	if err != nil {
		c.t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(box)
}

func (c *client) do(method, path string, body []byte, auth bool) *http.Response {
	c.t.Helper()
	req, _ := http.NewRequest(method, c.base+path, bytes.NewReader(body))
	if auth {
		req.Header.Set(authHeader, c.token(method, path))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	c.t.Cleanup(func() { res.Body.Close() })
	return res
}

func (c *client) seal(plain, aad []byte) []byte {
	box, err := c.s.session.Seal(plain, aad)
	if err != nil {
		c.t.Fatal(err)
	}
	return box
}

func (c *client) initUpload(name string, size int64, chunkSize int) (string, int) {
	c.t.Helper()
	meta, _ := json.Marshal(uploadInitRequest{Name: name, Size: size, ChunkSize: chunkSize})
	res := c.do("POST", "/api/upload", c.seal(meta, security.UploadInitAAD()), true)
	if res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		c.t.Fatalf("init: %d %s", res.StatusCode, b)
	}
	var out struct {
		ID          string
		TotalChunks int
	}
	json.NewDecoder(res.Body).Decode(&out)
	return out.ID, out.TotalChunks
}

func (c *client) upload(name string, data []byte, chunkSize int) {
	c.t.Helper()
	id, total := c.initUpload(name, int64(len(data)), chunkSize)
	// Send chunks in reverse to prove ordering does not matter.
	for i := total - 1; i >= 0; i-- {
		chunk := data[i*chunkSize : min((i+1)*chunkSize, len(data))]
		path := fmt.Sprintf("/api/upload/%s/%d", id, i)
		res := c.do("PUT", path, c.seal(chunk, security.UploadChunkAAD(id, i, total)), true)
		if res.StatusCode != 200 {
			b, _ := io.ReadAll(res.Body)
			c.t.Fatalf("chunk %d: %d %s", i, res.StatusCode, b)
		}
	}
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

func TestUploadSavesSafelyAndNeverOverwrites(t *testing.T) {
	s, c := startTestServer(t)
	data := randomBytes(minChunkSize*3 + 123)

	c.upload(`..\..\evil/../photo.jpg`, data, minChunkSize)
	c.upload("photo.jpg", data[:10], minChunkSize)

	got, err := os.ReadFile(filepath.Join(s.dir, "photo.jpg"))
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("first upload: err=%v equal=%v", err, bytes.Equal(got, data))
	}
	got, err = os.ReadFile(filepath.Join(s.dir, "photo (1).jpg"))
	if err != nil || !bytes.Equal(got, data[:10]) {
		t.Fatalf("second upload should not overwrite: err=%v", err)
	}
	entries, _ := os.ReadDir(s.dir)
	if len(entries) != 2 {
		t.Errorf("expected exactly 2 files in download dir, got %d", len(entries))
	}
}

func TestEmptyFileUpload(t *testing.T) {
	s, c := startTestServer(t)
	c.upload("empty.txt", nil, minChunkSize)
	if info, err := os.Stat(filepath.Join(s.dir, "empty.txt")); err != nil || info.Size() != 0 {
		t.Fatalf("empty upload: %v", err)
	}
}

func TestAPIRequiresAuth(t *testing.T) {
	_, c := startTestServer(t)
	for _, r := range []struct{ method, path string }{
		{"POST", "/api/upload"},
		{"PUT", "/api/upload/x/0"},
		{"DELETE", "/api/upload/x"},
		{"GET", "/api/download/x/0"},
		{"GET", "/api/ws"},
	} {
		if res := c.do(r.method, r.path, nil, false); res.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s without auth: %d", r.method, r.path, res.StatusCode)
		}
	}
	// The old arbitrary-file endpoints must not exist.
	for _, p := range []string{"/api/file?path=C:/Windows/win.ini", "/api/folder?path=C:/", "/api/bug", "/download-direct"} {
		if res := c.do("GET", p, nil, false); res.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s: %d, want 404", p, res.StatusCode)
		}
	}
}

func TestTokenReplayRejected(t *testing.T) {
	_, c := startTestServer(t)
	meta, _ := json.Marshal(uploadInitRequest{Name: "a", Size: 1, ChunkSize: minChunkSize})
	body := c.seal(meta, security.UploadInitAAD())
	tok := c.token("POST", "/api/upload")
	for i, want := range []int{200, 401} {
		req, _ := http.NewRequest("POST", c.base+"/api/upload", bytes.NewReader(body))
		req.Header.Set(authHeader, tok)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != want {
			t.Errorf("attempt %d: %d, want %d", i, res.StatusCode, want)
		}
	}
}

func TestChunkValidation(t *testing.T) {
	_, c := startTestServer(t)
	id, total := c.initUpload("f.bin", int64(minChunkSize)*2, minChunkSize)
	chunk := randomBytes(minChunkSize)

	// Sealed for index 1 but sent to index 0.
	if res := c.do("PUT", "/api/upload/"+id+"/0", c.seal(chunk, security.UploadChunkAAD(id, 1, total)), true); res.StatusCode != 401 {
		t.Errorf("misplaced chunk: %d", res.StatusCode)
	}
	// Truncated chunk.
	if res := c.do("PUT", "/api/upload/"+id+"/0", c.seal(chunk[:10], security.UploadChunkAAD(id, 0, total)), true); res.StatusCode != 400 {
		t.Errorf("short chunk: %d", res.StatusCode)
	}
	// Out of range.
	if res := c.do("PUT", "/api/upload/"+id+"/2", c.seal(chunk, security.UploadChunkAAD(id, 2, total)), true); res.StatusCode != 400 {
		t.Errorf("out of range: %d", res.StatusCode)
	}
	// Unsealed upload metadata.
	if res := c.do("POST", "/api/upload", []byte(`{"name":"x","size":1,"chunkSize":65536}`), true); res.StatusCode != 401 {
		t.Errorf("plaintext init: %d", res.StatusCode)
	}
}

func TestCancelAndCloseRemovePartialFiles(t *testing.T) {
	s, c := startTestServer(t)
	id, _ := c.initUpload("big.bin", 1<<20, minChunkSize)
	if res := c.do("DELETE", "/api/upload/"+id, nil, true); res.StatusCode != http.StatusNoContent {
		t.Fatalf("cancel: %d", res.StatusCode)
	}
	c.initUpload("big2.bin", 1<<20, minChunkSize)
	s.Close(context.Background())

	entries, _ := os.ReadDir(s.dir)
	for _, e := range entries {
		t.Errorf("leftover file after cancel/close: %s", e.Name())
	}
}

func TestHostHeaderChecked(t *testing.T) {
	_, c := startTestServer(t)
	req, _ := http.NewRequest("GET", c.base+"/", nil)
	req.Host = "attacker.example:80"
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusMisdirectedRequest {
		t.Errorf("foreign Host: %d", res.StatusCode)
	}
}

func TestIndexServedWithSecurityHeaders(t *testing.T) {
	_, c := startTestServer(t)
	res := c.do("GET", "/", nil, false)
	if res.StatusCode != 200 {
		t.Fatalf("index: %d", res.StatusCode)
	}
	csp := res.Header.Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "unsafe-eval") {
		t.Errorf("CSP: %q", csp)
	}
	if res.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Error("missing Referrer-Policy")
	}
	for _, p := range []string{"/app.js", "/cipher.js"} {
		if res := c.do("GET", p, nil, false); res.StatusCode != 200 {
			t.Errorf("%s: %d", p, res.StatusCode)
		}
	}
}

func (c *client) dialWS(origin string) (*websocket.Conn, error) {
	u := url.URL{Scheme: "ws", Host: c.s.hostport, Path: "/api/ws", RawQuery: "auth=" + c.token("GET", "/api/ws")}
	h := http.Header{}
	if origin != "" {
		h.Set("Origin", origin)
	}
	conn, res, err := websocket.DefaultDialer.Dial(u.String(), h)
	if res != nil {
		res.Body.Close()
	}
	return conn, err
}

func TestWebSocketRejectsForeignOrigin(t *testing.T) {
	_, c := startTestServer(t)
	if _, err := c.dialWS("http://evil.example"); err == nil {
		t.Error("foreign origin accepted")
	}
	if _, err := c.dialWS(""); err == nil {
		t.Error("missing origin accepted")
	}
}

func TestDownloadOfferedFile(t *testing.T) {
	s, c := startTestServer(t)

	src := filepath.Join(t.TempDir(), "report.pdf")
	data := randomBytes(downloadChunkSize + 4567)
	os.WriteFile(src, data, 0o644)

	if err := s.OfferFile(src); err != ErrNoPhone {
		t.Fatalf("offer without phone: %v", err)
	}

	conn, err := c.dialWS("http://" + s.hostport)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for s.PhoneCount() == 0 {
		time.Sleep(10 * time.Millisecond)
	}

	if err := s.OfferFile(src); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, box, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	plain, err := s.session.Open(box, security.WSServerAAD())
	if err != nil {
		t.Fatalf("offer not sealed correctly: %v", err)
	}
	var offer struct {
		Type, ID, Name    string
		Size              int64
		TotalChunks       int
	}
	json.Unmarshal(plain, &offer)
	if offer.Type != "offer" || offer.Name != "report.pdf" || offer.Size != int64(len(data)) || offer.TotalChunks != 2 {
		t.Fatalf("offer: %+v", offer)
	}

	var got []byte
	for i := 0; i < offer.TotalChunks; i++ {
		res := c.do("GET", fmt.Sprintf("/api/download/%s/%d", offer.ID, i), nil, true)
		box, _ := io.ReadAll(res.Body)
		chunk, err := s.session.Open(box, security.DownloadChunkAAD(offer.ID, i, offer.TotalChunks))
		if err != nil {
			t.Fatalf("chunk %d: %v (status %d)", i, err, res.StatusCode)
		}
		got = append(got, chunk...)
	}
	if !bytes.Equal(got, data) {
		t.Error("downloaded data differs")
	}

	// Changing the file after offering it invalidates the offer.
	os.WriteFile(src, []byte("changed"), 0o644)
	if res := c.do("GET", "/api/download/"+offer.ID+"/0", nil, true); res.StatusCode != http.StatusConflict {
		t.Errorf("changed file: %d", res.StatusCode)
	}
}

func TestSanitizeFilename(t *testing.T) {
	for in, want := range map[string]string{
		"photo.jpg":             "photo.jpg",
		`..\..\Windows\win.ini`: "win.ini",
		"../../etc/passwd":      "passwd",
		"a<b>c:d\"e|f?g*h.txt":  "a_b_c_d_e_f_g_h.txt",
		"CON":                   "_CON",
		"nul.txt":               "_nul.txt",
		"trailing. . ":          "trailing",
		"..":                    "file",
		"":                      "file",
		"tab\there.txt":         "tab_here.txt",
		"日本語.txt":               "日本語.txt",
	} {
		if got := SanitizeFilename(in); got != want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", in, got, want)
		}
	}
	long := strings.Repeat("é", 300) + ".mp4"
	if got := SanitizeFilename(long); len(got) > maxNameBytes || !strings.HasSuffix(got, ".mp4") {
		t.Errorf("long name: len=%d %q", len(got), got[len(got)-8:])
	}
}
