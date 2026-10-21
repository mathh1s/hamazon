package orders

import (
	"context"
	"database/sql"
	"time"

	"github.com/mathh1s/hamazon/internal/db"
)

const (
	NotifyCreated = "order_created"
	NotifyUpdated = "order_updated"
)

type Notification struct {
	ID            int64     `json:"id"`
	OrderID       int64     `json:"orderId"`
	Kind          string    `json:"kind"`
	State         string    `json:"state"`
	Attempts      int       `json:"attempts"`
	LastError     string    `json:"lastError"`
	NextAttemptAt time.Time `json:"nextAttemptAt"`
	CreatedAt     time.Time `json:"createdAt"`
}

const notificationColumns = `id, order_id, kind, state, attempts, last_error, next_attempt_at, created_at`

func scanNotification(sc interface{ Scan(...any) error }) (Notification, error) {
	var n Notification
	var next, created string
	err := sc.Scan(&n.ID, &n.OrderID, &n.Kind, &n.State, &n.Attempts, &n.LastError, &next, &created)
	n.NextAttemptAt, _ = db.ParseTime(next)
	n.CreatedAt, _ = db.ParseTime(created)
	return n, err
}

func (s *Service) DueNotifications(ctx context.Context, limit int) ([]Notification, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+notificationColumns+` FROM notifications
		WHERE state = 'pending' AND next_attempt_at <= ? ORDER BY id LIMIT ?`, db.Now(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Service) OrderNotifications(ctx context.Context, orderID int64) ([]Notification, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+notificationColumns+` FROM notifications WHERE order_id = ? ORDER BY id`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// PendingCreate reports whether the order's first message is still queued.
// An update queued behind it has nothing to edit yet, and the create message
// renders the current status anyway.
func (s *Service) PendingCreate(ctx context.Context, orderID int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE order_id = ? AND kind = ? AND state = 'pending'`,
		orderID, NotifyCreated).Scan(&n)
	return n > 0, err
}

func (s *Service) MarkSent(ctx context.Context, n Notification, messageID string) error {
	now := db.Now()
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE notifications SET state = 'sent', attempts = attempts + 1, last_error = '', updated_at = ? WHERE id = ?`,
			now, n.ID); err != nil {
			return err
		}
		if n.Kind == NotifyCreated {
			// The first message already shows the current status, so updates
			// queued before it went out have nothing left to say.
			if _, err := tx.ExecContext(ctx, `UPDATE notifications SET state = 'sent', updated_at = ? WHERE order_id = ? AND kind = ? AND state = 'pending'`,
				now, n.OrderID, NotifyUpdated); err != nil {
				return err
			}
		}
		if messageID == "" {
			return nil
		}
		_, err := tx.ExecContext(ctx, `UPDATE orders SET discord_message_id = ? WHERE id = ?`, messageID, n.OrderID)
		return err
	})
}

// MarkAttemptFailed records an error and schedules the next try, or gives up
// when retryAt is zero so an admin can retry by hand.
func (s *Service) MarkAttemptFailed(ctx context.Context, n Notification, reason string, retryAt time.Time) error {
	state := "pending"
	next := db.FormatTime(retryAt)
	if retryAt.IsZero() {
		state, next = "failed", db.Now()
	}
	if len(reason) > 500 {
		reason = reason[:500]
	}
	_, err := s.db.ExecContext(ctx, `UPDATE notifications SET state = ?, attempts = attempts + 1, last_error = ?, next_attempt_at = ?, updated_at = ?
		WHERE id = ?`, state, reason, next, db.Now(), n.ID)
	return err
}

// RetryNotifications puts every failed notification of an order back into
// the queue with a fresh attempt budget.
func (s *Service) RetryNotifications(ctx context.Context, orderID int64) (int, error) {
	now := db.Now()
	res, err := s.db.ExecContext(ctx, `UPDATE notifications SET state = 'pending', attempts = 0, next_attempt_at = ?, updated_at = ?
		WHERE order_id = ? AND state = 'failed'`, now, now, orderID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		s.onChange()
	}
	return int(n), nil
}

func (s *Service) ClearMessageID(ctx context.Context, orderID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE orders SET discord_message_id = NULL WHERE id = ?`, orderID)
	return err
}

func (s *Service) ByID(ctx context.Context, id int64) (Order, error) {
	return Load(ctx, s.db, "id = ?", id)
}

func (s *Service) StillPending(ctx context.Context, id int64) bool {
	var state string
	err := s.db.QueryRowContext(ctx, `SELECT state FROM notifications WHERE id = ?`, id).Scan(&state)
	return err == nil && state == "pending"
}
