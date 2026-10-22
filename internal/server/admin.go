package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/mathh1s/hamazon/internal/auth"
	"github.com/mathh1s/hamazon/internal/catalog"
	"github.com/mathh1s/hamazon/internal/orders"
	"github.com/mathh1s/hamazon/internal/validate"
)

type adminKey struct{}

func (s *Server) adminFromRequest(r *http.Request) (auth.Admin, bool) {
	c, err := r.Cookie(auth.AdminCookie)
	if err != nil {
		return auth.Admin{}, false
	}
	return s.admins.Session(r.Context(), c.Value)
}

// requireAdmin checks the session on the server for every admin request and
// a CSRF token for every change, so a hidden URL is never the protection.
func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, ok := s.adminFromRequest(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "admin_required", "Log in as an admin first")
			return
		}
		if r.Method != http.MethodGet && !admin.CheckCSRF(r.Header.Get("X-CSRF-Token")) {
			writeError(w, http.StatusForbidden, "csrf", "Your admin session expired, reload the page")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), adminKey{}, admin)))
	}
}

func currentAdmin(r *http.Request) auth.Admin {
	a, _ := r.Context().Value(adminKey{}).(auth.Admin)
	return a
}

func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if !s.loginLimit.Allow("admin:" + s.clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many attempts, wait a minute")
		return
	}
	var body struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	token, err := s.admins.Login(r.Context(), body.Name, body.Password)
	if errors.Is(err, auth.ErrBadLogin) {
		s.log.Warn("admin login failed", "name", body.Name, "ip", s.clientIP(r))
		writeError(w, http.StatusUnauthorized, "bad_login", "Wrong name or password")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: auth.AdminCookie, Value: token, Path: "/", MaxAge: int((7 * 24 * time.Hour).Seconds()),
		HttpOnly: true, Secure: s.isHTTPS(r), SameSite: http.SameSiteStrictMode,
	})
	s.log.Info("admin logged in", "name", body.Name)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.AdminCookie); err == nil {
		_ = s.admins.Logout(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: auth.AdminCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	currencies, err := s.catalog.Currencies(ctx)
	if err != nil {
		s.internalError(w, err)
		return
	}
	categories, err := s.catalog.Categories(ctx)
	if err != nil {
		s.internalError(w, err)
		return
	}
	failed, err := s.orders.FailedNotificationCount(ctx)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"currencies":          currencies,
		"categories":          categories,
		"webhookConfigured":   s.cfg.WebhookURL != "",
		"failedNotifications": failed,
		"assets":              s.assets.Status(),
	})
}

// adminOrder adds what only admins see to the player receipt.
type adminOrder struct {
	orders.Order
	ClientIP      string                `json:"clientIp"`
	DiscordSent   bool                  `json:"discordSent"`
	Events        []orders.Event        `json:"events"`
	Notifications []orders.Notification `json:"notifications"`
}

func (s *Server) loadAdminOrder(ctx context.Context, id int64) (adminOrder, error) {
	o, err := s.orders.ByID(ctx, id)
	if err != nil {
		return adminOrder{}, err
	}
	events, err := s.orders.Events(ctx, id)
	if err != nil {
		return adminOrder{}, err
	}
	notes, err := s.orders.OrderNotifications(ctx, id)
	if err != nil {
		return adminOrder{}, err
	}
	if events == nil {
		events = []orders.Event{}
	}
	if notes == nil {
		notes = []orders.Notification{}
	}
	return adminOrder{Order: o, ClientIP: o.ClientIP, DiscordSent: o.DiscordMessageID != "", Events: events, Notifications: notes}, nil
}

func (s *Server) handleAdminOrders(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	status := orders.Status(q.Get("status"))
	if status != "" && !status.Valid() {
		writeError(w, http.StatusBadRequest, "bad_request", "Unknown status filter")
		return
	}
	res, err := s.orders.List(r.Context(), orders.ListFilter{Status: status, Search: q.Get("q"), Page: page})
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func (s *Server) handleAdminOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Order not found")
		return
	}
	o, err := s.loadAdminOrder(r.Context(), id)
	if errors.Is(err, orders.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Order not found")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func (s *Server) handleAdminUpdateOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Order not found")
		return
	}
	var body struct {
		Status orders.Status `json:"status"`
		Note   string        `json:"note"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	admin := currentAdmin(r)
	if _, err := s.orders.UpdateStatus(r.Context(), id, body.Status, admin.Name, body.Note); err != nil {
		s.adminError(w, err)
		return
	}
	s.log.Info("order status changed", "order", id, "status", body.Status, "admin", admin.Name)
	s.handleAdminOrder(w, r)
}

func (s *Server) handleAdminRetry(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Order not found")
		return
	}
	if _, err := s.orders.RetryNotifications(r.Context(), id); err != nil {
		s.internalError(w, err)
		return
	}
	s.handleAdminOrder(w, r)
}

func (s *Server) handleAdminItems(w http.ResponseWriter, r *http.Request) {
	items, err := s.catalog.Items(r.Context(), true)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if items == nil {
		items = []catalog.Item{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleAdminCreateItem(w http.ResponseWriter, r *http.Request) {
	var in catalog.ItemInput
	if !decodeJSON(w, r, &in) {
		return
	}
	item, err := s.catalog.Create(r.Context(), in)
	if err != nil {
		s.adminError(w, err)
		return
	}
	s.log.Info("item created", "item", item.Name, "admin", currentAdmin(r).Name)
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) handleAdminUpdateItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Item not found")
		return
	}
	var in catalog.ItemInput
	if !decodeJSON(w, r, &in) {
		return
	}
	item, err := s.catalog.Update(r.Context(), id, in)
	if err != nil {
		s.adminError(w, err)
		return
	}
	s.log.Info("item updated", "item", item.Name, "admin", currentAdmin(r).Name)
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleAdminItemStock(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Item not found")
		return
	}
	var body struct {
		InStock bool `json:"inStock"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	item, err := s.catalog.SetInStock(r.Context(), id, body.InStock)
	if err != nil {
		s.adminError(w, err)
		return
	}
	s.log.Info("item stock changed", "item", item.Name, "in_stock", item.InStock, "admin", currentAdmin(r).Name)
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleAdminDeleteItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Item not found")
		return
	}
	if err := s.catalog.Delete(r.Context(), id); err != nil {
		s.adminError(w, err)
		return
	}
	s.log.Info("item deleted", "item", id, "admin", currentAdmin(r).Name)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) adminError(w http.ResponseWriter, err error) {
	var fields validate.Errors
	switch {
	case errors.As(err, &fields):
		f := map[string]any{}
		for k, v := range fields {
			f[k] = v
		}
		writeErrorDetails(w, http.StatusUnprocessableEntity, apiError{Code: "invalid", Message: "Please fix the highlighted fields", Fields: f})
	case errors.Is(err, catalog.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Item not found")
	case errors.Is(err, orders.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Order not found")
	default:
		s.internalError(w, err)
	}
}
