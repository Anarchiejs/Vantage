package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Anarchiejs/Vantage/internal/sql/gen"
)

// Config for the server.
type Config struct {
	Address string  // "127.0.0.1:7878"; any interface other than loopback is refused
	DB      *sql.DB // read-write, for the views
	ReadDB  *sql.DB // opened with mode=ro, for the SQL console
	UI      fs.FS   // files from web/dist; nil means no UI (API only)
	Queries string  // directory of saved queries (*.sql)
	Today   func() string
}

// Server is Vantage's HTTP server.
type Server struct {
	cfg     Config
	q       *gen.Queries
	updates *Broadcaster
	// console limits simultaneous SQL queries (a token is held during execution).
	console chan struct{}
}

// New prepares the server. Broadcaster() announces updates.
func New(cfg Config) (*Server, error) {
	if cfg.Today == nil {
		cfg.Today = func() string { return time.Now().Format("2006-01-02") }
	}
	if err := requireLoopback(cfg.Address); err != nil {
		return nil, err
	}
	return &Server{cfg: cfg, q: gen.New(cfg.DB), updates: NewBroadcaster(),
		console: make(chan struct{}, concurrentQueries)}, nil
}

// Broadcaster returns the SSE channel for updates.
func (s *Server) Broadcaster() *Broadcaster { return s.updates }

func requireLoopback(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if host == "localhost" || (ip != nil && ip.IsLoopback()) {
		return nil
	}
	return errors.New("the server only listens on loopback (127.0.0.1)")
}

// Handler returns every route, protected by the Host header check.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.viewRoutes(mux)
	mux.HandleFunc("POST /api/sql", s.executeSQL)
	mux.HandleFunc("GET /api/schema", s.schema)
	mux.HandleFunc("GET /api/queries", s.listQueries)
	mux.HandleFunc("POST /api/queries", s.saveQuery)
	mux.HandleFunc("GET /api/events", s.updates.ServeHTTP)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "unknown route: "+r.URL.Path)
	})
	if s.cfg.UI != nil {
		mux.Handle("/", spa(s.cfg.UI))
	}
	return localOnly(mux)
}

// Listen serves until the context is canceled.
func (s *Server) Listen(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.cfg.Address,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		s.updates.Close()
		srv.Shutdown(shutdown)
	}()
	log.Printf("Vantage on http://%s", s.cfg.Address)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// localOnly refuses any request whose Host header is not loopback. Without that, a malicious
// web page could read the API through DNS rebinding even though the process listens on 127.0.0.1.
// Writes (POST) also require a JSON body, which forces a CORS preflight in the browser
// and blocks forms from another site.
func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(r.Host); err == nil {
			host = h
		}
		if host != "127.0.0.1" && host != "localhost" && host != "[::1]" && host != "::1" {
			writeError(w, http.StatusForbidden, "host refused")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				writeError(w, http.StatusUnsupportedMediaType, "JSON body expected")
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" && !localOrigin(origin) {
				writeError(w, http.StatusForbidden, "origin refused")
				return
			}
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func localOrigin(origin string) bool {
	for _, prefix := range []string{"http://127.0.0.1:", "http://localhost:", "http://[::1]:"} {
		if strings.HasPrefix(origin, prefix) {
			return true
		}
	}
	return origin == "http://127.0.0.1" || origin == "http://localhost"
}

// spa serves the UI files; any unknown route returns index.html (client-side routing).
func spa(files fs.FS) http.Handler {
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(files, path); err == nil {
				if strings.HasPrefix(path, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'")
		http.ServeFileFS(w, r, files, "index.html")
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("JSON encode: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIError{Error: message})
}

// readJSON decodes a body limited to 1 MiB.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	// Deadline for reading the body only: a global ReadTimeout would also cut the SSE stream.
	http.NewResponseController(w).SetReadDeadline(time.Now().Add(10 * time.Second))
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return false
	}
	return true
}
