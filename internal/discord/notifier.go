package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/mathh1s/hamazon/internal/catalog"
	"github.com/mathh1s/hamazon/internal/orders"
)

const (
	maxAttempts  = 8
	baseBackoff  = 30 * time.Second
	maxBackoff   = time.Hour
	pollInterval = 15 * time.Second
)

// Notifier drains the notification outbox. Orders are saved first and the
// webhook is sent afterwards, so a Discord outage never loses an order; it
// only delays the message.
type Notifier struct {
	webhookURL string
	publicURL  string
	orders     *orders.Service
	catalog    *catalog.Store
	client     *http.Client
	log        *slog.Logger
	wake       chan struct{}
}

func NewNotifier(webhookURL, publicURL string, o *orders.Service, c *catalog.Store, log *slog.Logger) *Notifier {
	return &Notifier{
		webhookURL: webhookURL,
		publicURL:  publicURL,
		orders:     o,
		catalog:    c,
		client:     &http.Client{Timeout: 15 * time.Second},
		log:        log,
		wake:       make(chan struct{}, 1),
	}
}

// Wake asks the worker to look at the queue now instead of at the next tick.
func (n *Notifier) Wake() {
	select {
	case n.wake <- struct{}{}:
	default:
	}
}

func (n *Notifier) Run(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		n.drain(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-n.wake:
		}
	}
}

func (n *Notifier) drain(ctx context.Context) {
	for ctx.Err() == nil {
		due, err := n.orders.DueNotifications(ctx, 10)
		if err != nil {
			n.log.Error("load notifications", "err", err)
			return
		}
		if len(due) == 0 {
			return
		}
		for _, item := range due {
			n.process(ctx, item)
		}
	}
}

type sendError struct {
	err        error
	retryAfter time.Duration
	permanent  bool
	notFound   bool
}

func (e *sendError) Error() string { return e.err.Error() }

func (n *Notifier) process(ctx context.Context, item orders.Notification) {
	if n.webhookURL == "" {
		n.fail(ctx, item, &sendError{err: errors.New("HAMAZON_DISCORD_WEBHOOK_URL is not set"), permanent: true})
		return
	}

	order, err := n.orders.ByID(ctx, item.OrderID)
	if err != nil {
		n.fail(ctx, item, &sendError{err: fmt.Errorf("load order: %w", err), permanent: errors.Is(err, orders.ErrNotFound)})
		return
	}

	if item.Kind == orders.NotifyUpdated && order.DiscordMessageID == "" {
		pending, err := n.orders.PendingCreate(ctx, order.ID)
		if err != nil {
			n.fail(ctx, item, &sendError{err: err})
			return
		}
		if pending {
			_ = n.orders.MarkSent(ctx, item, "")
			return
		}
	}

	p, err := n.payloadFor(ctx, order, item.Kind)
	if err != nil {
		n.fail(ctx, item, &sendError{err: err})
		return
	}

	if order.DiscordMessageID != "" && item.Kind == orders.NotifyUpdated {
		err := n.edit(ctx, order.DiscordMessageID, p)
		var se *sendError
		if errors.As(err, &se) && se.notFound {
			// The message was deleted in Discord, post a fresh one instead.
			_ = n.orders.ClearMessageID(ctx, order.ID)
			p.Embeds[0].Title = "Hamazon Order Updated"
			id, postErr := n.post(ctx, p)
			if postErr != nil {
				n.fail(ctx, item, postErr)
				return
			}
			_ = n.orders.MarkSent(ctx, item, id)
			return
		}
		if err != nil {
			n.fail(ctx, item, err)
			return
		}
		_ = n.orders.MarkSent(ctx, item, "")
		return
	}

	id, err := n.post(ctx, p)
	if err != nil {
		n.fail(ctx, item, err)
		return
	}
	if err := n.orders.MarkSent(ctx, item, id); err != nil {
		n.log.Error("mark notification sent", "id", item.ID, "err", err)
	}
	n.log.Info("discord notification sent", "order", order.Code, "kind", item.Kind)
}

func (n *Notifier) payloadFor(ctx context.Context, o orders.Order, kind string) (payload, error) {
	currencies, err := n.catalog.Currencies(ctx)
	if err != nil {
		return payload{}, err
	}
	events, err := n.orders.Events(ctx, o.ID)
	if err != nil {
		return payload{}, err
	}
	var last *orders.Event
	if len(events) > 0 {
		last = &events[len(events)-1]
	}
	title := "New Hamazon Order"
	if kind == orders.NotifyUpdated && o.DiscordMessageID == "" {
		title = "Hamazon Order Updated"
	}
	return buildPayload(embedInput{order: o, lastEvent: last, currencies: currencies, publicURL: n.publicURL, title: title}), nil
}

func (n *Notifier) fail(ctx context.Context, item orders.Notification, err error) {
	var se *sendError
	if !errors.As(err, &se) {
		se = &sendError{err: err}
	}
	var retryAt time.Time
	if !se.permanent && item.Attempts+1 < maxAttempts {
		delay := se.retryAfter
		if delay == 0 {
			delay = min(baseBackoff<<item.Attempts, maxBackoff)
		}
		retryAt = time.Now().Add(delay)
	}
	n.log.Warn("discord notification failed", "id", item.ID, "order", item.OrderID, "attempt", item.Attempts+1,
		"err", se.err, "gave_up", retryAt.IsZero())
	if err := n.orders.MarkAttemptFailed(ctx, item, se.err.Error(), retryAt); err != nil {
		n.log.Error("record notification failure", "err", err)
	}
}

func (n *Notifier) post(ctx context.Context, p payload) (string, error) {
	u, err := url.Parse(n.webhookURL)
	if err != nil {
		return "", &sendError{err: err, permanent: true}
	}
	q := u.Query()
	q.Set("wait", "true")
	u.RawQuery = q.Encode()

	body, err := n.send(ctx, http.MethodPost, u.String(), p)
	if err != nil {
		return "", err
	}
	var msg struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &msg)
	return msg.ID, nil
}

func (n *Notifier) edit(ctx context.Context, messageID string, p payload) error {
	u, err := url.Parse(n.webhookURL)
	if err != nil {
		return &sendError{err: err, permanent: true}
	}
	u.Path += "/messages/" + url.PathEscape(messageID)
	// Username and avatar cannot be changed on an existing message.
	p.Username, p.AvatarURL = "", ""
	_, err = n.send(ctx, http.MethodPatch, u.String(), p)
	return err
}

func (n *Notifier) send(ctx context.Context, method, target string, p payload) ([]byte, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, &sendError{err: err, permanent: true}
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(data))
	if err != nil {
		return nil, &sendError{err: err, permanent: true}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Hamazon (private Minecraft shop)")

	resp, err := n.client.Do(req)
	if err != nil {
		// Strip the URL from transport errors, it contains the webhook token.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}
		return nil, &sendError{err: fmt.Errorf("request failed: %w", err)}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return body, nil
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, &sendError{err: errors.New("rate limited by Discord"), retryAfter: retryAfter(resp, body)}
	case resp.StatusCode == http.StatusNotFound && method == http.MethodPatch:
		return nil, &sendError{err: errors.New("message not found"), notFound: true}
	case resp.StatusCode >= 500:
		return nil, &sendError{err: fmt.Errorf("discord returned %d", resp.StatusCode)}
	default:
		return nil, &sendError{err: fmt.Errorf("discord returned %d: %s", resp.StatusCode, truncate(string(body), 300)), permanent: true}
	}
}

func retryAfter(resp *http.Response, body []byte) time.Duration {
	var rl struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if json.Unmarshal(body, &rl) == nil && rl.RetryAfter > 0 {
		return time.Duration(rl.RetryAfter*1000)*time.Millisecond + 250*time.Millisecond
	}
	if s, err := strconv.ParseFloat(resp.Header.Get("Retry-After"), 64); err == nil && s > 0 {
		return time.Duration(s*1000) * time.Millisecond
	}
	return 5 * time.Second
}
