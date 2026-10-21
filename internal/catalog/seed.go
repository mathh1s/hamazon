package catalog

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"fmt"

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
	var f seedFile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return fmt.Errorf("parse seed: %w", err)
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
			return fmt.Errorf("seed item %d (%s): %w", i, f.Items[i].Name, err)
		}
	}

	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
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
