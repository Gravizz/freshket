package pricing_test

import (
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
		menu := pricing.Menu{"X": {Code: "X", Name: "X set", Price: 1005}}
		got, err := calc.Calculate(menu, pricing.Order{Lines: []pricing.Line{{Code: "X", Qty: 1}}, Member: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Discounts) != 1 || got.Discounts[0].Amount != 101 || got.Total != 904 {
			t.Errorf("got %+v, want discount 101 and total 904", got)
		}
	})
}
