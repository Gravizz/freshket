package httpapi_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/gofiber/fiber/v3"

	"github.com/gravizz/freshket/backend/internal/httpapi"
	"github.com/gravizz/freshket/backend/internal/menu"
	"github.com/gravizz/freshket/backend/internal/rules"
)

func newTestApp(t *testing.T) *fiber.App {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1) // every connection to :memory: is a separate database
	t.Cleanup(func() { db.Close() })

	menuRepo, rulesRepo := menu.NewRepository(db), rules.NewRepository(db)
	for _, migrate := range []func(context.Context) error{menuRepo.Migrate, rulesRepo.Migrate} {
		if err := migrate(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	return httpapi.New(menuRepo, rulesRepo)
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

func postCalculate(t *testing.T, app *fiber.App, body string) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/orders/calculate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, string(raw)
}

func TestCalculateOrangeMember(t *testing.T) {
	app := newTestApp(t)

	resp, raw := postCalculate(t, app, `{"items":[{"code":"ORANGE","qty":5}],"member":true}`)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, raw)
	}
	var got struct {
		Subtotal  int64 `json:"subtotal"`
		Total     int64 `json:"total"`
		Discounts []struct {
			Amount int64 `json:"amount"`
		} `json:"discounts"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if got.Subtotal != 60000 || got.Total != 51840 || len(got.Discounts) != 2 ||
		got.Discounts[0].Amount != 2400 || got.Discounts[1].Amount != 5760 {
		t.Errorf("got %s, want subtotal 60000, total 51840, discounts [2400 5760]", raw)
	}
}

func TestCalculateEmptyOrderSerializesEmptyDiscounts(t *testing.T) {
	app := newTestApp(t)

	resp, raw := postCalculate(t, app, `{"items":[],"member":true}`)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, raw)
	}
	if !strings.Contains(raw, `"discounts":[]`) || !strings.Contains(raw, `"total":0`) {
		t.Errorf("body = %s, want discounts [] and total 0", raw)
	}
}

func TestCalculateRejectsBadInput(t *testing.T) {
	tests := []struct{ name, body string }{
		{"negative quantity", `{"items":[{"code":"RED","qty":-1}]}`},
		{"absurd quantity", `{"items":[{"code":"RED","qty":9000000000000}]}`},
		{"quantity is a string", `{"items":[{"code":"RED","qty":"two"}]}`},
		{"not json", `not json`},
	}

	app := newTestApp(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, raw := postCalculate(t, app, tt.body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want 400: %s", resp.StatusCode, raw)
			}
		})
	}
}

func TestMigrationsAreIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	menuRepo, rulesRepo := menu.NewRepository(db), rules.NewRepository(db)
	for i := 0; i < 2; i++ { // a restart runs the migrations again
		if err := menuRepo.Migrate(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := rulesRepo.Migrate(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	resp, raw := postCalculate(t, httpapi.New(menuRepo, rulesRepo), `{"items":[{"code":"ORANGE","qty":5}],"member":true}`)

	if resp.StatusCode != http.StatusOK || !strings.Contains(raw, `"total":51840`) || strings.Count(raw, `"label"`) != 2 {
		t.Errorf("body = %s, want total 51840 with exactly two discounts", raw)
	}
}
