package server

import (
	"errors"
	"net/http"
	"time"

	"github.com/mathh1s/hamazon/internal/catalog"
	"github.com/mathh1s/hamazon/internal/orders"
	"github.com/mathh1s/hamazon/internal/validate"
)

func (s *Server) requireAccess(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.access.Valid(r) {
			writeError(w, http.StatusUnauthorized, "access_required", "Enter the server password first")
			return
		}
		next(w, r)
	}
}

func (s *Server) handleAccess(w http.ResponseWriter, r *http.Request) {
	if !s.loginLimit.Allow(s.clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many attempts, wait a minute")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if !s.access.CheckPassword(body.Password) {
		writeErrorDetails(w, http.StatusUnauthorized, apiError{
			Code: "wrong_password", Message: "Wrong password",
			Fields: map[string]any{"password": "Wrong password"},
		})
		return
	}
	s.access.Grant(w, s.isHTTPS(r))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"access":          s.access.Valid(r),
		"floodgatePrefix": s.cfg.FloodgatePrefix,
		"admin":           nil,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
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
	items, err := s.catalog.Items(ctx, false)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if items == nil {
		items = []catalog.Item{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"currencies": currencies, "categories": categories, "items": items})
}

// receipt is what a player may see about their own order.
type receipt struct {
	Code      string        `json:"code"`
	Username  string        `json:"username"`
	Note      string        `json:"note"`
	Status    orders.Status `json:"status"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
	Lines     []orders.Line `json:"lines"`
	Totals    catalog.Price `json:"totals"`
}

func toReceipt(o orders.Order) receipt {
	return receipt{Code: o.Code, Username: o.Username, Note: o.Note, Status: o.Status,
		CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt, Lines: o.Lines, Totals: o.Totals}
}

func (s *Server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	ip := s.clientIP(r)
	if !s.orderLimit.Allow(ip) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "You are ordering too fast, wait a few minutes")
		return
	}
	var in orders.CreateInput
	if !decodeJSON(w, r, &in) {
		return
	}

	order, token, err := s.orders.Create(r.Context(), in, ip)
	if err != nil {
		s.orderError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"receiptToken": token, "receipt": toReceipt(order)})
}

func (s *Server) orderError(w http.ResponseWriter, err error) {
	var fields validate.Errors
	var changed *orders.PriceChangedError
	var unavailable *orders.UnavailableError
	var limit *orders.LimitError
	switch {
	case errors.As(err, &fields):
		f := map[string]any{}
		for k, v := range fields {
			f[k] = v
		}
		writeErrorDetails(w, http.StatusUnprocessableEntity, apiError{Code: "invalid", Message: "Please fix the highlighted fields", Fields: f})
	case errors.As(err, &changed):
		writeErrorDetails(w, http.StatusConflict, apiError{Code: "price_changed",
			Message: "Some prices changed since you filled your cart. Check the new prices and place the order again.",
			Details: map[string]any{"itemIds": changed.ItemIDs}})
	case errors.As(err, &unavailable):
		writeErrorDetails(w, http.StatusConflict, apiError{Code: "unavailable",
			Message: "Some items in your cart are no longer sold and were removed.",
			Details: map[string]any{"itemIds": unavailable.ItemIDs}})
	case errors.As(err, &limit):
		writeErrorDetails(w, http.StatusUnprocessableEntity, apiError{Code: "player_limit", Message: limit.Error(), Details: limit})
	case errors.Is(err, orders.ErrDuplicate):
		writeError(w, http.StatusConflict, "duplicate", "You placed this exact order a moment ago. Check your receipts before ordering again.")
	default:
		s.internalError(w, err)
	}
}

func (s *Server) handleReceipt(w http.ResponseWriter, r *http.Request) {
	if !s.receiptLimit.Allow(s.clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many requests, slow down")
		return
	}
	order, err := s.orders.ByReceipt(r.Context(), r.PathValue("token"))
	if errors.Is(err, orders.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "No order found for this receipt link")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toReceipt(order))
}

func (s *Server) internalError(w http.ResponseWriter, err error) {
	s.log.Error("request failed", "err", err)
	writeError(w, http.StatusInternalServerError, "internal", "Something went wrong on the server, try again")
}
