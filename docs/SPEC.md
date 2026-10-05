# Spec: Food Store Price Calculator

Status: ready-for-agent

## Problem Statement

A customer or cashier at a small food store with a fixed 7-item menu needs to know what an order costs. The price is not a plain sum: members get a discount, and ordering the same bundle-eligible item twice earns a bundle discount. Working this out by hand is slow and error-prone, and the store wants to add promotions later without rewriting the pricing rules.

## Solution

A small web app. The customer picks quantities from the menu, toggles whether they hold a member card, and immediately sees a price breakdown: subtotal, each discount that applied (with a label and amount), and the final total. The backend owns every pricing rule, so the screen only displays what the server returns. Promotions are independent, ordered rules, so a new promotion can be added without touching existing ones.

## User Stories

1. As a customer, I want to see all 7 menu items with their prices, so that I know what I can order.
2. As a customer, I want to add one more of an item with a single click, so that building an order is quick.
3. As a customer, I want to remove one of an item with a single click, so that I can correct mistakes.
4. As a customer, I want the quantity of an item to never go below zero, so that I can't create an invalid order by accident.
5. As a customer, I want to order several different items in one bill, so that I pay once for my whole meal.
6. As a customer, I want the price to update as soon as I change an item or the member toggle, so that I don't have to press a calculate button.
7. As a customer, I want to see the subtotal before discounts, so that I understand the undiscounted cost.
8. As a customer, I want each applied discount listed with a name and amount, so that I can see why the price dropped.
9. As a customer, I want to see the final total clearly separated from the line items, so that I know what I'll pay.
10. As a member, I want to tick a member-card option, so that the 10% member discount is applied.
11. As a member, I want the 10% to apply to the total after bundle discounts, so that the rules are predictable and consistent with the store's policy.
12. As a non-member, I want no member discount to appear, so that the price is not misrepresented.
13. As a customer ordering two Orange sets, I want a 5% discount on that pair, so that I benefit from buying a pair.
14. As a customer ordering two Pink sets, I want a 5% discount on that pair, so that the Pink promotion works like Orange.
15. As a customer ordering two Green sets, I want a 5% discount on that pair, so that the Green promotion works like Orange.
16. As a customer ordering five Orange sets, I want two pairs (four sets) discounted and the fifth at full price, so that odd leftovers are handled correctly.
17. As a customer ordering one Orange and one Pink, I want no pair discount, so that only identical-item pairs qualify.
18. As a customer ordering Red, Blue, Yellow, or Purple pairs, I want no pair discount, so that only the promoted items qualify.
19. As a customer, I want a pair discount line per eligible item, so that I can see which item earned it.
20. As a customer, I want an empty order to show a total of 0.00, so that the screen is never in a confusing state.
21. As a customer, I want amounts shown in baht with two decimals, so that prices read naturally.
22. As a customer, I want a clear error message if something goes wrong, so that I know to retry rather than trust a stale price.
23. As a store owner, I want pricing computed only on the server, so that the price I charge can't be altered from the browser.
24. As a store owner, I want money stored and calculated in whole satang, so that rounding errors never accumulate.
25. As a store owner, I want a defined rounding rule (half-up) for fractional satang, so that results are deterministic and auditable.
26. As a store owner, I want the menu stored in a database, so that item prices can change without a code release.
27. As a store owner, I want the menu seeded automatically on first run, so that a fresh install works immediately.
28. As a store owner, I want unknown item codes rejected, so that bad requests never produce a price.
29. As a store owner, I want negative quantities rejected, so that nobody can reduce a total with a negative line.
30. As a store owner, I want to add a new promotion by writing one new rule, so that existing promotions are not at risk when I do.
31. As a store owner, I want promotions applied in a fixed, documented order, so that stacking discounts is never ambiguous.
32. As a developer, I want the pricing logic free of HTTP and database code, so that I can test it in isolation.
33. As a developer, I want the calculator to take the menu as input, so that tests need no database.
34. As a developer, I want a table of golden cases in the tests, so that a rule change that alters a known price fails loudly.
35. As a reviewer, I want a README listing assumptions, so that I can judge the ambiguous parts of the brief against the author's reasoning.
36. As a reviewer, I want setup to be two commands per side, so that I can run the project quickly.

## Implementation Decisions

- **Modules.** A pricing module (pure domain, no I/O), a menu module (SQLite repository and seed data), an HTTP API module (Fiber handlers and request/response shapes), and a React frontend. Dependencies point inward: the API depends on pricing and menu; pricing depends on nothing.
- **Money.** All amounts are integer satang (1 THB = 100 satang) in the domain, the database, and the API. Fractional satang round half-up at each discount step. The frontend only formats satang as baht.
- **Pricing pipeline.** Subtotal is the sum of price × quantity across order lines. Discounts then apply in a fixed order: bundle discounts first, member discount second. Each discount sees the running total left by the previous one.
- **Discount abstraction.** Every promotion is an independent rule behind one small interface that, given the priced order lines, the member flag, and the running total, returns an applied discount (label, amount) or reports that it does not apply. The calculator takes an ordered list of rules. This is the extensibility seam; no registry or configuration-driven rule engine.
- **Bundle (pair) rule.** Applies only to Orange, Pink, and Green. For each eligible item, pairs = quantity ÷ 2 (integer division). The discount is 5% of the price of the paired sets (2 × price × pairs). An odd leftover set pays full price. Pairs must be the same item; mixed items do not pair. One applied discount per eligible item, labelled with the item.
- **Member rule.** 10% of the running total after bundle discounts, only when the member flag is set.
- **Result shape.** The calculator returns a breakdown: subtotal, list of applied discounts (label, amount), and total. It never returns just a number.
- **Validation.** Order lines with an unknown item code or a negative quantity are rejected with typed errors. Zero quantity lines are allowed and ignored. An empty order totals zero.
- **Menu data.** Seven items (Red 50, Green 40, Blue 30, Yellow 50, Pink 80, Purple 90, Orange 120 THB per set), stored in SQLite with code, name, and price in satang. Schema creation and seeding are idempotent and run at startup.
- **API contract.** `GET /api/menu` returns the list of items with code, name, price. `POST /api/orders/calculate` accepts the items (code, quantity) and the member flag, and returns subtotal, discounts, and total. Validation errors return HTTP 400; unexpected errors return 500.
- **Frontend.** One screen: menu list with plus/minus controls, a member checkbox, and the breakdown panel. It recalculates through the API whenever the quantities or member flag change, ignores out-of-order responses, and shows API errors inline. It contains no pricing logic.
- **Configuration.** Server port and database path come from environment variables with sensible defaults. The dev proxy target for the frontend is configurable.

## Testing Decisions

- **What makes a good test.** Assert external behavior only: given a menu and an order, the breakdown and total are X; given a request, the response is Y. No assertions on internal structure, rule ordering internals, or private helpers.
- **Seams (proposed, to be confirmed).** The primary seam is the calculator's `Calculate` operation, which holds nearly all the logic and needs no I/O. One thin secondary seam is the HTTP boundary, driven in-process against an in-memory SQLite database, to prove wiring, validation-to-status mapping, and the contract. The frontend gets one component test with the API module mocked. No other seams.
- **Pricing tests** are table-driven and cover the golden cases: Red+Green = 90.00; Red+Green member = 81.00; Orange ×5 = 576.00; Orange ×5 member = 518.40; Green ×2 + Pink ×3 = 308.00; empty order = 0.00. Add edges: odd quantities, mixed eligible items, non-eligible pairs, zero quantities, half-up rounding, unknown code, negative quantity.
- **API tests** cover the menu listing (7 items), a successful calculation, and a 400 for an unknown item.
- **Frontend test** covers selecting an item and toggling member, then asserting that the request sent to the API and the rendered breakdown match.
- **Prior art.** The existing scaffold already contains the pricing table test, the in-process API tests, and the frontend component test; extend those rather than introduce new styles.

## Out of Scope

- Authentication, real member-card lookup or validation (the member flag is a user-supplied boolean).
- Persisting orders, payment, receipts, or order history.
- Editing the menu through the UI or API.
- Promotions other than the two specified, and a configuration-driven rule engine.
- Mobile app, internationalization, and multi-currency support.
- Deployment, CI, and containerization.

## Further Notes

- The brief is ambiguous in three places; the decisions above resolve them and the README must record them under Assumptions: (1) "order doubles" means two of the same item, (2) the member discount applies after bundle discounts, (3) rounding is half-up in satang.
- The scaffold already exists with a stubbed calculator, so the red tests currently fail by design. The next step is implementing the two rules and the calculation so that they pass.
- Scoring criteria from the brief are readability, maintainability, extensibility, and logic, with extra credit for unit tests; keep the solution small enough to finish within the 60–120 minute budget.
