package orders

import (
	"context"
	"database/sql"
	"strings"

	"github.com/mathh1s/hamazon/internal/db"
	"github.com/mathh1s/hamazon/internal/validate"
)

type ListFilter struct {
	Status   Status
	Search   string
	Page     int
	PageSize int
}

type Summary struct {
	Order
	FailedNotifications int `json:"failedNotifications"`
}

type ListResult struct {
	Orders   []Summary `json:"orders"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
}

// List returns orders newest first, optionally filtered by status and a
// search over order code and player name.
func (s *Service) List(ctx context.Context, f ListFilter) (ListResult, error) {
	if f.PageSize <= 0 || f.PageSize > 100 {
		f.PageSize = 25
	}
	if f.Page < 1 {
		f.Page = 1
	}
	var where []string
	var args []any
	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if q := strings.TrimSpace(f.Search); q != "" {
		like := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.ToLower(q)) + "%"
		where = append(where, `(LOWER(code) LIKE ? ESCAPE '\' OR username_key LIKE ? ESCAPE '\')`)
		args = append(args, like, like)
	}
	cond := ""
	if len(where) > 0 {
		cond = " WHERE " + strings.Join(where, " AND ")
	}

	res := ListResult{Page: f.Page, PageSize: f.PageSize, Orders: []Summary{}}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders`+cond, args...).Scan(&res.Total); err != nil {
		return res, err
	}

	rows, err := s.db.QueryContext(ctx, `SELECT `+orderColumns+`,
		(SELECT COUNT(*) FROM notifications n WHERE n.order_id = orders.id AND n.state = 'failed')
		FROM orders`+cond+` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`,
		append(args, f.PageSize, (f.Page-1)*f.PageSize)...)
	if err != nil {
		return res, err
	}
	var ids []int64
	for rows.Next() {
		var sum Summary
		var failed int
		o, err := scanOrder(scanWithExtra{rows, &failed})
		if err != nil {
			rows.Close()
			return res, err
		}
		sum.Order, sum.FailedNotifications = o, failed
		res.Orders = append(res.Orders, sum)
		ids = append(ids, o.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}

	lines, err := loadLines(ctx, s.db, ids)
	if err != nil {
		return res, err
	}
	for i := range res.Orders {
		res.Orders[i].Lines = lines[res.Orders[i].ID]
		res.Orders[i].Totals = Totals(res.Orders[i].Lines)
	}
	return res, nil
}

// scanWithExtra lets scanOrder read the order columns while one extra
// trailing column lands in extra.
type scanWithExtra struct {
	rows  *sql.Rows
	extra *int
}

func (s scanWithExtra) Scan(dest ...any) error {
	return s.rows.Scan(append(dest, s.extra)...)
}

// UpdateStatus records the change with who made it and queues an edit of
// the order's Discord message.
func (s *Service) UpdateStatus(ctx context.Context, id int64, status Status, adminName, note string) (Order, error) {
	if !status.Valid() {
		return Order{}, validate.Errors{"status": "Unknown status"}
	}
	note = cleanNote(note)
	if len([]rune(note)) > MaxNoteLength {
		return Order{}, validate.Errors{"note": "Keep the note under 200 characters"}
	}

	var order Order
	changed := false
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		current, err := Load(ctx, tx, "id = ?", id)
		if err != nil {
			return err
		}
		if current.Status == status && note == "" {
			order = current
			return nil
		}
		now := db.Now()
		if _, err := tx.ExecContext(ctx, `UPDATE orders SET status = ?, updated_at = ? WHERE id = ?`, status, now, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_events (order_id, status, admin_name, note, created_at) VALUES (?, ?, ?, ?, ?)`,
			id, status, adminName, note, now); err != nil {
			return err
		}
		if err := enqueue(ctx, tx, id, NotifyUpdated); err != nil {
			return err
		}
		order, err = Load(ctx, tx, "id = ?", id)
		changed = err == nil
		return err
	})
	if err != nil {
		return Order{}, err
	}
	if changed {
		s.onChange()
	}
	return order, nil
}

// FailedNotificationCount is shown in the admin area so failures are noticed.
func (s *Service) FailedNotificationCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE state = 'failed'`).Scan(&n)
	return n, err
}
