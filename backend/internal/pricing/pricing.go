// Package pricing calculates the price of a food store order.
// It is pure domain logic: no I/O, no HTTP, no database.
package pricing

import (
	"errors"
	"fmt"
)

// Money is an amount in satang (1 THB = 100 satang).
type Money int64

// Item is a menu entry.
type Item struct {
	Code   string
	Name   string
	Price  Money
	Active bool
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
// A discount with a Validate() error method is validated before calculation.
type Discount interface {
	Apply(lines []PricedLine, member bool, runningTotal Money) (AppliedDiscount, bool)
}

// Claimer is a Discount that uses up the sets it discounts, so that later
// discounts cannot discount the same sets again. Claim returns the lines
// without those sets and must not modify its argument.
type Claimer interface {
	Claim(lines []PricedLine) []PricedLine
}

var (
	// ErrUnknownItem is returned when an order line references a code not on the menu.
	ErrUnknownItem = errors.New("unknown item")
	// ErrInvalidQuantity is returned when an order line has a negative quantity, or when a code's total exceeds MaxQuantity.
	ErrInvalidQuantity = errors.New("invalid quantity")
)

// Calculator applies its discounts, in order, to an order's subtotal.
type Calculator struct {
	discounts []Discount
}

// NewCalculator returns a Calculator that applies discounts in the given order.
func NewCalculator(discounts ...Discount) *Calculator {
	return &Calculator{discounts: discounts}
}

// MaxQuantity is the most sets of a single item one order may contain.
const MaxQuantity = 10_000

// Calculate prices the order against the menu. Ordered items must be active
// and valid; discounts that expose Validate are checked before any are applied.
func (c *Calculator) Calculate(menu Menu, order Order) (Breakdown, error) {
	lines, err := resolve(menu, order.Lines)
	if err != nil {
		return Breakdown{}, err
	}
	for _, d := range c.discounts {
		if v, ok := d.(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return Breakdown{}, err
			}
		}
	}
	b := Breakdown{Discounts: []AppliedDiscount{}}
	for _, l := range lines {
		b.Subtotal += l.Item.Price * Money(l.Qty)
	}
	b.Total = b.Subtotal
	for _, d := range c.discounts {
		applied, ok := d.Apply(lines, order.Member, b.Total)
		if !ok {
			continue
		}
		b.Discounts = append(b.Discounts, applied)
		b.Total -= applied.Amount
		if cl, ok := d.(Claimer); ok {
			lines = cl.Claim(lines)
		}
	}
	return b, nil
}

// resolve validates the lines against the menu and merges lines with the same
// code, keeping the order of first appearance. Zero-quantity codes are dropped.
func resolve(menu Menu, lines []Line) ([]PricedLine, error) {
	var out []PricedLine
	index := map[string]int{}
	for _, l := range lines {
		if l.Qty < 0 {
			return nil, fmt.Errorf("%w: %d for %q", ErrInvalidQuantity, l.Qty, l.Code)
		}
		item, ok := menu[l.Code]
		if !ok || !item.Active {
			return nil, fmt.Errorf("%w: %q", ErrUnknownItem, l.Code)
		}
		if err := item.Validate(); err != nil {
			return nil, err
		}
		if item.Code != l.Code {
			return nil, fmt.Errorf("%w: menu code %q does not match item code %q", ErrInvalidItem, l.Code, item.Code)
		}
		if l.Qty > MaxQuantity {
			return nil, fmt.Errorf("%w: %d for %q exceeds %d", ErrInvalidQuantity, l.Qty, l.Code, MaxQuantity)
		}
		i, seen := index[l.Code]
		if !seen {
			index[l.Code] = len(out)
			out = append(out, PricedLine{Item: item})
			i = len(out) - 1
		}
		out[i].Qty += l.Qty
		if out[i].Qty > MaxQuantity {
			return nil, fmt.Errorf("%w: %d for %q exceeds %d", ErrInvalidQuantity, out[i].Qty, l.Code, MaxQuantity)
		}
	}
	kept := out[:0]
	for _, l := range out {
		if l.Qty > 0 {
			kept = append(kept, l)
		}
	}
	return kept, nil
}
