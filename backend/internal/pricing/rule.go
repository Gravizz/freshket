package pricing

import (
	"fmt"
	"sort"
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
