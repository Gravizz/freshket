package pricing_test

import (
	"testing"

	"github.com/gravizz/freshket/backend/internal/pricing"
)

// seedRules is the store's promotions expressed as data: same-item pairs of
// Orange, Pink and Green at 5%, then 10% for members.
func seedRules() []pricing.Rule {
	pair := func(id int64, code string) pricing.Rule {
		return pricing.Rule{ID: id, Name: "pairs", ItemCode: code, GroupSize: 2, Percent: 5, Active: true}
	}
	return []pricing.Rule{
		pair(1, "ORANGE"), pair(2, "PINK"), pair(3, "GREEN"),
		{ID: 4, Name: "Member 10%", Percent: 10, MemberOnly: true, Active: true},
	}
}

func TestRulesReproduceTheStoreBehavior(t *testing.T) {
	tests := []struct {
		name   string
		lines  []pricing.Line
		member bool
		want   pricing.Money
	}{
		{"red and green", []pricing.Line{{Code: "RED", Qty: 1}, {Code: "GREEN", Qty: 1}}, false, 9000},
		{"red and green, member", []pricing.Line{{Code: "RED", Qty: 1}, {Code: "GREEN", Qty: 1}}, true, 8100},
		{"five orange", []pricing.Line{{Code: "ORANGE", Qty: 5}}, false, 57600},
		{"five orange, member", []pricing.Line{{Code: "ORANGE", Qty: 5}}, true, 51840},
		{"green pair and pink pair plus one", []pricing.Line{{Code: "GREEN", Qty: 2}, {Code: "PINK", Qty: 3}}, false, 30800},
		{"empty order, member", nil, true, 0},
	}

	calc := pricing.NewCalculator(pricing.Discounts(seedRules())...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustCalculate(t, calc, pricing.Order{Lines: tt.lines, Member: tt.member})
			if got.Total != tt.want {
				t.Errorf("Total = %d, want %d", got.Total, tt.want)
			}
		})
	}

	t.Run("labels and amounts: pair first, member second", func(t *testing.T) {
		got := mustCalculate(t, calc, pricing.Order{Lines: []pricing.Line{{Code: "ORANGE", Qty: 5}}, Member: true})
		want := []pricing.AppliedDiscount{
			{Label: "Orange set pairs ×2 (5%)", Amount: 2400},
			{Label: "Member 10%", Amount: 5760},
		}
		if len(got.Discounts) != len(want) {
			t.Fatalf("discounts = %+v, want %+v", got.Discounts, want)
		}
		for i := range want {
			if got.Discounts[i] != want[i] {
				t.Errorf("discount[%d] = %+v, want %+v", i, got.Discounts[i], want[i])
			}
		}
	})
}

func TestFullDiscountLeavesNothingForMember(t *testing.T) {
	rules := []pricing.Rule{
		{ID: 1, Name: "free red", ItemCode: "RED", GroupSize: 1, Percent: 100, Active: true},
		{ID: 2, Name: "Member 10%", Percent: 10, MemberOnly: true, Active: true},
	}

	got := mustCalculate(t, pricing.NewCalculator(pricing.Discounts(rules)...),
		pricing.Order{Lines: []pricing.Line{{Code: "RED", Qty: 1}}, Member: true})

	if got.Total != 0 || len(got.Discounts) != 1 {
		t.Errorf("got total %d with discounts %+v, want total 0 and only the 100%% discount", got.Total, got.Discounts)
	}
}

func TestMemberRuleSkipsEmptyOrder(t *testing.T) {
	got := mustCalculate(t, pricing.NewCalculator(pricing.Discounts(seedRules())...), pricing.Order{Member: true})

	if len(got.Discounts) != 0 {
		t.Errorf("discounts = %+v, want none", got.Discounts)
	}
}

func TestItemRuleCanRequireMembership(t *testing.T) {
	rules := []pricing.Rule{{ID: 1, Name: "members pairs", ItemCode: "ORANGE", GroupSize: 2, Percent: 5, MemberOnly: true, Active: true}}
	calc := pricing.NewCalculator(pricing.Discounts(rules)...)
	lines := []pricing.Line{{Code: "ORANGE", Qty: 2}}

	guest := mustCalculate(t, calc, pricing.Order{Lines: lines})
	if len(guest.Discounts) != 0 || guest.Total != 24000 {
		t.Errorf("non-member got %+v, want no discount and total 24000", guest)
	}
	member := mustCalculate(t, calc, pricing.Order{Lines: lines, Member: true})
	if len(member.Discounts) != 1 || member.Total != 22800 {
		t.Errorf("member got %+v, want one discount and total 22800", member)
	}
}

func TestDiscountsApplyItemRulesBeforeOrderRulesAndSkipInactive(t *testing.T) {
	rules := []pricing.Rule{
		{ID: 1, Name: "Member 10%", Percent: 10, MemberOnly: true, Active: true},
		{ID: 3, Name: "pairs", ItemCode: "ORANGE", GroupSize: 2, Percent: 5, Active: true},
		{ID: 2, Name: "pairs", ItemCode: "PINK", GroupSize: 2, Percent: 5, Active: false},
	}

	got := mustCalculate(t, pricing.NewCalculator(pricing.Discounts(rules)...), pricing.Order{
		Lines:  []pricing.Line{{Code: "ORANGE", Qty: 2}, {Code: "PINK", Qty: 2}},
		Member: true,
	})

	want := []pricing.AppliedDiscount{
		{Label: "Orange set pairs ×1 (5%)", Amount: 1200},
		{Label: "Member 10%", Amount: 3880},
	}
	if len(got.Discounts) != len(want) || got.Discounts[0] != want[0] || got.Discounts[1] != want[1] {
		t.Errorf("discounts = %+v, want %+v", got.Discounts, want)
	}
}

func TestDiscountsOrderRulesByID(t *testing.T) {
	rules := []pricing.Rule{
		{ID: 9, Name: "late", ItemCode: "RED", GroupSize: 1, Percent: 10, Active: true},
		{ID: 2, Name: "early", ItemCode: "RED", GroupSize: 1, Percent: 20, Active: true},
	}
	// Both rules look at the same sets; the lower ID is listed first.
	got := mustCalculate(t, pricing.NewCalculator(pricing.Discounts(rules)...), pricing.Order{Lines: []pricing.Line{{Code: "RED", Qty: 1}}})

	if len(got.Discounts) != 2 || got.Discounts[0].Label != "Red set early ×1 (20%)" {
		t.Errorf("discounts = %+v, want the lower-ID rule first", got.Discounts)
	}
}

func TestRuleGroupSize(t *testing.T) {
	rules := []pricing.Rule{{ID: 1, Name: "triple", ItemCode: "ORANGE", GroupSize: 3, Percent: 10, Active: true}}
	calc := pricing.NewCalculator(pricing.Discounts(rules)...)

	seven := mustCalculate(t, calc, pricing.Order{Lines: []pricing.Line{{Code: "ORANGE", Qty: 7}}})
	if len(seven.Discounts) != 1 || seven.Discounts[0].Amount != 7200 || seven.Discounts[0].Label != "Orange set triple ×2 (10%)" {
		t.Errorf("seven oranges: discounts = %+v, want one of 7200 labelled triple ×2 (10%%)", seven.Discounts)
	}
	two := mustCalculate(t, calc, pricing.Order{Lines: []pricing.Line{{Code: "ORANGE", Qty: 2}}})
	if len(two.Discounts) != 0 {
		t.Errorf("two oranges: discounts = %+v, want none", two.Discounts)
	}
}
