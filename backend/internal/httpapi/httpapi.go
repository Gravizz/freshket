// Package httpapi exposes the menu and calculator over HTTP.
package httpapi

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/gravizz/freshket/backend/internal/menu"
	"github.com/gravizz/freshket/backend/internal/pricing"
	"github.com/gravizz/freshket/backend/internal/rules"
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
func New(menu MenuStore, rules RuleStore) *fiber.App {
	h := &handler{menu: menu, rules: rules}
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

type adminItemDTO struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Price  int64  `json:"price"`
	Active bool   `json:"active"`
}

type ruleDTO struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	ItemCode   string `json:"itemCode"`
	GroupSize  int    `json:"groupSize"`
	Percent    int    `json:"percent"`
	MemberOnly bool   `json:"memberOnly"`
	Active     bool   `json:"active"`
}

func (d ruleDTO) rule() pricing.Rule {
	return pricing.Rule{ID: d.ID, Name: d.Name, ItemCode: d.ItemCode, GroupSize: d.GroupSize,
		Percent: d.Percent, MemberOnly: d.MemberOnly, Active: d.Active}
}

func (h *handler) listRules(c fiber.Ctx) error {
	rules, err := h.rules.ListAll(c.Context())
	if err != nil {
		return err
	}
	out := make([]ruleDTO, 0, len(rules))
	for _, r := range rules {
		out = append(out, ruleDTO{ID: r.ID, Name: r.Name, ItemCode: r.ItemCode, GroupSize: r.GroupSize,
			Percent: r.Percent, MemberOnly: r.MemberOnly, Active: r.Active})
	}
	return c.JSON(out)
}

func (h *handler) updateRule(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid rule id")
	}
	var body ruleDTO
	if err := c.Bind().JSON(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	body.ID = id
	if err := h.checkRule(c.Context(), body.rule()); err != nil {
		return err
	}
	if err := h.rules.Update(c.Context(), body.rule()); err != nil {
		if errors.Is(err, rules.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, err.Error())
		}
		return err
	}
	return c.JSON(body)
}

// checkRule validates the rule and that its item, if any, is on the menu.
func (h *handler) checkRule(ctx context.Context, r pricing.Rule) error {
	if err := r.Validate(); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if r.ItemCode == "" {
		return nil
	}
	items, err := h.menu.ListAll(ctx)
	if err != nil {
		return err
	}
	for _, it := range items {
		if it.Code == r.ItemCode {
			return nil
		}
	}
	return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("%v: unknown item %q", pricing.ErrInvalidRule, r.ItemCode))
}

func (h *handler) createRule(c fiber.Ctx) error {
	var body ruleDTO
	if err := c.Bind().JSON(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	if err := h.checkRule(c.Context(), body.rule()); err != nil {
		return err
	}
	created, err := h.rules.Create(c.Context(), body.rule())
	if err != nil {
		return err
	}
	body.ID = created.ID
	return c.Status(fiber.StatusCreated).JSON(body)
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
