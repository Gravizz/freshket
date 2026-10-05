# Admin-Managed Menu and Discount Rules Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** An admin can add menu items and discount rules (item + quantity condition, percent off) from a React `/admin` page, with no code change or restart.

**Architecture:** Discounts become data. A single `pricing.Rule` implements the existing `Discount` interface, so the `Calculator` is untouched; `pricing.Discounts(rules)` orders them (item-bound rules first, then whole-order rules, each by ID). Menu items and rules live in SQLite; `/api/orders/calculate` loads active items and active rules per request. Admin CRUD endpoints are unauthenticated by design (this is a simulation). The seed data reproduces today's behavior exactly (3 pair rules + member rule).

**Tech Stack:** Go 1.25, Fiber v3, `modernc.org/sqlite`, React 19 + Vite + Vitest.

**Spec:** `docs/SPEC.md`, amended by Task 6. This plan lifts two of its Out of Scope items (editing the menu through UI/API; configuration-driven rules). Authentication stays out of scope.

## Global Constraints

- Money is integer satang (`pricing.Money`, `int64`) in domain, DB, and API; fractional satang round half-up at each discount step (`percentOf`).
- Discounts apply in a fixed order: item-bound rules first, whole-order rules second, each group by ascending rule ID. Each rule sees the running total left by the previous one.
- A rule's condition is only: an item code and a group size (item-bound), and an optional member requirement. Its effect is only a whole-number percent, 1–100. No other condition or effect types.
- Item-bound rule: every complete group of `GroupSize` sets of `ItemCode` gets `Percent` off those sets; leftovers pay full price. Whole-order rule (`ItemCode` empty, `GroupSize` 0): `Percent` off the running total.
- `Calculate` returns a `Breakdown` (subtotal, discounts, total), never a bare number. Labels: item-bound `"<Item.Name> <Rule.Name> ×<groups> (<Percent>%)"`, whole-order = `Rule.Name` verbatim.
- Items and rules are deactivated (`active=false`), never deleted. Inactive items are unknown to ordering; inactive rules never apply.
- Item code: uppercase letters, digits, underscore, 1–20 chars, immutable after creation. Item name 1–60 chars. Price ≥ 1 satang.
- Unknown item, negative or over-`MaxQuantity` quantity, invalid item/rule → HTTP 400. Duplicate item code → 409. Missing item/rule on update → 404.
- Admin routes (`/api/admin/*`) have no authentication: this is a simulation, and the README says so in one line.
- `internal/pricing` imports nothing from Fiber, SQLite, or `net/http`. The frontend contains no pricing logic.
- The schema changed (`active` columns, `discount_rules` table): delete any local `backend/freshket.db` before first run. No in-place migration.
- Verification: `go test ./...`, `go vet ./...`, `gofmt -l .` (empty) in `backend`; `npm test`, `npm run lint`, `npm run build` in `frontend`.

## Review Focus

1. A tab still holds an item the admin just deactivated: ordering it must return 400 `unknown item`, not a price. Pinned in Task 3.
2. A rule pointing at a deactivated item: other orders still price normally and the rule silently does not apply. Pinned in Task 4.
3. Out-of-range rule input (percent 0, 101; group size 0 on an item rule; group size on a whole-order rule): 400, and `Percent: 100` followed by a member rule yields total 0 with no member line. Pinned in Task 1 and Task 4.
4. Duplicate or malformed item codes (`RED` twice, `red`, empty, 21 chars): 409 / 400. Pinned in Task 3.
5. Wrong JSON types in admin bodies (`"percent":"ten"`, `"price":"45"`): 400, not 500 or a silently zeroed field. Pinned in Task 3 and Task 4.

---

### Task 1: Rule and Item validation in the pricing domain

**Files:**
- Create: `backend/internal/pricing/rule.go`
- Create: `backend/internal/pricing/rule_test.go`
- Modify: `backend/internal/pricing/pricing.go` (add `Active bool` to `Item`)

**Interfaces:**
- Consumes: `Discount`, `PricedLine`, `AppliedDiscount`, `Money`, `percentOf` (in `discount.go`).
- Produces:
  - `var ErrInvalidItem`, `var ErrInvalidRule` (sentinels, wrapped with `%w`)
  - `func (i Item) Validate() error`
  - `type Rule struct { ID int64; Name string; ItemCode string; GroupSize int; Percent int; MemberOnly bool; Active bool }`
  - `func (r Rule) Validate() error`
  - `func (r Rule) Apply(lines []PricedLine, member bool, runningTotal Money) (AppliedDiscount, bool)` (implements `Discount`)
  - `func Discounts(rules []Rule) []Discount` (active rules only; item-bound first, then whole-order; each by ascending `ID`)
- `PairDiscount`, `MemberDiscount`, `DefaultDiscounts` stay until Task 2.

- [ ] **Step 1: Write the failing tests** in `rule_test.go` (package `pricing_test`, reuse `testMenu()` and `mustCalculate`). Add helper `seedRules() []pricing.Rule`: IDs 1–3 `{Name:"pairs", ItemCode: ORANGE|PINK|GREEN, GroupSize:2, Percent:5, Active:true}`, ID 4 `{Name:"Member 10%", Percent:10, MemberOnly:true, Active:true}`.
  - `TestRulesReproduceTheStoreBehavior`: the golden table (Red+Green 9000 / member 8100; Orange×5 57600 / member 51840; Green×2+Pink×3 30800; empty 0) through `NewCalculator(pricing.Discounts(seedRules())...)`, plus `ORANGE 5` member asserting discounts `[2400 "Orange set pairs ×2 (5%)", 5760 "Member 10%"]`.
  - `TestRuleGroupSize`: rule `{Name:"triple", ItemCode:"ORANGE", GroupSize:3, Percent:10, Active:true}`, `ORANGE 7` → one discount `7200`, label `"Orange set triple ×2 (10%)"`; `ORANGE 2` → none.
  - `TestRuleMemberOnlyAppliesToMembers`: an item-bound rule with `MemberOnly:true` gives no discount for a non-member and the same discount as without the flag for a member.
  - `TestFullDiscountLeavesNothingForMember`: item rule `Percent:100` on `RED 1`, plus the member rule → `Total == 0`, exactly one discount.
  - `TestDiscountsOrderingAndActive`: rules passed as `[member(ID 1), orange(ID 3), pink(ID 2, Active:false)]` → `Discounts` yields orange then member (length 2).
  - `TestRuleValidate` table → `ErrInvalidRule`: empty name; percent 0; percent 101; item rule with `GroupSize` 0; whole-order rule with `GroupSize` 3; `GroupSize` above `MaxQuantity`. Valid seed rules → nil.
  - `TestItemValidate` table → `ErrInvalidItem`: code `""`, `"red"`, 21-char code, `"A-B"`; empty name; 61-char name; price 0. Valid `{Code:"BLACK_2", Name:"Black set", Price:4500}` → nil.

- [ ] **Step 2: Run to verify failure**

Run: `cd backend && go test ./internal/pricing -run 'TestRule|TestFullDiscount|TestDiscountsOrdering|TestItemValidate' -v`
Expected: FAIL (compile error: `pricing.Rule` undefined)

- [ ] **Step 3: Implement** `Rule`, `Rule.Validate`, `Rule.Apply`, `Discounts`, `Item.Validate`, the two sentinels, and `Item.Active`. `Apply` returns false when `!Active`, when `MemberOnly && !member`, when an item-bound rule finds no complete group, or when the whole-order amount is zero. Item-bound amount is `percentOf(price × groups × GroupSize, Percent)`. Use `sort.SliceStable` by `ID` inside `Discounts`; check the code pattern with a package-level `regexp`.

- [ ] **Step 4: Run to verify pass**

Run: `cd backend && go test ./internal/pricing -v && go vet ./... && gofmt -l .`
Expected: PASS, no vet output, `gofmt -l` prints nothing.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/pricing && git commit -m "feat(pricing): data-driven discount rules and item validation"
```

---

### Task 2: Load items and rules from SQLite for calculation

**Files:**
- Create: `backend/internal/rules/rules.go`
- Modify: `backend/internal/menu/menu.go`
- Modify: `backend/internal/httpapi/httpapi.go`, `backend/internal/httpapi/httpapi_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/pricing/discount.go`, `backend/internal/pricing/discount_test.go`, `backend/internal/pricing/pricing_test.go` (delete the hard-coded types and their tests)

**Interfaces:**
- Consumes: `pricing.Rule`, `pricing.Discounts`, `pricing.Item` from Task 1.
- Produces:
  - `menu.Repository`: `ListActive(ctx) ([]pricing.Item, error)`, `ListAll(ctx) ([]pricing.Item, error)`; `Migrate` adds `active INTEGER NOT NULL DEFAULT 1` and seeds the 7 items only when the table is empty. `List` is removed.
  - `rules.Repository` (package `rules`): `NewRepository(db *sql.DB) *Repository`, `Migrate(ctx) error` (creates `discount_rules`, seeds the four `seedRules()` rows only when empty), `ListActive(ctx) ([]pricing.Rule, error)`, `ListAll(ctx) ([]pricing.Rule, error)`.
  - `httpapi.New(menu MenuStore, rules RuleStore) *fiber.App` where `MenuStore` has `ListActive`, `ListAll` and `RuleStore` has `ListActive`, `ListAll` (Tasks 3–4 extend both interfaces). `calculate` builds `NewCalculator(pricing.Discounts(rules)...)` per request from active rules and active items. `GET /api/menu` returns active items only.

- [ ] **Step 1: Write the failing test** in `httpapi_test.go`: change `newTestApp` to build both repositories on the in-memory DB, run both `Migrate`s, and call `httpapi.New(menuRepo, rulesRepo)`. The existing `TestListMenu`, `TestCalculate`, `TestCalculateOrangeMember` (discounts `[2400, 5760]`), empty-order, and bad-input tests are the acceptance for "seeded rules behave exactly as before". Add `TestMigrateIsIdempotent`: call both `Migrate`s a second time, then `POST` Orange ×5 member and assert the same `51840` and exactly two discounts (no duplicated seed rows).

- [ ] **Step 2: Run to verify failure**

Run: `cd backend && go test ./internal/httpapi -v`
Expected: FAIL (compile error: wrong `New` arguments / `rules` package missing)

- [ ] **Step 3: Implement** the `rules` package, the `menu` changes, the new `httpapi.New`, and update `main.go` (run both `Migrate`s). Then delete `PairDiscount`, `MemberDiscount`, `DefaultDiscounts` and every test that references them; Task 1's `TestRulesReproduceTheStoreBehavior` and friends already cover those cases. Rule `ItemCode` empty is stored as `''`, never NULL.

- [ ] **Step 4: Run to verify pass**

Run: `rm -f backend/freshket.db && cd backend && go test ./... && go vet ./... && gofmt -l .`
Expected: PASS everywhere, `gofmt -l` empty.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "refactor: load menu items and discount rules from SQLite"
```

---

### Task 3: Admin menu API

**Files:**
- Modify: `backend/internal/httpapi/httpapi.go`, `backend/internal/httpapi/httpapi_test.go`
- Modify: `backend/internal/menu/menu.go`

**Interfaces:**
- Consumes: `Item.Validate`, `ErrInvalidItem` from Task 1; `MenuStore` from Task 2.
- Produces:
  - `menu.Repository.Create(ctx, it pricing.Item) error` (→ `menu.ErrDuplicate` on existing code), `Update(ctx, it pricing.Item) error` (by `Code`; → `menu.ErrNotFound`). Add both to `MenuStore`.
  - `GET /api/admin/menu` → `[{code,name,price,active}]` (all items). `POST /api/admin/menu` body `{code,name,price,active}` → 201 with the item. `PUT /api/admin/menu/:code` body `{name,price,active}` → 200 with the item.

- [ ] **Step 1: Write the failing tests** (reuse `postCalculate`; add helper `adminRequest(t, app, method, path, body) (*http.Response, string)` that sends JSON):
  - `TestAdminAddedItemCanBeOrderedWithoutRestart` (the feature's acceptance test): `POST /api/admin/menu` `{"code":"BLACK","name":"Black set","price":4500,"active":true}` → 201; `GET /api/menu` now has 8 items; `POST /api/orders/calculate` `BLACK 2` → subtotal `9000`.
  - `TestAdminEditPrice`: `PUT /api/admin/menu/RED` price `6000` → calculate `RED 1` → `6000`.
  - `TestDeactivatedItemCannotBeOrdered`: `PUT /api/admin/menu/RED` `active:false` → `GET /api/menu` omits RED, `GET /api/admin/menu` still lists it with `active:false`, calculate `RED 1` → 400.
  - `TestAdminCreateItemRejectsBadInput` table: duplicate `RED` → 409; codes `""`, `"red"`, 21 chars → 400; price `0` → 400; empty name → 400; non-JSON body → 400; `"price":"45"` (string) → 400. `PUT` on unknown code → 404.

- [ ] **Step 2: Run to verify failure**

Run: `cd backend && go test ./internal/httpapi -run 'TestAdmin|TestDeactivated' -v`
Expected: FAIL (404 on admin routes / compile error on new repo methods)

- [ ] **Step 3: Implement** the repository methods and handlers. Map `ErrInvalidItem` → 400, `menu.ErrDuplicate` → 409, `menu.ErrNotFound` → 404. The `PUT` handler builds the item from the path code plus body, then runs `Validate`.

- [ ] **Step 4: Run to verify pass**

Run: `cd backend && go test ./... && go vet ./... && gofmt -l .`
Expected: PASS, `gofmt -l` empty.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(api): admin endpoints to add and edit menu items"
```

---

### Task 4: Admin discount-rule API

**Files:**
- Modify: `backend/internal/rules/rules.go`
- Modify: `backend/internal/httpapi/httpapi.go`, `backend/internal/httpapi/httpapi_test.go`

**Interfaces:**
- Consumes: `Rule.Validate`, `ErrInvalidRule` from Task 1; `RuleStore` from Task 2, `adminRequest` from Task 3.
- Produces:
  - `rules.Repository.Create(ctx, r pricing.Rule) (pricing.Rule, error)` (assigns `ID`), `Update(ctx, r pricing.Rule) error` (→ `rules.ErrNotFound`). Add both to `RuleStore`.
  - `GET /api/admin/rules` → `[{id,name,itemCode,groupSize,percent,memberOnly,active}]` (all rules, ascending id). `POST /api/admin/rules` same body without `id` → 201 with the rule. `PUT /api/admin/rules/:id` → 200 with the rule.
  - A non-empty `itemCode` must name an existing item (active or not), otherwise 400.

- [ ] **Step 1: Write the failing tests**:
  - `TestAdminAddedRuleChangesPricingWithoutRestart` (acceptance): `POST /api/admin/rules` `{"name":"triple","itemCode":"BLUE","groupSize":3,"percent":10,"memberOnly":false,"active":true}` → 201 with a non-zero `id`; calculate `BLUE 3` → one discount `900` labelled `"Blue set triple ×1 (10%)"`, total `8100`.
  - `TestAdminDeactivatedRuleStopsApplying`: `PUT` the seeded member rule (`id` from `GET /api/admin/rules`) with `active:false` → calculate `RED 1` member → no discounts.
  - `TestAdminRuleWithoutItemAppliesToWholeOrder`: new rule `{"name":"Welcome 5%","itemCode":"","groupSize":0,"percent":5,"memberOnly":false,"active":true}` → calculate `RED 1` non-member → discount `250`.
  - `TestRuleForDeactivatedItemDoesNotBreakOtherOrders`: create a rule on `BLUE`, deactivate `BLUE`; calculate `RED 1` → 200 with the unchanged price; calculate `BLUE 3` → 400.
  - `TestAdminCreateRuleRejectsBadInput` table → 400: percent `0` and `101`; item rule with `groupSize` 0; whole-order rule with `groupSize` 3; empty name; `itemCode` `"NOPE"`; `"percent":"ten"` (string) → 400. `PUT /api/admin/rules/999` → 404; `PUT /api/admin/rules/abc` → 400.

- [ ] **Step 2: Run to verify failure**

Run: `cd backend && go test ./internal/httpapi -run 'TestAdmin.*Rule|TestRuleFor' -v`
Expected: FAIL (404 on rule routes)

- [ ] **Step 3: Implement** repository methods and handlers; rule `PUT` takes `id` from the path and replaces the whole rule, running `Validate` and the item-exists check like `POST`.

- [ ] **Step 4: Run to verify pass**

Run: `cd backend && go test ./... && go vet ./... && gofmt -l .`
Expected: PASS, `gofmt -l` empty.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(api): admin endpoints to add and edit discount rules"
```

---

### Task 5: Admin page in React

**Files:**
- Create: `frontend/src/Admin.tsx`, `frontend/src/Admin.test.tsx`, `frontend/src/Root.tsx`
- Modify: `frontend/src/api.ts`, `frontend/src/main.tsx`

**Interfaces:**
- Consumes: Task 3–4 endpoints (`/api/admin/menu`, `/api/admin/rules`); `formatTHB` from `api.ts`.
- Produces in `api.ts`: types `AdminItem` (`MenuItem & { active: boolean }`) and `Rule` (`{ id, name, itemCode, groupSize, percent, memberOnly, active }`); `fetchAdminMenu()`, `createItem(item: AdminItem)`, `updateItem(item: AdminItem)`, `fetchRules()`, `createRule(rule: Omit<Rule,'id'>)`, `updateRule(rule: Rule)`. All reject with the response text on non-2xx.
- `Root.tsx` renders `<Admin />` when `location.hash === '#/admin'`, otherwise `<App />`, and re-renders on `hashchange`. `main.tsx` renders `<Root />`.

- [ ] **Step 1: Write the failing component test** in `Admin.test.tsx` (mock `./api` like `App.test.tsx` does):
  - `lets an admin add a menu item`: on mount `fetchAdminMenu` and `fetchRules` are called and their items render. Fill "Code" `BLACK`, "Name" `Black set`, "Price (THB)" `45.50`, submit "Add item" → `createItem` is called with `{ code:'BLACK', name:'Black set', price:4550, active:true }` and the new row appears.
  - `lets an admin add a discount rule`: choose item `RED` from the "Item" select, "Group size" `3`, "Percent" `10`, "Name" `triple`, submit "Add rule" → `createRule` is called with `{ name:'triple', itemCode:'RED', groupSize:3, percent:10, memberOnly:false, active:true }`.
  - `shows the server error`: `createItem` rejects with `"duplicate item code"` → that text is visible.

- [ ] **Step 2: Run to verify failure**

Run: `cd frontend && npm test`
Expected: FAIL (cannot resolve `./Admin`)

- [ ] **Step 3: Implement** `Admin.tsx` (loads both lists on mount; one table + one add form each for items and rules; per-row "Active" checkbox calls the matching update; selecting no item in the rule form means a whole-order rule and sends `groupSize: 0`; price input in baht converted to satang with `Math.round(parseFloat(v) * 100)`), the `api.ts` functions, `Root.tsx`, and the `main.tsx` change. Tailwind utility classes only.

- [ ] **Step 4: Run to verify pass**

Run: `cd frontend && npm test && npm run lint && npm run build`
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(ui): admin page for menu items and discount rules"
```

---

### Task 6: Docs, spec amendment, end-to-end check

**Files:**
- Modify: `docs/SPEC.md`, `README.md`, `AGENTS.md`

**Interfaces:**
- Consumes: everything above. Produces: nothing for later tasks.

- [ ] **Step 1: Amend `docs/SPEC.md`**: remove "Editing the menu through the UI or API" and "configuration-driven rule engine" from Out of Scope; add user stories for admin add/deactivate items and rules; replace the fixed-rule Implementation Decisions with the data-driven rule model from Global Constraints above (authentication stays out of scope); keep "no rule engine beyond item + group size + percent + member flag" as the new Out of Scope line.

- [ ] **Step 2: Update `README.md`**: the `#/admin` page; admin endpoints with one `curl` example for adding an item and a rule; one line that admin routes are unauthenticated because this is a simulation; the "delete `freshket.db` after pulling this change" note; replace "how to add a new promotion" with the admin flow; record the assumption that rule `Percent` is a whole number and labels follow the Global Constraints format.

- [ ] **Step 3: Update `AGENTS.md`**: replace the hard-coded "Domain rules" and discount-interface bullets with the rule model; new packages (`internal/rules`); new golden/acceptance tests; one line that admin routes are intentionally unauthenticated.

- [ ] **Step 4: Run the full verification set**

Run: `cd backend && go test ./... && go vet ./... && gofmt -l . ; cd ../frontend && npm test && npm run lint && npm run build`
Expected: all pass, `gofmt -l` empty.

- [ ] **Step 5: End-to-end smoke in the browser**

Run: `rm -f backend/freshket.db`, start `backend` and `frontend` via `.claude/launch.json`, open `/#/admin`, add item `BLACK` at 45.00, add rule `triple` on `BLACK` ×3 for 10%, open `/`, order 3 × Black.
Expected: no restart needed; page shows subtotal ฿135.00, `Black set triple ×1 (10%)` −฿13.50, total ฿121.50.

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "docs: document admin-managed menu and discount rules"
```
