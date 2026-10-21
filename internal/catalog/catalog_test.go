package catalog

import (
	"context"
	"errors"
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
