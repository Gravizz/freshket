package httpapi

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/gravizz/freshket/backend/internal/pricing"
	"github.com/gravizz/freshket/backend/internal/rules"
)

type bundleItemDTO struct {
	ItemCode string `json:"itemCode"`
	Qty      int    `json:"qty"`
}

// ruleDTO is a rule on the wire. An empty bundle is a whole-order rule.
type ruleDTO struct {
	ID         int64           `json:"id"`
	Name       string          `json:"name"`
	Bundle     []bundleItemDTO `json:"bundle"`
	Percent    int             `json:"percent"`
	MemberOnly bool            `json:"memberOnly"`
	Active     bool            `json:"active"`
}

func newRuleDTO(r pricing.Rule) ruleDTO {
	d := ruleDTO{ID: r.ID, Name: r.Name, Bundle: make([]bundleItemDTO, 0, len(r.Bundle)),
		Percent: r.Percent, MemberOnly: r.MemberOnly, Active: r.Active}
	for _, c := range r.Bundle {
		d.Bundle = append(d.Bundle, bundleItemDTO{ItemCode: c.ItemCode, Qty: c.Qty})
	}
	return d
}

func (d ruleDTO) rule() pricing.Rule {
	r := pricing.Rule{ID: d.ID, Name: d.Name, Percent: d.Percent, MemberOnly: d.MemberOnly, Active: d.Active}
	for _, c := range d.Bundle {
		r.Bundle = append(r.Bundle, pricing.Component{ItemCode: c.ItemCode, Qty: c.Qty})
	}
	return r
}

func (h *handler) listRules(c fiber.Ctx) error {
	all, err := h.rules.ListAll(c.Context())
	if err != nil {
		return err
	}
	out := make([]ruleDTO, 0, len(all))
	for _, r := range all {
		out = append(out, newRuleDTO(r))
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

// checkRule validates the rule and that every item of its bundle is on the menu.
func (h *handler) checkRule(ctx context.Context, r pricing.Rule) error {
	if err := r.Validate(); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if len(r.Bundle) == 0 {
		return nil
	}
	items, err := h.menu.ListAll(ctx)
	if err != nil {
		return err
	}
	known := make(map[string]bool, len(items))
	for _, it := range items {
		known[it.Code] = true
	}
	for _, c := range r.Bundle {
		if !known[c.ItemCode] {
			return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("%v: unknown item %q", pricing.ErrInvalidRule, c.ItemCode))
		}
	}
	return nil
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
	return c.Status(fiber.StatusCreated).JSON(newRuleDTO(created))
}
