package pricing

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Component is one item of a Bundle and how many sets of it the bundle needs.
type Component struct {
	ItemCode string
	Qty      int
}

// Rule is a configurable promotion.
//
// With a Bundle, every complete bundle in the order gets Percent off the sets
// it covers; a bundle of one component is "every N sets of one item". With an
// empty Bundle, Percent comes off the running total of the whole order. Rule
// implements Discount and Claimer.
type Rule struct {
	ID         int64
	Name       string
	Bundle     []Component
	Percent    int
	MemberOnly bool
	Active     bool
}

// Apply implements Discount, skipping inactive or invalid rules.
// Calculator returns validation errors before calling Apply.
func (r Rule) Apply(lines []PricedLine, member bool, runningTotal Money) (AppliedDiscount, bool) {
	if !r.Active || r.Validate() != nil || (r.MemberOnly && !member) {
		return AppliedDiscount{}, false
	}
	if len(r.Bundle) == 0 {
		amount := percentOf(runningTotal, int64(r.Percent))
		if amount == 0 {
			return AppliedDiscount{}, false
		}
		return AppliedDiscount{Label: r.Name, Amount: amount}, true
	}
	n := r.bundles(lines)
	if n == 0 {
		return AppliedDiscount{}, false
	}
	var covered Money
	for _, c := range r.Bundle {
		covered += priceOf(lines, c.ItemCode) * Money(c.Qty*n)
	}
	return AppliedDiscount{
		Label:  fmt.Sprintf("%s ×%d (%d%%)", r.Name, n, r.Percent),
		Amount: percentOf(covered, int64(r.Percent)),
	}, true
}

// Claim implements Claimer: it removes the sets the rule's bundles cover.
func (r Rule) Claim(lines []PricedLine) []PricedLine {
	if !r.Active || r.Validate() != nil {
		return lines
	}
	n := r.bundles(lines)
	if n == 0 {
		return lines
	}
	out := make([]PricedLine, len(lines))
	copy(out, lines)
	for _, c := range r.Bundle {
		for i := range out {
			if out[i].Item.Code == c.ItemCode {
				out[i].Qty -= c.Qty * n
			}
		}
	}
	return out
}

// bundles counts the complete bundles the lines hold: the fewest groups any
// component can fill. A whole-order rule, or a missing component, gives 0.
func (r Rule) bundles(lines []PricedLine) int {
	if len(r.Bundle) == 0 {
		return 0
	}
	n := -1
	for _, c := range r.Bundle {
		groups := qtyOf(lines, c.ItemCode) / c.Qty
		if n < 0 || groups < n {
			n = groups
		}
	}
	return n
}

// size is the number of sets in one bundle.
func (r Rule) size() int {
	total := 0
	for _, c := range r.Bundle {
		total += c.Qty
	}
	return total
}

func qtyOf(lines []PricedLine, code string) int {
	for _, l := range lines {
		if l.Item.Code == code {
			return l.Qty
		}
	}
	return 0
}

func priceOf(lines []PricedLine, code string) Money {
	for _, l := range lines {
		if l.Item.Code == code {
			return l.Item.Price
		}
	}
	return 0
}

// Discounts turns the active rules into the discounts a Calculator applies:
// bundle rules first, biggest bundle first, then whole-order rules; ties and
// whole-order rules go by ascending ID.
func Discounts(rules []Rule) []Discount {
	active := make([]Rule, 0, len(rules))
	for _, r := range rules {
		if r.Active {
			active = append(active, r)
		}
	}
	sort.SliceStable(active, func(i, j int) bool {
		a, b := active[i], active[j]
		aBundle, bBundle := len(a.Bundle) > 0, len(b.Bundle) > 0
		if aBundle != bBundle {
			return aBundle
		}
		if a.size() != b.size() {
			return a.size() > b.size()
		}
		return a.ID < b.ID
	})

	out := make([]Discount, 0, len(active))
	for _, r := range active {
		out = append(out, r)
	}
	return out
}

// percentOf returns percent% of amount, rounding half-up to a whole satang.
func percentOf(amount Money, percent int64) Money {
	return (amount*Money(percent) + 50) / 100
}

// ErrInvalidRule is returned when a discount rule breaks the rule constraints.
var ErrInvalidRule = errors.New("invalid rule")

// Validate checks the rule: a name of 1–60 characters, a whole-number percent
// of 1–100, and a bundle (possibly empty) whose components each name a valid
// item code once, with a quantity of 1..MaxQuantity.
func (r Rule) Validate() error {
	switch {
	case strings.TrimSpace(r.Name) == "" || utf8.RuneCountInString(r.Name) > 60:
		return fmt.Errorf("%w: name must be 1-60 characters", ErrInvalidRule)
	case r.Percent < 1 || r.Percent > 100:
		return fmt.Errorf("%w: percent must be between 1 and 100", ErrInvalidRule)
	}
	seen := map[string]bool{}
	for _, c := range r.Bundle {
		switch {
		case !itemCodePattern.MatchString(c.ItemCode):
			return fmt.Errorf("%w: bundle item code %q must be 1-20 characters of A-Z, 0-9 or _", ErrInvalidRule, c.ItemCode)
		case c.Qty < 1 || c.Qty > MaxQuantity:
			return fmt.Errorf("%w: bundle quantity for %q must be between 1 and %d", ErrInvalidRule, c.ItemCode, MaxQuantity)
		case seen[c.ItemCode]:
			return fmt.Errorf("%w: bundle lists %q more than once", ErrInvalidRule, c.ItemCode)
		}
		seen[c.ItemCode] = true
	}
	return nil
}

// ErrInvalidItem is returned when a menu item breaks the item rules.
var ErrInvalidItem = errors.New("invalid item")

var itemCodePattern = regexp.MustCompile(`^[A-Z0-9_]{1,20}$`)

// MaxPrice is the highest price of one set, in satang (THB 1,000,000). With
// MaxQuantity it keeps every line total far from int64 overflow.
const MaxPrice Money = 100_000_000

// Validate checks the item: code of 1–20 uppercase letters, digits or
// underscores; name of 1–60 characters; price from one satang to MaxPrice.
func (i Item) Validate() error {
	switch {
	case !itemCodePattern.MatchString(i.Code):
		return fmt.Errorf("%w: code must be 1-20 characters of A-Z, 0-9 or _", ErrInvalidItem)
	case strings.TrimSpace(i.Name) == "" || utf8.RuneCountInString(i.Name) > 60:
		return fmt.Errorf("%w: name must be 1-60 characters", ErrInvalidItem)
	case i.Price < 1 || i.Price > MaxPrice:
		return fmt.Errorf("%w: price must be between 1 and %d satang", ErrInvalidItem, MaxPrice)
	}
	return nil
}
