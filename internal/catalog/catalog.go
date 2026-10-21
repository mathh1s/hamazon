package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mathh1s/hamazon/internal/db"
	"github.com/mathh1s/hamazon/internal/validate"
)

var ErrNotFound = errors.New("item not found")

type Currency struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Texture string `json:"texture"`
	Sort    int    `json:"sort"`
}

type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
	Sort int    `json:"sort"`
}

// Price maps currency id to the amount required per purchase unit. Several
// entries mean the player pays all of them, e.g. 2 diamonds and 8 emeralds.
type Price map[string]int

type Item struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Texture         string `json:"texture"`
	CategoryID      string `json:"categoryId"`
	Count           int    `json:"count"`
	Description     string `json:"description"`
	MaxPerOrder     int    `json:"maxPerOrder"`
	PlayerLimit     *int   `json:"playerLimit"`
	PlayerLimitDays *int   `json:"playerLimitDays"`
	Enabled         bool   `json:"enabled"`
	Sort            int    `json:"sort"`
	Price           Price  `json:"price"`
}

// ItemInput is the editable part of an item, shared by the admin API and the seed.
type ItemInput struct {
	Name            string `json:"name" yaml:"name"`
	Texture         string `json:"texture" yaml:"texture"`
	CategoryID      string `json:"categoryId" yaml:"category"`
	Count           int    `json:"count" yaml:"count"`
	Description     string `json:"description" yaml:"description"`
	MaxPerOrder     int    `json:"maxPerOrder" yaml:"maxPerOrder"`
	PlayerLimit     *int   `json:"playerLimit" yaml:"playerLimit"`
	PlayerLimitDays *int   `json:"playerLimitDays" yaml:"playerLimitDays"`
	Enabled         *bool  `json:"enabled" yaml:"enabled"`
	Sort            int    `json:"sort" yaml:"sort"`
	Price           Price  `json:"price" yaml:"price"`
}

type Store struct {
	db *sql.DB
}

func NewStore(conn *sql.DB) *Store { return &Store{db: conn} }

func (s *Store) Currencies(ctx context.Context) ([]Currency, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, texture, sort FROM currencies ORDER BY sort, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Currency
	for rows.Next() {
		var c Currency
		if err := rows.Scan(&c.ID, &c.Name, &c.Texture, &c.Sort); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) Categories(ctx context.Context) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, icon, sort FROM categories ORDER BY sort, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Icon, &c.Sort); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

const itemColumns = `id, name, texture, category_id, count, description, max_per_order, player_limit, player_limit_days, enabled, sort`

func scanItem(sc interface{ Scan(...any) error }) (Item, error) {
	var it Item
	var limit, days sql.NullInt64
	err := sc.Scan(&it.ID, &it.Name, &it.Texture, &it.CategoryID, &it.Count, &it.Description,
		&it.MaxPerOrder, &limit, &days, &it.Enabled, &it.Sort)
	if limit.Valid && days.Valid {
		l, d := int(limit.Int64), int(days.Int64)
		it.PlayerLimit, it.PlayerLimitDays = &l, &d
	}
	it.Price = Price{}
	return it, err
}

func (s *Store) Items(ctx context.Context, includeDisabled bool) ([]Item, error) {
	return s.queryItems(ctx, s.db, includeDisabled, nil)
}

// ItemsByID loads the given items inside the caller's transaction so order
// validation and the insert see the same prices.
func (s *Store) ItemsByID(ctx context.Context, q db.Querier, ids []int64) (map[int64]Item, error) {
	items, err := s.queryItems(ctx, q, true, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]Item, len(items))
	for _, it := range items {
		out[it.ID] = it
	}
	return out, nil
}

func (s *Store) Item(ctx context.Context, id int64) (Item, error) {
	items, err := s.queryItems(ctx, s.db, true, []int64{id})
	if err != nil {
		return Item{}, err
	}
	if len(items) == 0 {
		return Item{}, ErrNotFound
	}
	return items[0], nil
}

func (s *Store) queryItems(ctx context.Context, q db.Querier, includeDisabled bool, ids []int64) ([]Item, error) {
	var where []string
	var args []any
	if !includeDisabled {
		where = append(where, "enabled = 1")
	}
	if ids != nil {
		if len(ids) == 0 {
			return nil, nil
		}
		where = append(where, "id IN ("+placeholders(len(ids))+")")
		for _, id := range ids {
			args = append(args, id)
		}
	}
	query := `SELECT ` + itemColumns + ` FROM items`
	if len(where) > 0 {
		query += ` WHERE ` + strings.Join(where, " AND ")
	}
	query += ` ORDER BY sort, id`

	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	var items []Item
	index := map[int64]int{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		index[it.ID] = len(items)
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return items, nil
	}

	priceRows, err := q.QueryContext(ctx, `SELECT item_id, currency_id, amount FROM item_prices`)
	if err != nil {
		return nil, err
	}
	defer priceRows.Close()
	for priceRows.Next() {
		var itemID int64
		var currency string
		var amount int
		if err := priceRows.Scan(&itemID, &currency, &amount); err != nil {
			return nil, err
		}
		if i, ok := index[itemID]; ok {
			items[i].Price[currency] = amount
		}
	}
	return items, priceRows.Err()
}

func (s *Store) Create(ctx context.Context, in ItemInput) (Item, error) {
	if err := s.Validate(ctx, &in); err != nil {
		return Item{}, err
	}
	var id int64
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		var err error
		id, err = insertItem(ctx, tx, in)
		return err
	})
	if err != nil {
		return Item{}, err
	}
	return s.Item(ctx, id)
}

func (s *Store) Update(ctx context.Context, id int64, in ItemInput) (Item, error) {
	if err := s.Validate(ctx, &in); err != nil {
		return Item{}, err
	}
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE items SET name = ?, texture = ?, category_id = ?, count = ?, description = ?,
			max_per_order = ?, player_limit = ?, player_limit_days = ?, enabled = ?, sort = ?, updated_at = ? WHERE id = ?`,
			in.Name, in.Texture, in.CategoryID, in.Count, in.Description, in.MaxPerOrder,
			in.PlayerLimit, in.PlayerLimitDays, *in.Enabled, in.Sort, db.Now(), id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM item_prices WHERE item_id = ?`, id); err != nil {
			return err
		}
		return insertPrices(ctx, tx, id, in.Price)
	})
	if err != nil {
		return Item{}, err
	}
	return s.Item(ctx, id)
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func insertItem(ctx context.Context, q db.Querier, in ItemInput) (int64, error) {
	now := db.Now()
	res, err := q.ExecContext(ctx, `INSERT INTO items (name, texture, category_id, count, description, max_per_order,
		player_limit, player_limit_days, enabled, sort, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Name, in.Texture, in.CategoryID, in.Count, in.Description, in.MaxPerOrder,
		in.PlayerLimit, in.PlayerLimitDays, *in.Enabled, in.Sort, now, now)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, insertPrices(ctx, q, id, in.Price)
}

func insertPrices(ctx context.Context, q db.Querier, itemID int64, price Price) error {
	currencies := make([]string, 0, len(price))
	for c := range price {
		currencies = append(currencies, c)
	}
	sort.Strings(currencies)
	for _, c := range currencies {
		if _, err := q.ExecContext(ctx, `INSERT INTO item_prices (item_id, currency_id, amount) VALUES (?, ?, ?)`, itemID, c, price[c]); err != nil {
			return err
		}
	}
	return nil
}

var texturePattern = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

// Validate normalizes the input in place and reports every problem at once
// so the admin form can highlight all bad fields together.
func (s *Store) Validate(ctx context.Context, in *ItemInput) error {
	currencies, err := s.Currencies(ctx)
	if err != nil {
		return err
	}
	categories, err := s.Categories(ctx)
	if err != nil {
		return err
	}
	return validateItem(in, currencies, categories)
}

func validateItem(in *ItemInput, currencies []Currency, categories []Category) error {
	errs := validate.Errors{}

	in.Name = strings.TrimSpace(in.Name)
	in.Texture = strings.TrimPrefix(strings.TrimSpace(in.Texture), "minecraft:")
	in.Description = strings.TrimSpace(in.Description)
	if in.Enabled == nil {
		enabled := true
		in.Enabled = &enabled
	}

	if in.Name == "" || len([]rune(in.Name)) > 64 {
		errs.Add("name", "Name must be 1 to 64 characters")
	}
	if !texturePattern.MatchString(in.Texture) {
		errs.Add("texture", "Texture must be a Minecraft item id like diamond_pickaxe")
	}
	if len([]rune(in.Description)) > 200 {
		errs.Add("description", "Description can be at most 200 characters")
	}
	if !containsID(categories, in.CategoryID, func(c Category) string { return c.ID }) {
		errs.Add("categoryId", "Unknown category")
	}
	if in.Count < 1 || in.Count > 64 {
		errs.Add("count", "Count must be between 1 and 64")
	}
	if in.MaxPerOrder < 1 || in.MaxPerOrder > 999 {
		errs.Add("maxPerOrder", "Max per order must be between 1 and 999")
	}
	if (in.PlayerLimit == nil) != (in.PlayerLimitDays == nil) {
		errs.Add("playerLimit", "Set both the player limit and its period, or neither")
	} else if in.PlayerLimit != nil {
		if *in.PlayerLimit < 1 || *in.PlayerLimit > 9999 {
			errs.Add("playerLimit", "Player limit must be between 1 and 9999")
		} else if *in.PlayerLimit < in.MaxPerOrder {
			errs.Add("playerLimit", "Player limit cannot be lower than max per order")
		}
		if *in.PlayerLimitDays < 1 || *in.PlayerLimitDays > 365 {
			errs.Add("playerLimitDays", "Period must be between 1 and 365 days")
		}
	}

	if len(in.Price) == 0 {
		errs.Add("price", "Set at least one currency")
	}
	if len(in.Price) > 2 {
		errs.Add("price", "A price can combine at most two currencies")
	}
	for currency, amount := range in.Price {
		if !containsID(currencies, currency, func(c Currency) string { return c.ID }) {
			errs.Add("price", fmt.Sprintf("Unknown currency %q", currency))
		} else if amount < 1 || amount > 100000 {
			errs.Add("price", "Amounts must be between 1 and 100000")
		}
	}

	return errs.Err()
}

func containsID[T any](list []T, id string, key func(T) string) bool {
	for _, v := range list {
		if key(v) == id {
			return true
		}
	}
	return false
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}
