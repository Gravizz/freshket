# Bundle Promotions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** An admin can define a promotion as a set bundle of several items with quantities (for example "Bundle A: Green ×2 + Red ×1 gets 12% off") and the calculator discounts every complete bundle in an order.

**Architecture:** `pricing.Rule` swaps its single `ItemCode` + `GroupSize` condition for a `Bundle []Component`. A one-component bundle is exactly the old "every N sets of one item" rule, so the seed pair rules become one-component bundles and nothing else in the pipeline changes. An empty bundle is still a whole-order rule. Because bundles can now share an item, a bundle claims the sets it discounts and later discounts only see the unclaimed sets; bigger bundles claim first.

**Tech Stack:** Go 1.26 + Fiber v3 + `modernc.org/sqlite`; React + TypeScript + Tailwind v4 + Vitest.

**Spec:** `docs/SPEC.md` (amended by Task 4). This plan lifts one Out of Scope item ("combos of different items") and replaces the `itemCode`/`groupSize` rule fields. Authentication stays out of scope.

## Global Constraints

- Money is integer satang (`pricing.Money`, `int64`); fractional satang round half-up at each discount step via `percentOf`.
- A condition is only: a bundle of components (`{ItemCode, Qty}`, no duplicate item codes), plus an optional members-only flag. An effect is only a whole-number percent, 1–100. No other condition or effect types, no expression language.
- Complete bundles = the minimum over components of `floor(unclaimed qty ÷ component qty)`. Each complete bundle gets `Percent` off the price of its sets; leftover sets pay full price.
- Discount order: bundle rules first, **larger bundles first** (bigger sum of component quantities), ties by ascending rule ID; then whole-order rules by ascending ID. Each rule sees the running total left by the previous one. `Active` and `MemberOnly` gate every rule.
- A set belongs to at most one bundle: after a bundle rule applies, its sets are removed from what later rules see.
- Labels: bundle rule `"<Rule.Name> ×<bundles> (<Percent>%)"`; whole-order rule = `Rule.Name` verbatim.
- Item code 1–20 chars of `A-Z`, `0-9`, `_`; component `Qty` 1..`MaxQuantity` (10,000); rule name 1–60 chars.
- `Discount.Apply(lines, member, runningTotal) (AppliedDiscount, bool)` keeps its signature; `AppliedDiscount` stays comparable with `==`.
- Items and rules are deactivated, never deleted. Invalid rule → 400; unknown item in a bundle → 400; missing rule on update → 404.
- `internal/pricing` imports nothing from Fiber, SQLite or `net/http`; the frontend never recomputes discounts.
- The schema changes (`discount_rules` loses `item_code` and `group_size`; new `rule_items` table): developers delete `backend/freshket.db` once. No in-place migration.
- Verification after every task that touches a side: `go test ./...`, `go vet ./...`, `gofmt -l .` (empty) in `backend`; `npm test`, `npm run lint`, `npm run build` in `frontend`.

## Review Focus

1. Two rules want the same set (Bundle A = Green ×2 + Red ×1 and the seed Green pair): no set is discounted twice, and Bundle A wins the Greens because it is bigger. Pinned in Task 1.
2. Unbalanced orders (Green ×5, Red ×2; Green ×4, Red ×1; Green ×2 only): only complete bundles are discounted, leftovers pay full price, a missing component means no discount. Pinned in Task 1.
3. Editing a bundle with `PUT` replaces its components instead of appending to them. Pinned in Task 2.
4. A rule created with `bundle` omitted or `null` is a whole-order rule and is returned with `"bundle":[]`, never `null`. Pinned in Task 2.
5. A bundle listing one item twice, with quantity 0, or with an unknown item code is rejected with 400 and nothing is stored. Pinned in Task 1 (domain) and Task 2 (API).

---

### Task 1: Bundle rules in the pricing domain

**Files:**
- Modify: `backend/internal/pricing/rule.go` (Rule, Apply, Discounts, Validate)
- Modify: `backend/internal/pricing/pricing.go` (Claimer interface, Calculate loop)
- Modify: `backend/internal/pricing/rule_test.go`, `backend/internal/pricing/rule_edges_test.go` (new rule shape, new labels)
- Create: `backend/internal/pricing/bundle_test.go`

**Interfaces:**
- Consumes: existing `PricedLine`, `AppliedDiscount`, `Discount`, `percentOf`, `itemCodePattern`, `MaxQuantity`, `ErrInvalidRule`.
- Produces (later tasks rely on these exact names):
  - `type Component struct { ItemCode string; Qty int }`
  - `type Rule struct { ID int64; Name string; Bundle []Component; Percent int; MemberOnly bool; Active bool }`: `ItemCode` and `GroupSize` are removed.
  - `func (r Rule) Claim(lines []PricedLine) []PricedLine`
  - `type Claimer interface { Claim(lines []PricedLine) []PricedLine }` in `pricing.go`
  - `Rule.Apply`, `Discounts`, `Rule.Validate` keep their signatures.

Other packages (`rules`, `httpapi`) stop compiling when `ItemCode`/`GroupSize` go away. Run only `go test ./internal/pricing/` in this task; Task 2 restores the full build.

- [ ] **Step 1: Move the existing pricing tests to the bundle shape**

In `rule_test.go` and `rule_edges_test.go` replace every `ItemCode: X, GroupSize: N` with `Bundle: []pricing.Component{{ItemCode: X, Qty: N}}`. `seedRules()` names become `"Orange pairs"`, `"Pink pairs"`, `"Green pairs"`. Inline rules named `"pairs"` (ORANGE, PINK) are renamed `"Orange pairs"` / `"Pink pairs"`. Update the expected labels: `"Orange pairs ×2 (5%)"`, `"Orange pairs ×1 (5%)"`; the rule named `"early"` over RED ×1 becomes label `"early ×1 (20%)"`; the `"triple"` rule over ORANGE ×3 becomes `"triple ×2 (10%)"`. In `TestDiscountsOrderRulesByID` both rules are one-component bundles of equal size, so ID order still decides.

- [ ] **Step 2: Write the failing bundle tests in `bundle_test.go`**

Package `pricing_test`, reusing `testMenu()` and `mustCalculate`. Define `bundleA := pricing.Rule{ID: 5, Name: "Bundle A", Bundle: []pricing.Component{{"GREEN", 2}, {"RED", 1}}, Percent: 12, Active: true}`. Table test `TestBundleDiscount` with calc built from `pricing.Discounts([]pricing.Rule{bundleA})`, asserting discount amount (or none) and total:

| order | amount | total |
|---|---|---|
| Green 2, Red 1 | 1560 | 11440 |
| Green 4, Red 2 | 3120 | 22880 |
| Green 5, Red 2 (leftover Green) | 3120 | 26880 |
| Green 4, Red 1 (Red limits to one bundle) | 1560 | 19440 |
| Green 2 only (component missing) | none | 8000 |
| Green 1, Red 1 (Green short) | none | 9000 |

One more subtest asserts the label of the first row is exactly `"Bundle A ×1 (12%)"`.

Further tests:
- `TestBundleClaimsItsSets`: rules `[bundleA, pairs GREEN×2 5% (ID 3)]`, order Green 2 + Red 1 → exactly one discount (`Bundle A ×1 (12%)`), total 11440; order Green 4 + Red 1 → discounts `[Bundle A ×1 (12%) 1560, Green pairs ×1 (5%) 400]`, total 19040.
- `TestLargerBundlesClaimFirstThenLowerID`: rule 9 `{RED,GREEN,BLUE}×1` 10% vs rule 2 `{RED,GREEN}×1` 10%, order Red, Green, Blue → one discount from rule 9, amount 1200, total 10800. Then two equal-size bundles (IDs 5 and 2) both needing the only Red → the ID 2 rule applies, amount 900, total 11100.
- `TestBundleRulesApplyBeforeWholeOrderRules`: `[member 10% ID 1, bundleA ID 5]`, member order Green 2 + Red 1 → discounts `[Bundle A ×1 (12%) 1560, Member 10% 1144]`, total 10296.
- `TestMemberOnlyBundle`: bundle with `MemberOnly` gives no discount to a guest, discounts a member.
- `TestBundleRoundsHalfUp`: menu item `X` at 1010 satang, bundle `{X,1}` 5% → amount 51.
- `TestRuleValidateBundle`: table of `Rule.Validate()` expecting `ErrInvalidRule` (use `errors.Is`) for: empty name, percent 0, percent 101, component qty 0, component qty 10001, item code `"red"`, empty item code, same item listed twice; and `nil` for: a valid two-item bundle, an empty bundle (whole-order rule).

- [ ] **Step 3: Run to confirm failure**

Run: `cd backend && go test ./internal/pricing/ 2>&1 | head -20`
Expected: build FAIL, `Component` / `Bundle` undefined.

- [ ] **Step 4: Implement the domain change**

In `rule.go`:
- Add `Component` and the new `Rule` struct with doc comments (an empty `Bundle` means whole-order rule).
- `func (r Rule) bundles(lines []PricedLine) int`: minimum over components of `unclaimed qty ÷ component qty`, looked up by item code in `lines`; 0 when the bundle is empty or a component's item is missing.
- `Apply`: member gate as before; empty bundle → whole-order branch unchanged; otherwise `n := r.bundles(lines)`, `n == 0` → not applied, else amount = `percentOf` of the sum of `item price × component qty × n` over components (prices come from `lines`), label `fmt.Sprintf("%s ×%d (%d%%)", r.Name, n, r.Percent)`.
- `Claim`: whole-order rule → return `lines` unchanged; bundle rule → a **copy** of `lines` with each component's quantity reduced by `component qty × bundles(lines)`; never mutate the argument.
- `Discounts`: keep the active filter and `sort.SliceStable`, new comparator: bundle rules before whole-order rules; among bundle rules larger `sum of component Qty` first, then ascending ID; whole-order rules ascending ID. Update its doc comment.
- `Validate`: keep the name and percent checks; drop the item/group-size cases; add per-component checks (`itemCodePattern` match, `Qty` between 1 and `MaxQuantity`) and a duplicate-item-code check, all wrapped in `ErrInvalidRule` with messages naming the problem. Update the doc comment.

In `pricing.go`:
- Add `Claimer` (doc: a Discount that uses up sets so later discounts cannot discount them again).
- In `Calculate`, after `b.Total -= applied.Amount`, add `if cl, ok := d.(Claimer); ok { lines = cl.Claim(lines) }`.

- [ ] **Step 5: Run the pricing tests**

Run: `cd backend && go test ./internal/pricing/ -v 2>&1 | tail -30` then `go vet ./internal/pricing/ && gofmt -l internal/pricing`
Expected: all PASS (including the six golden cases through `TestRulesReproduceTheStoreBehavior`), vet clean, no gofmt output.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/pricing
git commit -m "feat(pricing): bundle rules with set claiming"
```

---

### Task 2: Store and expose bundle rules

**Files:**
- Modify: `backend/internal/rules/rules.go` (schema, seed, list/create/update)
- Modify: `backend/internal/httpapi/admin_rules.go` (DTOs, checkRule)
- Modify: `backend/internal/httpapi/httpapi_test.go`

**Interfaces:**
- Consumes: Task 1's `pricing.Rule{ID, Name, Bundle []pricing.Component, Percent, MemberOnly, Active}` and `Rule.Validate()`.
- Produces: JSON rule shape `{ "id", "name", "bundle": [{ "itemCode", "qty" }], "percent", "memberOnly", "active" }` on `GET/POST /api/admin/rules` and `PUT /api/admin/rules/:id`. `httpapi.RuleStore` and the `rules.Repository` method signatures do not change.

- [ ] **Step 1: Update and add the failing HTTP tests**

In `httpapi_test.go`, rewrite every rule body from `"itemCode":X,"groupSize":N` to `"bundle":[{"itemCode":X,"qty":N}]` and drop those two fields for whole-order bodies (or send `"bundle":[]`). Adjust expectations:
- `TestFreshDatabaseMatchesTheBrief` wants rules exactly:
  `{"id":1,"name":"Orange pairs","bundle":[{"itemCode":"ORANGE","qty":2}],"percent":5,"memberOnly":false,"active":true}`, same for `Pink pairs` (id 2) and `Green pairs` (id 3), then `{"id":4,"name":"Member 10%","bundle":[],"percent":10,"memberOnly":true,"active":true}`.
- `TestAdminAddedRuleChangesPricingWithoutRestart` (BLUE ×3 "triple") now expects label `"triple ×1 (10%)"`.
- The validation table drops "item rule without group size" and "order rule with a group size" and gains rows (all expect 400): bundle component qty 0; qty 10001; the same item twice; unknown item code in the bundle; `bundle` not an array (`"bundle":"x"`); `"percent":"ten"` stays.
- The concurrency test body becomes a bundle body.

New tests:
- `TestAdminBundleRuleDiscountsMixedItemsWithoutRestart`: POST `{"name":"Bundle A","bundle":[{"itemCode":"GREEN","qty":2},{"itemCode":"RED","qty":1}],"percent":12,"memberOnly":false,"active":true}` → 201; calculate Green 2 + Red 1 → subtotal 13000, one discount `{"label":"Bundle A ×1 (12%)","amount":1560}`, total 11440 (the seed Green pair rule does not also fire).
- `TestAdminRuleUpdateReplacesBundle`: create a bundle with two components, `PUT` the same id with a single different component, `GET /api/admin/rules` shows exactly that one component for that id.
- `TestAdminRuleWithoutBundleIsWholeOrder`: POST with the `bundle` field omitted → 201 and body contains `"bundle":[]`; again with `"bundle":null`; calculate shows the whole-order discount.
- `TestRejectedBundleStoresNothing`: POST a bundle whose second component is an unknown item → 400, then `GET /api/admin/rules` still lists only the 4 seeded rules.

- [ ] **Step 2: Run to confirm failure**

Run: `cd backend && go test ./internal/httpapi/ 2>&1 | head -20`
Expected: build FAIL (`rules` and `httpapi` still reference removed fields).

- [ ] **Step 3: Implement the repository in `rules.go`**

- Schema: `discount_rules (id, name, percent, member_only, active)` (drop `item_code`, `group_size`) plus `rule_items (rule_id INTEGER NOT NULL REFERENCES discount_rules(id), item_code TEXT NOT NULL, qty INTEGER NOT NULL, PRIMARY KEY (rule_id, item_code))`; both created in `Migrate`.
- Seed: the three pair rules become one-component bundles named `Orange pairs`, `Pink pairs`, `Green pairs` (Qty 2, 5%); `Member 10%` unchanged. `Migrate` seeds through `Create` so components are written the same way.
- `list(ctx, where)`: ONE statement, `discount_rules r LEFT JOIN rule_items i ON i.rule_id = r.id`, ordered by `r.id, i.rowid` (insertion order keeps the component order the admin typed), scanning `i.item_code`/`i.qty` into `sql.NullString`/`sql.NullInt64` and folding consecutive rows with the same id into one `pricing.Rule`. A rule without components gets `Bundle: nil`. `ListActive` filters `r.active = 1`. A single query keeps rules and components consistent under concurrent writes.
- `Create` and `Update` run in a `db.BeginTx` transaction whose FIRST statement is a write (INSERT / UPDATE), so SQLite takes the write lock up front and the busy timeout applies instead of a read-to-write upgrade failure. `Update` then `DELETE FROM rule_items WHERE rule_id = ?` and re-inserts the new components; zero rows affected by the UPDATE returns `ErrNotFound` and rolls back. Wrap errors with `%w` as the file does today.

- [ ] **Step 4: Implement the DTOs in `admin_rules.go`**

- `type bundleItemDTO struct { ItemCode string \`json:"itemCode"\`; Qty int \`json:"qty"\` }`; `ruleDTO` field order `ID, Name, Bundle []bundleItemDTO, Percent, MemberOnly, Active` (this order fixes the JSON key order the fresh-database test pins).
- `func newRuleDTO(r pricing.Rule) ruleDTO` always returns a non-nil `Bundle` (empty slice), and `listRules`, `createRule` and `updateRule` all respond through it, replacing the hand-copied struct in `listRules`. `func (d ruleDTO) rule() pricing.Rule` maps back.
- `checkRule`: run `Rule.Validate()`, then require every component's item code to exist in `h.menu.ListAll` (inactive items count as existing, as today); failures are 400 and `pricing.ErrInvalidRule`-prefixed like the current message.

- [ ] **Step 5: Run the full backend suite**

Run: `cd backend && go test ./... && go vet ./... && gofmt -l .`
Expected: all PASS (including the concurrent write test and `TestMigrationsAreIdempotent`), vet clean, no gofmt output. Delete any local `backend/freshket.db` before a manual `go run ./cmd/server`.

- [ ] **Step 6: Commit**

```bash
git add backend
git commit -m "feat(rules): store bundle rules and expose them through the admin API"
```

---

### Task 3: Admin page bundle editor

**Files:**
- Modify: `frontend/src/api.ts` (types)
- Modify: `frontend/src/Admin.tsx` (rule form, rule list, preview)
- Modify: `frontend/src/Admin.test.tsx`

**Interfaces:**
- Consumes: Task 2's rule JSON shape.
- Produces: in `api.ts`, `type BundleItem = { itemCode: string; qty: number }` and `type Rule = { id: number; name: string; bundle: BundleItem[]; percent: number; memberOnly: boolean; active: boolean }`; `createRule(rule: Omit<Rule,'id'>)` and `updateRule(rule: Rule)` keep their signatures.

- [ ] **Step 1: Update and add the failing component tests**

In `Admin.test.tsx`: add `{ code: 'GREEN', name: 'Green set', price: 4000, active: true }` to the mocked admin menu; the paused-rule fixture becomes `{ id: 3, name: 'Member', bundle: [], percent: 10, memberOnly: true, active: true }`; the whole-order test expects `bundle: []` and no `itemCode`/`groupSize` keys.

Replace the "add a discount rule for an item" test with `lets an admin build a bundle rule`:
click `Add item to bundle` twice; pick `Green set` in `Bundle item 1` and type `2` in `Bundle quantity 1`; pick `Red set` in `Bundle item 2` and type `1` in `Bundle quantity 2`; type `Bundle A` in `Rule name` and `12` in `Percent`; submit. Expect `api.createRule` called with `{ name: 'Bundle A', bundle: [{ itemCode: 'GREEN', qty: 2 }, { itemCode: 'RED', qty: 1 }], percent: 12, memberOnly: false, active: true }`, and `Bundle A` appears in the list together with the text `2 × Green set + 1 × Red set`.

New tests:
- `previews the bundle before saving`: after filling the two rows and percent 12, the preview contains `2 × Green set + 1 × Red set` and `12% off`.
- `removes a bundle row`: add two rows, click `Remove bundle item 1`, only one `Bundle item` select remains.
- `shows the server error when a rule is rejected`: `createRule` rejects with `new Error('invalid rule: percent must be between 1 and 100')`, the alert shows that text.

- [ ] **Step 2: Run to confirm failure**

Run: `cd frontend && npm test -- Admin 2>&1 | tail -30`
Expected: FAIL (no `Add item to bundle` button, type errors on `bundle`).

- [ ] **Step 3: Implement**

- `api.ts`: add `BundleItem`, change `Rule` as above.
- `Admin.tsx`, following the file's existing pattern of small helper components defined in the same file:
  - State: replace `ruleItem` and `groupSize` with `bundleRows: { itemCode: string; qty: string }[]` (starts empty; empty means whole order).
  - `function BundleEditor({ items, rows, onChange })`: one row per entry with a select (`aria-label` `Bundle item N`, first option `Choose item`), a numeric qty input (`aria-label` `Bundle quantity N`, suffix `sets`), a remove button (`aria-label` `Remove bundle item N`), and an `Add item to bundle` button below the rows. Reuse `fieldClass`.
  - `addRule` sends `bundle: bundleRows.map(r => ({ itemCode: r.itemCode, qty: parseInt(r.qty, 10) }))` and resets the rows on success.
  - `function describeBundle(bundle: BundleItem[], itemName: (code: string) => string): string` returns `"2 × Green set + 1 × Red set"`; the rule list shows it, or `Whole order` when the bundle is empty. `describeRule` for the preview becomes `Every bundle of <describeBundle> gets <pct>% off<who>; sets outside a complete bundle pay full price.` for bundles (use `…` for unfilled values) and keeps the whole-order sentence otherwise.
  - Switch labels become `` `${rule.name} active` `` for every rule; the list-row colour badge uses the first component's `itemColor`, or `--color-fk-600` for whole-order rules; the panel subtitle becomes `Bundles apply first, biggest first, then whole-order rules.`

- [ ] **Step 4: Run all frontend checks**

Run: `cd frontend && npm test && npm run lint && npm run build`
Expected: all pass.

- [ ] **Step 5: Manual smoke check**

Run the backend (`cd backend && rm -f freshket.db && go run ./cmd/server`) and `cd frontend && npm run dev`. On `/#/admin` add Bundle A (Green ×2 + Red ×1, 12%), then on the store page order 2 Green + 1 Red and confirm a `Bundle A ×1 (12%)` line of −฿15.60 and a total of ฿114.40.

- [ ] **Step 6: Commit**

```bash
git add frontend
git commit -m "feat(admin): build bundle promotions from several items"
```

---

### Task 4: Docs and project instructions

**Files:**
- Modify: `README.md`, `docs/ARCHITECTURE.md`, `docs/SPEC.md`, `AGENTS.md`

**Interfaces:** none (text only).

- [ ] **Step 1: Update `AGENTS.md`**

Domain rules 2–4 and the Labels line describe bundle rules (condition: bundle of item × qty components, claiming, larger bundles first then ascending ID, label `"<Rule.Name> ×<bundles> (<Percent>%)"`); the API section shows the `bundle` array instead of `itemCode`/`groupSize`; the seed sentence names the pair rules as one-component bundles. The golden table is unchanged.

- [ ] **Step 2: Update `docs/SPEC.md`**

Add user story 46: "As a store owner, I want to define a bundle of several items with quantities (for example Green ×2 + Red ×1) and a percent off, so that I can promote set combinations." Rewrite the Data-driven rules and Item rule paragraphs for bundles, remove "combos of different items" from Out of Scope, update the API contract and the frontend paragraph (bundle editor), and note the schema change in Seed data.

- [ ] **Step 3: Update `docs/ARCHITECTURE.md`**

Sections to rewrite: the ER diagram and table text (`rule_items`), the seed table, §3.2 request payload, §4.2 discount order (bundle size desc, then ID), §4.3 `Rule.Apply` flowchart (bundles = min over components, claim step), §5 rule model table and class diagram, §4.5 worked examples with the new pair labels. Add a worked example: Green ×4 + Red ×1 with Bundle A and the Green pair rule → 19040.

- [ ] **Step 4: Update `README.md`**

Seed description and the one-time `rm freshket.db` note; the admin rules contract line and the `curl` example (Bundle A, Green ×2 + Red ×1, 12%, ordering 2 Green + 1 Red gives −฿15.60 and ฿114.40); under **Assumptions** replace the pair/rules bullets with: bundle = complete sets of all components (min across components), leftovers full price; a set belongs to one bundle; bigger bundles claim first, ties by rule ID; bundle rules apply before whole-order rules; labels. Update the **Design** bullet on how to add a promotion.

- [ ] **Step 5: Verify nothing stale remains and commit**

Run: `grep -rn "groupSize\|group_size\|GroupSize\|itemCode\":\"\"" README.md docs AGENTS.md backend frontend/src --include='*' | grep -v node_modules | grep -v "docs/superpowers/plans"`
Expected: no hits except historical plan files.

```bash
git add README.md docs AGENTS.md
git commit -m "docs: describe bundle promotions"
```
