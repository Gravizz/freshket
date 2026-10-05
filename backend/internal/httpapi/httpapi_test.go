package httpapi_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/gofiber/fiber/v3"

	"github.com/gravizz/freshket/backend/internal/database"
	"github.com/gravizz/freshket/backend/internal/httpapi"
	"github.com/gravizz/freshket/backend/internal/menu"
	"github.com/gravizz/freshket/backend/internal/rules"
)

// testConfig gives requests room to finish when many tests or goroutines
// compete for the CPU; Fiber's default of one second is flaky under load.
var testConfig = fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true}

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

func TestCalculateRejectsInvalidStoredPricing(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		message string
	}{
		{"negative item price", `UPDATE menu_items SET price = -5000 WHERE code = 'RED'`, "invalid item"},
		{"invalid rule percent", `UPDATE discount_rules SET percent = 200 WHERE id = 4`, "invalid rule"},
		{"zero bundle quantity", `UPDATE rule_items SET qty = 0 WHERE item_code = 'ORANGE'`, "invalid rule"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			db.SetMaxOpenConns(1)
			t.Cleanup(func() { db.Close() })
			menuRepo, rulesRepo := menu.NewRepository(db), rules.NewRepository(db)
			for _, migrate := range []func(context.Context) error{menuRepo.Migrate, rulesRepo.Migrate} {
				if err := migrate(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			// Bypass admin validation to pin the calculator's own error boundary.
			if _, err := db.Exec(tt.query); err != nil {
				t.Fatal(err)
			}
			app := httpapi.New(menuRepo, rulesRepo)
			req := httptest.NewRequest(http.MethodPost, "/api/orders/calculate",
				strings.NewReader(`{"items":[{"code":"RED","qty":1}],"member":true}`))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req, testConfig)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), tt.message) {
				t.Errorf("response = %d %s, want 400 containing %q", resp.StatusCode, body, tt.message)
			}
		})
	}
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
	resp, err := app.Test(req, testConfig)
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

func adminRequest(t *testing.T, app *fiber.App, method, path, body string) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, testConfig)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, string(raw)
}

func TestAdminAddedItemCanBeOrderedWithoutRestart(t *testing.T) {
	app := newTestApp(t)

	resp, raw := adminRequest(t, app, http.MethodPost, "/api/admin/menu",
		`{"code":"BLACK","name":"Black set","price":4500,"active":true}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", resp.StatusCode, raw)
	}

	_, menuRaw := adminRequest(t, app, http.MethodGet, "/api/menu", "")
	if strings.Count(menuRaw, `"code"`) != 8 {
		t.Errorf("menu = %s, want 8 items", menuRaw)
	}

	resp, calcRaw := postCalculate(t, app, `{"items":[{"code":"BLACK","qty":2}]}`)
	if resp.StatusCode != http.StatusOK || !strings.Contains(calcRaw, `"subtotal":9000`) {
		t.Errorf("calculate = %d %s, want 200 with subtotal 9000", resp.StatusCode, calcRaw)
	}
}

func TestAdminEditPrice(t *testing.T) {
	app := newTestApp(t)

	resp, raw := adminRequest(t, app, http.MethodPut, "/api/admin/menu/RED", `{"name":"Red set","price":6000,"active":true}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d, want 200: %s", resp.StatusCode, raw)
	}

	_, calcRaw := postCalculate(t, app, `{"items":[{"code":"RED","qty":1}]}`)
	if !strings.Contains(calcRaw, `"subtotal":6000`) {
		t.Errorf("calculate = %s, want subtotal 6000", calcRaw)
	}
}

func TestDeactivatedItemCannotBeOrdered(t *testing.T) {
	app := newTestApp(t)

	resp, raw := adminRequest(t, app, http.MethodPut, "/api/admin/menu/RED", `{"name":"Red set","price":5000,"active":false}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d, want 200: %s", resp.StatusCode, raw)
	}

	_, menuRaw := adminRequest(t, app, http.MethodGet, "/api/menu", "")
	if strings.Contains(menuRaw, `"RED"`) {
		t.Errorf("customer menu = %s, want RED omitted", menuRaw)
	}
	_, adminRaw := adminRequest(t, app, http.MethodGet, "/api/admin/menu", "")
	if !strings.Contains(adminRaw, `{"code":"RED","name":"Red set","price":5000,"active":false}`) {
		t.Errorf("admin menu = %s, want RED listed as inactive", adminRaw)
	}
	resp, _ = postCalculate(t, app, `{"items":[{"code":"RED","qty":1}]}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("calculate status = %d, want 400 for a deactivated item", resp.StatusCode)
	}
}

func TestAdminItemRejectsBadInput(t *testing.T) {
	item := func(code, name string, price int64) string {
		b, _ := json.Marshal(map[string]any{"code": code, "name": name, "price": price, "active": true})
		return string(b)
	}
	tests := []struct {
		name, method, path, body string
		want                     int
	}{
		{"duplicate code", http.MethodPost, "/api/admin/menu", item("RED", "Another red", 100), http.StatusConflict},
		{"empty code", http.MethodPost, "/api/admin/menu", item("", "X", 100), http.StatusBadRequest},
		{"lowercase code", http.MethodPost, "/api/admin/menu", item("red2", "X", 100), http.StatusBadRequest},
		{"code too long", http.MethodPost, "/api/admin/menu", item(strings.Repeat("A", 21), "X", 100), http.StatusBadRequest},
		{"code with symbol", http.MethodPost, "/api/admin/menu", item("A-B", "X", 100), http.StatusBadRequest},
		{"zero price", http.MethodPost, "/api/admin/menu", item("FREE", "X", 0), http.StatusBadRequest},
		{"price above the limit", http.MethodPost, "/api/admin/menu", item("BIGPRICE", "X", 100_000_001), http.StatusBadRequest},
		{"update with non-JSON body", http.MethodPut, "/api/admin/menu/RED", `nope`, http.StatusBadRequest},
		{"empty name", http.MethodPost, "/api/admin/menu", item("NONAME", "", 100), http.StatusBadRequest},
		{"name too long", http.MethodPost, "/api/admin/menu", item("LONGNAME", strings.Repeat("n", 61), 100), http.StatusBadRequest},
		{"price is a string", http.MethodPost, "/api/admin/menu", `{"code":"STR","name":"X","price":"45","active":true}`, http.StatusBadRequest},
		{"not json", http.MethodPost, "/api/admin/menu", `nope`, http.StatusBadRequest},
		{"update unknown code", http.MethodPut, "/api/admin/menu/NOPE", `{"name":"X","price":100,"active":true}`, http.StatusNotFound},
		{"update with zero price", http.MethodPut, "/api/admin/menu/RED", `{"name":"Red set","price":0,"active":true}`, http.StatusBadRequest},
	}

	app := newTestApp(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, raw := adminRequest(t, app, tt.method, tt.path, tt.body)
			if resp.StatusCode != tt.want {
				t.Errorf("status = %d, want %d: %s", resp.StatusCode, tt.want, raw)
			}
		})
	}
}

func TestAdminAddedRuleChangesPricingWithoutRestart(t *testing.T) {
	app := newTestApp(t)

	resp, raw := adminRequest(t, app, http.MethodPost, "/api/admin/rules",
		`{"name":"triple","bundle":[{"itemCode":"BLUE","qty":3}],"percent":10,"memberOnly":false,"active":true}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", resp.StatusCode, raw)
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(raw), &created); err != nil || created.ID == 0 {
		t.Errorf("created = %s, want a non-zero id", raw)
	}

	_, calcRaw := postCalculate(t, app, `{"items":[{"code":"BLUE","qty":3}]}`)
	var got struct {
		Total     int64 `json:"total"`
		Discounts []struct {
			Label  string `json:"label"`
			Amount int64  `json:"amount"`
		} `json:"discounts"`
	}
	if err := json.Unmarshal([]byte(calcRaw), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Discounts) != 1 || got.Discounts[0].Amount != 900 || got.Discounts[0].Label != "triple ×1 (10%)" || got.Total != 8100 {
		t.Errorf("calculate = %s, want one discount of 900 labelled \"triple ×1 (10%%)\" and total 8100", calcRaw)
	}
}

func TestAdminDeactivatedRuleStopsApplying(t *testing.T) {
	app := newTestApp(t)

	_, listRaw := adminRequest(t, app, http.MethodGet, "/api/admin/rules", "")
	var rules []struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		MemberOnly bool   `json:"memberOnly"`
	}
	if err := json.Unmarshal([]byte(listRaw), &rules); err != nil || len(rules) != 4 {
		t.Fatalf("rules = %s, want the 4 seeded rules", listRaw)
	}
	var memberID int64
	for _, r := range rules {
		if r.Name == "Member 10%" {
			memberID = r.ID
		}
	}

	resp, raw := adminRequest(t, app, http.MethodPut, fmt.Sprintf("/api/admin/rules/%d", memberID),
		`{"name":"Member 10%","bundle":[],"percent":10,"memberOnly":true,"active":false}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d, want 200: %s", resp.StatusCode, raw)
	}

	_, calcRaw := postCalculate(t, app, `{"items":[{"code":"RED","qty":1}],"member":true}`)
	if !strings.Contains(calcRaw, `"discounts":[]`) || !strings.Contains(calcRaw, `"total":5000`) {
		t.Errorf("calculate = %s, want no discounts and total 5000", calcRaw)
	}
}

func TestAdminRuleWithEmptyBundleAppliesToWholeOrder(t *testing.T) {
	app := newTestApp(t)

	resp, raw := adminRequest(t, app, http.MethodPost, "/api/admin/rules",
		`{"name":"Welcome 5%","bundle":[],"percent":5,"memberOnly":false,"active":true}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", resp.StatusCode, raw)
	}

	_, calcRaw := postCalculate(t, app, `{"items":[{"code":"RED","qty":1}]}`)
	if !strings.Contains(calcRaw, `{"label":"Welcome 5%","amount":250}`) || !strings.Contains(calcRaw, `"total":4750`) {
		t.Errorf("calculate = %s, want a 250 discount and total 4750 for a non-member", calcRaw)
	}
}

func TestRuleForDeactivatedItemDoesNotBreakOtherOrders(t *testing.T) {
	app := newTestApp(t)
	adminRequest(t, app, http.MethodPost, "/api/admin/rules",
		`{"name":"triple","bundle":[{"itemCode":"BLUE","qty":3}],"percent":10,"memberOnly":false,"active":true}`)
	adminRequest(t, app, http.MethodPut, "/api/admin/menu/BLUE", `{"name":"Blue set","price":3000,"active":false}`)

	resp, calcRaw := postCalculate(t, app, `{"items":[{"code":"RED","qty":1}]}`)
	if resp.StatusCode != http.StatusOK || !strings.Contains(calcRaw, `"total":5000`) {
		t.Errorf("RED order = %d %s, want 200 with total 5000", resp.StatusCode, calcRaw)
	}
	resp, _ = postCalculate(t, app, `{"items":[{"code":"BLUE","qty":3}]}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("BLUE order status = %d, want 400", resp.StatusCode)
	}
}

func TestAdminRuleRejectsBadInput(t *testing.T) {
	tests := []struct {
		name, method, path, body string
		want                     int
	}{
		{"percent zero", http.MethodPost, "/api/admin/rules", `{"name":"x","bundle":[{"itemCode":"RED","qty":2}],"percent":0,"active":true}`, http.StatusBadRequest},
		{"percent over 100", http.MethodPost, "/api/admin/rules", `{"name":"x","bundle":[{"itemCode":"RED","qty":2}],"percent":101,"active":true}`, http.StatusBadRequest},
		{"group size above the quantity limit", http.MethodPost, "/api/admin/rules", `{"name":"x","bundle":[{"itemCode":"RED","qty":10001}],"percent":5,"active":true}`, http.StatusBadRequest},
		{"empty name", http.MethodPost, "/api/admin/rules", `{"name":"","bundle":[{"itemCode":"RED","qty":2}],"percent":5,"active":true}`, http.StatusBadRequest},
		{"unknown item", http.MethodPost, "/api/admin/rules", `{"name":"x","bundle":[{"itemCode":"NOPE","qty":2}],"percent":5,"active":true}`, http.StatusBadRequest},
		{"percent is a string", http.MethodPost, "/api/admin/rules", `{"name":"x","bundle":[{"itemCode":"RED","qty":2}],"percent":"ten","active":true}`, http.StatusBadRequest},
		{"component qty zero", http.MethodPost, "/api/admin/rules", `{"name":"x","bundle":[{"itemCode":"RED","qty":0}],"percent":5,"active":true}`, http.StatusBadRequest},
		{"same item twice", http.MethodPost, "/api/admin/rules", `{"name":"x","bundle":[{"itemCode":"RED","qty":1},{"itemCode":"RED","qty":1}],"percent":5,"active":true}`, http.StatusBadRequest},
		{"unknown item in second component", http.MethodPost, "/api/admin/rules", `{"name":"x","bundle":[{"itemCode":"RED","qty":1},{"itemCode":"NOPE","qty":1}],"percent":5,"active":true}`, http.StatusBadRequest},
		{"bundle is not an array", http.MethodPost, "/api/admin/rules", `{"name":"x","bundle":"RED","percent":5,"active":true}`, http.StatusBadRequest},
		{"not json", http.MethodPost, "/api/admin/rules", `nope`, http.StatusBadRequest},
		{"update unknown id", http.MethodPut, "/api/admin/rules/999", `{"name":"x","bundle":[{"itemCode":"RED","qty":2}],"percent":5,"active":true}`, http.StatusNotFound},
		{"update with a non-numeric id", http.MethodPut, "/api/admin/rules/abc", `{"name":"x","bundle":[{"itemCode":"RED","qty":2}],"percent":5,"active":true}`, http.StatusBadRequest},
		{"update with non-JSON body", http.MethodPut, "/api/admin/rules/1", `nope`, http.StatusBadRequest},
		{"update with invalid percent", http.MethodPut, "/api/admin/rules/1", `{"name":"x","bundle":[{"itemCode":"RED","qty":2}],"percent":200,"active":true}`, http.StatusBadRequest},
	}

	app := newTestApp(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, raw := adminRequest(t, app, tt.method, tt.path, tt.body)
			if resp.StatusCode != tt.want {
				t.Errorf("status = %d, want %d: %s", resp.StatusCode, tt.want, raw)
			}
		})
	}
}

// A fresh database must hold exactly what the brief describes: seven items at
// their prices, a 5% pair rule for Orange, Pink and Green, and 10% for members.
func TestFreshDatabaseMatchesTheBrief(t *testing.T) {
	app := newTestApp(t)

	_, menuRaw := adminRequest(t, app, http.MethodGet, "/api/admin/menu", "")
	wantMenu := `[` +
		`{"code":"RED","name":"Red set","price":5000,"active":true},` +
		`{"code":"GREEN","name":"Green set","price":4000,"active":true},` +
		`{"code":"BLUE","name":"Blue set","price":3000,"active":true},` +
		`{"code":"YELLOW","name":"Yellow set","price":5000,"active":true},` +
		`{"code":"PINK","name":"Pink set","price":8000,"active":true},` +
		`{"code":"PURPLE","name":"Purple set","price":9000,"active":true},` +
		`{"code":"ORANGE","name":"Orange set","price":12000,"active":true}]`
	if menuRaw != wantMenu {
		t.Errorf("menu = %s\nwant   %s", menuRaw, wantMenu)
	}

	_, rulesRaw := adminRequest(t, app, http.MethodGet, "/api/admin/rules", "")
	wantRules := `[` +
		`{"id":1,"name":"Orange pairs","bundle":[{"itemCode":"ORANGE","qty":2}],"percent":5,"memberOnly":false,"active":true},` +
		`{"id":2,"name":"Pink pairs","bundle":[{"itemCode":"PINK","qty":2}],"percent":5,"memberOnly":false,"active":true},` +
		`{"id":3,"name":"Green pairs","bundle":[{"itemCode":"GREEN","qty":2}],"percent":5,"memberOnly":false,"active":true},` +
		`{"id":4,"name":"Member 10%","bundle":[],"percent":10,"memberOnly":true,"active":true}]`
	if rulesRaw != wantRules {
		t.Errorf("rules = %s\nwant   %s", rulesRaw, wantRules)
	}
}

// Admin writes and customer calculations hit the same SQLite file at once;
// none of them may fail with "database is locked".
func TestConcurrentWritesAndCalculationsDoNotFail(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "concurrent.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	menuRepo, rulesRepo := menu.NewRepository(db), rules.NewRepository(db)
	for _, migrate := range []func(context.Context) error{menuRepo.Migrate, rulesRepo.Migrate} {
		if err := migrate(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	app := httpapi.New(menuRepo, rulesRepo)

	const requests = 400
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		failed []string
		sem    = make(chan struct{}, 64)
	)
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			var resp *http.Response
			var raw string
			if i%5 == 0 {
				body := fmt.Sprintf(`{"name":"r%d","bundle":[{"itemCode":"RED","qty":2}],"percent":5,"memberOnly":false,"active":true}`, i)
				resp, raw = adminRequest(t, app, http.MethodPost, "/api/admin/rules", body)
			} else {
				resp, raw = postCalculate(t, app, `{"items":[{"code":"RED","qty":2}],"member":true}`)
			}
			if resp.StatusCode >= 500 {
				mu.Lock()
				failed = append(failed, fmt.Sprintf("%d %s", resp.StatusCode, raw))
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if len(failed) > 0 {
		t.Errorf("%d of %d requests failed, e.g. %s", len(failed), requests, failed[0])
	}
}

func TestAdminItemAcceptsPriceAtTheLimit(t *testing.T) {
	app := newTestApp(t)

	resp, raw := adminRequest(t, app, http.MethodPost, "/api/admin/menu",
		`{"code":"MAXPRICE","name":"Priciest set","price":100000000,"active":true}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", resp.StatusCode, raw)
	}

	// The largest allowed price times the largest allowed quantity must not overflow.
	resp, calcRaw := postCalculate(t, app, `{"items":[{"code":"MAXPRICE","qty":10000}],"member":true}`)
	if resp.StatusCode != http.StatusOK || !strings.Contains(calcRaw, `"subtotal":1000000000000`) {
		t.Errorf("calculate = %d %s, want 200 with subtotal 1000000000000", resp.StatusCode, calcRaw)
	}
}

func TestAdminBundleRuleDiscountsMixedItemsWithoutRestart(t *testing.T) {
	app := newTestApp(t)

	resp, raw := adminRequest(t, app, http.MethodPost, "/api/admin/rules",
		`{"name":"Bundle A","bundle":[{"itemCode":"GREEN","qty":2},{"itemCode":"RED","qty":1}],"percent":12,"memberOnly":false,"active":true}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", resp.StatusCode, raw)
	}

	_, calcRaw := postCalculate(t, app, `{"items":[{"code":"GREEN","qty":2},{"code":"RED","qty":1}]}`)
	want := `"subtotal":13000,"discounts":[{"label":"Bundle A ×1 (12%)","amount":1560}],"total":11440`
	if !strings.Contains(calcRaw, want) {
		t.Errorf("calculate = %s, want it to contain %s (the Green pair rule must not also fire)", calcRaw, want)
	}
}

func TestAdminRuleUpdateReplacesBundle(t *testing.T) {
	app := newTestApp(t)
	adminRequest(t, app, http.MethodPost, "/api/admin/rules",
		`{"name":"combo","bundle":[{"itemCode":"GREEN","qty":2},{"itemCode":"RED","qty":1}],"percent":12,"memberOnly":false,"active":true}`)

	resp, raw := adminRequest(t, app, http.MethodPut, "/api/admin/rules/5",
		`{"name":"combo","bundle":[{"itemCode":"BLUE","qty":3}],"percent":12,"memberOnly":false,"active":true}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d, want 200: %s", resp.StatusCode, raw)
	}

	_, listRaw := adminRequest(t, app, http.MethodGet, "/api/admin/rules", "")
	want := `{"id":5,"name":"combo","bundle":[{"itemCode":"BLUE","qty":3}],"percent":12,"memberOnly":false,"active":true}`
	if !strings.Contains(listRaw, want) {
		t.Errorf("rules = %s, want it to contain %s", listRaw, want)
	}
}

func TestAdminRuleWithoutBundleIsWholeOrder(t *testing.T) {
	for name, body := range map[string]string{
		"omitted": `{"name":"Welcome 5%","percent":5,"memberOnly":false,"active":true}`,
		"null":    `{"name":"Welcome 5%","bundle":null,"percent":5,"memberOnly":false,"active":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			app := newTestApp(t)
			resp, raw := adminRequest(t, app, http.MethodPost, "/api/admin/rules", body)
			if resp.StatusCode != http.StatusCreated || !strings.Contains(raw, `"bundle":[]`) {
				t.Fatalf("create = %d %s, want 201 with \"bundle\":[]", resp.StatusCode, raw)
			}
			_, listRaw := adminRequest(t, app, http.MethodGet, "/api/admin/rules", "")
			if strings.Contains(listRaw, `"bundle":null`) {
				t.Errorf("rules = %s, want no null bundle", listRaw)
			}
			_, calcRaw := postCalculate(t, app, `{"items":[{"code":"RED","qty":1}]}`)
			if !strings.Contains(calcRaw, `{"label":"Welcome 5%","amount":250}`) {
				t.Errorf("calculate = %s, want the whole-order discount", calcRaw)
			}
		})
	}
}

func TestRejectedBundleStoresNothing(t *testing.T) {
	app := newTestApp(t)

	resp, _ := adminRequest(t, app, http.MethodPost, "/api/admin/rules",
		`{"name":"x","bundle":[{"itemCode":"RED","qty":1},{"itemCode":"NOPE","qty":1}],"percent":5,"active":true}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}

	_, listRaw := adminRequest(t, app, http.MethodGet, "/api/admin/rules", "")
	if got := strings.Count(listRaw, `"id":`); got != 4 {
		t.Errorf("rules = %s, want only the 4 seeded rules", listRaw)
	}
}

func TestAdminRuleUpdateWithoutBundleReturnsEmptyBundle(t *testing.T) {
	app := newTestApp(t)

	resp, raw := adminRequest(t, app, http.MethodPut, "/api/admin/rules/4", `{"name":"Member 10%","percent":10,"memberOnly":true,"active":true}`)
	if resp.StatusCode != http.StatusOK || !strings.Contains(raw, `"bundle":[]`) {
		t.Errorf("update = %d %s, want 200 with \"bundle\":[]", resp.StatusCode, raw)
	}
}

func TestAdminTrimsSurroundingWhitespaceFromNames(t *testing.T) {
	app := newTestApp(t)

	resp, raw := adminRequest(t, app, http.MethodPost, "/api/admin/menu",
		`{"code":"BLACK","name":"  Black set  ","price":4500,"active":true}`)
	if resp.StatusCode != http.StatusCreated || !strings.Contains(raw, `"name":"Black set"`) {
		t.Errorf("create item = %d %s, want 201 with a trimmed name", resp.StatusCode, raw)
	}
	resp, raw = adminRequest(t, app, http.MethodPut, "/api/admin/menu/BLACK",
		`{"name":"\tBlack set 2 ","price":4500,"active":true}`)
	if resp.StatusCode != http.StatusOK || !strings.Contains(raw, `"name":"Black set 2"`) {
		t.Errorf("update item = %d %s, want 200 with a trimmed name", resp.StatusCode, raw)
	}
	_, raw = adminRequest(t, app, http.MethodGet, "/api/admin/menu", "")
	if strings.Contains(raw, `"name":" `) || strings.Contains(raw, ` ","price"`) {
		t.Errorf("admin menu = %s, want no stored padding", raw)
	}

	resp, raw = adminRequest(t, app, http.MethodPost, "/api/admin/rules",
		`{"name":"  Lunch deal ","bundle":[],"percent":5,"memberOnly":false,"active":true}`)
	if resp.StatusCode != http.StatusCreated || !strings.Contains(raw, `"name":"Lunch deal"`) {
		t.Errorf("create rule = %d %s, want 201 with a trimmed name", resp.StatusCode, raw)
	}
	_, raw = adminRequest(t, app, http.MethodGet, "/api/admin/rules", "")
	if strings.Contains(raw, `"name":"  Lunch`) || strings.Contains(raw, `deal "`) {
		t.Errorf("admin rules = %s, want no stored padding", raw)
	}
}

func TestAdminRejectsWhitespaceOnlyNames(t *testing.T) {
	app := newTestApp(t)

	resp, raw := adminRequest(t, app, http.MethodPost, "/api/admin/menu", `{"code":"BLANK","name":"   ","price":100,"active":true}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("item = %d %s, want 400", resp.StatusCode, raw)
	}
	resp, raw = adminRequest(t, app, http.MethodPost, "/api/admin/rules", `{"name":"   ","bundle":[],"percent":5,"active":true}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("rule = %d %s, want 400", resp.StatusCode, raw)
	}
}
