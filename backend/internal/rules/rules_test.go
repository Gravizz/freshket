package rules_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/gravizz/freshket/backend/internal/rules"
)

// A database file from before bundle rules keeps its old discount_rules table.
// Migrate must refuse it: reading old rows as bundle rules would turn the item
// rules into whole-order discounts and misprice every order.
func TestMigrateRefusesTheOldSchema(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE discount_rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, item_code TEXT NOT NULL DEFAULT '',
		group_size INTEGER NOT NULL DEFAULT 0, percent INTEGER NOT NULL,
		member_only INTEGER NOT NULL DEFAULT 0, active INTEGER NOT NULL DEFAULT 1);
		INSERT INTO discount_rules (name, item_code, group_size, percent) VALUES ('pairs', 'ORANGE', 2, 5);`)
	if err != nil {
		t.Fatal(err)
	}

	err = rules.NewRepository(db).Migrate(context.Background())

	if err == nil || !strings.Contains(err.Error(), "delete the database file") {
		t.Errorf("Migrate() = %v, want an error telling the admin to delete the database file", err)
	}
}
