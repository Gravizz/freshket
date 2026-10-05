// Command server runs the food store calculator API.
package main

import (
	"context"
	"log"
	"os"

	"github.com/gravizz/freshket/backend/internal/database"
	"github.com/gravizz/freshket/backend/internal/httpapi"
	"github.com/gravizz/freshket/backend/internal/menu"
	"github.com/gravizz/freshket/backend/internal/rules"
)

func main() {
	dbPath := getenv("DB_PATH", "freshket.db")
	addr := ":" + getenv("PORT", "8080")

	db, err := database.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	menuRepo, rulesRepo := menu.NewRepository(db), rules.NewRepository(db)
	if err := menuRepo.Migrate(context.Background()); err != nil {
		log.Fatalf("migrate menu: %v", err)
	}
	if err := rulesRepo.Migrate(context.Background()); err != nil {
		log.Fatalf("migrate rules: %v", err)
	}

	app := httpapi.New(menuRepo, rulesRepo)
	log.Fatal(app.Listen(addr))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
