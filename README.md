# Food Store Calculator

Freshket software engineer homework: a price calculator for a food store with member and bundle discounts. Menu items and discount rules are data, so an admin adds them from a web page with no code change.

- **Backend**: Go, Fiber v3, SQLite (`modernc.org/sqlite`, pure Go, no CGO)
- **Frontend**: React, TypeScript, Vite, Tailwind CSS v4

## Prerequisites

- Go 1.26+ (required by `modernc.org/sqlite`; with Go 1.21+ and network access the `go` command downloads it automatically)
- Node.js 22+

## Run

```bash
cd backend && go run ./cmd/server
```

This serves the API on `:8080` and creates and seeds `freshket.db` on first run (7 menu items, 3 pair rules and the member rule). `PORT` and `DB_PATH` override the defaults. If you have a `freshket.db` from an earlier version, delete it first: the schema changed.

```bash
cd frontend && npm install && npm run dev
```

Then open the URL Vite prints. Vite proxies `/api` to `:8080`, or to `API_URL` when it is set. The store page is `/`; the admin page is `/#/admin`.

## Test

```bash
cd backend && go test ./...
```

```bash
cd frontend && npm test
```

## API

All amounts are integer **satang** (1 THB = 100 satang).

- `GET /api/menu` returns `[{ "code": "RED", "name": "Red set", "price": 5000 }, ...]`
- `POST /api/orders/calculate`
  - request: `{ "items": [{ "code": "ORANGE", "qty": 5 }], "member": true }`
  - response: `{ "subtotal": 60000, "discounts": [{ "label": "...", "amount": 2400 }, ...], "total": 51840 }`
  - An unknown item code, a negative quantity or a quantity above 10,000 per item returns `400`.

Admin endpoints (no authentication: this is a simulation):

- `GET /api/admin/menu`, `POST /api/admin/menu`, `PUT /api/admin/menu/:code` with `{ "code", "name", "price", "active" }`. Codes are 1–20 characters of `A-Z`, `0-9`, `_`; price is from 1 satang to 100,000,000 satang (฿1,000,000). A duplicate code returns `409`.
- `GET /api/admin/rules`, `POST /api/admin/rules`, `PUT /api/admin/rules/:id` with `{ "name", "itemCode", "groupSize", "percent", "memberOnly", "active" }`.

```bash
curl -X POST localhost:8080/api/admin/menu -H 'Content-Type: application/json' \
  -d '{"code":"BLACK","name":"Black set","price":4500,"active":true}'
curl -X POST localhost:8080/api/admin/rules -H 'Content-Type: application/json' \
  -d '{"name":"triple","itemCode":"BLACK","groupSize":3,"percent":10,"memberOnly":false,"active":true}'
```

Ordering 3 × Black now shows `Black set triple ×1 (10%)` −฿13.50 and a total of ฿121.50, with no restart.

## Assumptions

- **Pair discount**: Orange, Pink and Green get 5% off each pair of the *same* item. With 5 Orange, 2 pairs (4 sets) are discounted and the 5th pays full price. Mixed pairs such as Orange + Pink don't count.
- **Discount order**: pair discounts apply first. The 10% member discount applies to the total after pair discounts.
- **Rules**: a condition is only an item and a group size (every N sets of that item), or no item for the whole order, plus an optional members-only flag. The effect is a whole-number percent from 1 to 100. Item rules apply before whole-order rules, each by rule ID. The seed rules reproduce the original pair and member promotions.
- **No deletes**: items and rules are switched off with `active=false`, because rules refer to items. An inactive item cannot be ordered and an inactive rule never applies. The admin page adds items and rules; switching them off is done through `PUT`.
- **Quantity cap**: at most 10,000 sets of one item per order, so totals cannot overflow.
- **Money**: amounts are integer satang end to end. A percentage that produces a fraction of a satang rounds half-up at each discount step.

## Design

Diagrams (architecture, ER, request flows, calculation logic) are in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

- `internal/pricing` is pure domain logic with no HTTP or DB imports, so it is unit-tested in isolation.
- Each promotion implements the `pricing.Discount` interface, and the `Calculator` applies them in order. `pricing.Rule` is the data-driven implementation, loaded from SQLite on every calculation. **To add a promotion**, add a rule on `/#/admin` or through `POST /api/admin/rules`; no code changes. A new kind of condition (for example a minimum total) would be a new `Discount` type.
- The backend is the single source of truth for prices. The frontend only displays the returned breakdown.
