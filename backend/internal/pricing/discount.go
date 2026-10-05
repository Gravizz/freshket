package pricing

import "fmt"

// PairDiscount gives 5% off every pair of the same item, identified by Code.
// An odd leftover set pays full price.
type PairDiscount struct {
	Code string
}

// Apply implements Discount.
func (d PairDiscount) Apply(lines []PricedLine, _ bool, _ Money) (AppliedDiscount, bool) {
	for _, l := range lines {
		if l.Item.Code != d.Code {
			continue
		}
		pairs := l.Qty / 2
		if pairs == 0 {
			return AppliedDiscount{}, false
		}
		return AppliedDiscount{
			Label:  fmt.Sprintf("%s pairs ×%d (5%%)", l.Item.Name, pairs),
			Amount: percentOf(l.Item.Price*Money(pairs*2), 5),
		}, true
	}
	return AppliedDiscount{}, false
}

// percentOf returns percent% of amount, rounding half-up to a whole satang.
func percentOf(amount Money, percent int64) Money {
	return (amount*Money(percent) + 50) / 100
}

// MemberDiscount gives a member 10% off the running total.
type MemberDiscount struct{}

// Apply implements Discount.
func (MemberDiscount) Apply(_ []PricedLine, member bool, runningTotal Money) (AppliedDiscount, bool) {
	if !member || runningTotal <= 0 {
		return AppliedDiscount{}, false
	}
	return AppliedDiscount{Label: "Member 10%", Amount: percentOf(runningTotal, 10)}, true
}

// DefaultDiscounts returns the store's promotions in the order they apply:
// 5% off each same-item pair of Orange, Pink or Green, then 10% member discount.
func DefaultDiscounts() []Discount {
	return []Discount{
		PairDiscount{Code: "ORANGE"},
		PairDiscount{Code: "PINK"},
		PairDiscount{Code: "GREEN"},
		MemberDiscount{},
	}
}
