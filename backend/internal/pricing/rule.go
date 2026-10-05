package pricing

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Rule is a configurable promotion.
//
// With ItemCode set, every complete group of GroupSize sets of that item gets
// Percent off those sets. With ItemCode empty, Percent comes off the running
// total of the whole order. Rule implements Discount.
type Rule struct {
	ID         int64
	Name       string
	ItemCode   string
	GroupSize  int
	Percent    int
	MemberOnly bool
	Active     bool
}

// Apply implements Discount.
func (r Rule) Apply(lines []PricedLine, member bool, runningTotal Money) (AppliedDiscount, bool) {
	if r.MemberOnly && !member {
		return AppliedDiscount{}, false
	}
	if r.ItemCode == "" {
		amount := percentOf(runningTotal, int64(r.Percent))
		if amount == 0 {
			return AppliedDiscount{}, false
		}
		return AppliedDiscount{Label: r.Name, Amount: amount}, true
	}
	for _, l := range lines {
		if l.Item.Code != r.ItemCode {
			continue
		}
		groups := l.Qty / r.GroupSize
		if groups == 0 {
			return AppliedDiscount{}, false
		}
		return AppliedDiscount{
			Label:  fmt.Sprintf("%s %s ×%d (%d%%)", l.Item.Name, r.Name, groups, r.Percent),
			Amount: percentOf(l.Item.Price*Money(groups*r.GroupSize), int64(r.Percent)),
		}, true
	}
	return AppliedDiscount{}, false
}

// Discounts turns the active rules into the discounts a Calculator applies:
// item rules first, then whole-order rules, each group by ascending ID.
func Discounts(rules []Rule) []Discount {
	active := make([]Rule, 0, len(rules))
	for _, r := range rules {
		if r.Active {
			active = append(active, r)
		}
	}
	sort.SliceStable(active, func(i, j int) bool {
		iItem, jItem := active[i].ItemCode != "", active[j].ItemCode != ""
		if iItem != jItem {
			return iItem
		}
		return active[i].ID < active[j].ID
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
// of 1–100, and a group size that matches its kind (1..MaxQuantity for an
// item rule, 0 for a whole-order rule).
func (r Rule) Validate() error {
	switch {
	case strings.TrimSpace(r.Name) == "" || utf8.RuneCountInString(r.Name) > 60:
		return fmt.Errorf("%w: name must be 1-60 characters", ErrInvalidRule)
	case r.Percent < 1 || r.Percent > 100:
		return fmt.Errorf("%w: percent must be between 1 and 100", ErrInvalidRule)
	case r.ItemCode != "" && (r.GroupSize < 1 || r.GroupSize > MaxQuantity):
		return fmt.Errorf("%w: group size must be between 1 and %d", ErrInvalidRule, MaxQuantity)
	case r.ItemCode == "" && r.GroupSize != 0:
		return fmt.Errorf("%w: a rule without an item must have group size 0", ErrInvalidRule)
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
