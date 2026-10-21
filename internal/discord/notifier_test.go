package discord

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mathh1s/hamazon/internal/catalog"
	"github.com/mathh1s/hamazon/internal/db"
	"github.com/mathh1s/hamazon/internal/orders"
)

type fakeDiscord struct {
	mu       sync.Mutex
	fail     int
	requests []string
	bodies   []payload
}

func (f *fakeDiscord) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
	var p payload
	body, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(body, &p)
	f.bodies = append(f.bodies, p)
	if f.fail > 0 {
		f.fail--
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	_, _ = w.Write([]byte(`{"id":"123456"}`))
}

func setup(t *testing.T, fake *fakeDiscord) (*Notifier, *orders.Service, catalog.Item) {
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
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	svc := orders.NewService(conn, store, []byte("0123456789abcdef0123456789abcdef"), ".", nil)
	n := NewNotifier(srv.URL+"/api/webhooks/1/token", "https://shop.example", svc, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	items, _ := store.Items(ctx, true)
	var iron catalog.Item
	for _, it := range items {
		if it.Texture == "iron_ingot" {
			iron = it
		}
	}
	return n, svc, iron
}

func TestOrderIsPostedAsEmbed(t *testing.T) {
	fake := &fakeDiscord{}
	n, svc, iron := setup(t, fake)
	ctx := context.Background()
	order, _, err := svc.Create(ctx, orders.CreateInput{
		IdempotencyKey: "key-aaaaaaaaaaaaaaaa", Username: "Cool_Steve", Note: "@everyone deliver to spawn",
		Lines: []orders.CreateLine{{ItemID: iron.ID, Quantity: 2, UnitPrice: iron.Price}},
	}, "")
	if err != nil {
		t.Fatal(err)
	}

	n.drain(ctx)
	if len(fake.requests) != 1 || !strings.HasPrefix(fake.requests[0], "POST /api/webhooks/1/token?wait=true") {
		t.Fatalf("unexpected requests %v", fake.requests)
	}
	e := fake.bodies[0].Embeds[0]
	all, _ := json.Marshal(e)
	text := string(all)
	for _, want := range []string{"New Hamazon Order", order.Code, `Cool\\_Steve`, "128×", "Iron Ingot", "**2** Diamonds", "**16** Gold Ingots", "@​everyone"} {
		if !strings.Contains(text, want) {
			t.Errorf("embed is missing %q: %s", want, text)
		}
	}
	if fake.bodies[0].AllowedMentions.Parse == nil {
		t.Error("mentions must be disabled explicitly")
	}

	saved, _ := svc.ByID(ctx, order.ID)
	if saved.DiscordMessageID != "123456" {
		t.Fatalf("message id not stored: %q", saved.DiscordMessageID)
	}
	if due, _ := svc.DueNotifications(ctx, 10); len(due) != 0 {
		t.Fatalf("notification still queued: %+v", due)
	}
}

func TestFailedSendIsRetriedLater(t *testing.T) {
	fake := &fakeDiscord{fail: 1}
	n, svc, iron := setup(t, fake)
	ctx := context.Background()
	order, _, err := svc.Create(ctx, orders.CreateInput{
		IdempotencyKey: "key-bbbbbbbbbbbbbbbb", Username: "Steve",
		Lines: []orders.CreateLine{{ItemID: iron.ID, Quantity: 1, UnitPrice: iron.Price}},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	n.drain(ctx)

	list, _ := svc.OrderNotifications(ctx, order.ID)
	if len(list) != 1 || list[0].State != "pending" || list[0].Attempts != 1 || list[0].LastError == "" {
		t.Fatalf("expected a scheduled retry, got %+v", list)
	}
	if due, _ := svc.DueNotifications(ctx, 10); len(due) != 0 {
		t.Fatal("retry must wait for its backoff")
	}
}

func TestStatusChangeEditsTheMessage(t *testing.T) {
	fake := &fakeDiscord{}
	n, svc, iron := setup(t, fake)
	ctx := context.Background()
	order, _, err := svc.Create(ctx, orders.CreateInput{
		IdempotencyKey: "key-cccccccccccccccc", Username: "Steve",
		Lines: []orders.CreateLine{{ItemID: iron.ID, Quantity: 1, UnitPrice: iron.Price}},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	n.drain(ctx)
	if _, err := svc.UpdateStatus(ctx, order.ID, orders.StatusFulfilled, "mathis", "handed over at spawn"); err != nil {
		t.Fatal(err)
	}
	n.drain(ctx)

	if len(fake.requests) != 2 || !strings.HasPrefix(fake.requests[1], "PATCH /api/webhooks/1/token/messages/123456") {
		t.Fatalf("expected an edit of the first message, got %v", fake.requests)
	}
	all, _ := json.Marshal(fake.bodies[1])
	for _, want := range []string{"Fulfilled", "mathis", "handed over at spawn", `"color":5635925`} {
		if !strings.Contains(string(all), want) {
			t.Errorf("edit is missing %q: %s", want, all)
		}
	}
}

func TestUpdateBeforeFirstSendIsFolded(t *testing.T) {
	fake := &fakeDiscord{}
	n, svc, iron := setup(t, fake)
	ctx := context.Background()
	order, _, _ := svc.Create(ctx, orders.CreateInput{
		IdempotencyKey: "key-dddddddddddddddd", Username: "Steve",
		Lines: []orders.CreateLine{{ItemID: iron.ID, Quantity: 1, UnitPrice: iron.Price}},
	}, "")
	if _, err := svc.UpdateStatus(ctx, order.ID, orders.StatusApproved, "mathis", ""); err != nil {
		t.Fatal(err)
	}
	n.drain(ctx)
	if len(fake.requests) != 1 {
		t.Fatalf("one message with the current status is enough, got %v", fake.requests)
	}
	if !strings.Contains(string(mustJSON(fake.bodies[0])), "Approved") {
		t.Fatal("first message must show the current status")
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
