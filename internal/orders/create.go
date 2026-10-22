package orders

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mathh1s/hamazon/internal/catalog"
	"github.com/mathh1s/hamazon/internal/db"
	"github.com/mathh1s/hamazon/internal/validate"
)

const (
	MaxLines        = 50
	MaxNoteLength   = 200
	duplicateWindow = 2 * time.Minute
)

type Service struct {
	db              *sql.DB
	catalog         *catalog.Store
	secret          []byte
	floodgatePrefix string
	onChange        func()
}

// NewService wires order storage. onChange is called after any commit that
// queues a Discord notification so the notifier can send it right away.
func NewService(conn *sql.DB, items *catalog.Store, secret []byte, floodgatePrefix string, onChange func()) *Service {
	if onChange == nil {
		onChange = func() {}
	}
	return &Service{db: conn, catalog: items, secret: secret, floodgatePrefix: floodgatePrefix, onChange: onChange}
}

type CreateLine struct {
	ItemID    int64         `json:"itemId"`
	Quantity  int           `json:"quantity"`
	UnitPrice catalog.Price `json:"unitPrice"`
}

type CreateInput struct {
	IdempotencyKey string       `json:"idempotencyKey"`
	Username       string       `json:"username"`
	Note           string       `json:"note"`
	Delivery       *Delivery    `json:"delivery"`
	Lines          []CreateLine `json:"lines"`
}

// Delivery is where the player wants the items. Y is optional because
// most players only know their X and Z by heart.
type Delivery struct {
	Dimension string `json:"dimension"`
	X         int    `json:"x"`
	Y         *int   `json:"y"`
	Z         int    `json:"z"`
}

const worldBorder = 29_999_984

func checkDelivery(d *Delivery, errs validate.Errors) {
	if d == nil {
		return
	}
	switch d.Dimension {
	case "overworld", "nether", "end":
	default:
		errs.Add("delivery", "Pick the Overworld, the Nether or the End")
	}
	if d.X < -worldBorder || d.X > worldBorder || d.Z < -worldBorder || d.Z > worldBorder {
		errs.Add("delivery", "Coordinates are outside the world border")
	}
	if d.Y != nil && (*d.Y < -64 || *d.Y > 320) {
		errs.Add("delivery", "Y must be between -64 and 320")
	}
}

// PriceChangedError means the cart was built against older prices. The
// player has to review the new prices instead of silently paying more.
type PriceChangedError struct{ ItemIDs []int64 }

func (e *PriceChangedError) Error() string { return "prices changed since the cart was filled" }

// UnavailableError lists cart items that were removed or disabled.
type UnavailableError struct{ ItemIDs []int64 }

func (e *UnavailableError) Error() string { return "some items are no longer available" }

// OutOfStockError lists cart items an admin marked as out of stock.
type OutOfStockError struct {
	ItemIDs []int64
	Names   []string
}

func (e *OutOfStockError) Error() string {
	verb := "are"
	if len(e.Names) == 1 {
		verb = "is"
	}
	return strings.Join(e.Names, ", ") + " " + verb + " out of stock right now"
}

// LimitError reports a per player limit that this order would exceed.
type LimitError struct {
	ItemID    int64  `json:"itemId"`
	Name      string `json:"name"`
	Limit     int    `json:"limit"`
	Days      int    `json:"days"`
	Remaining int    `json:"remaining"`
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("%s is limited to %d per player every %d days, %d left", e.Name, e.Limit, e.Days, e.Remaining)
}

var idempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9-]{16,64}$`)

// Create validates the cart against the live catalog and stores the order
// together with its first Discord notification in one transaction. The
// returned receipt token is the only way back to this order for the player.
func (s *Service) Create(ctx context.Context, in CreateInput, clientIP string) (Order, string, error) {
	username, note, err := s.checkInput(&in)
	if err != nil {
		return Order{}, "", err
	}
	token := s.receiptToken(in.IdempotencyKey)
	tokenHash := hashToken(token)

	var order Order
	created := false
	err = db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		existing, err := Load(ctx, tx, "idempotency_key = ?", in.IdempotencyKey)
		if err == nil {
			if !strings.EqualFold(existing.Username, username) {
				return validate.Errors{"idempotencyKey": "This checkout was already used for another player"}
			}
			order = existing
			return nil
		}
		if !errors.Is(err, ErrNotFound) {
			return err
		}

		lines, err := s.buildLines(ctx, tx, in.Lines)
		if err != nil {
			return err
		}
		if err := checkPlayerLimits(ctx, tx, strings.ToLower(username), lines); err != nil {
			return err
		}

		contentHash := contentHash(username, in.Lines)
		var recent int
		since := db.FormatTime(time.Now().Add(-duplicateWindow))
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders WHERE content_hash = ? AND created_at >= ? AND status != 'cancelled'`,
			contentHash, since).Scan(&recent); err != nil {
			return err
		}
		if recent > 0 {
			return ErrDuplicate
		}

		id, err := insertOrder(ctx, tx, username, note, in.Delivery, tokenHash, in.IdempotencyKey, contentHash, clientIP, lines)
		if err != nil {
			return err
		}
		order, err = Load(ctx, tx, "id = ?", id)
		created = err == nil
		return err
	})
	if err != nil {
		return Order{}, "", err
	}
	if created {
		s.onChange()
	}
	return order, token, nil
}

func (s *Service) checkInput(in *CreateInput) (string, string, error) {
	errs := validate.Errors{}
	if !idempotencyPattern.MatchString(in.IdempotencyKey) {
		errs.Add("idempotencyKey", "Missing checkout id, reload the page and try again")
	}
	username, ok := NormalizeUsername(in.Username, s.floodgatePrefix)
	if !ok {
		if strings.TrimSpace(in.Username) == "" {
			errs.Add("username", "Enter your Minecraft username")
		} else {
			errs.Add("username", "That is not a valid Minecraft username")
		}
	}
	checkDelivery(in.Delivery, errs)
	note := cleanNote(in.Note)
	if len([]rune(note)) > MaxNoteLength {
		errs.Add("note", fmt.Sprintf("Keep the note under %d characters", MaxNoteLength))
	}

	if len(in.Lines) == 0 {
		errs.Add("lines", "Your cart is empty")
	} else if len(in.Lines) > MaxLines {
		errs.Add("lines", fmt.Sprintf("An order can have at most %d different items", MaxLines))
	}
	seen := map[int64]bool{}
	for i, l := range in.Lines {
		field := "lines." + strconv.Itoa(i)
		if seen[l.ItemID] {
			errs.Add(field, "Item appears twice in the cart")
		}
		seen[l.ItemID] = true
		if l.Quantity < 1 {
			errs.Add(field, "Quantity must be at least 1")
		}
	}
	return username, note, errs.Err()
}

func (s *Service) buildLines(ctx context.Context, tx *sql.Tx, in []CreateLine) ([]Line, error) {
	ids := make([]int64, len(in))
	for i, l := range in {
		ids[i] = l.ItemID
	}
	items, err := s.catalog.ItemsByID(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	disabledDims, err := s.catalog.DisabledDimensions(ctx)
	if err != nil {
		return nil, err
	}

	var unavailable, changed []int64
	soldOut := &OutOfStockError{}
	errs := validate.Errors{}
	lines := make([]Line, 0, len(in))
	for i, l := range in {
		it, ok := items[l.ItemID]
		if !ok || !it.Enabled || it.Blocked(disabledDims) {
			unavailable = append(unavailable, l.ItemID)
			continue
		}
		if !it.InStock {
			soldOut.ItemIDs = append(soldOut.ItemIDs, it.ID)
			soldOut.Names = append(soldOut.Names, it.Name)
		}
		if !maps.Equal(it.Price, l.UnitPrice) {
			changed = append(changed, l.ItemID)
		}
		if l.Quantity > it.MaxPerOrder {
			errs.Add("lines."+strconv.Itoa(i), fmt.Sprintf("You can order at most %d of %s at once", it.MaxPerOrder, it.Name))
		}
		id := it.ID
		lines = append(lines, Line{ItemID: &id, Name: it.Name, Texture: it.Texture, Count: it.Count, Quantity: l.Quantity, UnitPrice: it.Price})
	}
	if len(unavailable) > 0 {
		return nil, &UnavailableError{ItemIDs: unavailable}
	}
	if len(soldOut.ItemIDs) > 0 {
		return nil, soldOut
	}
	if len(changed) > 0 {
		return nil, &PriceChangedError{ItemIDs: changed}
	}
	return lines, errs.Err()
}

func checkPlayerLimits(ctx context.Context, tx *sql.Tx, usernameKey string, lines []Line) error {
	for _, l := range lines {
		var limit, days sql.NullInt64
		if err := tx.QueryRowContext(ctx, `SELECT player_limit, player_limit_days FROM items WHERE id = ?`, *l.ItemID).Scan(&limit, &days); err != nil {
			return err
		}
		if !limit.Valid || !days.Valid {
			continue
		}
		since := db.FormatTime(time.Now().AddDate(0, 0, -int(days.Int64)))
		var used int
		err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(ol.quantity), 0) FROM order_lines ol JOIN orders o ON o.id = ol.order_id
			WHERE ol.item_id = ? AND o.username_key = ? AND o.status != 'cancelled' AND o.created_at >= ?`,
			*l.ItemID, usernameKey, since).Scan(&used)
		if err != nil {
			return err
		}
		if used+l.Quantity > int(limit.Int64) {
			return &LimitError{ItemID: *l.ItemID, Name: l.Name, Limit: int(limit.Int64), Days: int(days.Int64), Remaining: max(int(limit.Int64)-used, 0)}
		}
	}
	return nil
}

func insertOrder(ctx context.Context, tx *sql.Tx, username, note string, delivery *Delivery, tokenHash []byte, idemKey string,
	contentHash []byte, clientIP string, lines []Line) (int64, error) {
	now := db.Now()
	var dim string
	var x, y, z any
	if delivery != nil {
		dim, x, z = delivery.Dimension, delivery.X, delivery.Z
		if delivery.Y != nil {
			y = *delivery.Y
		}
	}
	var id int64
	for attempt := 0; ; attempt++ {
		res, err := tx.ExecContext(ctx, `INSERT INTO orders (code, username, username_key, note, status, receipt_hash, idempotency_key,
			content_hash, client_ip, created_at, updated_at, delivery_dimension, delivery_x, delivery_y, delivery_z)
			VALUES (?, ?, ?, ?, 'pending', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newCode(), username, strings.ToLower(username), note, tokenHash, idemKey, contentHash, clientIP, now, now, dim, x, y, z)
		if err != nil {
			if attempt < 5 && strings.Contains(err.Error(), "orders.code") {
				continue
			}
			return 0, err
		}
		id, err = res.LastInsertId()
		if err != nil {
			return 0, err
		}
		break
	}

	for i, l := range lines {
		price, err := json.Marshal(l.UnitPrice)
		if err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_lines (order_id, position, item_id, name, texture, count, quantity, unit_price)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, id, i, l.ItemID, l.Name, l.Texture, l.Count, l.Quantity, string(price)); err != nil {
			return 0, err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO order_events (order_id, status, note, created_at) VALUES (?, 'pending', 'Order placed', ?)`, id, now); err != nil {
		return 0, err
	}
	if err := enqueue(ctx, tx, id, "order_created"); err != nil {
		return 0, err
	}
	return id, nil
}

func enqueue(ctx context.Context, q db.Querier, orderID int64, kind string) error {
	now := db.Now()
	_, err := q.ExecContext(ctx, `INSERT INTO notifications (order_id, kind, state, next_attempt_at, created_at, updated_at)
		VALUES (?, ?, 'pending', ?, ?, ?)`, orderID, kind, now, now, now)
	return err
}

// ByReceipt finds an order by the token handed out at checkout. Only the
// hash is stored, so a leaked database does not reveal receipt links.
func (s *Service) ByReceipt(ctx context.Context, token string) (Order, error) {
	if len(token) < 20 || len(token) > 64 {
		return Order{}, ErrNotFound
	}
	return Load(ctx, s.db, "receipt_hash = ?", hashToken(token))
}

// receiptToken is derived from the client's checkout id, so a retried
// submit after a lost response gets the same receipt link back.
func (s *Service) receiptToken(idempotencyKey string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte("hamazon-receipt:" + idempotencyKey))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func contentHash(username string, lines []CreateLine) []byte {
	sorted := append([]CreateLine(nil), lines...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ItemID < sorted[j].ItemID })
	h := sha256.New()
	h.Write([]byte(strings.ToLower(username)))
	for _, l := range sorted {
		fmt.Fprintf(h, "|%d:%d", l.ItemID, l.Quantity)
	}
	return h.Sum(nil)
}

const codeAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// newCode makes a short order id that is easy to read out in game chat.
func newCode() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	out := make([]byte, len(b))
	for i, v := range b {
		out[i] = codeAlphabet[int(v)%len(codeAlphabet)]
	}
	return "HZ-" + string(out)
}

func cleanNote(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

var javaName = regexp.MustCompile(`^[A-Za-z0-9_]{3,16}$`)
var bedrockName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// NormalizeUsername accepts Java names and Bedrock names that carry the
// Floodgate prefix (spaces in gamertags become underscores in game).
func NormalizeUsername(raw, floodgatePrefix string) (string, bool) {
	name := strings.TrimSpace(raw)
	if javaName.MatchString(name) {
		return name, true
	}
	if floodgatePrefix == "" {
		return "", false
	}
	rest, ok := strings.CutPrefix(name, floodgatePrefix)
	if !ok {
		return "", false
	}
	rest = strings.ReplaceAll(rest, " ", "_")
	if rest == "" || len(floodgatePrefix)+len(rest) > 16 || !bedrockName.MatchString(rest) {
		return "", false
	}
	return floodgatePrefix + rest, true
}
