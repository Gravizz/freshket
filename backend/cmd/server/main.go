// Command server runs the food store calculator API.
package main

import (
	"context"
	"database/sql"
	"log"
	"os"

	_ "modernc.org/sqlite"

	"github.com/gravizz/freshket/backend/internal/httpapi"
	"github.com/gravizz/freshket/backend/internal/menu"
	"github.com/gravizz/freshket/backend/internal/pricing"
)

func main() {
	dbPath := getenv("DB_PATH", "freshket.db")
	addr := ":" + getenv("PORT", "8080")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := menu.NewRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	app := httpapi.New(repo, pricing.NewCalculator(pricing.DefaultDiscounts()...))
	log.Fatal(app.Listen(addr))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
