# Pricing Calculator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the stubbed calculator with the real pricing rules so every red test in `backend` goes green and the UI shows real breakdowns.

**Architecture:** `Calculator.Calculate` resolves order lines against the menu (aggregating duplicates, validating), computes the subtotal, then applies an ordered list of `Discount` rules, each seeing the running total left by the previous one. Rules live in their own file; `DefaultDiscounts()` wires the store's order. HTTP and frontend are already scaffolded and only need extra tests.

**Tech Stack:** Go 1.25, Fiber v3, `modernc.org/sqlite`, React 19 + Vite + Vitest.

**Spec:** `docs/SPEC.md` (project conventions in `AGENTS.md`)

## Global Constraints

- Money is integer satang (`pricing.Money`, `int64`), 1 THB = 100 satang, in domain, DB, and API. Fractional satang round half-up at each discount step.
- Discounts apply in a fixed order: bundle (pair) discounts first, member discount second. Each rule sees the running total after earlier rules.
- Pair rule: only Orange, Pink, Green; only the same item; pairs = quantity ÷ 2 (integer division); discount = 5% of the price of the paired sets; odd leftover pays full price; one applied discount per eligible item.
- Member rule: 10% of the running total after bundle discounts, only when the member flag is set.
- `Calculate` returns a `Breakdown` (subtotal, discounts, total), never a bare number.
- Unknown item code or negative quantity returns `ErrUnknownItem` / `ErrInvalidQuantity`; HTTP maps both to 400. Zero-quantity lines are allowed and ignored.
- `internal/pricing` imports nothing from Fiber, SQLite, or `net/http`. The calculator takes the menu as input.
- The frontend contains no pricing logic.
- Verification before finishing: `go test ./...`, `go vet ./...`, `gofmt -l .` (empty) in `backend`; `npm test`, `npm run lint`, `npm run build` in `frontend`.

## Review Focus

Failure modes the spec implies but the golden table does not exercise, most likely first:

1. Same code on two order lines (`ORANGE 1` + `ORANGE 1`): expected to count as one pair, not two unpaired singles. Pinned in Task 1 and Task 2.
2. Zero-quantity line (`RED 0`): expected to cost nothing and raise no error. Pinned in Task 1.
3. Absurd quantity (e.g. `9_000_000_000_000`): expected to be rejected with `ErrInvalidQuantity` (HTTP 400), never an overflowed or negative total. Pinned in Task 1 and Task 4. Decision added beyond the spec: `MaxQuantity = 10_000` per code after aggregation.
4. Empty order, with or without member: expected total 0, no member discount line, and `discounts` serialized as `[]`, never `null` (the UI maps over it). Pinned in Task 3 and Task 4.
5. Malformed or mistyped body (`{"items":[{"code":"RED","qty":"two"}]}`, non-JSON): expected 400, not 500. Pinned in Task 4.

---

### Task 1: Resolve lines and compute the subtotal

**Files:**
- Modify: `backend/internal/pricing/pricing.go`
- Modify: `backend/internal/pricing/pricing_test.go`

**Interfaces:**
- Consumes: existing types in `pricing.go` (`Menu`, `Line`, `Order`, `PricedLine`, `Breakdown`, `Discount`, `ErrUnknownItem`, `ErrInvalidQuantity`).
- Produces: `const MaxQuantity = 10_000`; `func (c *Calculator) Calculate(menu Menu, order Order) (Breakdown, error)` returning a `Breakdown` whose `Discounts` is a non-nil empty slice when nothing applies; unexported `resolve(menu Menu, lines []Line) ([]PricedLine, error)` returning one `PricedLine` per distinct code, in order of first appearance, zero-quantity codes omitted.

- [ ] **Step 1: Commit the untracked scaffold and spec as the baseline**

```bash
git add -A && git commit -m "chore: scaffold backend, frontend, spec and plan"
```

- [ ] **Step 2: Write the failing tests** in `pricing_test.go` (external package `pricing_test`, reuse `testMenu()`):
  - `TestCalculateSubtotal` using `pricing.NewCalculator()` (no rules), table cases asserting `Subtotal`, `Total`, `len(Discounts)==0` and `Discounts != nil`:
    - `RED 1, GREEN 1` → 9000
    - `RED 1, RED 2` (duplicate lines) → 15000
    - `RED 0` → 0, no error
  - Add to `TestCalculateRejectsInvalidLines` cases: empty code `""` → `ErrUnknownItem`; `RED` qty `pricing.MaxQuantity+1` → `ErrInvalidQuantity`; `RED 6000` + `RED 6000` (aggregate exceeds the cap) → `ErrInvalidQuantity`.

- [ ] **Step 3: Run to verify failure**

Run: `cd backend && go test ./internal/pricing -run 'TestCalculateSubtotal|TestCalculateRejectsInvalidLines' -v`
Expected: FAIL (`pricing: not implemented`; `pricing.MaxQuantity` undefined is a compile failure, also acceptable).

- [ ] **Step 4: Implement `Calculate` and `resolve`** in `pricing.go`. Validate each line (negative → `ErrInvalidQuantity`, unknown code → `ErrUnknownItem`, wrapped with `%w` and the offending code), sum quantities per code in a map plus an order slice, then check each aggregate against `MaxQuantity`. Subtotal is the sum of `Price × Qty` over resolved lines. Loop over `c.discounts`, subtract each applied amount from the running total, and append to `Discounts`. Delete `errNotImplemented`.

- [ ] **Step 5: Run to verify pass**

Run: `cd backend && go test ./internal/pricing -run 'TestCalculateSubtotal|TestCalculateRejectsInvalidLines' -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/pricing && git commit -m "feat(pricing): resolve order lines and compute subtotal"
```

---

### Task 2: Pair discount

**Files:**
- Create: `backend/internal/pricing/discount.go`
- Create: `backend/internal/pricing/discount_test.go`
- Modify: `backend/internal/pricing/pricing.go` (move `DefaultDiscounts` out; see Task 3)

**Interfaces:**
- Consumes: `Discount` interface, `PricedLine`, `AppliedDiscount`, `Money` from `pricing.go`; `Calculator.Calculate` from Task 1.
- Produces: `type PairDiscount struct{ Code string }` implementing `Discount` (value receiver); unexported `percentOf(amount Money, percent int64) Money` rounding half-up (used again in Task 3).

- [ ] **Step 1: Write the failing tests** in `discount_test.go`, all through `NewCalculator(...).Calculate(testMenu(), ...)`:
  - `TestPairDiscount` with `NewCalculator(pricing.PairDiscount{Code: "ORANGE"})`:
    - `ORANGE 5` → one discount, `Amount == 2400`, `Label == "Orange set pairs ×2 (5%)"`, `Total == 57600`
    - `ORANGE 1` → no discounts, `Total == 12000`
    - `ORANGE 1` + `ORANGE 1` (duplicate lines) → one discount, `Amount == 1200`
    - `RED 2` → no discounts (rule is bound to its code)
  - `TestPairDiscountsAreIndependentPerItem` with three rules (`ORANGE`, `PINK`, `GREEN`):
    - `ORANGE 1, PINK 1` → no discounts (mixed items never pair)
    - `GREEN 2, PINK 3` → two discounts, amounts 400 and 800, `Total == 30800`

- [ ] **Step 2: Run to verify failure**

Run: `cd backend && go test ./internal/pricing -run 'TestPairDiscount' -v`
Expected: FAIL (`pricing.PairDiscount` undefined)

- [ ] **Step 3: Implement `PairDiscount.Apply` and `percentOf`** in `discount.go`. Find the line for `d.Code`, `pairs := qty / 2`; if zero return `false`; amount is `percentOf(Money(pairs*2) * price, 5)`. Label format is exactly `"<Item.Name> pairs ×<pairs> (5%)"`. `percentOf` is `(amount*percent + 50) / 100` for non-negative amounts.

- [ ] **Step 4: Run to verify pass**

Run: `cd backend && go test ./internal/pricing -run 'TestPairDiscount' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/pricing && git commit -m "feat(pricing): add same-item pair discount"
```

---

### Task 3: Member discount and the store's rule order

**Files:**
- Modify: `backend/internal/pricing/discount.go`
- Modify: `backend/internal/pricing/discount_test.go`
- Modify: `backend/internal/pricing/pricing.go` (remove stub `DefaultDiscounts`)

**Interfaces:**
- Consumes: `PairDiscount`, `percentOf` from Task 2.
- Produces: `type MemberDiscount struct{}` implementing `Discount`, label exactly `"Member 10%"`; `func DefaultDiscounts() []Discount` returning `PairDiscount` for `ORANGE`, `PINK`, `GREEN` (in that order) followed by `MemberDiscount{}`.

- [ ] **Step 1: Write the failing tests** in `discount_test.go`:
  - `TestMemberDiscount` with `NewCalculator(pricing.MemberDiscount{})`:
    - `RED 1, GREEN 1`, member → one discount `Amount == 900`, `Label == "Member 10%"`, `Total == 8100`
    - same order, non-member → no discounts
    - empty order, member → no discounts, `Total == 0`
    - custom menu with one item `X` priced `1005` satang, `X 1`, member → `Amount == 101`, `Total == 904` (half-up)
  - `TestMemberDiscountAppliesAfterPairs` using `DefaultDiscounts()`: `ORANGE 5`, member → discounts `[2400, 5760]` in that order, `Total == 51840`.
- The existing `TestCalculateTotals` golden table is the acceptance test for this task.

- [ ] **Step 2: Run to verify failure**

Run: `cd backend && go test ./internal/pricing -v`
Expected: FAIL (`MemberDiscount` undefined / golden cases wrong)

- [ ] **Step 3: Implement `MemberDiscount.Apply` and `DefaultDiscounts`**: apply only when `member` is true and `runningTotal > 0`; amount is `percentOf(runningTotal, 10)`. Move `DefaultDiscounts` into `discount.go`.

- [ ] **Step 4: Run to verify the whole package passes**

Run: `cd backend && go test ./internal/pricing -v && go vet ./... && gofmt -l .`
Expected: PASS, no vet output, `gofmt -l` prints nothing.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/pricing && git commit -m "feat(pricing): add member discount and default rule order"
```

---

### Task 4: Pin the HTTP contract and finish verification

**Files:**
- Modify: `backend/internal/httpapi/httpapi_test.go`
- Modify: `README.md` only if the example response no longer matches real output

**Interfaces:**
- Consumes: `newTestApp(t)` helper already in `httpapi_test.go`; behavior from Tasks 1–3.
- Produces: nothing for later tasks.

- [ ] **Step 1: Write the failing-or-passing contract tests** (some may pass immediately; they pin behavior, so keep them):
  - `TestCalculateOrangeMember`: `ORANGE 5`, member → 200, `subtotal == 60000`, `total == 51840`, two discounts with amounts 2400 and 5760.
  - `TestCalculateEmptyOrder`: body `{"items":[],"member":true}` → 200, raw body contains `"discounts":[]`, `total == 0`.
  - `TestCalculateRejectsBadInput` table, each expecting 400:
    - `{"items":[{"code":"RED","qty":-1}]}`
    - `{"items":[{"code":"RED","qty":9000000000000}]}`
    - `{"items":[{"code":"RED","qty":"two"}]}`
    - `not json`

- [ ] **Step 2: Run**

Run: `cd backend && go test ./... -v`
Expected: PASS everywhere. If a bad-input case returns 500, fix the handler in `internal/httpapi/httpapi.go` so the failure maps to 400, then re-run.

- [ ] **Step 3: Run the full verification set**

Run: `cd backend && go test ./... && go vet ./... && gofmt -l . ; cd ../frontend && npm test && npm run lint && npm run build`
Expected: all pass, `gofmt -l` empty.

- [ ] **Step 4: Smoke-check the UI against the real backend**

Run: start the `backend` and `frontend` configurations from `.claude/launch.json`, open the frontend, add 5 × Orange and tick Member card.
Expected: page shows subtotal ฿600.00, a pairs line −฿24.00, `Member 10%` −฿57.60, total ฿518.40.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "test(httpapi): pin calculate contract and bad-input handling"
```
