# Freshket — Food Store Calculator

Take-home test for Freshket's Software Engineer role. The task is a price calculator for a food store, delivered as a small full-stack app. An admin page adds menu items and discount rules without code changes. Reviewers score on **readability, maintainability, extensibility, logic**, plus extra credit for unit tests. The recommended budget is 60–120 minutes, so keep scope **tight**: a small, clean, fully tested core is better than many features.

## Stack

- **Backend**: Go + Fiber, SQLite via `modernc.org/sqlite`, a pure-Go driver, so no CGO toolchain is needed.
- **Frontend**: React + TypeScript (Vite) + Tailwind CSS v4. Tailwind v4 is configured through the `@tailwindcss/vite` plugin and `@import "tailwindcss";` in CSS. It does not use `tailwind.config.js`.
- Check versions in `backend/go.mod` and `frontend/package.json`.

## Layout

```
backend/
  cmd/server/        # main: wires config, DB, Fiber app
  internal/pricing/  # the calculator: pure domain logic, zero I/O
  internal/menu/     # SQLite repository + seed for the 7 menu items
  internal/rules/    # SQLite repository + seed for the discount rules
  internal/httpapi/  # Fiber handlers, request/response DTOs
frontend/
  src/               # menu picker, member toggle, price breakdown, /#/admin page
README.md            # how to run, assumptions, design notes (reviewer-facing)
```

Dependencies point inward: `httpapi` → `pricing`, `menu` and `rules`. `pricing` imports nothing from Fiber, SQLite, or HTTP.

## Domain rules (`internal/pricing`)

Menu prices in THB per set: Red 50, Green 40, Blue 30, Yellow 50, Pink 80, Purple 90, Orange 120.

1. **Subtotal** = sum of price × quantity.
2. **Item rule** (`ItemCode` set): every complete group of `GroupSize` sets of that item gets `Percent` off those sets; leftovers pay full price. The seed has 5% pair rules (`GroupSize` 2) for Orange, Pink and Green.
3. **Whole-order rule** (`ItemCode` empty, `GroupSize` 0): `Percent` off the running total. The seed has the 10% member rule (`MemberOnly`).
4. Item rules apply first, then whole-order rules, each by ascending rule ID. `Active` and `MemberOnly` gate every rule.

Rules and menu items live in SQLite, so the seed rows above are only the starting data. The golden cases below use them.
Pin these golden cases as table-driven tests:

| Order | Member | Total (THB) |
|---|---|---|
| Red ×1, Green ×1 | no | 90.00 |
| Red ×1, Green ×1 | yes | 81.00 |
| Orange ×5 | no | 576.00 (600 − 5% of 480) |
| Orange ×5 | yes | 518.40 |
| Green ×2, Pink ×3 | no | 308.00 (320 − 4 − 8) |
| empty order | no | 0.00 |

Implementation rules:

- **Money is integer satang** (`int64`, 1 THB = 100 satang) everywhere: domain, DB, and API. When a percentage produces a fraction of a satang, round half-up at each discount step. State this in the README.
- Every discount implements one small interface, `Discount.Apply(lines, member, runningTotal) (AppliedDiscount, bool)`, and `pricing.Rule` is its only data-driven implementation. A condition is only item + group size (+ member flag) and an effect is only a whole-number percent. This is the **extensibility** reviewers look for, so stop there: no expression language and no other condition types.
- Labels: item rule `"<Item.Name> <Rule.Name> ×<groups> (<Percent>%)"`, whole-order rule = `Rule.Name`.
- Items and rules are deactivated (`active=false`), never deleted. Inactive items are unknown to ordering; inactive rules never apply.
- Item code is 1–20 characters of `A-Z`, `0-9`, `_`; name 1–60 characters; price at least 1 satang. Quantity per code is capped at `MaxQuantity` (10,000) so totals cannot overflow.
- The calculator takes the menu as input. It never loads the menu itself, so tests need no DB.
- Return a **breakdown**, not just a number: subtotal, each applied discount with label and amount, and total. The UI renders that breakdown directly.
- Reject unknown item codes, negative or over-limit quantities, and invalid items or rules with typed errors. `httpapi` maps them to 400 (duplicate item code 409, missing item or rule on update 404).
- Wherever the spec is ambiguous, pick the reading above and record it under "Assumptions" in the README. Don't silently invent a different rule.

## API

- `GET /api/menu` returns `[{ code, name, price }]`, with price in satang.
- `GET /api/admin/menu` lists all items (`active` included); `POST /api/admin/menu` and `PUT /api/admin/menu/:code` take `{ code, name, price, active }` (the code in the path wins on `PUT`).
- `GET /api/admin/rules` lists all rules; `POST /api/admin/rules` and `PUT /api/admin/rules/:id` take `{ name, itemCode, groupSize, percent, memberOnly, active }`.
- Admin routes are intentionally unauthenticated: this is a simulation.
- `POST /api/orders/calculate` takes `{ items: [{ code, qty }], member: bool }` and returns `{ subtotal, discounts: [{ label, amount }], total }`, all in satang.

The backend is the **single source of truth** for pricing. The frontend only formats satang as THB (`(n / 100).toFixed(2)`) and never recomputes discounts.

## Commands

```bash
cd backend && go test ./...            # must pass before you finish
cd backend && go run ./cmd/server      # API on :8080, creates/seeds SQLite file on first run (delete freshket.db after schema changes)
cd frontend && npm run dev             # Vite dev server, proxies /api → :8080
cd frontend && npm test                # Vitest + React Testing Library
```

## Testing

- `internal/pricing` holds most of the test effort: the golden table above plus edges (odd quantities, mixed bundle items, rounding, unknown code). Aim for every rule and branch covered.
- `internal/httpapi` gets a few integration tests through `app.Test(req)` against an in-memory SQLite (`:memory:` with `db.SetMaxOpenConns(1)`, because each connection to `:memory:` opens a separate database).
- `internal/httpapi` tests also pin the admin acceptance cases: add an item or a rule through the API, then calculate, with no restart.
- Frontend gets component tests with a mocked `api.ts`: the calculator page renders the returned breakdown, and the admin page adds items and rules and shows server errors.
- When you finish a change, run `go test ./...`, `go vet ./...`, `npm test`, `npm run lint` (oxlint), and `npm run build`. All of them must pass.

## Code style

- Go: `gofmt`, small packages, accept interfaces and return structs, wrap errors with `%w`. Write a doc comment on each exported identifier in `pricing`.
- React: function components, plain `fetch` inside a small `api.ts`, local state with `useState`/`useReducer`. Use Tailwind utility classes only, with no extra UI kit.
- Add a dependency only when the standard library or the existing stack can't do the job.

## README (deliverable)

Keep `README.md` current. It is what the reviewer reads first. It covers: prerequisites, run and test commands, the API contract, **Assumptions** (pair rule, discount order, rounding), and how an admin adds an item or a promotion.
