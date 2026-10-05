// Package menu stores the food store menu in SQLite.
package menu

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gravizz/freshket/backend/internal/pricing"
)

const schema = `
CREATE TABLE IF NOT EXISTS menu_items (
	code  TEXT PRIMARY KEY,
	name  TEXT NOT NULL,
	price INTEGER NOT NULL -- satang
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

// List returns all menu items in insertion order.
func (r *Repository) List(ctx context.Context) ([]pricing.Item, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT code, name, price FROM menu_items ORDER BY rowid`)
	if err != nil {
		return nil, fmt.Errorf("query menu: %w", err)
	}
	defer rows.Close()

	var items []pricing.Item
	for rows.Next() {
		var it pricing.Item
		if err := rows.Scan(&it.Code, &it.Name, &it.Price); err != nil {
			return nil, fmt.Errorf("scan menu item: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}
