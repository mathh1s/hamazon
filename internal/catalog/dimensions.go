package catalog

import (
	"context"
	"slices"
	"strings"

	"github.com/mathh1s/hamazon/internal/db"
)

// Dimensions an item can depend on. The overworld is always available, so
// it is never stored.
var Dimensions = []string{"nether", "end"}

func normalizeDimensions(in []string) ([]string, bool) {
	out := []string{}
	for _, d := range in {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" || d == "overworld" {
			continue
		}
		if !slices.Contains(Dimensions, d) {
			return nil, false
		}
		if !slices.Contains(out, d) {
			out = append(out, d)
		}
	}
	slices.Sort(out)
	return out, true
}

func splitDimensions(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}

// DisabledDimensions lists the dimensions an admin switched off, for
// example because the End is not open on the server yet.
func (s *Store) DisabledDimensions(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key FROM meta WHERE key LIKE 'dimension_off:%'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out = append(out, strings.TrimPrefix(key, "dimension_off:"))
	}
	return out, rows.Err()
}

func (s *Store) SetDimensionEnabled(ctx context.Context, dim string, enabled bool) error {
	if !slices.Contains(Dimensions, dim) {
		return ErrNotFound
	}
	if enabled {
		_, err := s.db.ExecContext(ctx, `DELETE FROM meta WHERE key = ?`, "dimension_off:"+dim)
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO meta (key, value) VALUES (?, ?)`, "dimension_off:"+dim, db.Now())
	return err
}

// Blocked reports whether an item needs a dimension that is switched off.
func (it Item) Blocked(disabled []string) bool {
	for _, d := range it.Dimensions {
		if slices.Contains(disabled, d) {
			return true
		}
	}
	return false
}
