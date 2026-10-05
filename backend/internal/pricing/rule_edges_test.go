package pricing_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gravizz/freshket/backend/internal/pricing"
)

func mustCalculate(t *testing.T, calc *pricing.Calculator, order pricing.Order) pricing.Breakdown {
	t.Helper()
	got, err := calc.Calculate(testMenu(), order)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	return got
}

func TestPairDiscountEdges(t *testing.T) {
	tests := []struct {
		name        string
		lines       []pricing.Line
		wantAmounts []pricing.Money
		wantTotal   pricing.Money
	}{
		{"single orange has no pair", []pricing.Line{{Code: "ORANGE", Qty: 1}}, nil, 12000},
		{"duplicate lines form one pair", []pricing.Line{{Code: "ORANGE", Qty: 1}, {Code: "ORANGE", Qty: 1}}, []pricing.Money{1200}, 22800},
		{"non-eligible item never pairs", []pricing.Line{{Code: "RED", Qty: 2}}, nil, 10000},
		{"mixed eligible items never pair", []pricing.Line{{Code: "ORANGE", Qty: 1}, {Code: "PINK", Qty: 1}}, nil, 20000},
		{"each eligible item gets its own discount, in rule order", []pricing.Line{{Code: "GREEN", Qty: 2}, {Code: "PINK", Qty: 3}}, []pricing.Money{800, 400}, 30800},
	}

	calc := pricing.NewCalculator(pricing.Discounts(seedRules()[:3])...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustCalculate(t, calc, pricing.Order{Lines: tt.lines})
			if len(got.Discounts) != len(tt.wantAmounts) {
				t.Fatalf("got %d discounts, want %d: %+v", len(got.Discounts), len(tt.wantAmounts), got.Discounts)
			}
			for i, want := range tt.wantAmounts {
				if got.Discounts[i].Amount != want {
					t.Errorf("discount[%d] = %d, want %d", i, got.Discounts[i].Amount, want)
				}
			}
			if got.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", got.Total, tt.wantTotal)
			}
		})
	}
}

func TestMemberDiscountEdges(t *testing.T) {
	calc := pricing.NewCalculator(pricing.Discounts(seedRules()[3:])...)

	t.Run("non-member gets no discount", func(t *testing.T) {
		got := mustCalculate(t, calc, pricing.Order{Lines: []pricing.Line{{Code: "RED", Qty: 1}}})
		if len(got.Discounts) != 0 || got.Total != 5000 {
			t.Errorf("got %+v, want no discounts and total 5000", got)
		}
	})

	t.Run("empty order for a member has no discount line", func(t *testing.T) {
		got := mustCalculate(t, calc, pricing.Order{Member: true})
		if len(got.Discounts) != 0 || got.Total != 0 {
			t.Errorf("got %+v, want no discounts and total 0", got)
		}
	})

	t.Run("fractional satang rounds half-up", func(t *testing.T) {
		menu := pricing.Menu{"X": {Code: "X", Name: "X set", Price: 1005, Active: true}}
		got, err := calc.Calculate(menu, pricing.Order{Lines: []pricing.Line{{Code: "X", Qty: 1}}, Member: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Discounts) != 1 || got.Discounts[0].Amount != 101 || got.Total != 904 {
			t.Errorf("got %+v, want discount 101 and total 904", got)
		}
	})
}

func TestCalculateRejectsInvalidRules(t *testing.T) {
	tests := []struct {
		name string
		rule pricing.Rule
	}{
		{"percent above 100", pricing.Rule{Name: "Bad percent", Percent: 200, Active: true}},
		{"zero percent", pricing.Rule{Name: "Bad percent", Active: true}},
		{"zero bundle quantity", pricing.Rule{Name: "Bad bundle", Percent: 5, Active: true, Bundle: []pricing.Component{{ItemCode: "RED", Qty: 0}}}},
		{"negative bundle quantity", pricing.Rule{Name: "Bad bundle", Percent: 5, Active: true, Bundle: []pricing.Component{{ItemCode: "RED", Qty: -1}}}},
		{"duplicate component", pricing.Rule{Name: "Bad bundle", Percent: 5, Active: true, Bundle: []pricing.Component{{ItemCode: "RED", Qty: 1}, {ItemCode: "RED", Qty: 1}}}},
		{"empty name", pricing.Rule{Percent: 5, Active: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calculators := map[string]*pricing.Calculator{
				"direct value":   pricing.NewCalculator(tt.rule),
				"direct pointer": pricing.NewCalculator(&tt.rule),
				"sorted rules":   pricing.NewCalculator(pricing.Discounts([]pricing.Rule{tt.rule})...),
			}
			for name, calc := range calculators {
				t.Run(name, func(t *testing.T) {
					_, err := calc.Calculate(testMenu(), pricing.Order{Lines: []pricing.Line{{Code: "RED", Qty: 1}}})
					if !errors.Is(err, pricing.ErrInvalidRule) {
						t.Errorf("Calculate() error = %v, want ErrInvalidRule", err)
					}
				})
			}
		})
	}
}

func TestDirectInactiveRuleDoesNotDiscountOrClaim(t *testing.T) {
	inactive := pricing.Rule{Name: "Paused bundle", Percent: 50, Bundle: []pricing.Component{{ItemCode: "RED", Qty: 1}}}
	active := inactive
	active.Name, active.Percent, active.Active = "Live bundle", 10, true
	got := mustCalculate(t, pricing.NewCalculator(inactive, active),
		pricing.Order{Lines: []pricing.Line{{Code: "RED", Qty: 1}}})
	if got.Total != 4500 || len(got.Discounts) != 1 || got.Discounts[0].Label != "Live bundle ×1 (10%)" {
		t.Errorf("got %+v, want only the live bundle and total 4500", got)
	}
}

func TestDirectRuleCallsSkipInactiveAndInvalidRules(t *testing.T) {
	lines := []pricing.PricedLine{{Item: testMenu()["RED"], Qty: 1}}
	rules := []pricing.Rule{
		{Name: "Paused order", Percent: 10},
		{Name: "Paused bundle", Percent: 10, Bundle: []pricing.Component{{ItemCode: "RED", Qty: 1}}},
		{Name: "Invalid order", Percent: 200, Active: true},
		{Name: "Invalid bundle", Percent: 10, Active: true, Bundle: []pricing.Component{{ItemCode: "RED", Qty: 0}}},
	}
	for _, rule := range rules {
		t.Run(rule.Name, func(t *testing.T) {
			if discount, ok := rule.Apply(lines, true, 5000); ok {
				t.Errorf("Apply() = %+v, true, want no discount", discount)
			}
			if remaining := rule.Claim(lines); !reflect.DeepEqual(remaining, lines) {
				t.Errorf("Claim() = %+v, want unchanged lines %+v", remaining, lines)
			}
		})
	}
}
