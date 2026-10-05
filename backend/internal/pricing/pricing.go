// Package pricing calculates the price of a food store order.
// It is pure domain logic: no I/O, no HTTP, no database.
package pricing

import "errors"

// Money is an amount in satang (1 THB = 100 satang).
type Money int64

// Item is a menu entry.
type Item struct {
	Code  string
	Name  string
	Price Money
}

// Menu maps an item code to its menu entry.
type Menu map[string]Item

// Line is one ordered item and its quantity.
type Line struct {
	Code string
	Qty  int
}

// Order is what the customer asks to be priced.
type Order struct {
	Lines  []Line
	Member bool
}

// PricedLine is a Line resolved against the Menu.
type PricedLine struct {
	Item Item
	Qty  int
}

// AppliedDiscount is one discount that reduced the total.
type AppliedDiscount struct {
	Label  string
	Amount Money
}

// Breakdown is the result of a calculation.
type Breakdown struct {
	Subtotal  Money
	Discounts []AppliedDiscount
	Total     Money
}

// Discount is one promotion. Apply returns the discount for the order given
// the running total after earlier discounts, and false when it does not apply.
type Discount interface {
	Apply(lines []PricedLine, member bool, runningTotal Money) (AppliedDiscount, bool)
}

var (
	// ErrUnknownItem is returned when an order line references a code not on the menu.
	ErrUnknownItem = errors.New("unknown item")
	// ErrInvalidQuantity is returned when an order line has a negative quantity.
	ErrInvalidQuantity = errors.New("invalid quantity")

	errNotImplemented = errors.New("pricing: not implemented")
)

// Calculator applies its discounts, in order, to an order's subtotal.
type Calculator struct {
	discounts []Discount
}

// NewCalculator returns a Calculator that applies discounts in the given order.
func NewCalculator(discounts ...Discount) *Calculator {
	return &Calculator{discounts: discounts}
}

// DefaultDiscounts returns the store's promotions in the order they apply:
// 5% off each same-item pair of Orange, Pink or Green, then 10% member discount.
func DefaultDiscounts() []Discount {
	// TODO: PairDiscount, MemberDiscount
	return nil
}

// Calculate prices the order against the menu.
func (c *Calculator) Calculate(menu Menu, order Order) (Breakdown, error) {
	// TODO: resolve lines, subtotal, apply c.discounts with half-up rounding
	return Breakdown{}, errNotImplemented
}
