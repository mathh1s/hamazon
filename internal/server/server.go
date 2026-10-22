package server

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/mathh1s/hamazon/internal/assets"
	"github.com/mathh1s/hamazon/internal/auth"
	"github.com/mathh1s/hamazon/internal/catalog"
	"github.com/mathh1s/hamazon/internal/config"
	"github.com/mathh1s/hamazon/internal/orders"
	"github.com/mathh1s/hamazon/internal/ratelimit"
)

type Deps struct {
	Catalog  *catalog.Store
	Orders   *orders.Service
	Access   *auth.Access
	Admins   *auth.Admins
	Assets   *assets.Manager
	Frontend fs.FS
}

type Server struct {
	cfg     config.Config
	log     *slog.Logger
	mux     *http.ServeMux
	catalog *catalog.Store
	orders  *orders.Service
	access  *auth.Access
	admins  *auth.Admins
	assets  *assets.Manager

	loginLimit   *ratelimit.Limiter
	orderLimit   *ratelimit.Limiter
	receiptLimit *ratelimit.Limiter
}

func New(cfg config.Config, log *slog.Logger, deps Deps) *Server {
	s := &Server{
		cfg: cfg, log: log, mux: http.NewServeMux(),
		catalog: deps.Catalog, orders: deps.Orders, access: deps.Access, admins: deps.Admins, assets: deps.Assets,
		loginLimit:   ratelimit.New(12*time.Second, 5),
		orderLimit:   ratelimit.New(2*time.Minute, 4),
		receiptLimit: ratelimit.New(2*time.Second, 30),
	}

	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	s.mux.HandleFunc("POST /api/access", s.handleAccess)
	s.mux.HandleFunc("GET /api/session", s.handleSession)
	s.mux.HandleFunc("GET /api/catalog", s.requireAccess(s.handleCatalog))
	s.mux.HandleFunc("POST /api/orders", s.requireAccess(s.handleCreateOrder))
	s.mux.HandleFunc("GET /api/receipts/{token}", s.requireAccess(s.handleReceipt))
	s.mux.HandleFunc("POST /api/admin/login", s.handleAdminLogin)
	s.mux.HandleFunc("POST /api/admin/logout", s.handleAdminLogout)
	s.mux.HandleFunc("GET /api/admin/overview", s.requireAdmin(s.handleAdminOverview))
	s.mux.HandleFunc("GET /api/admin/orders", s.requireAdmin(s.handleAdminOrders))
	s.mux.HandleFunc("GET /api/admin/orders/{id}", s.requireAdmin(s.handleAdminOrder))
	s.mux.HandleFunc("PATCH /api/admin/orders/{id}", s.requireAdmin(s.handleAdminUpdateOrder))
	s.mux.HandleFunc("POST /api/admin/orders/{id}/retry", s.requireAdmin(s.handleAdminRetry))
	s.mux.HandleFunc("GET /api/admin/items", s.requireAdmin(s.handleAdminItems))
	s.mux.HandleFunc("POST /api/admin/items", s.requireAdmin(s.handleAdminCreateItem))
	s.mux.HandleFunc("PUT /api/admin/items/{id}", s.requireAdmin(s.handleAdminUpdateItem))
	s.mux.HandleFunc("DELETE /api/admin/items/{id}", s.requireAdmin(s.handleAdminDeleteItem))
	s.mux.HandleFunc("PUT /api/admin/items/{id}/stock", s.requireAdmin(s.handleAdminItemStock))

	s.mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "Unknown API endpoint")
	})

	s.mux.Handle("GET /mc/", http.StripPrefix("/mc/", assetHandler(cfg.AssetsDir())))
	s.mux.Handle("/", spaHandler(deps.Frontend))
	return s
}

func (s *Server) Handler() http.Handler {
	return securityHeaders(sameOrigin(s.mux))
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
