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
	all, err := h.rules.ListAll(c.Context())
	if err != nil {
		return err
	}
	out := make([]ruleDTO, 0, len(all))
	for _, r := range all {
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
