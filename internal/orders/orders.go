package orders

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mathh1s/hamazon/internal/catalog"
	"github.com/mathh1s/hamazon/internal/db"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusApproved  Status = "approved"
	StatusFulfilled Status = "fulfilled"
	StatusCancelled Status = "cancelled"
)

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusApproved, StatusFulfilled, StatusCancelled:
		return true
	}
	return false
}

var (
	ErrNotFound  = errors.New("order not found")
	ErrDuplicate = errors.New("the same order was just placed")
)

type Line struct {
	ItemID    *int64        `json:"itemId"`
	Name      string        `json:"name"`
	Texture   string        `json:"texture"`
	Count     int           `json:"count"`
	Quantity  int           `json:"quantity"`
	UnitPrice catalog.Price `json:"unitPrice"`
}

type Order struct {
	ID               int64         `json:"id"`
	Code             string        `json:"code"`
	Username         string        `json:"username"`
	Note             string        `json:"note"`
	Status           Status        `json:"status"`
	CreatedAt        time.Time     `json:"createdAt"`
	UpdatedAt        time.Time     `json:"updatedAt"`
	Lines            []Line        `json:"lines"`
	Totals           catalog.Price `json:"totals"`
	ClientIP         string        `json:"-"`
	DiscordMessageID string        `json:"-"`
}

type Event struct {
	Status    Status    `json:"status"`
	AdminName string    `json:"adminName"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"createdAt"`
}

// Totals sums unit prices times quantity per currency.
func Totals(lines []Line) catalog.Price {
	out := catalog.Price{}
	for _, l := range lines {
		for currency, amount := range l.UnitPrice {
			out[currency] += amount * l.Quantity
		}
	}
	return out
}

const orderColumns = `id, code, username, note, status, created_at, updated_at, client_ip, COALESCE(discord_message_id, '')`

func scanOrder(sc interface{ Scan(...any) error }) (Order, error) {
	var o Order
	var created, updated string
	if err := sc.Scan(&o.ID, &o.Code, &o.Username, &o.Note, &o.Status, &created, &updated, &o.ClientIP, &o.DiscordMessageID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return o, ErrNotFound
		}
		return o, err
	}
	o.CreatedAt, _ = db.ParseTime(created)
	o.UpdatedAt, _ = db.ParseTime(updated)
	return o, nil
}

// Load reads one order with its lines. It takes a Querier so callers inside
// a transaction see their own writes.
func Load(ctx context.Context, q db.Querier, where string, arg any) (Order, error) {
	o, err := scanOrder(q.QueryRowContext(ctx, `SELECT `+orderColumns+` FROM orders WHERE `+where, arg))
	if err != nil {
		return o, err
	}
	lines, err := loadLines(ctx, q, []int64{o.ID})
	if err != nil {
		return o, err
	}
	o.Lines = lines[o.ID]
	o.Totals = Totals(o.Lines)
	return o, nil
}

func loadLines(ctx context.Context, q db.Querier, orderIDs []int64) (map[int64][]Line, error) {
	out := map[int64][]Line{}
	if len(orderIDs) == 0 {
		return out, nil
	}
	args := make([]any, len(orderIDs))
	marks := ""
	for i, id := range orderIDs {
		args[i] = id
		if i > 0 {
			marks += ","
		}
		marks += "?"
	}
	rows, err := q.QueryContext(ctx, `SELECT order_id, item_id, name, texture, count, quantity, unit_price
		FROM order_lines WHERE order_id IN (`+marks+`) ORDER BY order_id, position`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var orderID int64
		var itemID sql.NullInt64
		var l Line
		var price string
		if err := rows.Scan(&orderID, &itemID, &l.Name, &l.Texture, &l.Count, &l.Quantity, &price); err != nil {
			return nil, err
		}
		if itemID.Valid {
			id := itemID.Int64
			l.ItemID = &id
		}
		if err := json.Unmarshal([]byte(price), &l.UnitPrice); err != nil {
			return nil, fmt.Errorf("order %d line price: %w", orderID, err)
		}
		out[orderID] = append(out[orderID], l)
	}
	return out, rows.Err()
}

func loadEvents(ctx context.Context, q db.Querier, orderID int64) ([]Event, error) {
	rows, err := q.QueryContext(ctx, `SELECT status, COALESCE(admin_name, ''), note, created_at FROM order_events WHERE order_id = ? ORDER BY id`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var created string
		if err := rows.Scan(&e.Status, &e.AdminName, &e.Note, &created); err != nil {
			return nil, err
		}
		e.CreatedAt, _ = db.ParseTime(created)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Service) Events(ctx context.Context, orderID int64) ([]Event, error) {
	return loadEvents(ctx, s.db, orderID)
}
