# Food Store Calculator

Freshket software engineer homework: a price calculator for a 7-item food store, with member and bundle discounts.

- **Backend**: Go, Fiber v3, SQLite (`modernc.org/sqlite`, pure Go, no CGO)
- **Frontend**: React, TypeScript, Vite, Tailwind CSS v4

## Prerequisites

- Go 1.25+
- Node.js 22+

## Run

```bash
cd backend && go run ./cmd/server
```

This serves the API on `:8080` and creates and seeds `freshket.db` on first run. `PORT` and `DB_PATH` override the defaults.

```bash
cd frontend && npm install && npm run dev
```

Then open the URL Vite prints. Vite proxies `/api` to `:8080`, or to `API_URL` when it is set.

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
  - An unknown item code or a negative quantity returns `400`.

## Assumptions

- **Pair discount**: Orange, Pink and Green get 5% off each pair of the *same* item. With 5 Orange, 2 pairs (4 sets) are discounted and the 5th pays full price. Mixed pairs such as Orange + Pink don't count.
- **Discount order**: pair discounts apply first. The 10% member discount applies to the total after pair discounts.
- **Money**: amounts are integer satang end to end. A percentage that produces a fraction of a satang rounds half-up at each discount step.

## Design

- `internal/pricing` is pure domain logic with no HTTP or DB imports, so it is unit-tested in isolation.
- Each promotion implements the `pricing.Discount` interface, and the `Calculator` applies them in order. **To add a promotion**, write a new `Discount` type and add it to `DefaultDiscounts()`. Existing code stays unchanged.
- The backend is the single source of truth for prices. The frontend only displays the returned breakdown.
