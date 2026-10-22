package orders

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mathh1s/hamazon/internal/catalog"
	"github.com/mathh1s/hamazon/internal/db"
	"github.com/mathh1s/hamazon/internal/validate"
)

type fixture struct {
	svc   *Service
	items map[string]catalog.Item
	woken int
}

func setup(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatal(err)
	}
	store := catalog.NewStore(conn)
	if err := store.Seed(ctx, catalog.DefaultSeed); err != nil {
		t.Fatal(err)
	}
	f := &fixture{items: map[string]catalog.Item{}}
	f.svc = NewService(conn, store, []byte("0123456789abcdef0123456789abcdef"), ".", func() { f.woken++ })
	all, _ := store.Items(ctx, true)
	for _, it := range all {
		f.items[it.Texture] = it
	}
	return f
}

func (f *fixture) line(texture string, qty int) CreateLine {
	it := f.items[texture]
	return CreateLine{ItemID: it.ID, Quantity: qty, UnitPrice: it.Price}
}

func TestCreateComputesTotalsAndQueuesNotification(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	order, token, err := f.svc.Create(ctx, CreateInput{
		IdempotencyKey: "key-aaaaaaaaaaaaaaaa", Username: "Steve",
		Lines: []CreateLine{f.line("iron_ingot", 2), f.line("diamond_pickaxe", 1)},
	}, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if order.Totals["diamond"] != 2+4 || order.Totals["gold_ingot"] != 16 {
		t.Fatalf("unexpected totals %v", order.Totals)
	}
	if order.Status != StatusPending || f.woken != 1 {
		t.Fatalf("status %s, woken %d", order.Status, f.woken)
	}
	due, _ := f.svc.DueNotifications(ctx, 10)
	if len(due) != 1 || due[0].Kind != NotifyCreated {
		t.Fatalf("expected one queued notification, got %+v", due)
	}

	again, err := f.svc.ByReceipt(ctx, token)
	if err != nil || again.Code != order.Code {
		t.Fatalf("receipt lookup failed: %v", err)
	}
	if _, err := f.svc.ByReceipt(ctx, token[:len(token)-1]+"x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("tampered token must not resolve, got %v", err)
	}
}

func TestRetriedSubmitReturnsSameOrder(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	in := CreateInput{IdempotencyKey: "key-bbbbbbbbbbbbbbbb", Username: "Alex", Lines: []CreateLine{f.line("bread", 1)}}
	first, token1, err := f.svc.Create(ctx, in, "1.1.1.1")
	if err != nil {
		t.Fatal(err)
	}
	second, token2, err := f.svc.Create(ctx, in, "1.1.1.1")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || token1 != token2 || f.woken != 1 {
		t.Fatalf("retry created a second order: %d vs %d", first.ID, second.ID)
	}

	in.IdempotencyKey = "key-cccccccccccccccc"
	if _, _, err := f.svc.Create(ctx, in, "1.1.1.1"); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("expected duplicate protection, got %v", err)
	}
}

func TestStalePriceIsRejected(t *testing.T) {
	f := setup(t)
	l := f.line("elytra", 1)
	l.UnitPrice = catalog.Price{"diamond": 1}
	_, _, err := f.svc.Create(context.Background(), CreateInput{IdempotencyKey: "key-dddddddddddddddd", Username: "Steve", Lines: []CreateLine{l}}, "")
	var changed *PriceChangedError
	if !errors.As(err, &changed) {
		t.Fatalf("expected price change error, got %v", err)
	}
}

func TestLimits(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	_, _, err := f.svc.Create(ctx, CreateInput{IdempotencyKey: "key-eeeeeeeeeeeeeeee", Username: "Steve",
		Lines: []CreateLine{f.line("diamond_chestplate", 2)}}, "")
	var fields validate.Errors
	if !errors.As(err, &fields) || fields["lines.0"] == "" {
		t.Fatalf("expected max per order error, got %v", err)
	}

	if _, _, err := f.svc.Create(ctx, CreateInput{IdempotencyKey: "key-ffffffffffffffff", Username: "Steve",
		Lines: []CreateLine{f.line("elytra", 1)}}, ""); err != nil {
		t.Fatal(err)
	}
	_, _, err = f.svc.Create(ctx, CreateInput{IdempotencyKey: "key-gggggggggggggggg", Username: "STEVE",
		Lines: []CreateLine{f.line("elytra", 1), f.line("bread", 1)}}, "")
	var limit *LimitError
	if !errors.As(err, &limit) || limit.Remaining != 0 {
		t.Fatalf("expected per player limit across name casing, got %v", err)
	}
	if _, _, err := f.svc.Create(ctx, CreateInput{IdempotencyKey: "key-hhhhhhhhhhhhhhhh", Username: "Alex",
		Lines: []CreateLine{f.line("elytra", 1)}}, ""); err != nil {
		t.Fatalf("other players are not affected: %v", err)
	}
}

func TestNormalizeUsername(t *testing.T) {
	cases := map[string]string{
		"Steve":             "Steve",
		" Notch ":           "Notch",
		"ab":                "",
		"has space":         "",
		"way_too_long_name": "",
		".BedrockGuy":       ".BedrockGuy",
		".Cool Gamer":       ".Cool_Gamer",
		".":                 "",
		"bad-char":          "",
	}
	for in, want := range cases {
		got, ok := NormalizeUsername(in, ".")
		if got != want || ok != (want != "") {
			t.Errorf("%q: got %q %v, want %q", in, got, ok, want)
		}
	}
}

func TestOutOfStockBlocksOrder(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.svc.catalog.SetInStock(ctx, f.items["bread"].ID, false); err != nil {
		t.Fatal(err)
	}
	_, _, err := f.svc.Create(ctx, CreateInput{IdempotencyKey: "key-oooooooooooooooo", Username: "Steve",
		Lines: []CreateLine{f.line("bread", 1), f.line("torch", 1)}}, "")
	var soldOut *OutOfStockError
	if !errors.As(err, &soldOut) || len(soldOut.ItemIDs) != 1 || soldOut.Error() != "Bread is out of stock right now" {
		t.Fatalf("expected out of stock error, got %v", err)
	}
}

func TestStatusPollingAndNewOrders(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	first, token, err := f.svc.Create(ctx, CreateInput{IdempotencyKey: "key-pppppppppppppppp", Username: "Steve",
		Lines: []CreateLine{f.line("torch", 1)}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.UpdateStatus(ctx, first.ID, StatusApproved, "mathis", ""); err != nil {
		t.Fatal(err)
	}
	list, err := f.svc.StatusesByReceipt(ctx, []string{token, "unknown-token-unknown-token"})
	if err != nil || len(list) != 1 || list[0].Status != StatusApproved || list[0].Token != token {
		t.Fatalf("unexpected statuses %+v %v", list, err)
	}

	latest, none, err := f.svc.OrdersAfter(ctx, 0)
	if err != nil || latest != first.ID || len(none) != 0 {
		t.Fatalf("baseline should only report the latest id: %d %+v %v", latest, none, err)
	}
	second, _, err := f.svc.Create(ctx, CreateInput{IdempotencyKey: "key-qqqqqqqqqqqqqqqq", Username: "Alex",
		Lines: []CreateLine{f.line("bread", 1)}}, "")
	if err != nil {
		t.Fatal(err)
	}
	latest, fresh, err := f.svc.OrdersAfter(ctx, first.ID)
	if err != nil || latest != second.ID || len(fresh) != 1 || fresh[0].Code != second.Code || fresh[0].Texture != "bread" {
		t.Fatalf("new order not reported: %d %+v %v", latest, fresh, err)
	}
}

func TestDisabledDimensionBlocksOrder(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if err := f.svc.catalog.SetDimensionEnabled(ctx, "end", false); err != nil {
		t.Fatal(err)
	}
	_, _, err := f.svc.Create(ctx, CreateInput{IdempotencyKey: "key-dimdimdimdimdimd", Username: "Steve",
		Lines: []CreateLine{f.line("elytra", 1)}}, "")
	var unavailable *UnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("end items must not be orderable while the end is off, got %v", err)
	}
	if _, _, err := f.svc.Create(ctx, CreateInput{IdempotencyKey: "key-dimokdimokdimok", Username: "Steve",
		Lines: []CreateLine{f.line("blaze_rod", 1)}}, ""); err != nil {
		t.Fatalf("nether items stay orderable: %v", err)
	}
}

func TestDeliveryIsStoredAndValidated(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	y := 70
	order, _, err := f.svc.Create(ctx, CreateInput{IdempotencyKey: "key-delivery-ok-0001", Username: "Steve",
		Delivery: &Delivery{Dimension: "nether", X: -120, Y: &y, Z: 455},
		Lines:    []CreateLine{f.line("torch", 1)}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if order.Delivery == nil || order.Delivery.X != -120 || *order.Delivery.Y != 70 {
		t.Fatalf("delivery not stored: %+v", order.Delivery)
	}
	_, _, err = f.svc.Create(ctx, CreateInput{IdempotencyKey: "key-delivery-bad-001", Username: "Steve",
		Delivery: &Delivery{Dimension: "moon", X: 40_000_000, Z: 0},
		Lines:    []CreateLine{f.line("bread", 1)}}, "")
	var fields validate.Errors
	if !errors.As(err, &fields) || fields["delivery"] == "" {
		t.Fatalf("bad delivery must be rejected, got %v", err)
	}
}
