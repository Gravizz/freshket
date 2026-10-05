package httpapi_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/gofiber/fiber/v3"

	"github.com/gravizz/freshket/backend/internal/httpapi"
	"github.com/gravizz/freshket/backend/internal/menu"
	"github.com/gravizz/freshket/backend/internal/pricing"
)

func newTestApp(t *testing.T) *fiber.App {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1) // every connection to :memory: is a separate database
	t.Cleanup(func() { db.Close() })

	repo := menu.NewRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return httpapi.New(repo, pricing.NewCalculator(pricing.DefaultDiscounts()...))
}

func TestListMenu(t *testing.T) {
	app := newTestApp(t)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/menu", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var items []struct {
		Code  string `json:"code"`
		Price int64  `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 7 {
		t.Errorf("got %d items, want 7", len(items))
	}
}

func TestCalculate(t *testing.T) {
	app := newTestApp(t)

	body := `{"items":[{"code":"RED","qty":1},{"code":"GREEN","qty":1}],"member":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/orders/calculate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got struct {
		Subtotal int64 `json:"subtotal"`
		Total    int64 `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Subtotal != 9000 || got.Total != 8100 {
		t.Errorf("subtotal, total = %d, %d, want 9000, 8100", got.Subtotal, got.Total)
	}
}

func TestCalculateRejectsUnknownItem(t *testing.T) {
	app := newTestApp(t)

	body := `{"items":[{"code":"BLACK","qty":1}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/orders/calculate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
