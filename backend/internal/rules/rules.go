// Package rules stores discount rules in SQLite.
package rules

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gravizz/freshket/backend/internal/pricing"
)

// ErrNotFound is returned when updating a rule that does not exist.
var ErrNotFound = errors.New("rule not found")

const schema = `
CREATE TABLE IF NOT EXISTS discount_rules (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	name         TEXT NOT NULL,
	item_code    TEXT NOT NULL DEFAULT '',
	group_size   INTEGER NOT NULL DEFAULT 0,
	percent      INTEGER NOT NULL,
	member_only  INTEGER NOT NULL DEFAULT 0,
	active       INTEGER NOT NULL DEFAULT 1
);`

// seed reproduces the store's original promotions: same-item pairs of Orange,
// Pink and Green at 5%, then 10% for members.
var seed = []pricing.Rule{
	{Name: "pairs", ItemCode: "ORANGE", GroupSize: 2, Percent: 5, Active: true},
	{Name: "pairs", ItemCode: "PINK", GroupSize: 2, Percent: 5, Active: true},
	{Name: "pairs", ItemCode: "GREEN", GroupSize: 2, Percent: 5, Active: true},
	{Name: "Member 10%", Percent: 10, MemberOnly: true, Active: true},
}

// Repository reads and writes discount rules.
type Repository struct {
	db *sql.DB
}

// NewRepository returns a Repository backed by db.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Migrate creates the schema and seeds the original rules when the table is empty.
func (r *Repository) Migrate(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("create rules schema: %w", err)
	}
	var n int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM discount_rules`).Scan(&n); err != nil {
		return fmt.Errorf("count rules: %w", err)
	}
	if n > 0 {
		return nil
	}
	for _, rule := range seed {
		if _, err := r.db.ExecContext(ctx,
			`INSERT INTO discount_rules (name, item_code, group_size, percent, member_only, active) VALUES (?, ?, ?, ?, ?, ?)`,
			rule.Name, rule.ItemCode, rule.GroupSize, rule.Percent, rule.MemberOnly, rule.Active,
		); err != nil {
			return fmt.Errorf("seed rule %q: %w", rule.Name, err)
		}
	}
	return nil
}

// ListActive returns the rules that currently apply, in ID order.
func (r *Repository) ListActive(ctx context.Context) ([]pricing.Rule, error) {
	return r.list(ctx, `SELECT id, name, item_code, group_size, percent, member_only, active FROM discount_rules WHERE active = 1 ORDER BY id`)
}

// ListAll returns every rule, including deactivated ones, in ID order.
func (r *Repository) ListAll(ctx context.Context) ([]pricing.Rule, error) {
	return r.list(ctx, `SELECT id, name, item_code, group_size, percent, member_only, active FROM discount_rules ORDER BY id`)
}

func (r *Repository) list(ctx context.Context, query string) ([]pricing.Rule, error) {
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query rules: %w", err)
	}
	defer rows.Close()

	var out []pricing.Rule
	for rows.Next() {
		var rule pricing.Rule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.ItemCode, &rule.GroupSize, &rule.Percent, &rule.MemberOnly, &rule.Active); err != nil {
			return nil, fmt.Errorf("scan rule: %w", err)
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

// Create inserts a rule and returns it with its assigned ID.
func (r *Repository) Create(ctx context.Context, rule pricing.Rule) (pricing.Rule, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO discount_rules (name, item_code, group_size, percent, member_only, active) VALUES (?, ?, ?, ?, ?, ?)`,
		rule.Name, rule.ItemCode, rule.GroupSize, rule.Percent, rule.MemberOnly, rule.Active,
	)
	if err != nil {
		return pricing.Rule{}, fmt.Errorf("insert rule: %w", err)
	}
	rule.ID, err = res.LastInsertId()
	if err != nil {
		return pricing.Rule{}, fmt.Errorf("rule id: %w", err)
	}
	return rule, nil
}

// Update replaces the rule with rule.ID, or returns ErrNotFound.
func (r *Repository) Update(ctx context.Context, rule pricing.Rule) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE discount_rules SET name = ?, item_code = ?, group_size = ?, percent = ?, member_only = ?, active = ? WHERE id = ?`,
		rule.Name, rule.ItemCode, rule.GroupSize, rule.Percent, rule.MemberOnly, rule.Active, rule.ID,
	)
	if err != nil {
		return fmt.Errorf("update rule %d: %w", rule.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %d", ErrNotFound, rule.ID)
	}
	return nil
}
