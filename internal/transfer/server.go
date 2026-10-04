// MIT License
// Copyright (c) 2026 Project AERO Contributors

// Package transfer is Aero's LAN server: it serves the phone web client and
// moves files between phone and PC. Every API request is authenticated with
// the session key from the QR code, and every file byte, file name and control
// message is end-to-end encrypted (see package security for the protocol).
package transfer

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/username/aero/internal/security"
)

//go:embed web/*
var webFS embed.FS

const (
	// authHeader carries the request auth token.
	authHeader = "X-Aero-Auth"

	// uploadIdleTimeout discards uploads the phone has abandoned.
	uploadIdleTimeout = 15 * time.Minute
	// downloadTTL is how long a file offered to the phone stays downloadable.
	downloadTTL = time.Hour
	// maxActiveUploads caps concurrent uploads to bound open files and disk use.
	maxActiveUploads = 16
	// downloadChunkSize is the plaintext size of each PC-to-phone chunk.
	downloadChunkSize = 4 << 20
)

// Event reports transfer progress to the desktop UI.
type Event struct {
	Filename  string  `json:"filename"`
	Progress  float64 `json:"progress"`
	Speed     string  `json:"speed"`
	Status    string  `json:"status"`    // started | progress | completed | error
	Direction string  `json:"direction"` // receive (phone to PC) | send (PC to phone)
}

// Options configures a Server.
type Options struct {
	// IP is the local address to listen on (the network the user picked).
	IP string
	// PreferredPort is tried first; if it is in use, any free port is used.
	PreferredPort int
	// DownloadDir receives files sent from the phone.
	DownloadDir string
	// OnEvent receives progress events. It may be nil.
	OnEvent func(Event)
	// OnError is called if the server stops unexpectedly. It may be nil.
	OnError func(error)
}

// Server is a running transfer server bound to one session key.
type Server struct {
	session  *security.Session
	keyB64   string
	hostport string
	dir      string
	onEvent  func(Event)

	httpServer *http.Server
	stop       chan struct{}

	mu        sync.Mutex
	phones    map[*phone]struct{}
	uploads   map[string]*upload
	downloads map[string]*download
}

// Start generates a new session key, binds the listener and begins serving.
func Start(opts Options) (*Server, error) {
	if err := os.MkdirAll(opts.DownloadDir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create download folder: %w", err)
	}

	keyB64, key, err := security.GenerateSessionKey()
	if err != nil {
		return nil, err
	}
	session, err := security.NewSession(key)
	if err != nil {
		return nil, err
	}

	ln, err := listen(opts.IP, opts.PreferredPort)
	if err != nil {
		return nil, err
	}

	s := &Server{
		session:   session,
		keyB64:    keyB64,
		hostport:  ln.Addr().String(),
		dir:       opts.DownloadDir,
		onEvent:   opts.OnEvent,
		stop:      make(chan struct{}),
		phones:    make(map[*phone]struct{}),
		uploads:   make(map[string]*upload),
		downloads: make(map[string]*download),
	}

	s.httpServer = &http.Server{
		Handler:           s.routes(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	go func() {
		err := s.httpServer.Serve(tcpNoDelayListener{ln})
		if err != nil && !errors.Is(err, http.ErrServerClosed) && opts.OnError != nil {
			opts.OnError(err)
		}
	}()
	go s.janitor()

	log.Printf("[AERO] Listening on %s", s.hostport)
	return s, nil
}

// listen binds ip:preferred, falling back to an OS-assigned free port.
func listen(ip string, preferred int) (net.Listener, error) {
	if preferred > 0 {
		ln, err := net.Listen("tcp", net.JoinHostPort(ip, strconv.Itoa(preferred)))
		if err == nil {
			return ln, nil
		}
		log.Printf("[AERO] Port %d unavailable (%v); using a free port", preferred, err)
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(ip, "0"))
	if err != nil {
		return nil, fmt.Errorf("cannot listen on %s: %w", ip, err)
	}
	return ln, nil
}

// URL is the address to encode in the QR code, including the session key.
func (s *Server) URL() string {
	return "http://" + s.hostport + "/#k=" + s.keyB64
}

// Port is the TCP port actually in use.
func (s *Server) Port() string {
	_, port, _ := net.SplitHostPort(s.hostport)
	return port
}

// Close stops the server, disconnects phones, deletes partial uploads and
// wipes the session key. It is safe to call more than once.
func (s *Server) Close(ctx context.Context) error {
	s.mu.Lock()
	select {
	case <-s.stop:
		s.mu.Unlock()
		return nil
	default:
		close(s.stop)
	}
	for p := range s.phones {
		p.close()
		delete(s.phones, p)
	}
	for id, u := range s.uploads {
		u.abort()
		delete(s.uploads, id)
	}
	s.downloads = map[string]*download{}
	s.mu.Unlock()

	err := s.httpServer.Shutdown(ctx)
	if err != nil {
		s.httpServer.Close()
	}
	s.session.Destroy()
	return err
}

func (s *Server) emit(e Event) {
	if s.onEvent != nil {
		s.onEvent(e)
	}
}

// janitor expires abandoned uploads and stale download offers.
func (s *Server) janitor() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case now := <-t.C:
			s.mu.Lock()
			for id, u := range s.uploads {
				if now.Sub(u.lastActivity()) > uploadIdleTimeout {
					u.abort()
					delete(s.uploads, id)
					s.emit(Event{Filename: u.name, Status: "error", Direction: "receive"})
				}
			}
			for id, d := range s.downloads {
				if now.Sub(d.lastActivity()) > downloadTTL {
					delete(s.downloads, id)
				}
			}
			s.mu.Unlock()
		}
	}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// Public: the phone client itself. It contains no secrets.
	mux.HandleFunc("GET /{$}", s.serveStatic("web/index.html", "text/html; charset=utf-8"))
	mux.HandleFunc("GET /app.js", s.serveStatic("web/app.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("GET /cipher.js", s.serveStatic("web/cipher.js", "text/javascript; charset=utf-8"))

	// Authenticated API.
	mux.Handle("GET /api/ws", s.requireAuth(true, http.HandlerFunc(s.handleWS)))
	mux.Handle("POST /api/upload", s.requireAuth(false, http.HandlerFunc(s.handleUploadInit)))
	mux.Handle("PUT /api/upload/{id}/{index}", s.requireAuth(false, http.HandlerFunc(s.handleUploadChunk)))
	mux.Handle("DELETE /api/upload/{id}", s.requireAuth(false, http.HandlerFunc(s.handleUploadCancel)))
	mux.Handle("GET /api/download/{id}/{index}", s.requireAuth(false, http.HandlerFunc(s.handleDownloadChunk)))

	return s.checkHost(mux)
}

// checkHost rejects requests addressed to any other host name, which blocks
// DNS-rebinding attacks from websites the phone or PC might visit.
func (s *Server) checkHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != s.hostport {
			http.Error(w, "invalid host", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireAuth verifies the request's single-use auth token. Browsers cannot
// set headers on WebSocket requests, so the WebSocket route reads it from the
// query string instead.
func (s *Server) requireAuth(fromQuery bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get(authHeader)
		if fromQuery {
			token = r.URL.Query().Get("auth")
		}
		if err := s.session.VerifyToken(token, r.Method, r.URL.Path); err != nil {
			status := http.StatusUnauthorized
			if errors.Is(err, security.ErrSessionExhausted) {
				status = http.StatusGone
			}
			http.Error(w, err.Error(), status)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) serveStatic(name, contentType string) http.HandlerFunc {
	body, err := webFS.ReadFile(name)
	if err != nil {
		panic(err) // embedded at build time; missing means a broken build
	}
	return func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Type", contentType)
		h.Set("Cache-Control", "no-store")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy",
			"default-src 'none'; script-src 'self'; connect-src 'self' ws://"+s.hostport+"; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; "+
				"img-src 'self' data: blob:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		w.Write(body)
	}
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return hex.EncodeToString(b)
}

func formatSpeed(bytes int64, since time.Time) string {
	secs := time.Since(since).Seconds()
	if secs <= 0 {
		return ""
	}
	return fmt.Sprintf("%.1f MB/s", float64(bytes)/secs/(1<<20))
}

// partPath is where an in-progress upload is written before it is renamed.
func (s *Server) partPath(id string) string {
	return filepath.Join(s.dir, ".aero-"+id+".part")
}

// tcpNoDelayListener disables Nagle's algorithm on accepted connections.
type tcpNoDelayListener struct{ net.Listener }

func (l tcpNoDelayListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	if tc, ok := c.(*net.TCPConn); ok {
		tc.SetNoDelay(true)
	}
	return c, nil
}

// DefaultDownloadDir is <user Downloads folder>/Aero.
func DefaultDownloadDir() string {
	if dir, err := userDownloadsDir(); err == nil && dir != "" {
		return filepath.Join(dir, "Aero")
	}
	return filepath.Join(os.TempDir(), "Aero")
}
