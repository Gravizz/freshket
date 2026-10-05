// Package httpapi exposes the menu and calculator over HTTP.
package httpapi

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/gravizz/freshket/backend/internal/menu"
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
}

type handler struct {
	menu  MenuStore
	rules RuleStore
}

// New returns a Fiber app serving /api/menu and /api/orders/calculate.
func New(menu MenuStore, rules RuleStore) *fiber.App {
	h := &handler{menu: menu, rules: rules}
	app := fiber.New()
	api := app.Group("/api")
	api.Get("/menu", h.listMenu)
	api.Post("/orders/calculate", h.calculate)
	api.Get("/admin/menu", h.listAllItems)
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

type adminItemDTO struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Price  int64  `json:"price"`
	Active bool   `json:"active"`
}

// itemError maps repository errors to HTTP statuses.
func itemError(err error) error {
	switch {
	case errors.Is(err, menu.ErrDuplicate):
		return fiber.NewError(fiber.StatusConflict, err.Error())
	case errors.Is(err, menu.ErrNotFound):
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}
	return err
}

func (h *handler) listAllItems(c fiber.Ctx) error {
	items, err := h.menu.ListAll(c.Context())
	if err != nil {
		return err
	}
	out := make([]adminItemDTO, 0, len(items))
	for _, it := range items {
		out = append(out, adminItemDTO{Code: it.Code, Name: it.Name, Price: int64(it.Price), Active: it.Active})
	}
	return c.JSON(out)
}

func (h *handler) createItem(c fiber.Ctx) error {
	var body adminItemDTO
	if err := c.Bind().JSON(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	it := pricing.Item{Code: body.Code, Name: body.Name, Price: pricing.Money(body.Price), Active: body.Active}
	if err := it.Validate(); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if err := h.menu.Create(c.Context(), it); err != nil {
		return itemError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(body)
}

func (h *handler) updateItem(c fiber.Ctx) error {
	var body adminItemDTO
	if err := c.Bind().JSON(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	body.Code = c.Params("code")
	it := pricing.Item{Code: body.Code, Name: body.Name, Price: pricing.Money(body.Price), Active: body.Active}
	if err := it.Validate(); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if err := h.menu.Update(c.Context(), it); err != nil {
		return itemError(err)
	}
	return c.JSON(body)
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
	menu := make(pricing.Menu, len(items))
	for _, it := range items {
		menu[it.Code] = it
	}

	order := pricing.Order{Member: req.Member}
	for _, it := range req.Items {
		order.Lines = append(order.Lines, pricing.Line{Code: it.Code, Qty: it.Qty})
	}

	active, err := h.rules.ListActive(c.Context())
	if err != nil {
		return err
	}

	b, err := pricing.NewCalculator(pricing.Discounts(active)...).Calculate(menu, order)
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
