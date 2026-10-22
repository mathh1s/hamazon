package catalog

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/mathh1s/hamazon/internal/db"
)

//go:embed seed.yaml
var DefaultSeed []byte

type seedFile struct {
	Currencies []Currency  `yaml:"currencies"`
	Categories []Category  `yaml:"categories"`
	Items      []ItemInput `yaml:"items"`
}

// Seed adds currencies and categories that are missing and fills the item
// list once. Items are only seeded on the very first run, so deleting or
// editing items in the admin area is never undone by a restart.
func (s *Store) Seed(ctx context.Context, data []byte) error {
	f, err := parseSeed(data)
	if err != nil {
		return err
	}

	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := insertGroups(ctx, tx, f); err != nil {
			return err
		}
		var seeded int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM meta WHERE key = 'items_seeded'`).Scan(&seeded); err != nil {
			return err
		}
		if seeded > 0 {
			return nil
		}
		for _, it := range f.Items {
			if _, err := insertItem(ctx, tx, it); err != nil {
				return fmt.Errorf("seed %s: %w", it.Name, err)
			}
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO meta (key, value) VALUES ('items_seeded', ?)`, db.Now())
		return err
	})
}

func parseSeed(data []byte) (seedFile, error) {
	var f seedFile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return f, fmt.Errorf("parse seed: %w", err)
	}
	for i := range f.Currencies {
		if f.Currencies[i].Sort == 0 {
			f.Currencies[i].Sort = i + 1
		}
	}
	for i := range f.Categories {
		if f.Categories[i].Sort == 0 {
			f.Categories[i].Sort = i + 1
		}
	}

	for i := range f.Items {
		if f.Items[i].Sort == 0 {
			f.Items[i].Sort = i + 1
		}
		if err := validateItem(&f.Items[i], f.Currencies, f.Categories); err != nil {
			return f, fmt.Errorf("seed item %d (%s): %w", i, f.Items[i].Name, err)
		}
	}

	return f, nil
}

func insertGroups(ctx context.Context, tx *sql.Tx, f seedFile) error {
	for _, c := range f.Currencies {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO currencies (id, name, texture, sort) VALUES (?, ?, ?, ?)`,
			c.ID, c.Name, c.Texture, c.Sort); err != nil {
			return err
		}
	}
	for _, c := range f.Categories {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO categories (id, name, icon, sort) VALUES (?, ?, ?, ?)`,
			c.ID, c.Name, c.Icon, c.Sort); err != nil {
			return err
		}
	}
	return nil
}

// defaultKey identifies a default item across installs. Texture alone is
// not enough because enchanted books and music discs share one.
func defaultKey(texture, description string) string {
	return texture + "|" + description
}

// MissingDefaults lists default items that are neither in the catalog nor
// were deleted on purpose.
func (s *Store) MissingDefaults(ctx context.Context, data []byte) ([]ItemInput, error) {
	f, err := parseSeed(data)
	if err != nil {
		return nil, err
	}
	have := map[string]bool{}
	rows, err := s.db.QueryContext(ctx, `SELECT texture, description FROM items UNION SELECT key, '' FROM dismissed_defaults`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var a, b string
		if err := rows.Scan(&a, &b); err != nil {
			return nil, err
		}
		if strings.Contains(a, "|") {
			have[a] = true
		} else {
			have[defaultKey(a, b)] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []ItemInput
	for _, it := range f.Items {
		if !have[defaultKey(it.Texture, it.Description)] {
			out = append(out, it)
		}
	}
	return out, nil
}

// AddMissingDefaults inserts the missing defaults hidden, so an admin can
// check the prices before players see them. New categories come along.
func (s *Store) AddMissingDefaults(ctx context.Context, data []byte) ([]int64, error) {
	f, err := parseSeed(data)
	if err != nil {
		return nil, err
	}
	missing, err := s.MissingDefaults(ctx, data)
	if err != nil {
		return nil, err
	}
	var ids []int64
	err = db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := insertGroups(ctx, tx, f); err != nil {
			return err
		}
		hidden := false
		for _, it := range missing {
			it.Enabled = &hidden
			id, err := insertItem(ctx, tx, it)
			if err != nil {
				return fmt.Errorf("add %s: %w", it.Name, err)
			}
			ids = append(ids, id)
		}
		return nil
	})
	return ids, err
}
