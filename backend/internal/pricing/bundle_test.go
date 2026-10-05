package pricing_test

import (
	"errors"
	"testing"

	"github.com/gravizz/freshket/backend/internal/pricing"
)

func bundleA() pricing.Rule {
	return pricing.Rule{
		ID: 5, Name: "Bundle A", Percent: 12, Active: true,
		Bundle: []pricing.Component{{ItemCode: "GREEN", Qty: 2}, {ItemCode: "RED", Qty: 1}},
	}
}

func calcFor(rules ...pricing.Rule) *pricing.Calculator {
	return pricing.NewCalculator(pricing.Discounts(rules)...)
}

func TestBundleDiscount(t *testing.T) {
	tests := []struct {
		name       string
		lines      []pricing.Line
		wantAmount pricing.Money // 0 means no discount line
		wantTotal  pricing.Money
	}{
		{"one complete bundle", []pricing.Line{{Code: "GREEN", Qty: 2}, {Code: "RED", Qty: 1}}, 1560, 11440},
		{"two complete bundles", []pricing.Line{{Code: "GREEN", Qty: 4}, {Code: "RED", Qty: 2}}, 3120, 22880},
		{"leftover green pays full price", []pricing.Line{{Code: "GREEN", Qty: 5}, {Code: "RED", Qty: 2}}, 3120, 26880},
		{"red limits to one bundle", []pricing.Line{{Code: "GREEN", Qty: 4}, {Code: "RED", Qty: 1}}, 1560, 19440},
		{"missing component", []pricing.Line{{Code: "GREEN", Qty: 2}}, 0, 8000},
		{"green is short", []pricing.Line{{Code: "GREEN", Qty: 1}, {Code: "RED", Qty: 1}}, 0, 9000},
	}

	calc := calcFor(bundleA())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustCalculate(t, calc, pricing.Order{Lines: tt.lines})
			if tt.wantAmount == 0 {
				if len(got.Discounts) != 0 {
					t.Errorf("discounts = %+v, want none", got.Discounts)
				}
			} else if len(got.Discounts) != 1 || got.Discounts[0].Amount != tt.wantAmount {
				t.Errorf("discounts = %+v, want one of %d", got.Discounts, tt.wantAmount)
			}
			if got.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", got.Total, tt.wantTotal)
			}
		})
	}

	t.Run("label", func(t *testing.T) {
		got := mustCalculate(t, calc, pricing.Order{Lines: []pricing.Line{{Code: "GREEN", Qty: 2}, {Code: "RED", Qty: 1}}})
		if got.Discounts[0].Label != "Bundle A ×1 (12%)" {
			t.Errorf("label = %q, want %q", got.Discounts[0].Label, "Bundle A ×1 (12%)")
		}
	})
}

func TestBundleClaimsItsSets(t *testing.T) {
	greenPairs := pricing.Rule{ID: 3, Name: "Green pairs", Percent: 5, Active: true,
		Bundle: []pricing.Component{{ItemCode: "GREEN", Qty: 2}}}
	calc := calcFor(bundleA(), greenPairs)

	t.Run("bundle takes the greens, pair rule has none left", func(t *testing.T) {
		got := mustCalculate(t, calc, pricing.Order{Lines: []pricing.Line{{Code: "GREEN", Qty: 2}, {Code: "RED", Qty: 1}}})
		if len(got.Discounts) != 1 || got.Discounts[0].Label != "Bundle A ×1 (12%)" || got.Total != 11440 {
			t.Errorf("got %+v, want only Bundle A and total 11440", got)
		}
	})

	t.Run("unclaimed greens still pair up", func(t *testing.T) {
		got := mustCalculate(t, calc, pricing.Order{Lines: []pricing.Line{{Code: "GREEN", Qty: 4}, {Code: "RED", Qty: 1}}})
		want := []pricing.AppliedDiscount{
			{Label: "Bundle A ×1 (12%)", Amount: 1560},
			{Label: "Green pairs ×1 (5%)", Amount: 400},
		}
		if len(got.Discounts) != 2 || got.Discounts[0] != want[0] || got.Discounts[1] != want[1] || got.Total != 19040 {
			t.Errorf("got %+v, want discounts %+v and total 19040", got, want)
		}
	})
}

func TestLargerBundlesClaimFirstThenLowerID(t *testing.T) {
	order := pricing.Order{Lines: []pricing.Line{{Code: "RED", Qty: 1}, {Code: "GREEN", Qty: 1}, {Code: "BLUE", Qty: 1}}}
	pair := func(id int64, name, other string) pricing.Rule {
		return pricing.Rule{ID: id, Name: name, Percent: 10, Active: true,
			Bundle: []pricing.Component{{ItemCode: "RED", Qty: 1}, {ItemCode: other, Qty: 1}}}
	}

	t.Run("a bigger bundle beats a lower ID", func(t *testing.T) {
		triple := pricing.Rule{ID: 9, Name: "triple", Percent: 10, Active: true,
			Bundle: []pricing.Component{{ItemCode: "RED", Qty: 1}, {ItemCode: "GREEN", Qty: 1}, {ItemCode: "BLUE", Qty: 1}}}
		got := mustCalculate(t, calcFor(pair(2, "red+green", "GREEN"), triple), order)
		if len(got.Discounts) != 1 || got.Discounts[0].Amount != 1200 || got.Total != 10800 {
			t.Errorf("got %+v, want only the triple bundle: 1200 off, total 10800", got)
		}
	})

	t.Run("equal sizes go by ascending ID", func(t *testing.T) {
		got := mustCalculate(t, calcFor(pair(5, "red+blue", "BLUE"), pair(2, "red+green", "GREEN")), order)
		if len(got.Discounts) != 1 || got.Discounts[0].Label != "red+green ×1 (10%)" || got.Discounts[0].Amount != 900 || got.Total != 11100 {
			t.Errorf("got %+v, want only red+green: 900 off, total 11100", got)
		}
	})
}

func TestBundleRulesApplyBeforeWholeOrderRules(t *testing.T) {
	member := pricing.Rule{ID: 1, Name: "Member 10%", Percent: 10, MemberOnly: true, Active: true}

	got := mustCalculate(t, calcFor(member, bundleA()),
		pricing.Order{Lines: []pricing.Line{{Code: "GREEN", Qty: 2}, {Code: "RED", Qty: 1}}, Member: true})

	want := []pricing.AppliedDiscount{
		{Label: "Bundle A ×1 (12%)", Amount: 1560},
		{Label: "Member 10%", Amount: 1144},
	}
	if len(got.Discounts) != 2 || got.Discounts[0] != want[0] || got.Discounts[1] != want[1] || got.Total != 10296 {
		t.Errorf("got %+v, want discounts %+v and total 10296", got, want)
	}
}

func TestMemberOnlyBundle(t *testing.T) {
	rule := bundleA()
	rule.MemberOnly = true
	calc := calcFor(rule)
	lines := []pricing.Line{{Code: "GREEN", Qty: 2}, {Code: "RED", Qty: 1}}

	if got := mustCalculate(t, calc, pricing.Order{Lines: lines}); len(got.Discounts) != 0 {
		t.Errorf("guest discounts = %+v, want none", got.Discounts)
	}
	if got := mustCalculate(t, calc, pricing.Order{Lines: lines, Member: true}); len(got.Discounts) != 1 {
		t.Errorf("member discounts = %+v, want one", got.Discounts)
	}
}

func TestBundleRoundsHalfUp(t *testing.T) {
	menu := pricing.Menu{"X": {Code: "X", Name: "X set", Price: 1010, Active: true}}
	rule := pricing.Rule{ID: 1, Name: "x", Percent: 5, Active: true, Bundle: []pricing.Component{{ItemCode: "X", Qty: 1}}}

	got, err := calcFor(rule).Calculate(menu, pricing.Order{Lines: []pricing.Line{{Code: "X", Qty: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Discounts) != 1 || got.Discounts[0].Amount != 51 {
		t.Errorf("discounts = %+v, want one of 51 (50.5 rounds up)", got.Discounts)
	}
}

func TestRuleValidateBundle(t *testing.T) {
	rule := func(percent int, name string, bundle ...pricing.Component) pricing.Rule {
		return pricing.Rule{Name: name, Percent: percent, Bundle: bundle}
	}
	c := func(code string, qty int) pricing.Component { return pricing.Component{ItemCode: code, Qty: qty} }

	tests := []struct {
		name    string
		rule    pricing.Rule
		invalid bool
	}{
		{"two-item bundle", rule(12, "A", c("GREEN", 2), c("RED", 1)), false},
		{"empty bundle is a whole-order rule", rule(10, "Member"), false},
		{"empty name", rule(12, "", c("RED", 1)), true},
		{"percent zero", rule(0, "A", c("RED", 1)), true},
		{"percent over 100", rule(101, "A", c("RED", 1)), true},
		{"component qty zero", rule(12, "A", c("RED", 0)), true},
		{"component qty over the limit", rule(12, "A", c("RED", pricing.MaxQuantity+1)), true},
		{"lowercase item code", rule(12, "A", c("red", 1)), true},
		{"empty item code", rule(12, "A", c("", 1)), true},
		{"same item twice", rule(12, "A", c("RED", 1), c("RED", 1)), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if tt.invalid && !errors.Is(err, pricing.ErrInvalidRule) {
				t.Errorf("Validate() = %v, want ErrInvalidRule", err)
			}
			if !tt.invalid && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}
