package catalog

import (
	"context"
	"database/sql"
	"maps"

	"github.com/mathh1s/hamazon/internal/db"
)

// Terms are the parts of an item that decide what a player pays.
type Terms struct {
	Count           int   `json:"count"`
	MaxPerOrder     int   `json:"maxPerOrder"`
	PlayerLimit     *int  `json:"playerLimit"`
	PlayerLimitDays *int  `json:"playerLimitDays"`
	Price           Price `json:"price"`
}

type PriceUpdate struct {
	ItemID  int64  `json:"itemId"`
	Name    string `json:"name"`
	Texture string `json:"texture"`
	Locked  bool   `json:"locked"`
	Current Terms  `json:"current"`
	Default Terms  `json:"default"`
}

func termsOf(it Item) Terms {
	return Terms{Count: it.Count, MaxPerOrder: it.MaxPerOrder, PlayerLimit: it.PlayerLimit, PlayerLimitDays: it.PlayerLimitDays, Price: it.Price}
}

func termsOfInput(in ItemInput) Terms {
	return Terms{Count: in.Count, MaxPerOrder: in.MaxPerOrder, PlayerLimit: in.PlayerLimit, PlayerLimitDays: in.PlayerLimitDays, Price: in.Price}
}

func sameInt(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func sameTerms(a, b Terms) bool {
	return a.Count == b.Count && a.MaxPerOrder == b.MaxPerOrder && sameInt(a.PlayerLimit, b.PlayerLimit) &&
		sameInt(a.PlayerLimitDays, b.PlayerLimitDays) && maps.Equal(a.Price, b.Price)
}

// PriceUpdates lists items whose price terms differ from the current
// defaults. Items edited by hand are included but marked as locked.
func (s *Store) PriceUpdates(ctx context.Context, data []byte) ([]PriceUpdate, error) {
	f, err := parseSeed(data)
	if err != nil {
		return nil, err
	}
	defaults := map[string]ItemInput{}
	for _, it := range f.Items {
		defaults[defaultKey(it.Texture, it.Description)] = it
	}
	items, err := s.Items(ctx, true)
	if err != nil {
		return nil, err
	}
	out := []PriceUpdate{}
	for _, it := range items {
		def, ok := defaults[defaultKey(it.Texture, it.Description)]
		if !ok {
			continue
		}
		cur, want := termsOf(it), termsOfInput(def)
		if sameTerms(cur, want) {
			continue
		}
		out = append(out, PriceUpdate{ItemID: it.ID, Name: it.Name, Texture: it.Texture, Locked: it.PriceLocked, Current: cur, Default: want})
	}
	return out, nil
}

// ApplyPriceUpdates moves the chosen items to their default terms and
// unlocks them, so they follow future default changes again.
func (s *Store) ApplyPriceUpdates(ctx context.Context, data []byte, ids []int64) (int, error) {
	updates, err := s.PriceUpdates(ctx, data)
	if err != nil {
		return 0, err
	}
	chosen := map[int64]bool{}
	for _, id := range ids {
		chosen[id] = true
	}
	applied := 0
	err = db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		for _, u := range updates {
			if !chosen[u.ItemID] {
				continue
			}
			d := u.Default
			if _, err := tx.ExecContext(ctx, `UPDATE items SET count = ?, max_per_order = ?, player_limit = ?, player_limit_days = ?,
				price_locked = 0, updated_at = ? WHERE id = ?`, d.Count, d.MaxPerOrder, d.PlayerLimit, d.PlayerLimitDays, db.Now(), u.ItemID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM item_prices WHERE item_id = ?`, u.ItemID); err != nil {
				return err
			}
			if err := insertPrices(ctx, tx, u.ItemID, d.Price); err != nil {
				return err
			}
			applied++
		}
		return nil
	})
	return applied, err
}
