package catalog

import (
	"context"
	"errors"
	"maps"
	"path/filepath"
	"testing"

	"github.com/mathh1s/hamazon/internal/db"
	"github.com/mathh1s/hamazon/internal/validate"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	return NewStore(conn)
}

func TestSeedRunsOnce(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	if err := s.Seed(ctx, DefaultSeed); err != nil {
		t.Fatal(err)
	}
	items, err := s.Items(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 100 {
		t.Fatalf("expected the default catalog, got %d items", len(items))
	}

	if err := s.Delete(ctx, items[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Seed(ctx, DefaultSeed); err != nil {
		t.Fatal(err)
	}
	again, _ := s.Items(ctx, true)
	if len(again) != len(items)-1 {
		t.Fatalf("seed re-added items: %d -> %d", len(items), len(again))
	}
}

func TestMixedPriceRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	if err := s.Seed(ctx, DefaultSeed); err != nil {
		t.Fatal(err)
	}
	created, err := s.Create(ctx, ItemInput{
		Name: "Test Item", Texture: "minecraft:diamond_hoe", CategoryID: "tools", Count: 1, MaxPerOrder: 2,
		Price: Price{"diamond": 2, "emerald": 8},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Texture != "diamond_hoe" || created.Price["diamond"] != 2 || created.Price["emerald"] != 8 || !created.Enabled {
		t.Fatalf("unexpected item %+v", created)
	}
}

func TestValidationCollectsAllErrors(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	if err := s.Seed(ctx, DefaultSeed); err != nil {
		t.Fatal(err)
	}
	limit := 1
	_, err := s.Create(ctx, ItemInput{
		Name: "", Texture: "Bad Texture", CategoryID: "nope", Count: 65, MaxPerOrder: 4, PlayerLimit: &limit,
		Price: Price{"diamond": 1, "emerald": 1, "gold_ingot": 1},
	})
	var verr validate.Errors
	if !errors.As(err, &verr) {
		t.Fatalf("expected validation errors, got %v", err)
	}
	for _, field := range []string{"name", "texture", "categoryId", "count", "playerLimit", "price"} {
		if _, ok := verr[field]; !ok {
			t.Errorf("missing error for %s: %v", field, verr)
		}
	}
}

func TestMissingDefaultsSkipsDeletedItems(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	small := []byte(`
currencies: [{ id: diamond, name: Diamond, texture: diamond }]
categories: [{ id: misc, name: Misc, icon: stick }]
items:
  - { name: Stick, texture: stick, category: misc, count: 1, maxPerOrder: 1, price: { diamond: 1 } }
`)
	if err := s.Seed(ctx, small); err != nil {
		t.Fatal(err)
	}
	missing, err := s.MissingDefaults(ctx, DefaultSeed)
	if err != nil || len(missing) < 300 {
		t.Fatalf("expected the full default list to be missing, got %d %v", len(missing), err)
	}
	ids, err := s.AddMissingDefaults(ctx, DefaultSeed)
	if err != nil || len(ids) != len(missing) {
		t.Fatalf("added %d of %d: %v", len(ids), len(missing), err)
	}
	added, _ := s.Item(ctx, ids[0])
	if added.Enabled {
		t.Fatal("new defaults must start hidden")
	}
	if err := s.Delete(ctx, ids[0]); err != nil {
		t.Fatal(err)
	}
	if again, _ := s.MissingDefaults(ctx, DefaultSeed); len(again) != 0 {
		t.Fatalf("deleted items must not come back, got %d", len(again))
	}
}

func TestPriceUpdatesRespectLocks(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	if err := s.Seed(ctx, DefaultSeed); err != nil {
		t.Fatal(err)
	}
	if ups, _ := s.PriceUpdates(ctx, DefaultSeed); len(ups) != 0 {
		t.Fatalf("a fresh install matches the defaults, got %d updates", len(ups))
	}
	items, _ := s.Items(ctx, true)
	a, b := items[0], items[1]

	_, err := s.db.ExecContext(ctx, `UPDATE item_prices SET amount = amount + 5 WHERE item_id IN (?, ?)`, a.ID, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	edited := ItemInput{Name: b.Name, Texture: b.Texture, CategoryID: b.CategoryID, Count: b.Count, MaxPerOrder: b.MaxPerOrder,
		PlayerLimit: b.PlayerLimit, PlayerLimitDays: b.PlayerLimitDays, Price: Price{"diamond": 99}}
	if _, err := s.Update(ctx, b.ID, edited); err != nil {
		t.Fatal(err)
	}

	ups, err := s.PriceUpdates(ctx, DefaultSeed)
	if err != nil || len(ups) != 2 {
		t.Fatalf("expected two updates, got %+v %v", ups, err)
	}
	for _, u := range ups {
		if (u.ItemID == b.ID) != u.Locked {
			t.Fatalf("only the hand edited item must be locked: %+v", u)
		}
	}
	n, err := s.ApplyPriceUpdates(ctx, DefaultSeed, []int64{a.ID})
	if err != nil || n != 1 {
		t.Fatalf("applied %d: %v", n, err)
	}
	again, _ := s.Item(ctx, a.ID)
	if !maps.Equal(again.Price, a.Price) {
		t.Fatalf("price not restored: %v vs %v", again.Price, a.Price)
	}
	if ups, _ := s.PriceUpdates(ctx, DefaultSeed); len(ups) != 1 || ups[0].ItemID != b.ID {
		t.Fatalf("the locked item should be the only update left, got %+v", ups)
	}
}
