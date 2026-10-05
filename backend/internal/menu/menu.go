// Package menu stores the food store menu in SQLite.
package menu

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gravizz/freshket/backend/internal/pricing"
)

var (
	// ErrDuplicate is returned when creating an item whose code already exists.
	ErrDuplicate = errors.New("duplicate item code")
	// ErrNotFound is returned when updating an item that does not exist.
	ErrNotFound = errors.New("item not found")
)

const schema = `
CREATE TABLE IF NOT EXISTS menu_items (
	code  TEXT PRIMARY KEY,
	name  TEXT NOT NULL,
	price INTEGER NOT NULL, -- satang
	active INTEGER NOT NULL DEFAULT 1
);`

var seed = []pricing.Item{
	{Code: "RED", Name: "Red set", Price: 5000},
	{Code: "GREEN", Name: "Green set", Price: 4000},
	{Code: "BLUE", Name: "Blue set", Price: 3000},
	{Code: "YELLOW", Name: "Yellow set", Price: 5000},
	{Code: "PINK", Name: "Pink set", Price: 8000},
	{Code: "PURPLE", Name: "Purple set", Price: 9000},
	{Code: "ORANGE", Name: "Orange set", Price: 12000},
}

// Repository reads menu items from SQLite.
type Repository struct {
	db *sql.DB
}

// NewRepository returns a Repository backed by db.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Migrate creates the schema and inserts the seed items that are missing.
func (r *Repository) Migrate(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	for _, it := range seed {
		if _, err := r.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO menu_items (code, name, price) VALUES (?, ?, ?)`,
			it.Code, it.Name, it.Price,
		); err != nil {
			return fmt.Errorf("seed %s: %w", it.Code, err)
		}
	}
	return nil
}

// ListActive returns the items customers can order, in insertion order.
func (r *Repository) ListActive(ctx context.Context) ([]pricing.Item, error) {
	return r.list(ctx, `SELECT code, name, price, active FROM menu_items WHERE active = 1 ORDER BY rowid`)
}

// ListAll returns every item, including deactivated ones, in insertion order.
func (r *Repository) ListAll(ctx context.Context) ([]pricing.Item, error) {
	return r.list(ctx, `SELECT code, name, price, active FROM menu_items ORDER BY rowid`)
}

func (r *Repository) list(ctx context.Context, query string) ([]pricing.Item, error) {
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query menu: %w", err)
	}
	defer rows.Close()

	var items []pricing.Item
	for rows.Next() {
		var it pricing.Item
		if err := rows.Scan(&it.Code, &it.Name, &it.Price, &it.Active); err != nil {
			return nil, fmt.Errorf("scan menu item: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// Create inserts a new menu item, or returns ErrDuplicate if the code exists.
func (r *Repository) Create(ctx context.Context, it pricing.Item) error {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO menu_items (code, name, price, active) VALUES (?, ?, ?, ?) ON CONFLICT(code) DO NOTHING`,
		it.Code, it.Name, it.Price, it.Active,
	)
	if err != nil {
		return fmt.Errorf("insert %s: %w", it.Code, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %s", ErrDuplicate, it.Code)
	}
	return nil
}

// Update replaces the name, price and active flag of the item with it.Code,
// or returns ErrNotFound.
func (r *Repository) Update(ctx context.Context, it pricing.Item) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE menu_items SET name = ?, price = ?, active = ? WHERE code = ?`,
		it.Name, it.Price, it.Active, it.Code,
	)
	if err != nil {
		return fmt.Errorf("update %s: %w", it.Code, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %s", ErrNotFound, it.Code)
	}
	return nil
}
