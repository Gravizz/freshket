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
	percent      INTEGER NOT NULL,
	member_only  INTEGER NOT NULL DEFAULT 0,
	active       INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE IF NOT EXISTS rule_items (
	rule_id   INTEGER NOT NULL REFERENCES discount_rules(id),
	item_code TEXT NOT NULL,
	qty       INTEGER NOT NULL,
	PRIMARY KEY (rule_id, item_code)
);`

// seed reproduces the store's original promotions: same-item pairs of Orange,
// Pink and Green at 5% (bundles of one item), then 10% for members.
var seed = []pricing.Rule{
	{Name: "Orange pairs", Bundle: []pricing.Component{{ItemCode: "ORANGE", Qty: 2}}, Percent: 5, Active: true},
	{Name: "Pink pairs", Bundle: []pricing.Component{{ItemCode: "PINK", Qty: 2}}, Percent: 5, Active: true},
	{Name: "Green pairs", Bundle: []pricing.Component{{ItemCode: "GREEN", Qty: 2}}, Percent: 5, Active: true},
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
		if _, err := r.Create(ctx, rule); err != nil {
			return fmt.Errorf("seed rule %q: %w", rule.Name, err)
		}
	}
	return nil
}

// listQuery reads rules with their bundle components in one statement, so a
// concurrent write cannot leave a rule and its components out of step.
// Components come back in the order they were inserted.
const listQuery = `
SELECT r.id, r.name, r.percent, r.member_only, r.active, i.item_code, i.qty
FROM discount_rules r LEFT JOIN rule_items i ON i.rule_id = r.id
%s
ORDER BY r.id, i.rowid`

// ListActive returns the rules that currently apply, in ID order.
func (r *Repository) ListActive(ctx context.Context) ([]pricing.Rule, error) {
	return r.list(ctx, `WHERE r.active = 1`)
}

// ListAll returns every rule, including deactivated ones, in ID order.
func (r *Repository) ListAll(ctx context.Context) ([]pricing.Rule, error) {
	return r.list(ctx, ``)
}

func (r *Repository) list(ctx context.Context, where string) ([]pricing.Rule, error) {
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(listQuery, where))
	if err != nil {
		return nil, fmt.Errorf("query rules: %w", err)
	}
	defer rows.Close()

	var out []pricing.Rule
	for rows.Next() {
		var (
			rule pricing.Rule
			code sql.NullString
			qty  sql.NullInt64
		)
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Percent, &rule.MemberOnly, &rule.Active, &code, &qty); err != nil {
			return nil, fmt.Errorf("scan rule: %w", err)
		}
		if n := len(out); n > 0 && out[n-1].ID == rule.ID {
			rule = out[n-1]
			out = out[:n-1]
		}
		if code.Valid {
			rule.Bundle = append(rule.Bundle, pricing.Component{ItemCode: code.String, Qty: int(qty.Int64)})
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

// Create inserts a rule and its bundle and returns it with its assigned ID.
func (r *Repository) Create(ctx context.Context, rule pricing.Rule) (pricing.Rule, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return pricing.Rule{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	// The first statement of a transaction is a write, so SQLite takes the
	// write lock up front and the busy timeout applies.
	res, err := tx.ExecContext(ctx,
		`INSERT INTO discount_rules (name, percent, member_only, active) VALUES (?, ?, ?, ?)`,
		rule.Name, rule.Percent, rule.MemberOnly, rule.Active,
	)
	if err != nil {
		return pricing.Rule{}, fmt.Errorf("insert rule: %w", err)
	}
	if rule.ID, err = res.LastInsertId(); err != nil {
		return pricing.Rule{}, fmt.Errorf("rule id: %w", err)
	}
	if err := insertBundle(ctx, tx, rule); err != nil {
		return pricing.Rule{}, err
	}
	if err := tx.Commit(); err != nil {
		return pricing.Rule{}, fmt.Errorf("commit rule: %w", err)
	}
	return rule, nil
}

// Update replaces the rule with rule.ID and its whole bundle, or returns ErrNotFound.
func (r *Repository) Update(ctx context.Context, rule pricing.Rule) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`UPDATE discount_rules SET name = ?, percent = ?, member_only = ?, active = ? WHERE id = ?`,
		rule.Name, rule.Percent, rule.MemberOnly, rule.Active, rule.ID,
	)
	if err != nil {
		return fmt.Errorf("update rule %d: %w", rule.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %d", ErrNotFound, rule.ID)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM rule_items WHERE rule_id = ?`, rule.ID); err != nil {
		return fmt.Errorf("clear bundle of rule %d: %w", rule.ID, err)
	}
	if err := insertBundle(ctx, tx, rule); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit rule %d: %w", rule.ID, err)
	}
	return nil
}

func insertBundle(ctx context.Context, tx *sql.Tx, rule pricing.Rule) error {
	for _, c := range rule.Bundle {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO rule_items (rule_id, item_code, qty) VALUES (?, ?, ?)`,
			rule.ID, c.ItemCode, c.Qty,
		); err != nil {
			return fmt.Errorf("insert bundle item %q of rule %d: %w", c.ItemCode, rule.ID, err)
		}
	}
	return nil
}
