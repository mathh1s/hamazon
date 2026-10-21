package server

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/mathh1s/hamazon/internal/config"
)

type Server struct {
	cfg config.Config
	log *slog.Logger
	mux *http.ServeMux
}

func New(cfg config.Config, log *slog.Logger, frontend fs.FS) *Server {
	s := &Server{cfg: cfg, log: log, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.mux.Handle("GET /mc/", http.StripPrefix("/mc/", assetHandler(cfg.AssetsDir())))
	s.mux.Handle("/", spaHandler(frontend))
	return s
}

func (s *Server) Handler() http.Handler {
	return securityHeaders(s.mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; font-src 'self'; connect-src 'self'; media-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		// Receipt URLs carry their access token, so they must never leak via Referer.
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
