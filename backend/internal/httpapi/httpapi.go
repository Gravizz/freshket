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
	ListActive(ctx context.Context) ([]pricing.Item, error)
	ListAll(ctx context.Context) ([]pricing.Item, error)
	Create(ctx context.Context, it pricing.Item) error
	Update(ctx context.Context, it pricing.Item) error
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

// RuleStore lists the discount rules that currently apply.
type RuleStore interface {
	ListActive(ctx context.Context) ([]pricing.Rule, error)
	ListAll(ctx context.Context) ([]pricing.Rule, error)
	Create(ctx context.Context, rule pricing.Rule) (pricing.Rule, error)
	Update(ctx context.Context, rule pricing.Rule) error
}

type handler struct {
	menu  MenuStore
	rules RuleStore
}

// New returns a Fiber app serving /api/menu and /api/orders/calculate.
func New(menuStore MenuStore, ruleStore RuleStore) *fiber.App {
	h := &handler{menu: menuStore, rules: ruleStore}
	app := fiber.New()
	api := app.Group("/api")
	api.Get("/menu", h.listMenu)
	api.Post("/orders/calculate", h.calculate)
	api.Get("/admin/menu", h.listAllItems)
	api.Get("/admin/rules", h.listRules)
	api.Post("/admin/rules", h.createRule)
	api.Put("/admin/rules/:id", h.updateRule)
	api.Post("/admin/menu", h.createItem)
	api.Put("/admin/menu/:code", h.updateItem)
	return app
}

func (h *handler) listMenu(c fiber.Ctx) error {
	items, err := h.menu.ListActive(c.Context())
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

	items, err := h.menu.ListActive(c.Context())
	if err != nil {
		return err
	}
	byCode := make(pricing.Menu, len(items))
	for _, it := range items {
		byCode[it.Code] = it
	}

	order := pricing.Order{Member: req.Member}
	for _, it := range req.Items {
		order.Lines = append(order.Lines, pricing.Line{Code: it.Code, Qty: it.Qty})
	}

	active, err := h.rules.ListActive(c.Context())
	if err != nil {
		return err
	}

	b, err := pricing.NewCalculator(pricing.Discounts(active)...).Calculate(byCode, order)
	if errors.Is(err, pricing.ErrUnknownItem) || errors.Is(err, pricing.ErrInvalidQuantity) ||
		errors.Is(err, pricing.ErrInvalidItem) || errors.Is(err, pricing.ErrInvalidRule) {
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
