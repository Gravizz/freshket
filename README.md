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

This serves the API on `:8080` and creates and seeds `freshket.db` on first run (7 menu items, 3 pair rules, each a bundle of one item, and the member rule). `PORT` and `DB_PATH` override the defaults. If you have a `freshket.db` from an earlier version, delete it first: the schema changed.

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
- `GET /api/admin/rules`, `POST /api/admin/rules`, `PUT /api/admin/rules/:id` with `{ "name", "bundle": [{ "itemCode", "qty" }], "percent", "memberOnly", "active" }`. An empty or missing `bundle` is a whole-order rule.

```bash
curl -X POST localhost:8080/api/admin/menu -H 'Content-Type: application/json' \
  -d '{"code":"BLACK","name":"Black set","price":4500,"active":true}'
curl -X POST localhost:8080/api/admin/rules -H 'Content-Type: application/json' \
  -d '{"name":"Bundle A","bundle":[{"itemCode":"GREEN","qty":2},{"itemCode":"RED","qty":1}],"percent":12,"memberOnly":false,"active":true}'
```

Ordering 2 × Green and 1 × Red now shows `Bundle A ×1 (12%)` −฿15.60 and a total of ฿114.40, with no restart.

## Assumptions

- **Pair discount**: Orange, Pink and Green get 5% off each pair of the *same* item. With 5 Orange, 2 pairs (4 sets) are discounted and the 5th pays full price. Mixed pairs such as Orange + Pink don't count. A pair rule is just a bundle of one item with quantity 2.
- **Bundles**: a promotion is a bundle of items with quantities (for example Green ×2 + Red ×1) and a percent off. The number of complete bundles is the fewest any component can fill; each complete bundle gets the percent off its sets, and leftover sets pay full price. Green ×5 + Red ×2 holds two bundles and one full-price Green.
- **Overlapping bundles**: a set belongs to at most one bundle. Bigger bundles (more sets in total) claim sets first, ties go to the lower rule ID, and a later rule only sees the unclaimed sets. So Bundle A beats the Green pair rule for the same Greens, and any Greens left over can still pair up.
- **Discount order**: bundle rules apply first (biggest first), then whole-order rules by rule ID. The 10% member discount applies to the total after bundle discounts.
- **Rules**: a condition is only a bundle of item × quantity components (an empty bundle means the whole order), plus an optional members-only flag. The effect is a whole-number percent from 1 to 100. Labels read `<rule name> ×<bundles> (<percent>%)`. The seed rules reproduce the original pair and member promotions.
- **No deletes**: items and rules are switched off with `active=false`, because rules refer to items. An inactive item cannot be ordered and an inactive rule never applies. The admin page adds items and rules, and each row has an active switch that calls `PUT`. On the admin page the item code is picked from a honeycomb tray of 37 colours: the colour's six hex digits (for example `7B3FE4`) become the code, and the store tints the item with that colour.
- **Quantity cap**: at most 10,000 sets of one item per order, so totals cannot overflow. The store's `+` button stops at the cap.
- **Input checks**: the server validates every request (item code, name 1–60 characters, price ฿0.01–฿1,000,000, rule percent 1–100, bundle quantities 1–10,000, no repeated item in a bundle) and trims the padding off names. The admin page checks the same limits first and shows the message under the field after a failed submit, so a typo never needs a round trip. Prices are read from the typed digits (`45`, `45.5`, `45.50`); `12abc`, `1e3`, `1,50` and a third decimal are rejected rather than guessed at. The colour tray greys out colours already used as item codes, a bundle cannot list the same item twice, and the submit buttons and switches lock while a save is in flight so a double click cannot add a rule twice.
- **Stale baskets**: if an item is taken off the menu while a customer holds it in the basket, the failed price check refreshes the menu, drops that set from the basket and says so, rather than leaving the customer on a permanent error.
- **Money**: amounts are integer satang end to end. A percentage that produces a fraction of a satang rounds half-up at each discount step.

## Design

Diagrams (architecture, ER, request flows, calculation logic) are in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

- `internal/pricing` is pure domain logic with no HTTP or DB imports, so it is unit-tested in isolation.
- Each promotion implements the `pricing.Discount` interface, and the `Calculator` applies them in order. `pricing.Rule` is the data-driven implementation, loaded from SQLite on every calculation. **To add a promotion**, add a rule on `/#/admin` or through `POST /api/admin/rules`; no code changes. A new kind of condition (for example a minimum total) would be a new `Discount` type; one that uses up sets would also implement `Claimer`.
- The backend is the single source of truth for prices. The frontend only displays the returned breakdown.
