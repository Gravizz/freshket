// Package httpapi exposes the menu and calculator over HTTP.
package httpapi

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/gravizz/freshket/backend/internal/pricing"
)

// MenuStore lists the items on the menu.
type MenuStore interface {
	List(ctx context.Context) ([]pricing.Item, error)
}

type menuItemDTO struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Price int64  `json:"price"`
}

type calculateRequest struct {
	Items []struct {
		Code string `json:"code"`
		Qty  int    `json:"qty"`
	} `json:"items"`
	Member bool `json:"member"`
}

type discountDTO struct {
	Label  string `json:"label"`
	Amount int64  `json:"amount"`
}

type calculateResponse struct {
	Subtotal  int64         `json:"subtotal"`
	Discounts []discountDTO `json:"discounts"`
	Total     int64         `json:"total"`
}

type handler struct {
	menu MenuStore
	calc *pricing.Calculator
}

// New returns a Fiber app serving /api/menu and /api/orders/calculate.
func New(menu MenuStore, calc *pricing.Calculator) *fiber.App {
	h := &handler{menu: menu, calc: calc}
	app := fiber.New()
	api := app.Group("/api")
	api.Get("/menu", h.listMenu)
	api.Post("/orders/calculate", h.calculate)
	return app
}

func (h *handler) listMenu(c fiber.Ctx) error {
	items, err := h.menu.List(c.Context())
	if err != nil {
		return err
	}
	out := make([]menuItemDTO, 0, len(items))
	for _, it := range items {
		out = append(out, menuItemDTO{Code: it.Code, Name: it.Name, Price: int64(it.Price)})
	}
	return c.JSON(out)
}

func (h *handler) calculate(c fiber.Ctx) error {
	var req calculateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}

	items, err := h.menu.List(c.Context())
	if err != nil {
		return err
	}
	menu := make(pricing.Menu, len(items))
	for _, it := range items {
		menu[it.Code] = it
	}

	order := pricing.Order{Member: req.Member}
	for _, it := range req.Items {
		order.Lines = append(order.Lines, pricing.Line{Code: it.Code, Qty: it.Qty})
	}

	b, err := h.calc.Calculate(menu, order)
	if errors.Is(err, pricing.ErrUnknownItem) || errors.Is(err, pricing.ErrInvalidQuantity) {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if err != nil {
		return err
	}

	resp := calculateResponse{
		Subtotal:  int64(b.Subtotal),
		Discounts: make([]discountDTO, 0, len(b.Discounts)),
		Total:     int64(b.Total),
	}
	for _, d := range b.Discounts {
		resp.Discounts = append(resp.Discounts, discountDTO{Label: d.Label, Amount: int64(d.Amount)})
	}
	return c.JSON(resp)
}
