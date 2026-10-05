package pricing_test

import (
	"errors"
	"testing"

	"github.com/gravizz/freshket/backend/internal/pricing"
)

func testMenu() pricing.Menu {
	items := []pricing.Item{
		{Code: "RED", Name: "Red set", Price: 5000},
		{Code: "GREEN", Name: "Green set", Price: 4000},
		{Code: "BLUE", Name: "Blue set", Price: 3000},
		{Code: "YELLOW", Name: "Yellow set", Price: 5000},
		{Code: "PINK", Name: "Pink set", Price: 8000},
		{Code: "PURPLE", Name: "Purple set", Price: 9000},
		{Code: "ORANGE", Name: "Orange set", Price: 12000},
	}
	m := pricing.Menu{}
	for _, it := range items {
		it.Active = true
		m[it.Code] = it
	}
	return m
}

func TestCalculateRejectsInvalidItems(t *testing.T) {
	tests := []struct {
		name    string
		item    pricing.Item
		wantErr error
	}{
		{"inactive", pricing.Item{Code: "RED", Name: "Red set", Price: 5000}, pricing.ErrUnknownItem},
		{"negative price", pricing.Item{Code: "RED", Name: "Red set", Price: -5000, Active: true}, pricing.ErrInvalidItem},
		{"zero price", pricing.Item{Code: "RED", Name: "Red set", Active: true}, pricing.ErrInvalidItem},
		{"over-limit price", pricing.Item{Code: "RED", Name: "Red set", Price: pricing.MaxPrice + 1, Active: true}, pricing.ErrInvalidItem},
		{"empty name", pricing.Item{Code: "RED", Price: 5000, Active: true}, pricing.ErrInvalidItem},
		{"invalid code", pricing.Item{Code: "red", Name: "Red set", Price: 5000, Active: true}, pricing.ErrInvalidItem},
		{"mismatched code", pricing.Item{Code: "GREEN", Name: "Red set", Price: 5000, Active: true}, pricing.ErrInvalidItem},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pricing.NewCalculator().Calculate(pricing.Menu{"RED": tt.item},
				pricing.Order{Lines: []pricing.Line{{Code: "RED", Qty: 1}}})
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Calculate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestCalculateRejectsInvalidLines(t *testing.T) {
	tests := []struct {
		name    string
		line    pricing.Line
		wantErr error
	}{
		{"unknown code", pricing.Line{Code: "BLACK", Qty: 1}, pricing.ErrUnknownItem},
		{"negative quantity", pricing.Line{Code: "RED", Qty: -1}, pricing.ErrInvalidQuantity},
	}

	calc := pricing.NewCalculator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := calc.Calculate(testMenu(), pricing.Order{Lines: []pricing.Line{tt.line}})
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestCalculateSubtotal(t *testing.T) {
	calc := pricing.NewCalculator() // no rules: subtotal only

	got, err := calc.Calculate(testMenu(), pricing.Order{Lines: []pricing.Line{{Code: "RED", Qty: 1}, {Code: "GREEN", Qty: 1}}})
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	if got.Subtotal != 9000 || got.Total != 9000 {
		t.Errorf("Subtotal, Total = %d, %d, want 9000, 9000", got.Subtotal, got.Total)
	}
	if got.Discounts == nil || len(got.Discounts) != 0 {
		t.Errorf("Discounts = %#v, want non-nil empty slice", got.Discounts)
	}
}

func TestCalculateQuantityLimit(t *testing.T) {
	tests := []struct {
		name  string
		lines []pricing.Line
	}{
		{"single line over the limit", []pricing.Line{{Code: "RED", Qty: pricing.MaxQuantity + 1}}},
		{"duplicate lines add up over the limit", []pricing.Line{{Code: "RED", Qty: 6000}, {Code: "RED", Qty: 6000}}},
		{"absurd quantity", []pricing.Line{{Code: "RED", Qty: 9_000_000_000_000}}},
	}

	calc := pricing.NewCalculator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := calc.Calculate(testMenu(), pricing.Order{Lines: tt.lines})
			if !errors.Is(err, pricing.ErrInvalidQuantity) {
				t.Errorf("error = %v, want %v", err, pricing.ErrInvalidQuantity)
			}
		})
	}
}

func TestCalculateIgnoresZeroQuantity(t *testing.T) {
	got, err := pricing.NewCalculator().Calculate(testMenu(), pricing.Order{Lines: []pricing.Line{{Code: "RED", Qty: 0}}})
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	if got.Subtotal != 0 || got.Total != 0 {
		t.Errorf("Subtotal, Total = %d, %d, want 0, 0", got.Subtotal, got.Total)
	}
}
