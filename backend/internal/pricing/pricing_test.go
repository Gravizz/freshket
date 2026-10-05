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
		m[it.Code] = it
	}
	return m
}

func TestCalculateTotals(t *testing.T) {
	tests := []struct {
		name   string
		lines  []pricing.Line
		member bool
		want   pricing.Money
	}{
		{"red and green", []pricing.Line{{"RED", 1}, {"GREEN", 1}}, false, 9000},
		{"red and green, member", []pricing.Line{{"RED", 1}, {"GREEN", 1}}, true, 8100},
		{"five orange: two pairs discounted", []pricing.Line{{"ORANGE", 5}}, false, 57600},
		{"five orange, member", []pricing.Line{{"ORANGE", 5}}, true, 51840},
		{"green pair and pink pair plus one", []pricing.Line{{"GREEN", 2}, {"PINK", 3}}, false, 30800},
		{"empty order", nil, false, 0},
	}

	calc := pricing.NewCalculator(pricing.DefaultDiscounts()...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calc.Calculate(testMenu(), pricing.Order{Lines: tt.lines, Member: tt.member})
			if err != nil {
				t.Fatalf("Calculate() error = %v", err)
			}
			if got.Total != tt.want {
				t.Errorf("Total = %d, want %d", got.Total, tt.want)
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

	calc := pricing.NewCalculator(pricing.DefaultDiscounts()...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := calc.Calculate(testMenu(), pricing.Order{Lines: []pricing.Line{tt.line}})
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
